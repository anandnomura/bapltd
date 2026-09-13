# Bounded Authority Plane (BAP): Comprehensive Features & Architecture Guide
## Zero-Standing Privilege (ZSP) & Defense-in-Depth for Autonomous AI Developer Agents

This document is the master architectural reference and executive overview for the **Bounded Authority Plane (BAP)**. It details every subsystem, security invariant, and engineering feature built into the platform, providing an authoritative guide to demonstrate how BAP enforces **Zero-Standing Privilege (ZSP)** over AI coding agents (such as Claude Code, GitHub Copilot CLI, and autonomous worker daemons).

---

## 1. Executive Summary & The Problem Space

### 1.1 The Vulnerability of Modern AI Coding Agents
Autonomous AI agents are given shell access, filesystem inspection tools, and developer credentials. Traditional security controls fail to protect modern engineering environments because:
1. **Static, Standing Credentials**: Developer machines and CI nodes typically store long-lived tokens in environment variables (`AWS_ACCESS_KEY_ID`, `GITHUB_TOKEN`, `.env` files). A prompt-injected or compromised agent can immediately dump and exfiltrate these secrets.
2. **Over-Privileged Shell Execution**: Standard toolkits execute shell commands with the full privileges of the logged-in developer. There is no sub-process boundary preventing an agent from running `curl https://evil.com`, `rm -rf /`, or renaming sensitive files to bypass crude regex filters.
3. **Lack of Cryptographic Identity**: Autonomous agents often run as anonymous local scripts without verifiable workload identities (SPIFFE) or cryptographic attestation of the executing binary.
4. **Latency Bottlenecks in Centralized Proxies**: Routing every shell keystroke through a remote cloud proxy introduces 200–500ms latency, degrading the developer experience and breaking offline workflows.

### 1.2 The BAP Solution: Bounded Authority Plane & Zero-Standing Privilege (ZSP)
BAP decouples **authorization policy** and **cryptographic identity** from **local execution speed**:
- **Zero-Standing Privilege (ZSP)**: No long-lived authority is granted to any agent. All authority is minted dynamically as ephemeral, short-lived On-Behalf-Of (OBO) tokens (15–30m TTL) that are single-use and atomically burned upon execution.
- **Sub-2ms In-Process Enforcement**: `bapedge` (the Local Trusted Daemon / LTD) embeds a pure Go Cedar policy engine directly on the host machine. Every tool call and shell command is evaluated in under 2 milliseconds without blocking the developer.
- **Cryptographic Attestation & SPIFFE Workload Identity**: Every agent must prove its binary integrity via SHA-256 checksum whitelisting and is assigned an official SPIFFE URI (`spiffe://{trust_domain}/app/{app_id}/instance/{instance_id}`).
- **Fail-Secure Offline Resilience**: If the central control plane experiences an outage, edge agents continue executing permitted developer toolchains with zero downtime, while all security invariants remain 100% enforced offline.

---

## 2. Master Feature Matrix

The following table summarizes all capabilities implemented in the BAP architecture:

