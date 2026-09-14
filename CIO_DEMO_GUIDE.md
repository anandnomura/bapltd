# BAP Executive Demonstration Guide for the CIO

> **Goal**: Deliver a stunning, high-impact presentation showing how the **Bounded Authority Plane (BAP)** enables safe enterprise adoption of autonomous AI coding agents (Claude Code, GitHub Copilot) with Zero-Standing Privilege, sub-2ms developer velocity, and real-time observability.

---

## 1. Demonstration Modes

BAP provides two tailored demonstration experiences:

### Option A: The Executive CIO Demo (3-Act Visual Presentation)
Designed for C-level leadership (CIO, CISO, VP of Engineering). Zero technical jargon, punchy, 1-click interactive validation directly from the browser cockpit:
```powershell
.\cio_demo.bat
```
- **Act 1 ("Fleet Visibility & Scale")**: Spins up 5 live Claude Code workloads with SPIFFE dual-identity. One-click toggle scales up to 25 enterprise nodes.
- **Act 2 ("Velocity vs Perimeter Defense")**: 1-click test of complex safe build pipeline (<2ms pass) vs rogue credential exfiltration (blocked by Gateway PEP).
- **Act 3 ("The Boardroom Cockpit")**: Global CISO Emergency Kill-Switch and live SHA-256 Merkle audit chain verification.

### Option B: The Technical Architecture Deep Dive
Designed for lead architects, security engineers, and developer tools teams who want to step through raw CLI shims, Cedar forbid rules, and process injection defenses:
```powershell
.\tech_demo.bat
```

---

## 2. The 30-Second Opening Elevator Pitch

When you begin the meeting with your CIO, set the stage with this opening statement:

> *"Across the industry, engineering teams are rapidly adopting autonomous AI coding agents like Claude Code and GitHub Copilot. While these tools accelerate delivery, they introduce a critical enterprise security risk: they have terminal and shell access, meaning an autonomous agent can inadvertently dump production `.env` credentials, access private SSH keys, or exfiltrate IP.*
>
> *Today, I want to show you **Bounded Authority Plane (BAP)**. BAP is our zero-trust execution broker that sits between the AI agent and the host machine. Every agent has an attested cryptographic identity (SPIFFE), every developer identity is bound via corporate SSO/apiKeyHelper, and every command is evaluated locally in under 2 milliseconds by an embedded Cedar policy engine.*
>
> *Let me show you live how it stops attacks before a process ever spawns, with zero lag for developers."*

---

## 3. Step-by-Step Presentation Script

| Step | Topic | What You Say to the CIO | What You Do in Terminal | What the CIO Sees on the Browser |
| :---: | :--- | :--- | :--- | :--- |
| **1** | **Live Workload Enrollment & Identity** | *"First, let's see how BAP solves agent shadow-IT. When Claude Code starts, BAP binds the developer's corporate identity to the agent's cryptographic SPIFFE identity."* | Press `[Enter]` on **Step 1** | **Live Radar** pulses green (`1 ACTIVE AGENT`). A floating toast appears: `⚡ NEW WORKLOAD ENROLLED (LIVE)`. Claude Code chip appears on the radar ribbon. |
| **2** | **Zero-Friction Developer Velocity** | *"Security cannot slow down engineering. Safe toolchain commands like `git status` or `python` must execute at native speed. Watch the latency."* | Press `[Enter]` on **Step 2** | The Zero-Trust Pipeline lights up green: `✓ ZERO-TRUST VERIFIED: Permitted Execution Active (1ms)`. Allowed counter increments. |
| **3** | **Critical Secret Disclosure Blocked** | *"Now, the nightmare scenario: Claude Code is prompted to inspect configuration and attempts to dump the `.env` file with production API keys. Watch what happens."* | Press `[Enter]` on **Step 3** | Pipeline flashes bright **RED** shield: `⛔ ZERO-TRUST SHIELD: Pre-Process Interception Denied (1ms)`. Cedar forbid rule displayed. **Zero shell spawned.** |
| **4** | **Evasive Attack Bypasses Thwarted** | *"Naive filters look for `cat .env`. What if the agent tries to evade detection by renaming the file first with `ren .env notes.txt`?"* | Press `[Enter]` on **Step 4** | Evasive rename is caught and blocked immediately. Denied counter increments. |
| **5** | **Data Loss Prevention (Egress)** | *"What if an agent tries to exfiltrate proprietary source code to an external server via `curl`?"* | Press `[Enter]` on **Step 5** | Egress blocked cold at the process boundary. |
| **6** | **Cryptographic Anti-Tamper Audit** | *"Can a rogue actor or malicious script modify or delete local logs on the laptop? Every event is chained with SHA-256. Let's verify."* | Press `[Enter]` on **Step 6** | Console runs `bapedge verify-log` and verifies the sequential hash chain: `[+] HASH CHAIN INTACT`. |
| **7** | **Autonomous Python Agent (SDK Pattern)** | *"BAP is not limited to Claude Code. ANY internal Python agent -- built with LangChain, CrewAI, AutoGen, or custom LLM loops -- integrates in 3 lines using our Python SDK."* | Press `[Enter]` on **Step 7** | Watch the **AMBER** `Python Agent` chip pulse on the Live Radar. Toast appears: `⚡ NEW WORKLOAD ENROLLED`. Both ALLOW and DENY counters increment as its attacks (`cat .env`, `curl`) are intercepted! |
| **8** | **The Rogue Agent Defense (Gateway PEP)** | *"What if an autonomous rogue script ignores the BAP SDK and opens raw network sockets directly to our core banking APIs? BAP enforces a Dual-PEP model: edge speed + non-bypassable Gateway PEP at the perimeter."* | Press `[Enter]` on **Step 8** | Watch `rogue_agent.py` get dropped at the perimeter (`401 Unauthorized`), while `governed_agent.py` presents its BAP Grant and retrieves protected financial records (`200 OK`). |
| **9** | **Real Claude Code & Session Teardown** | *"BAP runs seamlessly across environments. In the office, it detects `claude-code.cmd`. On a personal laptop, it detects `claude`. Now let's conclude with graceful teardown and burden pruning."* | Select `1` (automated prompt), `2` (interactive terminal), or `3` (skip) | Live Claude Code executes if selected; browser displays `○ WORKLOAD TERMINATED` toast; session transitions to `CLOSED`. |

