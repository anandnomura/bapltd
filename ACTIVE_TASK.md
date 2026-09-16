# Active Task: Claude Concurrency, Infosec Sanitization & Build Stabilization

## Sprint Goal
- Enable developers to launch multiple concurrent Claude Code sessions in the same directory and across different directories without mutex locks or crashes.
- Prevent corporate InfoSec alerts by eliminating all false-positive trigger strings (`evil.com`, `malware`, `payload.exe`) from default test runs.
- Isolate all runtime artifacts into `.bap/` instead of polluting workspace roots.
- Ensure end-to-end multi-platform build and automated test suites pass 100%.

---

## Completed & Fixed

1. **Purged Corporate InfoSec False Positives**:
   - Sanitized 20 files across `.py`, `.go`, `.ps1`, `.bat`, `.sh`, `.md`, `.html`.
   - Replaced trigger domains/names with non-threatening RFC-compliant internal identifiers (`untrusted-test.internal`, `pkg`, `setup.bin`).
   - Zero occurrences of `evil.com` remain in repository.

2. **Isolated Negative/Deny Security Tests**:
   - Excluded negative tests from `run_all_tests.bat`, `test_complex_cases.ps1`, `simple_local_test.bat`, and `test_financial_reconciler.py`.
   - Created dedicated `run_negative_testcases.bat` for manual, on-demand compliance verification (19/19 deny checks pass).

3. **Workspace Root Cleanliness**:
   - Runtime configuration and session markers moved into `.bap/` (`.bap/sessions/`, `.bap/session.json`, `.bap/bap-config.json`).
   - Recompiled `bapedge.exe` and `cchook/interceptor.exe`.

4. **Multi-Instance Claude Code Concurrency**:
   - Replaced single-instance workspace mutex in `run_claude_bap.ps1` and `run_claude_bap.bat` with reference-counted session tracking under `.claude/.bap-recovery.json`.
   - First instance takes custody of `.claude/settings.json` and starts watchdog; subsequent instances join active session count without settings contention.
   - Original settings restored only when the final session exits.

5. **Fixed PowerShell Read-Only `$PID` Overwrite & Typecasting**:
   - Renamed loop variable `$pId` to `$sessPid` across `run_claude_bap.ps1` and `.bat` (PowerShell is case-insensitive, so `$pId = ...` attempted to overwrite built-in `$PID`).
   - Fixed typecast precedence: `[int]($s.launcher_pid)` instead of `[int]$s.launcher_pid` (which threw cast exceptions on custom objects).

6. **Eliminated Interactive Hangs (`Read-Host`)**:
   - Gated `Read-Host` server prompt behind `$canPrompt` (only executes if zero arguments passed AND interactive console).
   - Scripts and `--version` calls now execute immediately without blocking.

7. **Cross-Directory Global Binary Resolution**:
   - When developers run `run_claude_bap.bat` from any directory outside the repo, `Resolve-Binary` and `Get-ConfigServerUrl` search well-known locations (`%USERPROFILE%\pyprj\bapltd`, `%USERPROFILE%\bin`, `%USERPROFILE%\.bap`).
   - Copied `bapedge.exe` and `interceptor.exe` to `%USERPROFILE%\bin`.

8. **Teardown File Lock Contention**:
   - Watchdog process is now explicitly stopped prior to file restoration, eliminating Windows sharing locks (`The process cannot access the file ... because it is being used by another process`).
   - Added retries with backoff in `Restore-TrackedFile`.

9. **Concurrency Verification Passed**:
   - Automated tests in `scripts/test_claude_concurrency.ps1` pass 100%:
     - Simultaneous overlapping sessions in same directory exit cleanly with original settings restored.
     - Concurrent sessions in separate directories run in parallel with zero conflicts.

---

10. **Multi-Platform Build (`build_all_platforms.bat`)**:
    - Fixed recursion in `Safe-CopyItem` and synchronized `run_claude_bap.ps1` into role packages.
    - Verified: Built static zero-dependency binaries for Windows, Linux (amd64/arm64), and macOS (Intel/Apple Silicon) in 46.9s.
    - Verified: All 72 fleet distribution assets audited and confirmed in exact sync (0 stale copies).

11. **Comprehensive End-to-End Test Suite (`run_all_tests.bat`)**:
    - Fixed kill-switch state discovery in `authorizer.go` to prioritize directory-specific policy state.
    - Ran full test suite across all 12 stages: 31/31 tests passed (Exit Code 0).

---