| # | Feature / Capability | Subsystem | Technical Implementation | Security & ZSP Value |
| :--- | :--- | :--- | :--- | :--- |
| **1** | **Self-Service Pre-Registration** | `bapcontrolplane` | API-driven `POST /api/v1/agents/pre-register` with unauthenticated/token access. | Eliminates manual ticket queues; enables frictionless app self-onboarding. |
| **2** | **Single-Use OTC Token Engine** | `bapcontrolplane` | Mints high-entropy `LTD-OTC-xxxx-xxxx` tokens; atomically burned upon enrollment. | Guarantees registration codes cannot be intercepted and replayed. |
| **3** | **Multi-Instance Fleet Quotas** | `bapcontrolplane` | Mints `BAP-FLEET-xxxx-xxxx` tokens; tracks `max_instances` vs `enrolled_count` under mutex lock. | Controls fleet scaling; prevents unauthorized rogue instances from joining an app fleet. |
| **4** | **SPIFFE Workload Identities** | `bapcontrolplane` | Mints `spiffe://{trust_domain}/app/{app_id}/instance/{instance_id}` URIs. | Zero-trust microservice federation; standards-based workload identity for every agent. |
| **5** | **JWT-SVID Grant Minter** | `bapcontrolplane` | Issues HMAC-SHA256 tokens setting `sub` and `spiffe_id` claims, instance ID, and bounded scopes. | Ephemeral On-Behalf-Of (OBO) authority with strictly enforced expiration (TTL 15–30m). |
| **6** | **Atomic Grant Consumption** | `bapcontrolplane` | Downstream PEP endpoint `/api/v1/grants/consume` validating and marking grants consumed. | Neutralizes token theft and replay attacks; stolen tokens cannot be reused. |
| **7** | **Binary Hash Attestation** | `bapcontrolplane` | SHA-256 verification against pre-registered whitelists with `prod` (strict) vs `dev` (TOFU) profiles. | Prevents rogue binary spoofing, malware injection, or modified agents from accessing authority. |
| **8** | **In-Process Cedar Policy Engine** | `bapedge` | Pure Go embedded Cedar evaluator evaluating `permit` and `forbid` rules in `<2ms`. | Zero latency impact on developer workflow; eliminates remote network round-trips for policy checks. |
| **9** | **Case-Insensitive Token Normalization** | `bapedge` | Automatically lowercases executables, full commands, and arguments before Cedar evaluation. | Completely neutralizes casing evasion attacks (e.g., `CURL`, `CAT .env`, `Move-Item`). |
| **10** | **Evasive Rename & Redirection Defense** | `bapedge` | Cedar forbid rules inspecting file moves (`ren`, `Move-Item`) and output redirection (`> leak.txt`). | Prevents agents from moving secrets to temporary files or piping them to disk. |
| **11** | **Subprocess OS Sandboxing** | `bapedge` | OS primitives: Landlock LSM (Linux) disabling network sockets; Win32 token isolation (Windows). | Defense-in-depth: even if a binary executes, kernel-level sandboxing blocks unauthorized socket calls. |
| **12** | **Fail-Secure Offline Resilience** | `bapedge` | Local policy cache in `~/.ltd/policy/` (`policy.cedar`, `schema.json`, `policy-state.json`). | Developers can code on planes or during server outages; forbid invariants strictly enforced offline. |
| **13** | **Persistent Emergency Lock** | `bapedge` | Persistent `kill_switch: true` in local policy state surviving restarts and network partitions. | Disconnecting a laptop from the network cannot undo or bypass a revoked kill switch. |
| **14** | **Dynamic Central Policy Sync** | `bapcontrolplane` | Version and digest synchronization via `POST /api/v1/policy/sync`. | Security teams can push policy updates or trigger emergency lockdowns across the entire fleet instantly. |
| **15** | **Hierarchical Kill-Switches** | `bapcontrolplane` | Per-instance `/api/v1/agents/revoke` vs Fleet-wide `/api/v1/apps/revoke`. | Granular blast radius: revoke one compromised container or terminate all instances under an app. |
| **16** | **Agent Liveness Heartbeats** | `bapcontrolplane` | Instance heartbeat ping `/api/v1/instances/heartbeat` updating `last_heartbeat_at`. | Active health tracking; dead or stale instances are detected and reclaimed. |
| **17** | **Tamper-Evident SHA-256 Chain** | `bapcontrolplane` | Sequential hash-chain linking: $H_n = \text{SHA256}(H_{n-1} \parallel \text{Event})$. | Cryptographically detects any retroactive modification or deletion of edge audit logs. |
| **18** | **Local Telemetry Logger** | `bapedge` | Non-blocking structured append to `ltd-audit.jsonl` (timestamp, source, command, PID, latency, reason). | Comprehensive local observability for every single tool call and shell execution. |
| **19** | **Claude Code PreToolUse Hook** | `cchook` | Pure Go interceptor parsing Claude Code tool call JSON; evaluates Bash, Read, View, Edit, Write. | Transparent zero-trust enforcement for Claude Code; blocks forbidden tools before process creation. |
| **20** | **Local Ollama Integration** | Workspace | Configures Claude Code with local Ollama (`localhost:11434`, model `claude-3-5-sonnet-20241022`). | Allows testing real AI agent interactions completely offline and privately on a single laptop. |
| **21** | **GitHub Copilot CLI Shim** | `copilot` | Command-line wrapper shims (`copilot-wrap.bat`, `copilot-wrap.ps1`, `copilot-wrap.sh`). | Transparent interception for GitHub Copilot CLI workflows. |
| **22** | **HTTPS / TLS Mutual Transport** | `bapcontrolplane` | Production PKI cert flags (`-tls`, `-tls-cert`, `-tls-key`) + automated self-signed generator (`-tls-auto`). | Encrypts control plane communications; edge verifies trust via `--ca-cert` or `--insecure`. |
| **23** | **Interactive Activity Inspector** | Workspace | Web UI dashboard (`http://localhost:8080/inspector` & `inspector.html`) with live topological visualizer. | Real-time observability; replayers and demonstrates every allow/deny event, grant, and registration. |
| **24** | **Step-by-Step Scenario Replayer** | Inspector UI | Scrubbing playback controls (Play, Pause, Step, Speed $0.5\times$–$5\times$) with 4 pre-loaded scenarios. | Interactive presentation tool to showcase zero-trust enforcement to stakeholders and auditors. |
| **25** | **Automated Master Test Harness** | Workspace | One-click automated verification (`run_all_tests.bat`, `test_control_plane.py`, `demo_activity.py`). | 41/41 passing automated tests guaranteeing zero regressions across all security invariants. |
| **26** | **Real-Time Edge Telemetry Streaming** | `bapedge` | Synchronous non-blocking HTTP streaming via `transmitter.go` to `/api/v1/audit/ingest` (150ms timeout). | Real-time central visibility; edge decisions streamed to control plane without impacting developer speed. |
| **27** | **Self-Test Telemetry Suppression** | `bapedge` | Smart filtering (`BAP_TEST_MODE`, `LTD_TEST`, source/command test detection) suppressing test traffic. | Zero pollution of central audit stores during CI/pytest/test runs, while preserving local verification logs. |
| **28** | **Agent Session Lifecycle Engine** | `bapcontrolplane` | Session APIs (`/api/v1/sessions/start`, `/end`, list) tracking active workloads, PIDs, and allow/deny metrics. | Comprehensive session-level governance; tracks agent lifecycles across Claude Code, Copilot, and CLI workers. |
| **29** | **Live Workload Radar & Presence Detection** | Inspector UI | Real-time presence banner with pulsing beacon, active agent chips, PID tracking, and live toast alerts. | Executive-ready (CIO) visibility: instantly shows who is actively running, what tools they invoke, and blocked threats. |
| **30** | **50K Ingestion Scale & Tiered Storage** | Platform Architecture | High-throughput ingestion benchmarked at 35,620 events/sec; dual-tier edge JSONL + central SQLite/DuckDB/ClickHouse. | Scalable enterprise data architecture; handles intensive agent fleets with sub-second queries and verifiable chains. |

