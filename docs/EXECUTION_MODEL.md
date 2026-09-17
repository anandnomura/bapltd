# BAP Execution & Containment Model (BAP-200)

## 1. Core Architectural Design Decision

**Broker-Owned Execution is the primary BAP enforcement model.**

In enterprise AI agent governance, security cannot rely on advisory instructions, client-side honesty, or uncontained post-decision execution. The BAP platform contract guarantees:

1. **Broker-Owned Execution as the Sole Executor**: Protected agent actions are executed directly by `bapedge` within an isolated sandbox boundary. BAPEdge evaluates Cedar policy, establishes sandbox containment (process isolation, restricted token injection, network PEP routing, and workspace jails), runs the action, captures output and exit status, and generates a cryptographic **Execution Receipt**.
2. **Decision-Only Hooks Limitation**: Decision-only hooks evaluate policy and emit audit telemetry, but do not own the runtime execution. They are **acceptable for audit/shadow mode** or when an independent OS boundary (e.g. hardened VM, gVisor container) already enforces isolation. Decision-only hooks are **not sufficient for containment** in developer workspaces because uncontained child processes, subtle shell expansions, and environment variables cannot be sealed post-decision.
3. **The Original Native Action Must Never Run After BAPEdge**: The platform contract guarantees that an action is executed **exactly once**. Once BAPEdge executes an action inside the sandbox, the native uncontained tool is never invoked.

---

## 2. Platform Integration Architecture

```mermaid
flowchart TD
    subgraph AgentPlatforms ["Agent Host Platforms"]
        Claude["Claude Code"]
        Copilot["GitHub Copilot CLI"]
        MCPClient["IDE / Cursor / Windsurf"]
        SDKAgent["Custom Agent (Python/Node)"]
    end

    subgraph BAPPEP ["BAP Policy Enforcement Point (PEP)"]
        Hook["cchook / interceptor.exe<br/>(PreToolUse PEP)"]
        CopilotShim["copilot-interceptor.exe<br/>(Execution Shim)"]
        MCPServer["bapedge mcp<br/>(Model Context Protocol)"]
        SDKClient["bap_sdk"]
    end

    subgraph BAPCore ["BAPEdge Broker Execution Engine"]
        Authorizer["Cedar Authorization Engine<br/>(policy.cedar)"]
        TamperGuard["Anti-Tampering Invariant Engine"]
        Sandbox["Sandboxed Execution Engine<br/>(Process Tree & Token Isolation)"]
        ReceiptEngine["Cryptographic Receipt Engine<br/>(SHA-256 / SPIFFE Provenance)"]
        RiskTracker["Session Threat Correlator<br/>(Correlated Risk Events)"]
    end

    subgraph TargetWorkloads ["Target Workloads & OS Boundary"]
        AllowedProcess["Sandboxed Process<br/>(CORP_OBO_TOKEN Injected)"]
        GatewayPEP["BAP Gateway PEP<br/>(:9090 Network Proxy)"]
    end

    Claude -->|PreToolUse JSON| Hook
    Hook -->|updatedInput: bapedge exec| Claude
    Claude -->|Executes Sole Broker| Authorizer

    Copilot -->|Shell Delegation| CopilotShim
    CopilotShim -->|bapedge exec| Authorizer

    MCPClient -->|bap_exec / bap_read_file| MCPServer
    MCPServer -->|bapedge exec| Authorizer

    SDKAgent -->|session.exec()| SDKClient
    SDKClient -->|bapedge exec| Authorizer

    Authorizer --> TamperGuard
    TamperGuard --> Sandbox
    Sandbox --> AllowedProcess
    Sandbox --> ReceiptEngine
    Sandbox --> RiskTracker
    AllowedProcess -->|HTTP / APIs| GatewayPEP
```

### 2.1 Claude Code Integration Contract
- **Enforcement Mode (`enforce`)**:
  - The `PreToolUse` hook intercepts command actions (`Bash`).
  - The hook validates Cedar policy via `bapedge check`.
  - If allowed: The hook uses Claude Code's native **`updatedInput`** capability to rewrite the command into:
    `bapedge.exe exec --source claude-code --session-id <sessionID> -- <original-command>`
  - Claude Code natively spawns `bapedge.exe` as its execution child. BAPEdge is the **sole executor**, executing the command **once** inside the sandbox, capturing output, exit code, and generating the cryptographic receipt.
  - If denied: The hook returns `permissionDecision: "deny"`. Claude Code halts execution immediately with **zero side effects**.
  - Direct file inspection/modification tools (`Read`, `View`, `Write`, `Edit`) are checked for directory traversal, sensitive credential files (`.env`, `.aws`, `.ssh`), and BAP protected assets.
- **Audit / Shadow Mode (`audit` / `shadow`)**:
  - The hook calls `bapedge check` (`--decision-only`), records telemetry to the Control Plane, and returns `permissionDecision: "allow"` with the original un-rewritten command for passive observation.

### 2.2 GitHub Copilot CLI Integration Contract
- GitHub Copilot CLI executes commands through `copilot-interceptor.exe`.
- The interceptor replaces native shell execution by delegating directly to `bapedge exec`. BAPEdge executes once in the sandbox and returns stdout, stderr, and exit status.

### 2.3 Model Context Protocol (MCP) Integration Contract
- AI agents in IDEs (Cursor, Windsurf, Claude Desktop) interact with tools defined by `bapedge mcp` (`bap_exec`, `bap_read_file`, `bap_write_file`).
- BAPEdge executes every tool call inside the broker sandbox. Uncontained local execution is disabled in the agent configuration.