## Status: Previous Sprint Completed (100% Tests & Builds Green)

---

# Active Sprint: Binary Architecture Organization & Deduplication

## Objective
Standardize and reorganize all BAP executables so there is a single canonical set of binaries, zero duplicate or differently-sized binaries, and clean separation between Server, Edge, and Agent Interceptor roles.

## Binary Taxonomy
| Role | Component | Canonical Binary Name | Description |
| :--- | :--- | :--- | :--- |
| **Server** | Control Plane | `bapcontrolplane` | Central CA, attestation, policy & audit server |
| **Server** | Dashboard | `bapdashboard` | Standalone HTTPS UI dashboard server |
| **Server** | Gateway | `bapgateway` | Zero-trust PEP reverse proxy gateway |
| **Edge** | Edge Daemon | `bapedge` | Machine node daemon, telemetry watcher & CLI |
| **Hook** | Claude Code Interceptor | `interceptor` / `cchook-interceptor` | Claude Code PreToolUse PEP hook |
| **Hook** | GitHub Copilot Interceptor | `copilot-interceptor` | GitHub Copilot CLI execution PEP hook |

## Checklist & Progress

- [x] **Step 1: Unify Build Flags Across Builders**
  - Standardized `build_binaries.bat` and `build_all_platforms.ps1` to use identical `-trimpath -ldflags "-s -w"` flags. All outputs produce identical byte-for-byte binary sizes across the fleet.
- [x] **Step 2: Remove Stale & Legacy Duplicate Binaries**
  - Deleted 14 stale legacy copies (`server.exe`, `ltd-agent.exe`, `*.old`, `*.next.exe`, `*.test.exe`, `bapgateway.exe~`, and staging folders).
  - Eliminated duplicate `bapedge.exe` copies from `cchook/` and `copilot/`.
- [x] **Step 3: Standardize `dist/` Role Package Layout**
  - `dist/windows-amd64/`: Core suite (`bapcontrolplane`, `bapdashboard`, `bapgateway`, `bapedge`, `cchook-interceptor`, `copilot-interceptor`).
  - `dist/windows-amd64/claude-client/`: Clean developer seat package containing ONLY `bapedge.exe`, `interceptor.exe`, `run_claude_bap.bat`, `run_claude_bap.ps1`, `bap-config.json`, `policy.cedar`, `schema.json`.
  - Deleted conflicting `cchook-interceptor.exe` from `claude-client/`.
- [x] **Step 4: Update Reference Scripts & Paths**
  - Verified `run_claude_bap.ps1`, `run_claude_bap.bat`, `test_control_plane.py`, and test runners reference canonical paths.
- [x] **Step 5: Verify Rebuild and Fleet Audit**
  - `build_binaries.bat`: Passed with Exit Code 0 in ~15s (exact 1:1 binary sizes verified).
  - `scripts/test_claude_concurrency.ps1`: Passed with Exit Code 0 (same & different directory concurrency).
  - Stale asset audit: 72/72 fleet distribution assets confirmed in exact sync.

---

# Active Sprint: MVP Enterprise Readiness Pack

## Goals
Implement the top 3 high-impact enterprise MVP capabilities:
1. **1-Click Developer Onboarding (`install_bap_client.bat`)**: Single-script deployment to `%USERPROFILE%\bin` with automatic User `PATH` configuration.
2. **Pre-Flight Diagnostic Health Check (`bap_doctor.bat`)**: Instant 5-point environment and hook health diagnosis.
3. **Enterprise Rollout Mode: Audit/Shadow vs Enforce (`bap-config.json` & `bapedge`)**: Support silent policy monitoring (`audit` mode) with telemetry tagging alongside zero-trust hard blocking (`enforce` mode).

## Completed Tasks
- [x] **Task 1: 1-Click Developer Installer (`install_bap_client.bat` & `scripts/install_client.ps1`)**
  - Copies canonical binaries from `dist/windows-amd64/claude-client/` to `%USERPROFILE%\bin`.
  - Idempotently adds `%USERPROFILE%\bin` to User `PATH` via PowerShell environment API.
  - Updates current session environment so `run_claude_bap` works immediately.
  - Verified: 7 files deployed, PATH configured, 0 errors.
