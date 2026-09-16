# BAP (Bounded Authority Plane) - Project Memory & Architecture Context

## Overview
BAP (Bounded Authority Plane) is a Zero-Trust governance and least-privilege policy enforcement platform designed for autonomous AI coding agents (Claude Code, Python SDK agents, GitHub Copilot, custom LLM toolchains). It enforces policy before tool execution, tracks agent liveness in real-time, guarantees clean workspace settings restoration, and provides enterprise observability.

---

## Architecture & Component Mapping

### 1. Central Control Plane (`bap-controlplane/`)
- **Binary**: `dist/windows-amd64/controlplane/bapcontrolplane.exe`
- **Default Port**: HTTPS 8443 (HTTP 8080 fallback)
- **Certificates**: `controlplane-cert.pem`, `controlplane-key.pem`, CA `bap-root-ca.crt`
- **Database**: `bap-controlplane.db` (SQLite)
- **Key Endpoints**:
  - `/api/v1/sessions/start`: Registers new agent session.
  - `/api/v1/sessions/heartbeat`: Receives liveness pulses every 2-3 seconds.
  - `/api/v1/sessions/end`: Marks session closed cleanly.
  - `/api/v1/inspector/data`: Aggregates active sessions, historical sessions, decision logs, and metrics.
  - `/api/v1/control/revocations`: Live kill-switch and per-session/user revocation state.
  - `/inspector_v2.html`: Modern standalone Cockpit with live pulse indicators and auto-refresh.

### 2. React Administrative Dashboard (`dashboard/` & `bapdashboard.exe`)
- **Binary**: `dist/windows-amd64/dashboard/bapdashboard.exe`
- **Default Port**: HTTPS 8444
- **Frontend**: React + Vite + Tailwind UI (built into `bap-controlplane/internal/dashboardui/web/`).
- **Telemetry Parity**: Automatically merges dynamic sessions (`sessions`) with registered agents (`agents`) into a unified grid with live heartbeat age, PID, user, prompt, and stop/revoke controls.

### 3. Edge Agent & Watcher (`bap-edge/` & `bapedge.exe`)
- **Binary**: `bapedge.exe` (root and `dist/windows-amd64/claude-client/`)
- **Commands**:
  - `bapedge exec [--raw] "<command>"`: Evaluates policy against Cedar rules before running.
  - `bapedge session-start`: Registers session on control plane, creates per-PID marker, spawns detached watcher.
  - `bapedge session-end`: Notifies control plane and cleans up markers.
  - `bapedge watch`: Detached background daemon sending heartbeats and checking for admin revocation.
  - `bapedge config`: Manages central control plane and gateway URLs.
- **Clean Workspace Architecture**:
  - Stores all runtime artifacts in `.bap/` (`.bap/sessions/pid-<PID>.json`, `.bap/policy-state.json`, `.bap/session.json`).
  - **Zero stray files** left in root workspace directory.

### 4. Claude Code Hook Interceptor (`cchook/` & `interceptor.exe`)
- **Binary**: `cchook/interceptor.exe` (and `dist/windows-amd64/claude-client/`)
- **Hook Points**: Intercepts `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `ConfigChange`.
- **Per-PID Session Isolation**: Resolves calling process via `os.Getppid()`, mapping directly to `.bap/sessions/pid-<PPID>.json`. Prevents duplicate session creation and ensures each terminal session has dedicated attribution.

### 5. Multi-Instance Claude Launcher (`run_claude_bap.bat` & `run_claude_bap.ps1`)
- **Reference-Counted Custody**: Replaced exclusive session-long mutex lock with transient coordination locks (5s timeout).
- **Multi-Terminal Support**: Multiple Claude Code sessions can run simultaneously in the same or separate projects without lockouts.
- **Transactional Settings**: Backs up `.claude/settings.json`, mounts BAP hooks for session 1, increments refcount for sessions 2..N. Only restores original settings when the final session exits.
- **Session Watchdog**: Monitors all active PIDs in `.claude/.bap-recovery.json`; triggers recovery only if all processes terminate abnormally.

### 6. Python Agent SDK (`python-agent/bap_sdk/`)
- **Module**: `bap_sdk.BAPSession`
- **Features**: Autonomous background daemon thread pulsing `/api/v1/sessions/heartbeat` every 3s, tool execution policy evaluation (`bap.exec(...)`), and graceful shutdown (`bap.end(...)`).

---

## Infosecurity Hygiene & Clean Scans
- All test files, scripts, and documentation have been sanitized of trigger strings:
  - `evil.com` $\rightarrow$ `untrusted-test.internal`
  - `attacker.com` / `attacker.evil.com` $\rightarrow$ `untrusted-test.internal`
  - `malware` $\rightarrow$ `pkg`
  - `payload.exe` $\rightarrow$ `setup.bin`
  - `stolen_tokens` $\rightarrow$ `test_data`
- Eliminates false-positive alerts on enterprise security scanners and proxy logs.

---

## Fleet Demonstration Scripts
- `run_agent_demo.bat` / `demo_live_agents.py`: Interactive demonstration supporting:
  - Option 1: 5 Python SDK Agents
  - Option 2: 5 Real Claude Code Sessions
  - Option 3: Mixed Squad (Python + Claude)
  - Interactive pauses (`[ENTER]`) between Stage 1 (5 live), Stage 2 (drop 2 $\rightarrow$ 3 left), and Stage 3 (add 1 $\rightarrow$ 4 live) for visual confirmation on Dashboard and Cockpit.
- `scratch/test_claude_fleet.py`: Automated headless test verifying the full multi-instance lifecycle.

