# 🛡️ Bounded Authority Plane (BAP)
### *Zero-Trust Security Posture (ZSP) for Autonomous AI Agents*

> **Open-Source AI Agent Sandboxing, Policy Enforcement, and Cryptographic Governance.**  
> *Empowering autonomous agents like Claude Code and GitHub Copilot to build software boldly — without giving them the keys to the kingdom.*

---

## 💡 The AI Developer Dilemma

We are witnessing an unprecedented shift in software engineering: **autonomous AI agents are no longer just generating text snippets; they are running terminal commands, editing production files, and interacting with network APIs.**

Tools like Claude Code, GitHub Copilot CLI, and agentic workflows operate directly on developer workstations. But this breakthrough velocity introduces a terrifying new attack surface:
- **Prompt Injections & Jailbreaks**: Malicious inputs disguised in code comments or web pages can hijack an agent to steal `~/.aws/credentials` or `.env` secrets.
- **Accidental Mass Destruction**: Hallucinations can lead to catastrophic deletions (`rmdir /s /q C:\` or `Remove-Item -Recurse`).
- **Data Exfiltration**: Rogue or compromised agents can open reverse TCP sockets or exfiltrate private source code via unauthorized network calls.

Until now, security teams faced an impossible tradeoff: **either cripple the AI agent's autonomy with rigid permission prompts on every command, or grant it unchecked access to developer machines.**

---

## 🎯 What is BAP?

**Bounded Authority Plane (BAP)** is an open-source, dual-layer governance kernel built from the ground up with a **Zero-trust Security Posture (ZSP)** at its heart.

BAP establishes a cryptographic boundary around AI agents. It acts as an intelligent, transparent gatekeeper that evaluates every shell invocation, file edit, and network call against declarative zero-trust policies in **less than 1.5 milliseconds** — completely offline, with zero disruption to developer flow.

### The ZSP Philosophy (Zero-Trust Security Posture)
1. **Never Trust, Always Verify**: Every tool invocation requested by an agent is intercepted and evaluated before process creation.
2. **Least Privilege by Default**: Agents only have authority within the active workspace root. Navigating outside is strictly forbidden.
3. **Fail-Secure Invariant**: If central servers go down or network partitions occur, BAP never fails open. Local cached policies continue protecting the host with 100% offline resilience.
4. **Transparent Governance**: Agents require **zero code modifications**. Standard developer toolchains (`git`, `python`, `npm`, `go`, `cargo`) work at native speed.

---

## 🏗️ Architecture: The Dual-PEP Model

BAP divides governance into an ultra-low latency **Client-Side Edge Broker** and a centralized **Governance Control Plane**:

```mermaid
graph TD
    subgraph["Central_Infrastructure - Central Control Plane (Port 8443 / 8444)"]
        CP["bapcontrolplane (HTTPS REST API)"]
        DASH["bapdashboard (React Web UI)"]
        CHAIN["Tamper-Evident SHA-256 Audit Chain"]
        CEDAR_MASTER["Authoritative Policy Store"]
        
        CP --- DASH
        CP --- CHAIN
        CP --- CEDAR_MASTER
    end

    subgraph Developer_Seat ["Developer Seat (Local Workstation)"]
        subgraph Agents ["AI Agent Runtimes"]
            CLAUDE["Claude Code CLI"]
            COPILOT["GitHub Copilot CLI"]
            MCP_IDE["IDE (Cursor / VSCode via MCP)"]
        end

        subgraph Interceptors ["PEP Interceptors"]
            HOOK["interceptor (PreToolUse)"]
            COPSHIM["copilot-interceptor"]
        end

        subgraph BAP_Edge ["bapedge — Local Trusted Daemon"]
            ENGINE["In-Process Cedar Engine (<1.5ms)"]
            CACHE["Local Policy Cache (policy.cedar)"]
            LOCAL_AUDIT[".bap/ & ltd-audit.jsonl"]
        end

        CLAUDE -->|PreToolUse JSON| HOOK
        COPILOT -->|CLI Intercept| COPSHIM
        MCP_IDE -->|JSON-RPC stdio| BAP_Edge

        HOOK --> BAP_Edge
        COPSHIM --> BAP_Edge
        BAP_Edge --> ENGINE
        ENGINE --> CACHE
        BAP_Edge --> LOCAL_AUDIT
    end

    BAP_Edge -.->|Async Telemetry Stream (150ms)| CP
    BAP_Edge -.->|Remote Policy Sync & Kill-Switch| CP
```

### Core Components
- **`bapedge`**: The local trusted gatekeeper. Powered by an embedded pure-Go AWS Cedar evaluation engine, it evaluates rules in memory in sub-2ms.
- **`interceptor`**: Seamlessly integrates with Claude Code's native `PreToolUse` lifecycle hook without changing a single line of Claude's source code.
- **`copilot-interceptor`**: Wraps GitHub Copilot CLI executions to prevent defense evasion and credential access.
- **`bapcontrolplane`**: Central API server managing agent registration, binary attestation, dynamic Cedar policy distribution, and fleet-wide emergency kill-switches.
- **`bapdashboard`**: Real-time visual activity radar (`inspector_v2.html` on 8443 and React UI on 8444) rendering agent actions, allow/deny ratios, and tamper-evident logs.

---

## ✨ Key Features

| Capability | How It Protects You |
|---|---|
| **AWS Cedar Policy Kernel** | Author human-readable, mathematically verifiable policies (e.g., allow `git` and `npm`, forbid access to `~/.aws`, `.env`, and raw sockets). |
| **Workspace Sandboxing** | Confines agent file operations strictly to the project directory. Directory traversals (`dir ..`, path escapes) are blocked automatically. |
| **Sub-2ms Decision Latency** | Evaluates policies in-process. Developers never feel lag or input stalls while pairing with AI. |
| **Offline-First Zero-Trust** | Network down? Airplane mode? `bapedge` enforces local cached policies without skipping a beat. |
| **Progressive Enterprise Rollout** | Start in **Audit/Shadow Mode** to observe and baseline agent actions with zero disruption, then flip to **Enforce Mode** for strict Zero-Trust hard blocking. |
| **Multi-Instance Concurrency** | Run multiple Claude Code terminals across different repos simultaneously with zero lockouts or variable collisions. |
| **Self-Healing Session Guard** | A detached watchdog monitors Claude Code PIDs; if your terminal abruptly closes or crashes, your original environment settings are restored instantly. |
| **Tamper-Evident SHA-256 Audits** | Telemetry logs are cryptographically chained ($H_n = \text{SHA256}(H_{n-1} \parallel \text{Event})$). Any retrospective log tampering is immediately detected. |
| **Emergency Kill-Switch** | Revoke a compromised agent session or lock down the entire fleet with a single API call from security operations. |

---

## ⚡ Quickstart: Up and Running in 60 Seconds

### 1. One-Click Developer Installation (Windows)
From the repository root or release package:
```batch
install_bap_client.bat
```
*This deploys canonical client binaries to `%USERPROFILE%\bin` and automatically configures your User `PATH`.*

### 2. Pre-Flight Health Check
Verify your environment, binaries, Claude Code CLI, and Cedar engine:
```batch
bap_doctor.bat
```
*Returns a clean 5-domain diagnostic report confirming your workstation is ready.*

### 3. Launch Governed Claude Code
Launch Claude Code under BAP Zero-Trust protection:
```batch
run_claude_bap.bat
```
*All agent bash executions, file reads, and tool calls are now transparently governed!*

---

## 🛡️ Cedar Policies in Action

BAP uses declarative AWS Cedar policies. Here is how simple it is to enforce zero-trust rules:

```cedar
// 1. Allow standard developer toolchains within the workspace
permit (
    principal,
    action in [Action::"exec", Action::"tool_use"],
    resource
)
when {
    context.executable in ["git", "python", "go", "npm", "node", "mvn", "cargo"]
};

// 2. FORBID access to sensitive credentials and environment files
forbid (
    principal,
    action,
    resource
)
when {
    context.full_command like "*.env*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.ssh*"
};

// 3. FORBID unconstrained network egress utilities
forbid (
    principal,
    action,
    resource
)
when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "*tcpclient*"
};
```

When an agent attempts a forbidden action (like reading `.env`), BAP halts execution and returns intelligent remediation guidance:
```text
[DENIED] Denial triggered by: policy policy2
[SUGGESTION] Direct access or tampering with .env credential files is strictly prohibited. Access required configuration via sandboxed environment variables or the corporate Secret Store.
```

---

## 📊 Live Observability & Dashboards

BAP provides dual visibility cockpits for developers and security teams:

- **Activity Inspector Cockpit** (`https://localhost:8443/inspector_v2.html`): Zero-dependency single-file HUD with a real-time agent presence radar, allow/deny ratios, live telemetry logs, and instantaneous session kill-switches.
- **Enterprise React Dashboard** (`https://localhost:8444/dashboard/`): Modern administrative interface for fleet-wide policy management, binary attestation tracking, and cryptographic audit chain verification.

