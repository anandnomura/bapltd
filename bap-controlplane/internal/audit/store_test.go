package audit

import (
	"testing"
)

func TestAuditStore_IngestAndVerify(t *testing.T) {
	store := NewStore()

	events := []Event{
		{
			EventID:     "ev-01",
			Timestamp:   "2026-09-12T10:00:00Z",
			Source:      "claude-code",
			Executable:  "git",
			FullCommand: "git status",
			Decision:    "allow",
			ExitCode:    0,
		},
		{
			EventID:     "ev-02",
			Timestamp:   "2026-09-12T10:00:05Z",
			Source:      "copilot",
			Executable:  "cat",
			FullCommand: "cat .env",
			Decision:    "deny",
			Reason:      "policy1",
			ExitCode:    1,
		},
	}

	ingested, err := store.Ingest(events)
	if err != nil || ingested != 2 {
		t.Fatalf("expected 2 ingested, got %d, err: %v", ingested, err)
	}

	// Ingesting duplicate should be skipped
	ingestedDup, _ := store.Ingest([]Event{events[0]})
	if ingestedDup != 0 {
		t.Fatalf("expected 0 ingested for duplicate, got %d", ingestedDup)
	}

	// Verify chain integrity
	valid, err := store.VerifyChain()
	if err != nil || !valid {
		t.Fatalf("chain verification failed: %v", err)
	}

	list := store.List(10)
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].PreviousHash != "genesis-bapltd-control-plane" {
		t.Errorf("expected genesis previous hash on first event")
	}
	if list[1].PreviousHash != list[0].EventHash {
		t.Errorf("expected second event to chain to first event hash")
	}
}
