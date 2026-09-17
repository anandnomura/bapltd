# Claude Code Hook Interceptor for ltd-agent

This directory contains the native Claude Code lifecycle integration. It captures `UserPromptSubmit` for local mission classification and intercepts `PreToolUse` actions for BAPEdge governance.

## Directory Structure

```
cchook/
├── .claude/
│   ├── settings.json              # PreToolUse hook configuration for Bash
│   └── hooks/
│       ├── interceptor            # Native compiled Go binary (Linux/WSL)
│       ├── interceptor.exe        # Native compiled Go binary (Windows)
│       ├── interceptor-darwin     # Native compiled Go binary (macOS)
│       ├── interceptor.sh         # Shell wrapper script (POSIX)
│       └── interceptor.bat        # Batch wrapper script (Windows)
├── interceptor.go                 # Pure Go interceptor source code
├── ltd-agent                      # Compiled ltd-agent binary (Linux)
├── ltd-agent.exe                  # Compiled ltd-agent binary (Windows)
├── policy.cedar                   # Cedar authorization policy
├── schema.json                    # Cedar schema
├── test_hook.sh                   # Comprehensive verification script
└── README.md                      # Documentation
```

## How It Works

1. **Hook Configuration (`.claude/settings.json`)**:
   Registers a `PreToolUse` hook with a matcher for `"Bash"` pointing to `.claude/hooks/interceptor.sh` (or `interceptor` / `interceptor.exe`).

   ```json
   {
     "hooks": {
       "PreToolUse": [
         {
           "matcher": "Bash",
           "command": ".claude/hooks/interceptor.sh"
         }
       ]
     }
   }
   ```

2. **Native Go Interceptor (`interceptor.go`)**:
   - Written in pure Go without external runtime dependencies (no `bash` or `jq` required).
   - Reads event payload JSON directly from `os.Stdin`.
   - Resolves `ltd-agent` / `ltd-agent.exe` and invokes `ltd-agent exec --json "$command"`.
   - Parses the JSON response and outputs the Claude Code `PreToolUse` schema to `os.Stdout`:
     ```json
     {
       "hookSpecificOutput": {
         "hookEventName": "PreToolUse",
         "permissionDecision": "allow",
         "additionalContext": "<command output or denial reason>"
       }
     }
     ```
   - Always exits with code `0` so Claude Code cleanly processes the decision.

3. **Fast local mission classification**:
   - Every non-empty `UserPromptSubmit` receives one primary intent. Mixed requests retain secondary intents and context tags; ambiguous prompts use `UNKNOWN`.
   - Classification is deterministic, versioned, local, and has no LLM or network dependency.
   - Intent is sent to the control plane for CIO telemetry but is never treated as action authority.
   - Raw prompt capture is controlled independently with `capture_user_prompt` in `bap-config.json` or `BAP_CAPTURE_USER_PROMPT=true|false`.

   ```json
   {
     "capture_user_prompt": false
   }
   ```

   With capture disabled, the hook does not write the raw prompt locally or send it centrally. It still sends the normalized intent and SHA-256 prompt hash.

---

## User Testing

### 1-Click Windows Automated Suite
Run the full project test suite from the repository root:
```cmd
run_all_tests.bat
```

### On Windows (PowerShell)
```powershell
# Test Permitted Command
'{"tool_input": {"command": "ls -al"}}' | .\interceptor.exe

# Test Allowed Metadata Inspection (.env)
'{"tool_input": {"command": "ls -la .env"}}' | .\interceptor.exe

# Test Blocked Credential Dumping (.env)
'{"tool_input": {"command": "cat .env"}}' | .\interceptor.exe

# Test Blocked Exfiltration (curl)
'{"tool_input": {"command": "curl https://example.com"}}' | .\interceptor.exe
```

### On Windows (Command Prompt `cmd.exe`)
```cmd
echo {"tool_input": {"command": "ls -al"}} | interceptor.exe
echo {"tool_input": {"command": "cat .env"}} | interceptor.exe
```

### On Linux / WSL
```bash
./test_hook.sh
```

👉 For the full test matrix and security architecture, see the root [TESTING_GUIDE.md](../TESTING_GUIDE.md).