To start the Control Plane with auto-restart supervision:
- **Windows**: `start_controlplane_supervisor.bat`
- **Linux**: `./scripts/supervise_controlplane.sh start`

---

## 🧪 Headless Automated Resiliency Suite

Verify system integrity, auto-restart capabilities, concurrency scaling, and offline fail-secure boundaries headlessly:

```batch
run_resiliency_test.bat
```
*(Optionally pass `-Json` for machine-readable CI/CD pipeline assertions.)*

---

## 🗺️ Project Documentation

- 📖 **[Central MVP Deployment Guide](MVP_DEPLOYMENT.md)** — Step-by-step enterprise deployment, seat onboarding, and operator manual.
- 📋 **[JIRA Product Backlog & Stories](JIRA_STORIES.md)** — Comprehensive user stories, acceptance criteria, and future enhancement roadmap.
- 🏛️ **[System Architecture Reference](ARCHITECTURE.md)** — In-depth architectural design, trust boundaries, identity models, and storage engine.
- 📡 **[API Specification](API_GUIDE.md)** — Complete OpenAPI/REST contract for Control Plane and Edge CLI interfaces.
- 🗺️ **[Project Navigation Map](PROJECT_MAP.md)** — Index of all repository modules, directories, and assets.

---

## 🤝 Join the Open-Source Movement

Autonomous AI agents are transforming engineering velocity. But without foundational security, enterprise adoption will stall under risk and fear.

**BAP is built to make autonomous agents safe, trustworthy, and auditable.**

We welcome contributors, security researchers, and AI toolchain builders! Feel free to open issues, submit pull requests, or start a discussion.

*Licensed under the Apache License 2.0. Built with security, speed, and developer joy at heart.*

