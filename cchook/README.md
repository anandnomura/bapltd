# Claude Code Hook Interceptor for ltd-agent

This directory contains the configuration and script to intercept [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code) `Bash` tool calls and route them through the `ltd-agent` zero-trust execution broker.

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
   Registers a `PreToolUse` hook with a matcher for `"Bash"` pointing to `.claude/hooks/interceptor.sh` (or `.claude/hooks/interceptor` / `.claude/hooks/interceptor.exe`).

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
   - Resolves `ltd-agent` / `ltd-agent.exe` and invokes `ltd-agent exec "$command"`.
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

---

## User Testing

### On Windows (PowerShell)
```powershell
# Test Permitted Command
'{"tool_input": {"command": "ls"}}' | .\.claude\hooks\interceptor.exe

# Test Forbid Rule (.env)
'{"tool_input": {"command": "ls -la .env"}}' | .\.claude\hooks\interceptor.exe
```

### On Linux / WSL
```bash
./test_hook.sh
```

👉 For the full test matrix, see the root [TESTING_GUIDE.md](../TESTING_GUIDE.md).