- [x] **Task 2: Pre-Flight Diagnostic Health Check (`bap_doctor.bat` & `scripts/bap_doctor.ps1`)**
  - Tests Core BAP binaries & client tools in `%USERPROFILE%\bin` or repo.
  - Tests Claude Code CLI executable resolution and version (`v2.1.273`).
  - Tests Control Plane reachability & TLS handshake latency.
  - Tests Cedar policy engine compilation, rule validity, and sub-millisecond evaluation latency.
  - Tests Workspace `.bap/` directory isolation and active transaction lockouts.
  - Verified: `bap_doctor.bat` reports 8/9 operational checks passed (offline-safe when CP is idle).
- [x] **Task 3: Enterprise Audit/Shadow Mode vs Enforce Mode**
  - Added `"enforcement_mode": "enforce"` | `"audit"` support in `bap-config.json`, CLI flag `--mode`, and `BAP_ENFORCEMENT_MODE` env var.
  - Updated `bap-edge`: when in `audit` mode, logs violations as `shadow_deny` in audit telemetry with policy details, prints `[BAP AUDIT MODE] Policy violation detected: ... Execution permitted in audit mode.`, and permits command execution.
  - When in `enforce` mode (default), maintains strict Zero-Trust hard blocking.
  - Updated `copilot_interceptor.go` and MCP server to support audit warning reflection.
  - Verified: Automated tests confirm enforce mode blocks credential access (`exit 1`) while audit mode allows execution (`exit 0`) with structured warnings and `shadow_deny` audit events.
- [x] **Task 4: Full Suite & Concurrency Verification**
  - Verified `build_binaries.bat`: 7/7 stages passed (exit 0).
  - Verified `run_all_tests.bat`: 31/31 stages passed (exit 0).
  - Verified `scripts/test_claude_concurrency.ps1`: 100% passed (both same-directory and multi-directory concurrency).
  - Preserved Cedar security rules (`*comsvcs*`, `*minidump*`, `*sekurlsa*`, `*set-mppreference*`, `*disablerealtimemonitoring*`).
  - Audited all `.bat` and `.ps1` scripts to ensure zero escalated or EDR heuristic commands exist.

---

# Active Sprint: Linux Background Supervision & Headless Resiliency Testing

## Goals
1. **Linux Background Auto-Restart Supervisor**: Provide production-grade background daemon supervision (`start`, `stop`, `status`, `restart`) with automatic restart on process death, plus a standard systemd service unit.
2. **Negative Test Isolation Policy**: Isolate all adversarial simulation tests strictly into `run_negative_testcases.bat` (never invoked by automated suites); ignore from indexing via `.cursorignore`.
3. **Automated Headless Resiliency, Scaling, & Integrity Suite**: Verify that the new system withstands process crashes, restarts, scaling bursts, and total server outages without breaking zero-trust integrity.

## Completed Tasks
- [x] **Task 1: Linux Background Auto-Restart Supervisor & Systemd Daemon Service**
  - Created `scripts/supervise_controlplane.sh` (supports `start`, `stop`, `status`, `restart`, and `foreground`).
  - Automatically respawns `bapcontrolplane` within 1 second if killed (`kill -9`, SIGTERM, crash).
  - Created `start_controlplane_supervisor.sh` root wrapper.
  - Created `dist/systemd/bapcontrolplane.service` for systemd integration (`Restart=always`, `RestartSec=2s`).
- [x] **Task 2: .cursorignore & Negative Test Policy Enforcement**
  - Updated `.cursorignore` to ignore `run_negative_testcases.bat`.
  - Confirmed `run_all_tests.bat`, regression suites, and resiliency tests never invoke negative/flagged simulation tests.
- [x] **Task 3: Automated Headless Resiliency, Scaling, & Failover Test Suite**
  - Created `scripts/test_resiliency_headless.ps1` and `run_resiliency_test.bat`.
  - Verified 10/10 automated assertions passing (100% system integrity):
    1. Supervisor initialization & process health (PID logged, port 8443 online).
    2. Dead process auto-respawn: killed PID automatically revived with new PID in ~1.1s.
    3. Port 8443 HTTPS listener fully restored post-kill.
    4. Concurrency scaling: 20 concurrent workers evaluated policy decisions in 1057 ms (~52.8 ms/decision).
    5. Simulated total control plane outage: confirmed port 8443 closed.
    6. Safe commands permitted offline in < 200 ms (Offline-First Zero-Trust).
    7. Workspace boundary containment strictly enforced offline (`ExitCode: 1`, fail-secure).
    8. Enterprise Audit/Shadow mode operable offline (permitted with shadow warning, `ExitCode: 0`).
    9. Service reconnection and telemetry resumption verified.
    10. JSON telemetry output mode (`-Json`) fully operational for headless CI/CD.

