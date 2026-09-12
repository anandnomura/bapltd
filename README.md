# bapltd: Zero-Trust Execution Broker for AI Agents

`bapltd` (`ltd-agent`) is a high-performance, pure Go (no cgo) zero-trust execution broker designed for AI agents operating on Linux, WSL, macOS, and Windows. It secures tool executions for **Claude Code**, **GitHub Copilot**, and automated agent systems.

---

## Architecture Overview

```text
                           +-------------------------------+
                           |    AI Agent (Claude / Copilot)|
                           +---------------+---------------+
                                           | Tool Call ("Bash", "Terminal")
                                           v
                           +---------------+---------------+
                           |   Interceptor Hook / Wrapper  |
                           |  (cchook / copilot / VS Code) |
                           +---------------+---------------+
                                           | Invokes ltd-agent exec --source <agent>
                                           v
                           +---------------+---------------+
                           |    ltd-agent Execution Broker |
                           |                               |
                           |  1. Intelligent Normalization |
                           |     - Strips shell artifacts  |
                           |     - Preserves quoted spaces |
                           |  2. Cedar Policy Engine       |
                           |     - Whitelist Dev Tools     |
                           |     - Strict Anti-Evasion     |
                           +-------+---------------+-------+
                                   |               |
                           (Denied)|               |(Allowed)
                                   v               v
                        Exit 1 (JSON Error)   [Sandboxed Execution]
                                              - Linux: CLONE_NEWUSER/PID/NS/NET
                                              - Windows: Process Group Isolation
                                                + 30-50ms fast cmd.exe runner
                                              - Output sanitized (CRLF/spaces)
                                              - Secret injected (CORP_OBO_TOKEN)
                                              - Exit 0 (Output / JSON)
                                   |               |
                                   +-------+-------+
                                           |
                                           v
                           +---------------+---------------+
                           |   Structured Audit Logger     |
                           |     (ltd-audit.jsonl)         |
                           |  - Telemetry & Latency        |
                           |  - Anomaly & Evasion Learning |
                           +-------------------------------+
```

---

## 1. Build Commands

Compile natively or cross-compile for all operating systems (pure Go, `CGO_ENABLED=0`):

### Windows (AMD64)
```cmd
:: Build ltd-agent broker
cd ltd-agent
go build -o ltd-agent.exe .
cd ..

:: Build Claude Code interceptor
cd cchook
go build -o interceptor.exe interceptor.go
cd ..

:: Build GitHub Copilot interceptor
cd copilot
go build -o copilot_interceptor.exe copilot_interceptor.go
cd ..
```

### Linux / WSL (AMD64)
```bash
# Build ltd-agent broker
cd ltd-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ltd-agent .
chmod +x ltd-agent
cd ..

# Build Claude Code interceptor
cd cchook
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o interceptor interceptor.go
chmod +x interceptor
cd ..

# Build GitHub Copilot interceptor
cd copilot
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o copilot_interceptor copilot_interceptor.go
chmod +x copilot_interceptor
cd ..
```

### macOS (Darwin AMD64 / Apple Silicon ARM64)
```bash
# Build ltd-agent broker for macOS
cd ltd-agent
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o ltd-agent-darwin .
cd ..
```

---

## 2. Run Commands

### Interactive CLI Execution (Direct Output)
```cmd
:: Standard developer commands (permitted)
ltd-agent\ltd-agent.exe exec "ls -al"
ltd-agent\ltd-agent.exe exec "git status"
ltd-agent\ltd-agent.exe exec "go version"
ltd-agent\ltd-agent.exe exec "java -version"
ltd-agent\ltd-agent.exe exec "python --version"

:: Commands with arguments containing spaces (automatically encoded)
ltd-agent\ltd-agent.exe exec python -c "import sys; print(sys.argv[1])" "hello world"
ltd-agent\ltd-agent.exe exec 'java -Dapp.name="My Custom App" -version'

:: Chaining and pipes
ltd-agent\ltd-agent.exe exec "echo hello && echo world"
ltd-agent\ltd-agent.exe exec "ls -al | grep -i README"
```

### Output Format Flags
- `--raw`: Outputs raw text regardless of whether stdout is connected to a terminal.
- `--json`: Formats response as a JSON object (`{"allowed": true, "output": "..."}`).
- `--source <name>`: Tags the calling agent in audit logs (`claude-code`, `copilot`, `cli`). Defaults to `LTD_SOURCE` or `cli`.
- `--audit-log <path>`: Specifies audit log file location (defaults to `LTD_AUDIT_LOG` or `ltd-audit.jsonl`, `'off'` to disable).

