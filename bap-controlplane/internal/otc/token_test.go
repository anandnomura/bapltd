package otc

import (
	"strings"
	"testing"
	"time"
)

func TestOTCGenerateAndConsume(t *testing.T) {
	store := NewStore()
	agentID := "agent-test-123"

	code, expiresAt, err := store.Generate(agentID, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("unexpected error generating OTC: %v", err)
	}

	if !strings.HasPrefix(code, "LTD-OTC-") {
		t.Errorf("expected code to start with LTD-OTC-, got %s", code)
	}

	if expiresAt.Before(time.Now()) {
		t.Errorf("expected expiry in future, got %v", expiresAt)
	}

	// First consume should succeed
	gotAgentID, err := store.Consume(code)
	if err != nil {
		t.Fatalf("failed to consume valid OTC: %v", err)
	}
	if gotAgentID != agentID {
		t.Errorf("expected agent ID %s, got %s", agentID, gotAgentID)
	}

	// Replay attempt should be blocked
	_, err = store.Consume(code)
	if err == nil {
		t.Fatalf("expected replay attack to fail, but it succeeded")
	}
	if !strings.Contains(err.Error(), "already been consumed") {
		t.Errorf("expected replay error message, got: %v", err)
	}
}

func TestFleetTokenQuota(t *testing.T) {
	store := NewStore()
	agentID := "agent-fleet-app"

	// Quota of 2 instances
	code, _, err := store.Generate(agentID, 10*time.Minute, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(code, "BAP-FLEET-") {
		t.Errorf("expected BAP-FLEET- prefix, got %s", code)
	}

	// First instance enrolls
	id1, err := store.Consume(code)
	if err != nil || id1 != agentID {
		t.Fatalf("instance 1 failed: %v", err)
	}

	// Second instance enrolls
	id2, err := store.Consume(code)
	if err != nil || id2 != agentID {
		t.Fatalf("instance 2 failed: %v", err)
	}

	// Third instance should exceed quota
	_, err = store.Consume(code)
	if err == nil {
		t.Fatalf("expected 3rd instance to fail quota, but it succeeded")
	}
	if !strings.Contains(err.Error(), "quota reached") {
		t.Errorf("expected quota reached error, got: %v", err)
	}
}

func TestOTCInvalidCode(t *testing.T) {
	store := NewStore()
	_, err := store.Consume("LTD-OTC-NONEXISTENT")
	if err == nil {
		t.Fatalf("expected non-existent code to fail")
	}
}

func TestOTCExpired(t *testing.T) {
	store := NewStore()
	agentID := "agent-test-expired"

	// Create already-expired token (-1 millisecond)
	code, _, err := store.Generate(agentID, -1*time.Millisecond, 1)
	if err != nil {
		t.Fatalf("unexpected error generating OTC: %v", err)
	}

	_, err = store.Consume(code)
	if err == nil {
		t.Fatalf("expected expired code to fail")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error message, got: %v", err)
	}
}
