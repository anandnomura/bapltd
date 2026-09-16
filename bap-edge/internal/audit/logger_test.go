package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditLogWritesValidJSONLines(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.jsonl")

	entry1 := AuditEntry{
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		ClientPID:   1234,
		Executable:  "git",
		Arguments:   "status",
		FullCommand: "git status",
		Decision:    "allow",
		DurationMs:  45,
		ExitCode:    0,
	}

	entry2 := AuditEntry{
		Timestamp:   time.Now().UTC(),
		Source:      "copilot",
		ClientPID:   5678,
		Executable:  "curl",
		FullCommand: "curl https://untrusted-test.internal",
		Decision:    "deny",
		Reason:      "policy1 forbid egress",
		DurationMs:  10,
		ExitCode:    1,
		Error:       "blocked by cedar",
	}

	if err := Log(entry1, logFile); err != nil {
		t.Fatalf("Log entry 1 failed: %v", err)
	}
	if err := Log(entry2, logFile); err != nil {
		t.Fatalf("Log entry 2 failed: %v", err)
	}

	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []AuditEntry
	for scanner.Scan() {
		var e AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("Failed to unmarshal audit line: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}
	if entries[0].Source != "claude-code" || entries[0].Decision != "allow" {
		t.Errorf("Entry 1 mismatch: %+v", entries[0])
	}
	if entries[1].Source != "copilot" || entries[1].Decision != "deny" {
		t.Errorf("Entry 2 mismatch: %+v", entries[1])
	}
}

func TestAuditLogDisabled(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.jsonl")

	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		Decision:  "allow",
	}

	if err := Log(entry, "off"); err != nil {
		t.Fatalf("Log with 'off' failed: %v", err)
	}
	if _, err := os.Stat(logFile); !os.IsNotExist(err) {
		t.Fatalf("Expected log file not to exist when logging is 'off'")
	}
}

func TestTamperEvidentHashAndVerification(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit_tamper.jsonl")

	entry1 := AuditEntry{
		EventID:     "ev-1",
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		Executable:  "git",
		FullCommand: "git status",
		Decision:    "allow",
		ExitCode:    0,
	}

	entry2 := AuditEntry{
		EventID:     "ev-2",
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		Executable:  "cat",
		FullCommand: "cat .env",
		Decision:    "deny",
		ExitCode:    1,
	}

	if err := Log(&entry1, logFile); err != nil {
		t.Fatalf("Log entry 1 failed: %v", err)
	}
	if err := Log(&entry2, logFile); err != nil {
		t.Fatalf("Log entry 2 failed: %v", err)
	}

	// Verify intact log
	valid, count, err := VerifyLocalLog(logFile)
	if err != nil || !valid || count != 2 {
		t.Fatalf("Expected log to be valid with 2 entries, got valid=%v, count=%d, err=%v", valid, count, err)
	}
}

func TestTamperDetectionOnModifiedEntry(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit_tamper_attack.jsonl")

	entry := AuditEntry{
		EventID:     "ev-attack-1",
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		Executable:  "cat",
		FullCommand: "cat .env",
		Decision:    "deny",
		ExitCode:    1,
	}

	if err := Log(&entry, logFile); err != nil {
		t.Fatalf("Log entry failed: %v", err)
	}

	// Read content and tamper with decision (change "deny" to "allow")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	tamperedContent := []byte(string(content))
	tamperedContent = []byte(bytes.Replace(tamperedContent, []byte(`"decision":"deny"`), []byte(`"decision":"allow"`), 1))
	if err := os.WriteFile(logFile, tamperedContent, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verification must detect the tampering!
	valid, _, err := VerifyLocalLog(logFile)
	if valid || err == nil {
		t.Fatalf("Expected tamper detection to fail verification, but got valid=%v, err=%v", valid, err)
	}
}

func TestRemoveEntryPrunesBurden(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit_prune.jsonl")

	entry1 := AuditEntry{
		EventID:     "ev-prune-1",
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		FullCommand: "ls",
		Decision:    "allow",
	}
	entry2 := AuditEntry{
		EventID:     "ev-prune-2",
		Timestamp:   time.Now().UTC(),
		Source:      "claude-code",
		FullCommand: "git branch",
		Decision:    "allow",
	}

	_ = Log(&entry1, logFile)
	_ = Log(&entry2, logFile)

	// Remove entry1
	if err := RemoveEntry("ev-prune-1", logFile); err != nil {
		t.Fatalf("RemoveEntry failed: %v", err)
	}

	// Verify only entry2 remains
	content, _ := os.ReadFile(logFile)
	if bytes.Contains(content, []byte("ev-prune-1")) {
		t.Fatalf("Expected ev-prune-1 to be removed from log")
	}
	if !bytes.Contains(content, []byte("ev-prune-2")) {
		t.Fatalf("Expected ev-prune-2 to remain in log")
	}

	// Remove entry2 (all entries gone -> file must truncate to 0 bytes)
	if err := RemoveEntry("ev-prune-2", logFile); err != nil {
		t.Fatalf("RemoveEntry failed: %v", err)
	}

	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("Expected file size to be 0 bytes after pruning all entries, got %d", info.Size())
	}
}
