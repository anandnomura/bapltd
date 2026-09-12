package authz

import (
	"testing"
	"time"

	"bap-controlplane/pkg/types"
)

func TestTokenMinter_MintAndVerify(t *testing.T) {
	secret := "test-secret-key-very-secure-12345"
	minter := NewTokenMinter(secret, 10*time.Minute)

	agent := &types.RegisteredAgent{
		AgentID:         "agent-unit-test",
		AppID:           "test-app",
		AgentName:       "TestAgent",
		EnvProfile:      types.ProfileDev,
		PermittedScopes: []string{"cli:exec", "git:read"},
	}

	token, exp, err := minter.Mint(agent, "sha256-hash-abc", []string{"cli:exec"})
	if err != nil {
		t.Fatalf("failed to mint token: %v", err)
	}

	if exp.Before(time.Now()) {
		t.Fatalf("expected expiration in future")
	}

	claims, err := minter.Verify(token)
	if err != nil {
		t.Fatalf("failed to verify valid token: %v", err)
	}

	if claims.Sub != agent.AgentID {
		t.Errorf("expected Sub %s, got %s", agent.AgentID, claims.Sub)
	}
	if claims.AppID != agent.AppID {
		t.Errorf("expected AppID %s, got %s", agent.AppID, claims.AppID)
	}
	if len(claims.Scopes) != 1 || claims.Scopes[0] != "cli:exec" {
		t.Errorf("expected requested scope cli:exec, got %v", claims.Scopes)
	}
}

func TestTokenMinter_InvalidSignature(t *testing.T) {
	minter1 := NewTokenMinter("secret-one", 10*time.Minute)
	minter2 := NewTokenMinter("secret-two", 10*time.Minute)

	agent := &types.RegisteredAgent{AgentID: "agent-tamper"}
	token, _, err := minter1.Mint(agent, "hash", nil)
	if err != nil {
		t.Fatalf("failed to mint token: %v", err)
	}

	// Verify with wrong key
	_, err = minter2.Verify(token)
	if err == nil {
		t.Fatalf("expected verification to fail with different secret key")
	}
}

func TestTokenMinter_Consume(t *testing.T) {
	minter := NewTokenMinter("secret-test-consume", 10*time.Minute)
	agent := &types.RegisteredAgent{
		AgentID:         "agent-consumer",
		AppID:           "test-app",
		PermittedScopes: []string{"payments:charge"},
	}

	token, _, err := minter.Mint(agent, "some-hash", []string{"payments:charge"})
	if err != nil {
		t.Fatalf("failed to mint: %v", err)
	}

	// 1. Consume with matching scope -> Succeeded
	claims, err := minter.Consume(token, "payments:charge")
	if err != nil {
		t.Fatalf("expected valid consume, got: %v", err)
	}
	if claims.Sub != agent.AgentID {
		t.Errorf("expected agent ID %s, got %s", agent.AgentID, claims.Sub)
	}

	// 2. Consume again (replay) -> Fails
	_, err = minter.Consume(token, "payments:charge")
	if err == nil {
		t.Fatalf("expected replay consumption to fail")
	}

	// 3. Consume with unauthorized scope
	token2, _, _ := minter.Mint(agent, "some-hash", []string{"payments:charge"})
	_, err = minter.Consume(token2, "admin:delete")
	if err == nil {
		t.Fatalf("expected unauthorized scope to fail")
	}
}