### 2.4 Agent SDK Integration Contract
- Programmatic Python/Node agents invoke `bap.exec()` via the BAP SDK.
- The SDK executes `bapedge exec` directly.

---

## 3. Tool Semantics & Execution Matrix

| Action / Tool | Integration Vector | Enforcement Mechanism | Single Execution Guarantee | Side Effect on Deny |
| :--- | :--- | :--- | :--- | :--- |
| **Bash / Shell** | `PreToolUse` / `copilot-interceptor` | Rewritten to `bapedge exec` via `updatedInput` | Yes (runs only within BAPEdge) | Zero (no subshell spawned) |
| **Read / View** | Direct hook inspection | Boundary & credential regex validation | Yes (read occurs once) | Zero (file not opened) |
| **Write / Edit** | Direct hook inspection | Boundary check + Anti-tamper protection | Yes (written once via broker) | Zero (file not modified) |
| **Python** | Interpreter invocation | Cedar script inspection + Socket/Egress blocks | Yes (sandboxed interpreter) | Zero (interpreter not spawned) |
| **PowerShell** | Interpreter invocation | Cedar command inspection + Encoded command block | Yes (sandboxed pwsh) | Zero (pwsh not spawned) |
| **MCP Tools** | JSON-RPC stdio | `bapedge mcp` tool handler | Yes (BAP executes once) | Zero (tool returns error) |
| **Child Processes** | Process tree containment | Job Object / Process group isolation | Yes (governed by root process) | Zero (child not spawned) |

---

## 4. Tamper Resistance Guarantees

AI agents cannot modify, rename, truncate, delete, or disable security infrastructure. The following assets are strictly protected across all vectors (`Write`, `Edit`, `Bash`, `Python`, `PowerShell`):

1. **Cedar Policies**: `policy.cedar`, `*.cedar`
2. **Hook Configurations**: `.claude/settings.json`, `.claude/hooks/*`
3. **Session & State Files**: `.bap/`, `bap-config.json`, `.bap-session.json`, `.bap-revoked`
4. **BAP Binaries**: `bapedge.exe`, `interceptor.exe`, `bapcontrolplane.exe`, `bapgateway.exe`

Any command or file tool attempting to target these assets triggers an immediate **Security Invariant Violation** with result `DENIED_TAMPER` and zero side effects.

---

## 5. Correlated Risk Events & Threat Escalation

When an agent encounters a denial and attempts alternative techniques to bypass security controls (e.g. attempting `cat .env`, followed by `type .env`, followed by `python -c "open('.env')"`), BAPEdge correlates these actions:

- Denials are tracked per `session_id` in `.bap/risk_state.json`.
- Consecutive denials escalate the session threat level:
  - 1 Denial: `LOW`
  - 2–3 Denials: `ELEVATED` (emits `[CORRELATED RISK EVENT]` warning)
  - 4+ Denials: `CRITICAL` (triggers automated session revocation candidate in Control Plane)
- Telemetry events transmitted to the central Control Plane maintain an audit trail of adversarial progression.

---

## 6. Cryptographic Execution Receipt Specification

Every execution (whether allowed, denied, or audit-evaluated) issues an immutable structured receipt returned in `ExecResponse.Receipt`:

```json
{
  "allowed": true,
  "output": "v1.4.2",
  "exit_code": 0,
  "mode": "enforce",
  "receipt": {
    "receipt_id": "rcpt-e8ecd65dad9a-1789613769",
    "request_hash": "sha256:cc364b91585d15bc116d2df4cf33b3d5946f58e181dc6bc8642f4ea6914934b9",
    "identity": "spiffe://bap.internal/app/financial-reconciler/instance/desktop-e63v3uh-fa31b07c",
    "delegation": "user:User->agent:claude-code",
    "policy_version": "sha256:517c94488bee2d281c2df09ac34f81fc",
    "policy_hash": "sha256:517c94488bee2d281c2df09ac34f81fc",
    "sandbox_profile": "bap-broker-standard",
    "session_id": "sess-claude-pid-10196",
    "timestamp": "2026-09-17T02:56:09.9788853Z",
    "result": "ALLOWED_EXECUTED"
  }
}
```

### Receipt Fields:
- **`receipt_id`**: Cryptographic unique identifier derived from request hash, session, and timestamp nonce.
- **`request_hash`**: SHA-256 digest of the normalized input command.
- **`identity`**: SPIFFE workload identity or enrolled agent identity (`spiffe://bap.internal/...`).
- **`delegation`**: On-behalf-of provenance chain (`user:<user>->agent:<agent>`).
- **`policy_version`**: SHA-256 digest of active Cedar policy.
- **`sandbox_profile`**: Active containment profile (`bap-broker-standard`, `bap-decision-only`).
- **`result`**:
  - `ALLOWED_EXECUTED`: Action authorized and executed once inside BAP broker sandbox.
  - `DENIED_POLICY`: Action blocked by Cedar authorization policy.
  - `DENIED_TAMPER`: Action blocked by Anti-Tampering security invariant.
  - `EXECUTION_FAILED`: Action authorized, but underlying process returned non-zero exit code.
  - `DECISION_ONLY`: Policy evaluated without execution (audit/check mode).
  - `AUDIT_PERMITTED`: Violation detected but permitted under audit/shadow mode.