---

## 3. Test Commands

### Windows Automated 1-Click Test Suite (27 Checks)
Run the root test suite to recompile, test unit code, verify policies, check anti-evasion, and validate hooks:
```cmd
run_all_tests.bat
```
Output:
```text
Total Passed : 27
Total Failed : 0
[OVERALL STATUS] SUCCESS - All tests passed!
```

### Go Unit Tests
```cmd
cd ltd-agent
go test -v ./...
cd ..
```

### Linux / WSL End-to-End Suite
```bash
./test_e2e.sh
```

---

## 4. Claude Code Integration & Managed Settings

Claude Code executes bash commands via tool use. You can secure Claude Code using project-level configuration or tamper-proof **Managed Settings**.

### Step 1: Project-Level Hook (`.claude/settings.json`)
In your project repository, create `.claude/settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "cchook/interceptor.exe"
      }
    ]
  }
}
```
*(On Linux/macOS, use `"cchook/interceptor"` or `"cchook/interceptor.sh"`)*.

### Step 2: Enterprise Managed Settings (Tamper-Proof)
To enforce `ltd-agent` globally so agents or users cannot disable the hook or bypass policies, configure Claude Code **Managed Settings**:

#### Windows:
Create or edit `%PROGRAMDATA%\Anthropic\ClaudeCode\managed-settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "C:\\ProgramData\\Anthropic\\ClaudeCode\\hooks\\interceptor.exe"
      }
    ]
  }
}
```

#### Linux / macOS:
Create or edit `/etc/claude-code/managed-settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "/etc/claude-code/hooks/interceptor"
      }
    ]
  }
}
```

### Testing Claude Code Hook
Test tool-call payloads with `interceptor.exe`:
```powershell
# Allowed command
'{"tool_input": {"command": "ls -al"}}' | .\cchook\interceptor.exe

# Blocked command (cat .env)
'{"tool_input": {"command": "cat .env"}}' | .\cchook\interceptor.exe

# Blocked evasive rename attack
'{"tool_input": {"command": "powershell -Command Move-Item .env junk"}}' | .\cchook\interceptor.exe
```

---

## 5. GitHub Copilot Integration

GitHub Copilot executes commands in the integrated terminal (VS Code Agent Mode) or via GitHub Copilot CLI.

### Step 1: VS Code Terminal Integration (`.vscode/settings.json`)
Configure VS Code to route terminal command executions through `copilot-wrap.bat`:
```json
{
  "terminal.integrated.profiles.windows": {
    "CopilotSandbox": {
      "path": "${workspaceFolder}\\copilot\\copilot-wrap.bat",
      "overrideName": true
    }
  },
  "terminal.integrated.defaultProfile.windows": "CopilotSandbox"
}
```

### Step 2: GitHub Copilot CLI (`gh copilot`)
Wrap command execution in PowerShell or Bash:
```powershell
function ?? {
    $cmd = gh copilot suggest -t shell "$args"
    if ($cmd) {
        .\copilot\copilot-wrap.bat "$cmd"
    }
}
```

### Testing Copilot Integration
```cmd
copilot\copilot-wrap.bat "git status"
copilot\copilot-wrap.bat "cat .env"
```

---

## 6. Audit Logging & Learning Telemetry

Every execution is logged to `ltd-audit.jsonl` in JSON Lines format:
```json
{"timestamp":"2026-09-12T01:14:04.589Z","source":"copilot","client_pid":12352,"executable":"git","arguments":"status","full_command":"git status","decision":"allow","duration_ms":125,"exit_code":0}
{"timestamp":"2026-09-12T01:14:08.870Z","source":"claude-code","client_pid":10932,"executable":"cat","arguments":".env","full_command":"cat .env","decision":"deny","reason":"Denial triggered by: policy policy1","duration_ms":2,"exit_code":1}
```

### How to Learn from Audit Logs
1. **Find Unexpected Denials**:
   Filter the audit log for unexpected denials to refine policy rules:
   ```powershell
   Get-Content ltd-audit.jsonl | ConvertFrom-Json | Where-Object { $_.decision -eq 'deny' }
   ```
2. **Review Command Patterns**:
   Inspect what commands Claude or Copilot ran during a session:
   ```powershell
   Get-Content ltd-audit.jsonl | ConvertFrom-Json | Select-Object timestamp, source, executable, decision, duration_ms
   ```
3. **Detect Evasion Attempts**:
   Identify attempts to rename, copy, or redirect `.env` or exfiltrate credentials.

