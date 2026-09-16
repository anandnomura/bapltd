# BAP (Bounded Authority Plane) Enterprise Integration Guide

The **Bounded Authority Plane (BAP)** delivers kernel-level, zero-trust cryptographic governance for AI coding agents and LLM execution environments.

This guide provides drop-in, copy-pasteable configuration for every major AI developer tool and agent environment.

---

## Architecture at a Glance

```
       ┌───────────────────────────────────────────────────────────┐
       │                 CENTRAL BAP CONTROL PLANE                 │
       │   - Automated OTC Registration & SPIFFE Workload Minting  │
       │   - Authoritative Cedar Policy Bundles & Schema Invariants│
       │   - Live Workload Radar & Tamper-Evident Hash-Chain Audit │
       └─────────────────────────────┬─────────────────────────────┘
                                     │ Heartbeats, Grants, Telemetry
    ┌────────────────────────────────┴────────────────────────────────┐
    │                                                                 │
    ▼                                                                 ▼
┌─────────────────────────────────┐               ┌─────────────────────────────────┐
│     AI AGENTS & IDE INTERFACES  │               │      MODEL CONTEXT PROTOCOL     │
│  - Google Antigravity           │               │  - Claude Desktop               │
│  - Claude Code (cchook)         │               │  - VS Code Copilot              │
│  - GitHub Copilot CLI           │               │  - Cursor & Windsurf            │
│  - Python Agent SDK (bap_sdk)   │               │  - Custom MCP Clients           │
└────────────────┬────────────────┘               └────────────────┬────────────────┘
                 │                                                 │
                 ▼                                                 ▼
      ┌─────────────────────────────────────────────────────────────────┐
      │             BAP EDGE LOCAL TRUSTED DAEMON (bapedge.exe)        │
      │  - Sub-2ms Local Cedar Authorizer (0ms Offline Resilience)     │
      │  - Automatic Agent JSON Detection & Actionable Auto-Remediation │
      │  - Injected Corporate OBO Token & Host Credential Sandboxing    │
      └─────────────────────────────────────────────────────────────────┘
```

---

## 1. Google Antigravity Integration

Google Antigravity supports dual integration modes: **Shell Governance** (via repository rules) and **Native Model Context Protocol (MCP)**.

### Mode A: Zero-Trust Shell Governance (Active Pair Programming)

