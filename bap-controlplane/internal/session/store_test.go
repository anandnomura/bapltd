package session

import (
	"testing"
	"time"

	"bap-controlplane/internal/audit"
)

func TestSessionStore_Lifecycle(t *testing.T) {
	store := NewStore()

	// 1. Start a session
	sess, err := store.Start(SessionStartRequest{
		SessionID: "sess-claude-test1",
		AppID:     "claude-code",
		ClientPID: 1234,
		Hostname:  "dev-box",
	})
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}
	if sess.Status != "active" {
		t.Fatalf("expected active status, got %s", sess.Status)
	}

	// 2. Record events
	store.RecordEvent("sess-claude-test1", audit.Event{
		EventID:     "ev-1",
		Executable:  "git",
		FullCommand: "git status",
		Decision:    "allow",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	})
	store.RecordEvent("sess-claude-test1", audit.Event{
		EventID:     "ev-2",
		Executable:  "cat",
		FullCommand: "cat .env",
		Decision:    "deny",
		Reason:      "Blocked credential file",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	})

	// 3. Inspect session
	detail, err := store.Get("sess-claude-test1")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if detail.TotalEvents != 2 {
		t.Errorf("expected 2 total events, got %d", detail.TotalEvents)
	}
	if detail.AllowedCount != 1 {
		t.Errorf("expected 1 allowed, got %d", detail.AllowedCount)
	}
	if detail.DeniedCount != 1 {
		t.Errorf("expected 1 denied, got %d", detail.DeniedCount)
	}
	if len(detail.Events) != 2 {
		t.Errorf("expected 2 events in detail, got %d", len(detail.Events))
	}

	// 4. Auto-instantiate on event with unknown session ID
	store.RecordEvent("sess-copilot-auto", audit.Event{
		EventID:     "ev-3",
		Source:      "copilot",
		Executable:  "ls",
		FullCommand: "ls",
		Decision:    "allow",
	})
	autoSess, err := store.Get("sess-copilot-auto")
	if err != nil {
		t.Fatalf("auto session not created: %v", err)
	}
	if autoSess.AppID != "copilot" {
		t.Errorf("expected auto session app_id copilot, got %s", autoSess.AppID)
	}

	// 5. List sessions
	list := store.List(10)
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions listed, got %d", len(list))
	}
	// Latest first
	if list[0].SessionID != "sess-copilot-auto" {
		t.Errorf("expected latest session first, got %s", list[0].SessionID)
	}

	// 6. End session
	if err := store.End("sess-claude-test1", "normal exit"); err != nil {
		t.Fatalf("failed to end session: %v", err)
	}
	closed, _ := store.Get("sess-claude-test1")
	if closed.Status != "closed" {
		t.Errorf("expected status closed, got %s", closed.Status)
	}
	if closed.EndedAt == nil {
		t.Errorf("expected ended_at to be set")
	}
	if closed.CloseReason != "normal exit" {
		t.Errorf("expected close_reason 'normal exit', got %q", closed.CloseReason)
	}

	// 7. Test PurgeStale
	staleSess, _ := store.Start(SessionStartRequest{
		SessionID: "sess-stale-test",
		AppID:     "claude-code",
	})
	// Artificially age the session
	staleSess.LastActiveAt = time.Now().UTC().Add(-2 * time.Hour)
	purged := store.PurgeStale(1 * time.Hour)
	if purged != 1 {
		t.Errorf("expected 1 session purged, got %d", purged)
	}
	purgedSess, _ := store.Get("sess-stale-test")
	if purgedSess.Status != "closed" {
		t.Errorf("expected purged session to be closed, got %s", purgedSess.Status)
	}

	// 7b. Test Heartbeat resurrects and updates LastActiveAt
	resurrected, err := store.Heartbeat("sess-stale-test")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if resurrected.Status != "active" {
		t.Errorf("expected session to be resurrected to active, got %s", resurrected.Status)
	}

	// 8. Test Reset
	_, _ = store.Start(SessionStartRequest{SessionID: "sess-reset-1", AppID: "test"})
	_, _ = store.Start(SessionStartRequest{SessionID: "sess-reset-2", AppID: "test"})
	resetCount := store.Reset()
	if resetCount < 2 {
		t.Errorf("expected at least 2 sessions reset, got %d", resetCount)
	}
	r1, _ := store.Get("sess-reset-1")
	if r1.Status != "closed" {
		t.Errorf("expected reset session to be closed, got %s", r1.Status)
	}
}