---

## 3. Deep-Dive Architecture Pillars

```mermaid
graph TD
    subgraph Governance ["Central Control Plane (bapcontrolplane)"]
        REG["Agent Registry & Multi-Instance Fleet Quotas"]
        SPIFFE["SPIFFE Workload Authority (JWT-SVID)"]
        ATTEST["SHA-256 Binary Integrity Verifier"]
        GRANT["OBO Ephemeral Grant Minter & Purger"]
        POLICY_SRV["Central Cedar Policy & Schema Store"]
        AUDIT_CHAIN["Tamper-Evident SHA-256 Audit Chain"]
        KILL_SWITCH["Hierarchical Revocation (App vs Instance)"]
        
        REG --- SPIFFE
        REG --- ATTEST
        GRANT --- AUDIT_CHAIN
    end

    subgraph Edge ["Developer Machine / Worker Node (bapedge)"]
        subgraph Agents ["AI Coding Agents"]
            CLAUDE["Claude Code (Ollama / Anthropic)"]
            COPILOT["GitHub Copilot CLI"]
            WORKER["Autonomous Worker"]
        end

        subgraph Interceptors ["Zero-Trust Interceptor Layer"]
            CCHOOK["cchook (PreToolUse Hook)"]
            COPSHIM["copilot-wrap (CLI Shim)"]
        end

        subgraph Daemon ["Local Trusted Daemon (bapedge)"]
            PEP["Policy Enforcement Point (<2ms)"]
            CEDAR["Embedded Cedar Evaluator (Normalized)"]
            CACHE["Local Cache (~/.ltd/policy/)"]
            AUDIT_LOG["Local Audit Log (ltd-audit.jsonl)"]
            SANDBOX["Subprocess Sandbox (Landlock/Win32)"]
        end

        CLAUDE --> CCHOOK
        COPILOT --> COPSHIM
        WORKER --> PEP

        CCHOOK --> PEP
        COPSHIM --> PEP

        PEP --> CEDAR
        CEDAR --> CACHE
        PEP --> AUDIT_LOG
        PEP --> SANDBOX
    end

    subgraph Target ["Downstream Resources"]
        SHELL["Developer Toolchain (git, python, npm, go)"]
        SECRETS["Protected Assets (.env, id_rsa, AWS Keys)"]
        INTERNET["Network Egress (curl, wget, APIs)"]
    end

    SANDBOX -->|Permit Rule Matched| SHELL
    PEP -.->|Forbid Invariant Triggered (1ms)| SECRETS
    PEP -.->|Forbid Invariant Triggered (1ms)| INTERNET

    %% Control Plane Links
    PEP <-->|OTC / Fleet Token Enrollment| REG
    PEP <-->|Ephemeral JWT-SVID Grants| GRANT
    PEP <-->|Dynamic Bundle Sync| POLICY_SRV
    PEP -->|Audit Telemetry Streaming| AUDIT_CHAIN
```

