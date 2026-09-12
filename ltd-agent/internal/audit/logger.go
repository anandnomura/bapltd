package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEntry represents a structured log record for an execution request.
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"`
	ClientPID   int       `json:"client_pid"`
	Executable  string    `json:"executable"`
	Arguments   string    `json:"arguments,omitempty"`
	FullCommand string    `json:"full_command"`
	Decision    string    `json:"decision"` // "allow" or "deny"
	Reason      string    `json:"reason,omitempty"`
	DurationMs  int64     `json:"duration_ms"`
	ExitCode    int       `json:"exit_code"`
	Error       string    `json:"error,omitempty"`
}

var mu sync.Mutex

// DefaultLogPath returns the default audit log file location.
// It checks LTD_AUDIT_LOG environment variable first, defaulting to "ltd-audit.jsonl".
func DefaultLogPath() string {
	if envPath := os.Getenv("LTD_AUDIT_LOG"); envPath != "" {
		return envPath
	}
	return "ltd-audit.jsonl"
}

// Log records an AuditEntry to the specified log file in JSON Lines format.
// If logPath is "off" or "none", logging is skipped.
func Log(entry AuditEntry, logPath string) error {
	if logPath == "off" || logPath == "none" {
		return nil
	}
	if logPath == "" {
		logPath = DefaultLogPath()
	}

	mu.Lock()
	defer mu.Unlock()

	// Ensure parent directory exists
	dir := filepath.Dir(logPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

