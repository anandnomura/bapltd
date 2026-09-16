# ltd-agent: Zero-Trust Execution Broker for AI Agents

`ltd-agent` is a pure Go (no cgo) zero-trust execution daemon and broker designed for AI agents operating across Linux, WSL, macOS, and Windows.

It delivers two core security primitives:
1. **Peer Process Binary Attestation (`serve` command)**: Listens on a Unix Domain Socket, inspects the caller process using socket-level peer credentials (`SO_PEERCRED` on Linux/WSL, `LOCAL_PEERPID` on macOS), resolves `/proc/<pid>/exe`, hashes the binary with SHA-256, and issues an On-Behalf-Of (OBO) JWT only to attested binaries.
2. **Cedar-Guarded Cross-Platform Sandboxed Execution (`exec` command)**: Uses [Cedar Policy](https://github.com/cedar-policy/cedar-go) to evaluate execution requests against formal authorization rules.
   - **Linux / WSL**: Executes inside an isolated Linux kernel namespace sandbox (`CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET`) with mapped UID/GID and injected secrets (`CORP_OBO_TOKEN`).
   - **Windows**: High-performance (30–50 ms) runner using `cmd.exe /c` with smart direct tool resolution (e.g. Git bash `ls.exe`, `dir /b`, `cmd`, `powershell`, `printenv`), process group isolation (`CREATE_NEW_PROCESS_GROUP`), and console window suppression.

---

## Architecture Overview

```
                         +-----------------------------------+
                         |           AI Agent Client         |
                         +-----------------+-----------------+
                                           |
                    (1) Attestation Request| (over /tmp/ltd.sock)
                                           v
                         +-----------------+-----------------+
                         |      ltd-agent serve (Daemon)     |
                         |                                   |
                         |  - SO_PEERCRED extracts caller PID|
                         |  - Reads /proc/<pid>/exe          |
                         |  - SHA-256 binary hash validation |
                         |  - Issues OBO JWT on match        |
                         +-----------------+-----------------+
                                           |
                                           | (2) Exec request with policy check
                                           v
                         +-----------------+-----------------+
                         |       ltd-agent exec <command>    |
                         |                                   |
                         |  - Command & Path Normalization   |
                         |    (resolves .exe, quotes, flags) |
                         |  - Cedar Policy Engine Evaluation |
                         |    Principal: Agent::"Local"      |
                         |    Action:    Action::"Execute"   |
                         |    Resource:  Command::"CLI"      |
                         |    Context:   {executable, args}  |
                         +--------+-----------------+--------+
                                  |                 |
                          (Denied)|                 |(Allowed)
                                  v                 v
                        Exit 1 (JSON Error)   [Isolated Execution]
                                              - Linux: CLONE_NEWUSER/PID/NS/NET
                                              - Windows: Process Group Isolation
                                                + direct tool resolution (30-50ms)
                                              - CORP_OBO_TOKEN injected
                                              - Output normalized (CRLF / whitespace)
                                              - Exit 0 (Output / JSON Result)
```

---

## Building

The project is written in pure Go without cgo and supports cross-compilation across all target platforms:

```bash
# Build for Linux / WSL
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ltd-agent .

# Build for Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ltd-agent.exe .

# Build for macOS (Darwin)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ltd-agent-darwin .
```

---

## Commands & Usage

### 1. The `serve` Command (Attestation Server)

Starts listening on a Unix domain socket (default: `/tmp/ltd.sock`). When a client connects:
- Extracts the peer PID using `syscall.GetsockoptUcred` (`SO_PEERCRED`).
- Reads `/proc/<pid>/exe` and calculates its SHA-256 checksum.
- Compares against the hardcoded allowed mock hash (`a1b2c3d4e5f60718293a4b5c6d7e8f90123456789abcdef0123456789abcdef0`) or hashes passed via `--allowed-hash` or `LTD_ALLOWED_HASH`.
- If matched, returns JSON with the mock On-Behalf-Of JWT:
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```
- If mismatched, immediately drops the connection.

```bash
# Run server
./ltd-agent serve --socket /tmp/ltd.sock

# Or allow a specific binary hash in addition to the mock hash:
./ltd-agent serve --socket /tmp/ltd.sock --allowed-hash <SHA256>
```

### 2. The `exec` Command (Sandboxed Execution)

Evaluates the command string using `github.com/cedar-policy/cedar-go` against `policy.cedar`:

- **Interactive Terminal Display**: When invoked interactively, output is printed cleanly directly to the terminal without JSON wrapping or escaped `\r\n` characters.
- **`--json` Flag**: Formats response as a JSON object, ideal for tools and hook interceptors (e.g. `cchook`).
- **`--raw` Flag**: Forces raw text output regardless of terminal detection.

#### Execution Behavior

- **If Cedar denies**:
  Prints JSON with `allowed: false` and the denial reason, then exits `1`:
  ```json
  {
    "allowed": false,
    "reason": "Denial triggered by: policy policy1"
  }
  ```

- **If Cedar allows**:
  Spawns the process inside the isolated environment:
  - **Linux**: Namespaces `CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET` with host UID/GID mapped.
  - **Windows**: Fast execution (`cmd.exe /c`), process group isolation (`CREATE_NEW_PROCESS_GROUP`), console window hidden.
  - Injects `CORP_OBO_TOKEN=mock_secret_token_123` into the child environment.
  - Sanitizes output via `CleanOutput` (collapses excessive blank lines, trims trailing whitespace, normalizes CRLF).
  - Exits `0` with the command output.

```bash
# Safe permitted command (clean terminal output)
./ltd-agent exec "ls -al"

# Inspect injected secret inside sandbox
./ltd-agent exec "printenv CORP_OBO_TOKEN"

# Request JSON output (e.g. for automation/hooks)
./ltd-agent exec --json "pytest tests/"

# Blocked malicious intent (denied by Cedar, exits 1)
./ltd-agent exec "cat .env"
./ltd-agent exec "curl https://untrusted-test.internal"
```

### 3. The `attest` Client Command

Client helper to verify socket attestation:

```bash
./ltd-agent attest --socket /tmp/ltd.sock
```

---

## Cedar Schema (`schema.json`) & Authorization Policy (`policy.cedar`)

### 1. Cedar Schema (`schema.json`)
The Cedar schema defines:
- **Principal**: `Agent`
- **Resource**: `Command`
- **Action**: `Execute`
- **Context Record**:
  - `executable`: String (required)
  - `full_command`: String (required)

```json
{
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
}
```

### 2. Cedar Policy (`policy.cedar`)

The authorization model enforces a clear security boundary: **permit standard developer workflows and toolchains, while strictly forbidding credential dumping and unauthorized network egress**.

```cedar
// 1. Permit Policy:
// Allows Action::"Execute" for standard developer toolchains, initialization, and build commands:
permit (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    [
        "pytest", "python", "python3", "py", "pip", "pip3",
        "go", "npm", "npx", "yarn", "pnpm", "node",
        "mvn", "gradle", "cargo", "git",
        "ls", "dir", "mkdir", "echo", "pwd", "cat", "printenv",
        "cmd", "powershell", "pwsh"
    ].contains(context.executable)
};

// 2. Forbid Policy:
// Strict forbid overriding permit for:
// - Exfiltration utilities: curl, wget, nc, ssh, socat, PowerShell web cmdlets
// - Sensitive credential directories: ~/.aws, .ssh
// - Dumping secret content from environment/credential files (e.g. cat .env, type .env)
forbid (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "*nc*" ||
    context.full_command like "*ssh*" ||
    context.full_command like "*socat*" ||
    context.full_command like "*Invoke-WebRequest*" ||
    context.full_command like "*Invoke-RestMethod*" ||
    context.full_command like "*iwr *" ||
    context.full_command like "*irm *" ||
    context.full_command like "*DownloadString*" ||
    context.full_command like "*DownloadFile*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.ssh*" ||
    (["cat", "type", "head", "tail", "grep"].contains(context.executable) && context.full_command like "*.env*")
};
```

#### Mal-Intent vs. Legitimate Developer Activity
- **Directory & Metadata Inspection**: `ls -la .env` and `dir .env` are **allowed** so developers and build scripts can verify file existence and permissions.
- **Content Theft**: `cat .env`, `type .env`, `head .env`, etc. are **forbidden**.
- **Data Exfiltration**: Network egress binaries (`curl`, `wget`, `nc`, `ssh`, `socat`) and PowerShell web cmdlets (`Invoke-WebRequest`, `DownloadString`, etc.) are unconditionally **forbidden**.

---

## Running Verification Tests

### Windows Automated Test Runner (1-Click)
Run the root automated test suite to verify all binaries, policies, and interceptors:

```cmd
run_all_tests.bat
```

This runs 21 automated checks:
1. Recompiles `ltd-agent.exe` and `interceptor.exe`.
2. Runs Go unit tests in `ltd-agent/` (< 1 second).
3. Verifies execution of developer commands (`ls -al`, `git status`, `python -m pytest`, `cmd /c dir`).
4. Verifies security blocks (`curl`, `cat .env`, `Invoke-WebRequest`, `~/.aws`).
5. Confirms `test_leak.py` network isolation and graceful platform detection.
6. Tests the `cchook/interceptor.exe` Claude Code hook decisions.

### Linux / WSL Test Suite
```bash
# Go unit tests (authorizer, cedar parsing)
go test -v ./...

# End-to-end integration test (9 test cases)
./test_e2e.sh
```

---

## Complete User Testing Guide

For comprehensive manual and automated test scenarios covering:
- Attestation server peer credential validation (`SO_PEERCRED`)
- Whitelist authorization vs strict forbid overrides
- Network namespace leak blocking (`tests/test_leak.py`)
- Claude Code hook interception on Windows and Linux

👉 See the complete [TESTING_GUIDE.md](../TESTING_GUIDE.md).