---

### Pillar 1: Agent Registration & SPIFFE Workload Identities
- **The Gap in Current Systems**: AI agents run as unauthenticated scripts. An operator cannot distinguish between a legitimate worker and an impersonating script.
- **BAP Implementation**:
  - App owners pre-register agent definitions via declarative JSON (`POST /api/v1/agents/pre-register`).
  - For single agents: BAP mints an ephemeral, single-use `LTD-OTC-xxxx-xxxx` code.
  - For agent fleets (CI runners, containers): BAP mints a quota-based `BAP-FLEET-xxxx-xxxx` token tracking `max_instances`.
  - Upon enrollment (`POST /api/v1/agents/register`), every instance receives an official **SPIFFE Workload Identifier**:
    $$\text{spiffe://}\{\text{trust\_domain}\}/\text{app}/\{\text{app\_id}\}/\text{instance}/\{\text{instance\_id}\}$$
  - The trust domain defaults to `bap.internal` (customizable via `-trust-domain` flag or `BAP_TRUST_DOMAIN`).

---

### Pillar 2: Cryptographic Binary Attestation & Integrity Envelopes
- **The Gap**: Attackers can tamper with the agent's executable or inject malicious wrappers into the PATH.
- **BAP Implementation**:
  - `bapedge` computes the SHA-256 checksum of its own executing binary on disk.
  - In `production` profiles: The control plane strictly validates this hash against the pre-registered whitelist. Any tampered or unauthorized binary is rejected with `403 Forbidden`.
  - In `development` profiles: Provides Trust-On-First-Use (TOFU) or flexible updates to allow rapid developer iteration while maintaining full auditability.

---

### Pillar 3: Zero-Standing Privilege (ZSP) & Ephemeral Authority Grants
- **The Gap**: AI agents typically possess perpetual root or administrative access. If compromised, the blast radius is catastrophic.
- **BAP Implementation**:
  - Agents possess **zero standing privileges**. No long-lived API tokens or persistent credentials exist on the edge.
  - When an agent needs to execute an authorized operation, it acquires a short-lived On-Behalf-Of (OBO) grant from `bapcontrolplane` (`POST /api/v1/grants/acquire`).
  - The grant is an HMAC-SHA256 JWT-SVID carrying:
    - `sub` & `spiffe_id`: The full SPIFFE URI of the agent instance.
    - `instance_id`: Unique node identifier.
    - `scopes`: Scoped permissions (e.g. `["cli:exec"]`).
    - `exp`: Default TTL of 15 to 30 minutes.
  - **Atomic Consumption**: Downstream services or local brokers call `POST /api/v1/grants/consume` to validate and burn the token. If an attacker intercepts the token from memory or logs, replay attempts return `403 Forbidden`.

---

### Pillar 4: `bapedge` Local Trusted Daemon & Sub-2ms Cedar Engine
- **The Gap**: Remote API gateways introduce 200–500ms latency, making interactive command-line development unbearable.
- **BAP Implementation**:
  - `bapedge` resides directly on the host machine and embeds the Amazon Cedar policy evaluator in pure Go.
  - Cedar evaluates the tuple `(Principal, Action, Resource, Context)` in **under 2 milliseconds**.
  - **Casing Evasion Hardening**: All executables, full command strings, and arguments are normalized to lowercase prior to evaluation, preventing bypasses via `CURL`, `Move-Item .env`, or `CAT .env`.
  - **Forbid Overrides Permit**: Cedar guarantees that a single `forbid` rule takes absolute precedence over any `permit` rule.

---

### Pillar 5: Fail-Secure Offline Resilience & Network Partition State Machine
- **The Gap**: Centralized cloud security tools break when network connectivity is lost (e.g. developers on laptops, offline air-gapped environments).
- **BAP Implementation**:
  - `bapedge` maintains an authenticated local cache in `~/.ltd/policy/` containing `policy.cedar`, `schema.json`, and `policy-state.json`.
  - If `bapcontrolplane` is offline or unreachable:
    1. Standard developer toolchains (`git status`, `python build.py`, `npm test`) evaluate against the local cache in ~1ms with **zero downtime**.
    2. All security forbid invariants (blocking `.env` dumping, egress exfiltration, credential theft) remain **100% active and enforced offline**.
    3. If no cache exists, `bapedge` **fails secure** (denies execution with exit code 1).
    4. Emergency kill-switches persist offline: network disconnection cannot bypass a locked agent.

---

### Pillar 6: Transparent AI Agent Interceptors (Claude Code & Copilot)
- **The Gap**: Forcing developers to rewrite their agent prompts or abandon standard tools creates friction and rejection.
- **BAP Implementation**:
  - **Claude Code**: Uses native `PreToolUse` hooks (`cchook/interceptor.exe`). Intercepts `Bash`, `Read`, `View`, `Edit`, and `Write` calls transparently.
  - **Claude Code + Ollama**: Complete integration with local Ollama models (`claude-3-5-sonnet-20241022:latest` on `localhost:11434`) via `run_claude_ollama.bat` for offline, private AI development.
  - **GitHub Copilot CLI**: Cross-platform wrapper shims (`copilot-wrap.bat`, `copilot-wrap.ps1`, `copilot-wrap.sh`) intercepting command invocations before shell execution.

---

### Pillar 7: Hierarchical Kill-Switches & Liveness Heartbeats
- **The Gap**: When an agent goes rogue, operators must manually terminate processes across hundreds of machines.
- **BAP Implementation**:
  - **Per-Instance Revocation** (`POST /api/v1/agents/revoke`): Immediately isolates a single rogue container/node without affecting sibling instances in the fleet.
  - **Fleet-Wide App Kill-Switch** (`POST /api/v1/apps/revoke`): Instant emergency kill-switch revoking all active instances of an application fleet across the entire infrastructure with a single API call.
  - **Liveness Heartbeats** (`POST /api/v1/instances/heartbeat`): Edge instances maintain active status; dead or disconnected nodes are flagged automatically.

---

### Pillar 8: Tamper-Evident SHA-256 Audit Chain, Clean Handshake & Zero-Burden Pruning
- **The Gap**: Attackers who compromise an edge machine can edit or erase local audit logs to hide their tracks. In traditional setups, logs remain trapped on developer laptops, creating ever-growing storage burdens and security blind spots.
- **BAP Implementation**:
  - **Local Tamper-Evident Hash Chain**: Every execution decision is logged to `ltd-audit.jsonl` with an SHA-256 integrity hash linked to the previous entry ($H_n = \text{SHA256}(H_{n-1} \parallel \text{EventData})$). The `bapedge verify-log` tool verifies the entire local chain; any modified fields, deleted lines, or tampered commands are immediately flagged as tampering.
  - **Clean Handshake Transmission**: `bapedge` automatically streams events via `internal/audit/transmitter.go` to `POST /api/v1/audit/ingest` with a synchronous, non-blocking 150ms HTTP timeout. The server returns a cryptographic receipt: `{"ingested": 1, "chain_valid": true, "receipt_hash": "...", "status": "acknowledged"}`.
  - **Edge Burden Removal**: Upon receiving this verified handshake acknowledgment from the server, `bapedge` immediately prunes the transmitted record from `ltd-audit.jsonl`, truncating the local file to 0 bytes so developer laptops carry zero disk burden! If the server is offline or the handshake is unverified, the entry remains safely stored in the local tamper-evident log as a fail-secure spool.
  - **Self-Test Suppression**: Local test runs (`run_all_tests.bat`, pytest, unit checks) set `BAP_TEST_MODE=1` or are automatically identified by command patterns. Test events are logged locally to `ltd-audit.jsonl` for verification assertions but are suppressed from network transmission, preventing synthetic test pollution on the central server.
  - **Central Cryptographic Hash Chaining**: The central server sequences incoming events into an immutable SHA-256 blockchain:
    $$H_n = \text{SHA256}(H_{n-1} \parallel \text{EventData})$$
  - The verification endpoint (`GET /api/v1/audit/events`) validates the chain from the genesis block; any retroactive log tampering or record deletion immediately breaks verification.

---

### Pillar 9: HTTPS / TLS Transport Security & Auto-Cert Generator
- **The Gap**: Setting up TLS certificates in dev/test environments is tedious and often leads developers to disable encryption.
- **BAP Implementation**:
  - `bapcontrolplane` supports `-tls`, `-tls-cert`, and `-tls-key` for production enterprise PKI.
  - Built-in `-tls-auto` flag: Automatically generates an in-memory or on-disk self-signed ECDSA certificate for `localhost` and `127.0.0.1`, exporting `controlplane-cert.pem`.
  - Edge clients use `--ca-cert` to establish root-of-trust or `--insecure` for local automated testing.

---

### Pillar 10: BAP Activity Inspector & Interactive Live Replayer
- **The Gap**: Zero-trust controls are often invisible black boxes that developers and executives find difficult to understand, debug, or demonstrate.
- **BAP Implementation**:
  - Zero-dependency web UI served directly by `bapcontrolplane` at `http://localhost:8080/inspector` (or standalone `inspector.html`).
  - **Live Workload Radar**: Pulsing status beacon displaying total active agent workloads with real-time presence chips (`Claude Code [PID: ...]`, `Copilot CLI [PID: ...]`). Clicking a presence chip immediately filters the event stream to that specific agent.
  - **Floating Toast Notifications**: Instant, animated visual alerts for newly connected clients, denied tool calls (red shield), and session completions.
  - **Topological Packet Visualizer**: Live animated packet flow showing the journey of every execution request through interceptors, Cedar PEP, sandbox, and central audit chain.
  - **Interactive Playback Scrubber**: Controls for Play/Pause, Step Forward/Back, and scrubbing speeds ($0.5\times$ to $5\times$) with pre-loaded enterprise scenarios and live telemetry streaming.
  - **Turnkey Operation**: Turnkey 1-click launch via `start_inspector.bat`, live demonstration script `demo_live_client.bat`, and shutdown via `stop_inspector.bat`.

---

### Pillar 11: Agent Execution Session Engine & Enterprise Lifecycle Management
- **The Gap**: Micro-operations executed by autonomous agents occur in isolation. Security teams lack cohesive session-level visibility to know when an agent began working, how many tools were attempted, and what proportion were denied.
- **BAP Implementation**:
  - **Session Lifecycle APIs**: `bapcontrolplane` provides dedicated endpoints (`POST /api/v1/sessions/start`, `POST /api/v1/sessions/end`, `GET /api/v1/sessions`, `GET /api/v1/sessions/{id}`).
  - **Turnkey Agent Wrappers**: `run_claude_ollama.bat` and agent interceptors generate a unique `BAP_SESSION_ID`, register session start with process metadata (PID, source, model), pass the session ID through all tool executions, and formally close the session on shutdown.
  - **Metric Aggregation**: Every ingested audit event is linked to its active session, automatically tracking total events, allow counts, deny counts, and duration.
  - **Session Inspection**: The Inspector UI includes a dedicated **Agent Sessions** tab displaying session cards with allow/deny progress bars, duration, and one-click "Inspect Stream ->" filtering.

---

### Pillar 12: High-Scale Ingestion (50K Event PT) & Dual-Tier Storage Strategy
- **The Gap**: Enterprise fleets with hundreds of AI agents generate massive streams of command telemetry. Central logging services can quickly bottleneck, while choosing the wrong database leads to excessive maintenance overhead.
- **BAP Implementation**:
  - **50,000-Event Performance Test**: Validated with `tests/perf_test_50k.py`, achieving **35,620 events/second** ingestion throughput (1.40s total ingestion time in 1,000-event batches with 60.5ms average batch latency) and full SHA-256 chain verification in 32.6ms.
  - **Dual-Tier Storage Strategy**:
    1. **Edge Tier (LTD)**: Zero-dependency, append-only JSON Lines (`ltd-audit.jsonl`). Guarantees sub-millisecond local writes, zero database drivers required on edge developer laptops, and local survivability across power cuts.
    2. **Central Tier (Control Plane)**:
       - *Local / Single-Node / Appliance*: Embedded **SQLite in WAL (Write-Ahead Logging) mode** or **DuckDB**. Zero administration, ACID transactions, sub-millisecond session/event queries, and easily backupable as a single file.
       - *Cloud Enterprise / Fleet Deployment*: **ClickHouse** or **TimescaleDB** for multi-terabyte log retention, columnar compression (10:1 ratio), and multi-tenant analytics across thousands of developer machines.

---

## 4. End-to-End Threat & Evasion Defense Matrix

| Threat ID | Threat Vector | Malicious Agent Action | BAP Defense-in-Depth Mitigation |
| :--- | :--- | :--- | :--- |
| **E1** | **Secret Content Dumping** | Agent runs `cat .env` or `type .env`. | Cedar forbid rule inspects `*.env*`. Intercepted in $1\text{ms}$; process never spawns. |
| **E2** | **Casing Evasion Attack** | Agent runs `CURL https://evil.com` or `CAT .env`. | `bapedge` normalizes all tokens to lowercase before Cedar evaluation. Strictly blocked. |
| **E3** | **Evasive File Renaming** | Agent runs `ren .env backup.txt` or `Move-Item .env`. | Cedar forbid rules inspect wildcard renames and moves. Execution denied. |
| **E4** | **Output Redirection Leak** | Agent runs `ls -la .env > leak.txt`. | Arguments containing `.env` and redirection operators are matched by Cedar forbid rules. |
| **E5** | **Network Data Exfiltration** | Agent runs `curl`, `wget`, `nc`, `ncat`, `socat`. | Egress binaries are forbidden at Cedar layer; Landlock sandbox disables network sockets entirely. |
| **E6** | **Rogue Agent Impersonation** | Attacker runs an unapproved binary claiming to be a worker. | Control plane requires pre-registration and binary attestation. Hash mismatch returns `403`. |
| **E7** | **Fleet Quota Overflow** | Attacker spins up 50 rogue containers using an app ID. | Fleet tokens enforce strict instance quotas (`max_instances`). Excess enrollments return `401`. |
| **E8** | **Token Theft & Replay** | Attacker intercepts an authority grant JWT from logs. | Downstream services call `/api/v1/grants/consume` to burn the token. Replayed tokens return `403`. |
| **E9** | **Network Outage Exploit** | Attacker severs connection to server hoping edge fails open. | Edge defaults to fail-secure. If cached, prior invariants are enforced offline; otherwise denied. |
| **E10** | **Audit Log Tampering** | Attacker deletes or edits lines in local audit logs. | Central audit logs are linked into a sequential SHA-256 chain. Tampering breaks the hash chain. |
| **E11** | **Telemetry Flooding / Test Poisoning** | Synthetic test suites flood central server with fake audit records. | `bapedge` filters test-mode events (`BAP_TEST_MODE=1`, test patterns) from network transmission; zero server pollution. |
| **E12** | **Ghost / Unmonitored Sessions** | Rogue or zombie agent processes execute commands without supervision. | Session Lifecycle Engine enforces session start/end tracking and displays live client badges on the Radar. |

---

## 5. How to Explain This Architecture to Stakeholders

When presenting this architecture to engineering leadership, security teams, or clients, use these structured talking points:

### The 30-Second Elevator Pitch
> *"BAP (Bounded Authority Plane) is an enterprise zero-trust execution broker for AI coding agents. Instead of giving Claude Code or Copilot standing administrative privileges or unrestricted shell access, BAP enforces Zero-Standing Privilege: every agent has an attested cryptographic identity (SPIFFE), executes with short-lived ephemeral grants, and every shell command is intercepted and evaluated locally in under 2 milliseconds by an embedded Cedar policy engine. It stops secret exfiltration and rogue actions before a process ever spawns, runs 100% offline during network outages, streams real-time telemetry to the control plane, and provides executive-ready observability through the Live Workload Radar and Activity Inspector."*

### Key Strategic Value Propositions
1. **Zero-Standing Privilege (ZSP) for AI**: Eliminates perpetual root/admin keys. Authority is ephemeral, bounded, and atomically burned upon execution.
2. **Sub-2ms Zero-Friction Developer Experience**: Developers do not experience lag. Toolchain commands (`git`, `python`, `go`, `npm`) execute instantly on the edge.
3. **Fail-Secure Offline Independence**: Operates with complete autonomy on laptops and air-gapped environments without failing open.
4. **Defense-in-Depth**: Multiple redundant layers: PreToolUse interceptors $\rightarrow$ Cedar authorization $\rightarrow$ Token burning $\rightarrow$ OS sandboxing $\rightarrow$ Tamper-evident audit chain.
5. **Live Fleet Radar & CIO Observability**: The BAP Inspector features a real-time presence radar, active agent badges, live toasts, and complete session metrics to give leadership instant, undeniable visual proof of zero-trust control.
6. **Enterprise High-Throughput Scale**: Benchmarked at 35,620 events/second with verified SHA-256 blockchain integrity and clean separation of synthetic test data from production telemetry.
7. **Human-to-Workload Dual Identity Binding**: Binds the human operator identity to the agent SPIFFE workload ID, providing unambiguous attribution of which human triggered what autonomous action without disturbing upstream LLM auth.

---

## 6. Dual Identity Binding & 5-Stage Identity Resolution Pipeline

BAP enforces **Dual-Identity Attribution**: every execution event binds the **Human Operator** (`user_id`, `user_email`) to the **Autonomous Workload** (`spiffe_id`).

### Stage 1: `apiKeyHelper` / Corporate User ID Token (Primary)
Claude Code and corporate development environments frequently employ `apiKeyHelper` (configured in `~/.claude.json`, `~/.claude/settings.json`, or environment variables) to fetch short-lived corporate SSO/OIDC tokens.
- `bapedge` inspects `apiKeyHelper` or `BAP_ID_TOKEN` / `CLAUDE_ID_TOKEN`.
- If a JWT is returned, `bapedge` extracts claims (`email`, `upn`, `preferred_username`, `sub`) to identify the human user.
- **Zero Disturbance**: `bapedge` never overwrites, intercepts, or disrupts Claude Code's upstream LLM authentication flow.

### Fallback Stages (For Personal Laptops & Unconfigured Environments):
When running on an unmanaged personal laptop or sandbox without `apiKeyHelper`:
- **Stage 2**: System Environment Variables (`BAP_USER_EMAIL`, `BAP_USER_ID`, `USERNAME`, `USER`, `LOGNAME`).
- **Stage 3**: Cross-Platform Native Standard Library (`os/user.Current()`).
- **Stage 4**: Filesystem User Home Directory Basename (`filepath.Base(os.UserHomeDir())`).
- **Stage 5 (Worst-Case Fallback)**: Gracefully degrades to `"NA"` — ensuring the system **never panics, crashes, or blocks workflows**.