---

## 4. Tough CIO Questions & Winning Answers

### Q1: *"Will this slow down my developers?"*
> **Answer**: *"No. BAP evaluates Cedar policies locally in memory on the edge node. Execution latency is under 2 milliseconds — faster than human perception. Developers experience zero lag when running builds, tests, or git commands."*

### Q2: *"What happens when developers are on a plane or the network drops?"*
> **Answer**: *"BAP is 100% fail-secure and offline-capable. Invariants and policies are cached cryptographically on the device. If the central control plane is unreachable, the agent continues working safely without failing open."*

### Q3: *"Can we govern our own internal Python AI agents (LangChain, CrewAI, AutoGen)?"*
> **Answer**: *"Yes. We provide a lightweight, zero-dependency Python SDK (`bap-sdk`). Any Python agent wraps tool execution with `with BAPSession(...) as bap: bap.exec(...)`. It automatically acquires ephemeral SPIFFE credentials, enforces Cedar invariants, and streams telemetry to the control plane in real time."*

### Q4: *"Does this require disturbing our corporate SSO or Claude Code's `apiKeyHelper`?"*
> **Answer**: *"Not at all. Corporate `apiKeyHelper` operates strictly upstream between the developer and Anthropic/LLM gateway. BAP operates strictly downstream at the operating system execution boundary. BAP Stage 1 natively parses the corporate ID token to bind user identity without touching or disturbing the authentication flow."*

### Q5: *"Can a developer or compromised process tamper with the local audit trail?"*
> **Answer**: *"No. Local logs are protected by a sequential SHA-256 cryptographic hash-chain ($H_n = \text{SHA256}(H_{n-1} \parallel \text{EventData})$). Modifying or deleting a line immediately breaks the chain and triggers a tamper alarm on `bapedge verify-log`."*

### Q6: *"Can we see fleet-wide activity across 1,000 developers?"*
> **Answer**: *"Yes. Edge nodes stream structured telemetry to the BAP Control Plane. We tested central log aggregation at **35,620 events/second** under a 50,000-event stress test with 100% hash-chain validity."*

### Q7: *"What stops a rogue or malicious agent from just bypassing the Python SDK and writing raw `requests.post()` socket calls directly to our microservices?"*
> **Answer**: *"That is the exact reason enterprise security requires our **Dual-PEP Architecture**. Client-side SDKs provide cooperative speed on developer machines, but all internal microservices and core banking databases are placed behind an **API Gateway Policy Enforcement Point (Envoy Proxy, Istio, or BAP Gateway)**. The Gateway demands a cryptographically signed BAP Grant (JWT-SVID) on every request via `ext_authz`. If a rogue agent attempts direct socket access without going through BAP, the Gateway terminates the connection with `401 Unauthorized`. The internal database is never contacted."*

### Q8: *"Deploying new custom executables across corporate fleets takes 2 months of Infosec review. How do we deploy this in production?"*
> **Answer**: *"We architected two deployment options specifically to address enterprise deployment friction:
> - **Option 1 (Production Enterprise Cloud & Linux)**: Uses **Envoy Proxy on Podman / Docker / Kubernetes**. The `envoyproxy/envoy` container image is an open-source CNCF standard already vetted and pre-approved in corporate container registries. It uses standard `envoy.filters.http.ext_authz` filters with zero custom binaries required on the network perimeter.
> - **Option 3 (Instant Desktop Demo & Edge Nodes)**: For developer workstations, edge brokers, and executive demos, we provide a zero-dependency pure-Go native PEP (`bapgateway.exe`) that starts in under 5ms without requiring containers, hypervisors, or complex virtualization setup. Both options share identical authorization semantics and BAP Grant consumption protocols."*



