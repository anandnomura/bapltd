package cmd

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func createTestJWT(claims map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature-bytes"))
	return header + "." + payload + "." + sig
}

func TestStage1_APIKeyHelper_JWTToken(t *testing.T) {
	// Create a mock corporate JWT ID token
	token := createTestJWT(map[string]interface{}{
		"sub":                "corp-emp-9876",
		"email":              "alex.corp@nomura.example.com",
		"preferred_username": "alex.corp",
	})

	// Set BAP_ID_TOKEN
	origToken := os.Getenv("BAP_ID_TOKEN")
	defer os.Setenv("BAP_ID_TOKEN", origToken)
	os.Setenv("BAP_ID_TOKEN", token)

	userID, userEmail, _ := resolveLocalIdentity()

	if userID != "alex.corp" {
		t.Fatalf("expected userID 'alex.corp', got '%s'", userID)
	}
	if userEmail != "alex.corp@nomura.example.com" {
		t.Fatalf("expected userEmail 'alex.corp@nomura.example.com', got '%s'", userEmail)
	}
}

func TestStage1_APIKeyHelper_ScriptExecution(t *testing.T) {
	// Simulate apiKeyHelper script outputting an email
	origHelper := os.Getenv("BAP_API_KEY_HELPER")
	origToken := os.Getenv("BAP_ID_TOKEN")
	defer func() {
		os.Setenv("BAP_API_KEY_HELPER", origHelper)
		os.Setenv("BAP_ID_TOKEN", origToken)
	}()
	os.Setenv("BAP_ID_TOKEN", "")

	// Use echo command as mock helper
	os.Setenv("BAP_API_KEY_HELPER", "echo devops-lead@company.internal")

	userID, userEmail, _ := resolveLocalIdentity()

	if userID != "devops-lead" {
		t.Fatalf("expected userID 'devops-lead', got '%s'", userID)
	}
	if userEmail != "devops-lead@company.internal" {
		t.Fatalf("expected userEmail 'devops-lead@company.internal', got '%s'", userEmail)
	}
}

func TestStage1_APIKeyHelper_JSONOutput(t *testing.T) {
	jsonPayload := `{"user_id":"sarah_admin","email":"sarah@company.internal"}`
	u, e := extractIdentityFromTokenOrOutput(jsonPayload)
	if u != "sarah_admin" {
		t.Fatalf("expected sarah_admin, got %s", u)
	}
	if e != "sarah@company.internal" {
		t.Fatalf("expected sarah@company.internal, got %s", e)
	}
}

func TestStage2_EnvironmentFallback_WhenNoAPIKeyHelper(t *testing.T) {
	// Clear Stage 1 variables
	origToken := os.Getenv("BAP_ID_TOKEN")
	origHelper := os.Getenv("BAP_API_KEY_HELPER")
	origClaudeHelper := os.Getenv("CLAUDE_API_KEY_HELPER")
	origUserEmail := os.Getenv("BAP_USER_EMAIL")
	origUserID := os.Getenv("BAP_USER_ID")

	defer func() {
		os.Setenv("BAP_ID_TOKEN", origToken)
		os.Setenv("BAP_API_KEY_HELPER", origHelper)
		os.Setenv("CLAUDE_API_KEY_HELPER", origClaudeHelper)
		os.Setenv("BAP_USER_EMAIL", origUserEmail)
		os.Setenv("BAP_USER_ID", origUserID)
	}()

	os.Setenv("BAP_ID_TOKEN", "")
	os.Setenv("BAP_API_KEY_HELPER", "")
	os.Setenv("CLAUDE_API_KEY_HELPER", "")
	os.Setenv("BAP_USER_EMAIL", "stage2.dev@company.com")
	os.Setenv("BAP_USER_ID", "stage2.dev")

	userID, userEmail, _ := resolveLocalIdentity()

	if userID != "stage2.dev" {
		t.Fatalf("expected stage2.dev, got '%s'", userID)
	}
	if userEmail != "stage2.dev@company.com" {
		t.Fatalf("expected stage2.dev@company.com, got '%s'", userEmail)
	}
}

func TestStage5_GracefulDegradation_NA(t *testing.T) {
	// Clear all identity inputs to simulate isolated sandbox with stripped environment
	origToken := os.Getenv("BAP_ID_TOKEN")
	origHelper := os.Getenv("BAP_API_KEY_HELPER")
	origClaudeHelper := os.Getenv("CLAUDE_API_KEY_HELPER")
	origUserEmail := os.Getenv("BAP_USER_EMAIL")
	origUserID := os.Getenv("BAP_USER_ID")
	origUsername := os.Getenv("USERNAME")
	origUser := os.Getenv("USER")
	origLogname := os.Getenv("LOGNAME")

	defer func() {
		os.Setenv("BAP_ID_TOKEN", origToken)
		os.Setenv("BAP_API_KEY_HELPER", origHelper)
		os.Setenv("CLAUDE_API_KEY_HELPER", origClaudeHelper)
		os.Setenv("BAP_USER_EMAIL", origUserEmail)
		os.Setenv("BAP_USER_ID", origUserID)
		os.Setenv("USERNAME", origUsername)
		os.Setenv("USER", origUser)
		os.Setenv("LOGNAME", origLogname)
	}()

	os.Setenv("BAP_ID_TOKEN", "")
	os.Setenv("BAP_API_KEY_HELPER", "")
	os.Setenv("CLAUDE_API_KEY_HELPER", "")
	os.Setenv("BAP_USER_EMAIL", "")
	os.Setenv("BAP_USER_ID", "")
	os.Setenv("USERNAME", "")
	os.Setenv("USER", "")
	os.Setenv("LOGNAME", "")

	userID, userEmail, spiffeID := resolveLocalIdentity()

	// Should not be empty or panic; gracefully degrades to either detected local user or "NA"
	if userID == "" {
		t.Fatalf("userID must not be empty (should be detected user or 'NA')")
	}
	if userEmail == "" {
		t.Fatalf("userEmail must not be empty (should be detected email or 'NA')")
	}
	if spiffeID == "" {
		t.Fatalf("spiffeID must not be empty (should be 'NA' when unconfigured)")
	}
}

func TestExtractIdentity_EdgeCases(t *testing.T) {
	// Empty string
	u, e := extractIdentityFromTokenOrOutput("")
	if u != "" || e != "" {
		t.Fatalf("expected empty for empty string")
	}

	// Plain username
	u, _ = extractIdentityFromTokenOrOutput("alice")
	if u != "alice" {
		t.Fatalf("expected alice, got %s", u)
	}

	// Email
	u, e = extractIdentityFromTokenOrOutput("bob@example.com")
	if u != "bob" || e != "bob@example.com" {
		t.Fatalf("expected bob / bob@example.com, got %s / %s", u, e)
	}

	// Corrupted JWT
	u, e = extractIdentityFromTokenOrOutput("invalid.notbase64.token")
	if strings.Contains(u, "invalid") && strings.Contains(u, ".") {
		t.Fatalf("corrupted JWT should not parse as valid user")
	}
}