Place a [`GEMINI.md`](file:///c:/Users/User/pyprj/bapltd/GEMINI.md) file in your workspace root:

```markdown
# BAP Zero-Trust Policy & Shell Governance for Antigravity

This workspace is governed by the Local Trusted Daemon (`bapedge.exe`).

## Mandatory Execution Rules
1. **Always Use BAP Edge Execution Broker**:
   - All shell executions MUST run through `bapedge.exe exec`:
     ```powershell
     .\bapedge.exe exec "<command>"
     ```
   - Never run raw, ungoverned shell commands on the host machine.
2. **Zero-Trust Policy Enforcement**:
   - Commands are evaluated against Cedar invariants before execution.
   - If a command is denied, inspect the `suggestion` in the structured response and guide the user or proceed with the authorized alternative.
3. **Identity & Audit Logging**:
   - Set `LTD_SOURCE=antigravity` and `BAP_SESSION_ID=sess-antigravity` so operations stream to the Live Workload Radar and `ltd-audit.jsonl`.
```

### Mode B: Model Context Protocol (MCP) Configuration

Configure `bapmcp.exe` in `C:\Users\<User>\.gemini\config\mcp_config.json`:

```json
{
  "mcpServers": {
    "bap-zero-trust": {
      "command": "C:\\path\\to\\bapltd\\bapmcp.exe",
      "args": [],
      "env": {
        "LTD_SOURCE": "antigravity-mcp",
        "BAP_SESSION_ID": "sess-antigravity"
      }
    }
  }
}
```

**Exposed MCP Tools:**
- `bap_execute(command, raw_output)`: Executes commands in a governed kernel sandbox.
- `bap_explain_policy(command)`: Returns real-time Cedar authorization simulation without executing.
- `bap_status()`: Queries live cryptographic attestation, control plane endpoint, and hash-chain status.

---

## 2. Claude Code Integration (Anthropic)

Claude Code integrates seamlessly with BAP using Anthropic's **PreToolUse Hook** interface via [`cchook/interceptor.exe`](file:///c:/Users/User/pyprj/bapltd/cchook/interceptor.exe).

### Configuration (`~/.claude/config.json`)

```json
{
  "hooks": {
    "preToolUse": {
      "command": "C:\\path\\to\\bapltd\\cchook\\interceptor.exe",
      "environment": {
        "LTD_SOURCE": "claude-code",
        "BAP_SESSION_ID": "sess-claude-worker"
      }
    }
  }
}
```

### How It Works:
1. When Claude Code intends to run `Bash` or inspect files (`Read`, `View`), it passes JSON to `stdin` of `interceptor.exe`.
2. BAP inspects the payload in sub-2ms:
   - Safe developer commands (`git`, `pytest`, `npm`, `go`, local LLM queries) are authorized (`"permissionDecision": "allow"`).
   - Egress attempts, `.env` reads, private keys, or evasive renames are blocked immediately (`"permissionDecision": "deny"`).
3. The hook response delivers structured remediation suggestions directly into Claude's prompt context.

---

## 3. Claude Desktop Integration

Add BAP to Claude Desktop by editing `%APPDATA%\Claude\claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "bap-zero-trust": {
      "command": "C:\\path\\to\\bapltd\\bapmcp.exe",
      "args": [],
      "env": {
        "LTD_SOURCE": "claude-desktop"
      }
    }
  }
}
```

Claude Desktop will instantly discover the zero-trust execution broker tools on startup.

---

## 4. GitHub Copilot Integration (VS Code)

GitHub Copilot supports BAP both via **VS Code MCP** and through **CLI Shell Wrappers**.

### Option A: VS Code Copilot MCP Configuration
Add to `.vscode/settings.json` or Copilot MCP settings:

```json
{
  "github.copilot.chat.mcpServers": {
    "bap-edge": {
      "command": "C:\\path\\to\\bapltd\\bapmcp.exe",
      "args": [],
      "env": {
        "LTD_SOURCE": "copilot-mcp"
      }
    }
  }
}
```

### Option B: Copilot CLI Wrapper Scripts
For command-line copilot tasks, invoke commands through [`copilot-wrap.bat`](file:///c:/Users/User/pyprj/bapltd/copilot-wrap.bat) or [`copilot-wrap.ps1`](file:///c:/Users/User/pyprj/bapltd/copilot-wrap.ps1):

```cmd
copilot-wrap.bat "git status"
```

---

## 5. Cursor & Windsurf IDE Integration

In Cursor or Windsurf, configure standard MCP stdio settings in **Settings > MCP**:

- **Name**: `bap-zero-trust`
- **Command**: `C:\path\to\bapltd\bapmcp.exe`
- **Arguments**: *(leave empty)*
- **Environment**:
  ```
  LTD_SOURCE=cursor-ide
  BAP_SESSION_ID=sess-cursor
  ```

---

## 6. Python Agent SDK (`bap_sdk`)

For autonomous Python agents (LangChain, AutoGen, CrewAI, or bespoke agents), use the native BAP SDK:

```python
import os
from bap_sdk import BAPClient

# Initialize governed agent client
client = BAPClient(
    source="my-autonomous-agent",
    session_id="sess-auto-01",
    control_plane_url="http://localhost:8080"
)

# 1. Start Governed Session (registers with Live Workload Radar)
session = client.start_session()
print(f"Enrolled in BAP Fleet: {session.session_id}")

try:
    # 2. Execute safe command
    result = client.execute("git status")
    if result.allowed:
        print("Git Output:\n", result.output)

    # 3. Adversarial attempts are blocked with intelligent guidance
    bad_result = client.execute("cat .env")
    if not bad_result.allowed:
        print("Blocked by Cedar Invariant:", bad_result.reason)
        print("Remediation Suggestion:", bad_result.suggestion)

finally:
    # 4. Clean Deregistration
    client.end_session()
```

---

## 7. Verifying in the Live Control Plane Radar

1. Launch the BAP Control Plane:
   ```cmd
   bapcontrolplane.exe -port 8080
   ```
2. Open [`http://localhost:8080/inspector.html`](http://localhost:8080/inspector.html) in your browser.
3. The **Live Workload Radar** dynamically updates:
   - Active agent chips appear in real-time (`Antigravity IDE`, `Claude Code`, `Copilot CLI`, `Python Agent`).
   - Clicking any client chip filters the activity timeline to that agent's audit stream.
   - Real-time toast notifications announce enrollment and clean deregistration.
   - 100% cryptographic anti-tamper hash-chain verification guarantees forensic integrity.

---

## Integration Summary Table

| Tool / Environment | Mechanism | Config Location | Primary Benefit |
| :--- | :--- | :--- | :--- |
| **Google Antigravity** | Workspace Rule (`GEMINI.md`) + MCP | Root `GEMINI.md` / `mcp_config.json` | Active pair-programming enforcement + full tool discovery |
| **Claude Code** | PreToolUse Hook (`cchook`) | `~/.claude/config.json` | Zero-overhead stdin/stdout interception of Bash & file tools |
| **Claude Desktop** | Stdio MCP Server | `claude_desktop_config.json` | Chat-driven zero-trust command execution |
| **GitHub Copilot** | VS Code MCP & Wrapper scripts | `.vscode/settings.json` / CLI wrapper | Transparent policy enforcement across IDE & terminal |
| **Cursor / Windsurf** | Stdio MCP Server | IDE MCP Settings | AI agent sandbox with automatic remediation advice |
| **Python SDK** | Pure-Python SDK (`bap_sdk`) | In-code `BAPClient` | Enterprise agent fleet management with OTC & SPIFFE |

