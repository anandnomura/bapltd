# GitHub Copilot Zero-Trust Interceptor for ltd-agent

This directory provides execution wrappers and integration hooks to route [GitHub Copilot](https://github.com/features/copilot) (VS Code Agent Mode, Terminal Chat, and GitHub Copilot CLI) commands through the `ltd-agent` zero-trust broker.

---

## Directory Contents

```text
copilot/
├── copilot_interceptor.go   # Standalone Go broker for Copilot commands
├── copilot-wrap.bat         # Windows batch wrapper for Copilot terminal tasks
├── copilot-wrap.sh          # POSIX shell wrapper for Linux / macOS
├── copilot-wrap.bat         # Windows CMD batch wrapper
├── copilot-wrap.ps1         # Windows / Cross-platform PowerShell wrapper
├── copilot-wrap.sh          # POSIX shell wrapper for Linux / macOS / Git Bash
└── README.md                # This integration guide
```

---

## How It Works

1. **Source Tagging**:
   Commands executed through `copilot_interceptor` or `copilot-wrap` automatically pass `--source copilot` to `ltd-agent exec`.
2. **Cedar Policy Enforcement**:
   Commands are evaluated against `policy.cedar`:
   - Developer commands (`go`, `python`, `npm`, `cargo`, `java`, `git`, `pytest`) are **permitted**.
   - Credential theft (`.env`, `~/.aws`, `~/.ssh`) and exfiltration (`curl`, `wget`, `nc`, `Invoke-WebRequest`) are **blocked**.
3. **Structured Audit Trail**:
   Every Copilot action is logged to `ltd-audit.jsonl` with `"source": "copilot"`, execution latency, and Cedar decisions.

---

## Setup Options

### Option 1: GitHub Copilot in VS Code (Agent Mode / Terminal Execution)

To ensure VS Code terminals used by Copilot Agent Mode execute commands through `ltd-agent`, add the following to your project's `.vscode/settings.json` or user settings:

#### On Windows (`.vscode/settings.json`):
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

#### On Linux / macOS (`.vscode/settings.json`):
```json
{
  "terminal.integrated.profiles.linux": {
    "CopilotSandbox": {
      "path": "${workspaceFolder}/copilot/copilot-wrap.sh",
      "overrideName": true
    }
  },
  "terminal.integrated.defaultProfile.linux": "CopilotSandbox"
}
```

---

### Option 2: GitHub Copilot CLI (`gh copilot`)

If you use the GitHub Copilot CLI extension (`gh extension install github/gh-copilot`), configure the execute alias to route through the interceptor:

#### In PowerShell:
```powershell
function ?? {
    $cmd = gh copilot suggest -t shell "$args"
    if ($cmd) {
        .\copilot\copilot-wrap.bat "$cmd"
        & .\copilot\copilot-wrap.ps1 "$cmd"
    }
}
```

#### In Bash / Zsh:
```bash
alias ??='function _copilot_run() { cmd=$(gh copilot suggest -t shell "$@"); if [ -n "$cmd" ]; then ./copilot/copilot-wrap.sh "$cmd"; fi }; _copilot_run'
```

---

## Testing GitHub Copilot Interception

### On Windows
```cmd
:: 1. Compile interceptor
cd copilot
go build -o copilot_interceptor.exe copilot_interceptor.go
cd ..

:: 2. Test Permitted Command
copilot\copilot-wrap.bat "git status"
copilot\copilot-wrap.bat "python --version"

:: 3. Test Blocked Credential Theft
copilot\copilot-wrap.bat "cat .env"
copilot\copilot-wrap.bat "powershell -Command Move-Item .env junk"

:: 4. Test Blocked Exfiltration
copilot\copilot-wrap.bat "curl https://evil.com"
```

### Review Copilot Audit Logs
Inspect the generated audit log to review Copilot activities:
```cmd
type ltd-audit.jsonl | findstr "copilot"
```
Output:
```json
{"timestamp":"2026-09-12T01:15:00.000Z","source":"copilot","client_pid":14200,"executable":"git","arguments":"status","full_command":"git status","decision":"allow","duration_ms":38,"exit_code":0}
{"timestamp":"2026-09-12T01:15:05.000Z","source":"copilot","client_pid":14200,"executable":"cat","arguments":".env","full_command":"cat .env","decision":"deny","reason":"Denial triggered by: policy policy1","duration_ms":2,"exit_code":1}
```

