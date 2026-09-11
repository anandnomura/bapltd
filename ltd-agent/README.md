# ltd-agent: Zero-Trust Execution Broker for AI Agents

`ltd-agent` is a pure Go (no cgo) zero-trust execution daemon and broker designed for AI agents operating on Linux, WSL, macOS, and Windows.

It delivers two core security primitives:
1. **Peer Process Binary Attestation (`serve` command)**: Listens on a Unix Domain Socket, inspects the caller process using socket-level peer credentials (`SO_PEERCRED` on Linux/WSL, `LOCAL_PEERPID` on macOS), resolves `/proc/<pid>/exe`, hashes the binary with SHA-256, and issues an On-Behalf-Of (OBO) JWT only to attested binaries.
2. **Cedar-Guarded Kernel Sandbox Execution (`exec` command)**: Uses [Cedar Policy](https://github.com/cedar-policy/cedar-go) to evaluate execution requests against formal authorization rules. If permitted, runs the command inside an isolated Linux kernel namespace sandbox (`CLONE_NEWUSER | CLONE_NEWPID | CLONE_NEWNS | CLONE_NEWNET`) with mapped UID/GID and injected secrets (`CORP_OBO_TOKEN`).

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
                         |  - Cedar Policy Engine Evaluation |
                         |    Principal: Agent::"Local"      |
                         |    Action:    Action::"Execute"   |
                         |    Resource:  Command::"CLI"      |
                         |    Context:   {executable, args}  |
                         +--------+-----------------+--------+
                                  |                 |
                          (Denied)|                 |(Allowed)
                                  v                 v
                        Exit 1 (JSON Error)  [Linux Kernel Sandbox]
                                             - CLONE_NEWUSER (mapped UID/GID)
                                             - CLONE_NEWPID
                                             - CLONE_NEWNS
                                             - CLONE_NEWNET
                                             - CORP_OBO_TOKEN injected
                                             - Exit 0 (JSON Result)
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

- **If Cedar denies**:
  Prints JSON with `allowed: false` and the denial reason, then exits `1`:
  ```json
  {
    "allowed": false,
    "reason": "Denial triggered by: policy policy1"
  }
  ```

- **If Cedar allows**:
  Spawns the process inside a Linux kernel sandbox:
  - `syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET`
  - Maps host UID and GID so the unprivileged child process retains file read permissions.
  - Injects `CORP_OBO_TOKEN=mock_secret_token_123` into the child environment.
  - Captures combined `stdout` and `stderr`.
  - Prints JSON with `allowed: true` and exits `0`:
    ```json
    {
      "allowed": true,
      "output": "Hello from Linux sandbox\n"
    }
    ```

```bash
# Safe permitted command
./ltd-agent exec "echo Hello World"

# Inspect injected secret inside sandbox
./ltd-agent exec printenv CORP_OBO_TOKEN

# Forbidden command (denied by Cedar, exits 1)
./ltd-agent exec "rm -rf /"
```

### 3. The `attest` Client Command

Client helper to verify socket attestation:

```bash
./ltd-agent attest --socket /tmp/ltd.sock
```

---

## Cedar Schema (`schema.json`) & Authorization Policy (`policy.cedar`)

### 1. Cedar Schema (`schema.json`)
The mock Cedar schema defines:
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

```cedar
// 1. Permit Policy: Allow if context.executable in ["pytest", "npm", "mvn", "git", "ls"]
permit (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    ["pytest", "npm", "mvn", "git", "ls"].contains(context.executable)
};

// 2. Forbid Policy: Overrides permit if context.full_command contains forbidden substrings
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
```

---

## Running Verification Tests

Run the automated test suite:

```bash
# Go unit tests (authorizer, cedar parsing)
go test -v ./...

# End-to-end integration test in Linux / WSL (9 test cases)
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

