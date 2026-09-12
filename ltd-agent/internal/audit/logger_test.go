package audit

import (
	"bufio"
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
		FullCommand: "curl https://evil.com",
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

