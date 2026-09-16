# BAP project memory

> Working guide for maintainers and coding agents. Source inspection and the fast
> test layer were last checked on 2026-09-16. Prefer code and executable scripts
> over prose when they disagree.

## What this repository is

BAP (Bounded Authority Plane) is a governance and policy-enforcement system for
AI agents. It places a local broker between an agent and command/tool execution,
maintains central enrollment/session/policy state, issues short-lived grants, and
can enforce grants again at an outbound gateway.

The current runtime has four distinct concerns:

1. `bapedge` is the local trusted daemon (LTD), CLI, policy decision/enforcement
   point, audit producer, watcher, attestation service, and MCP stdio server.
2. `bapcontrolplane` is the API-only central service for enrollment, registry,
   grants, policy distribution, audit ingestion, and session lifecycle.
3. `bapdashboard` is a separate HTTPS service. It embeds the React build and
   reverse-proxies `/api/` to the control plane so the browser has one origin.
4. `bapgateway` protects example downstream APIs by consuming a grant centrally
   (the default) or verifying it locally when explicitly configured.

Agent-specific adapters invoke the edge broker:

- `cchook/` handles Claude Code lifecycle/tool hooks (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, and directory traversal enforcement).
- `copilot/` contains the Copilot command interceptor and shell wrappers.
- `python-agent/` is a zero-third-party-dependency Python SDK.
- `mcp/` contains client configuration examples; MCP itself is implemented by
  `bapedge mcp`. A binary named `bapmcp` enters MCP mode when run without args.
- `envoy/` is an alternate/demo gateway path, not the primary native gateway.

Legacy names still occur in code and docs: `ltd-agent` means `bapedge`, and
`ltd-service`/`bapservice` means `bapcontrolplane`.

## Repository map and ownership

| Path | Owns / contains |
| --- | --- |
| `bap-edge/` | Go edge CLI and packages: `config`, `register`, `sync`, `serve`, `exec`, `mcp`, `attest`, `watch`, `verify-log` |
| `bap-edge/internal/authz/workspace.go` | Directory containment & shell escape analyzer (`IsPathOutsideWorkspace`, `CheckCommandWorkspaceEscape`) |
| `bap-controlplane/cmd/server/` | Control-plane executable (persists admin token from `.bap-admin-token`, SQLite state) |
| `bap-controlplane/cmd/dashboard/` | Standalone dashboard executable |
| `bap-controlplane/internal/api/` | HTTP routes, auth/CORS, lifecycle (Stop, Revoke, Restore), security tests |
| `bap-controlplane/internal/dashboardui/` | Embedded React assets and same-origin API proxy |
| `bap-gateway/` | Native Go gateway PEP and protected demo APIs |
| `dashboard/` | React 19 + Vite source, Node tests, Playwright tests |
| `inspector_v2.html` | Modern unified single-file inspector with real-time active filtering and stop/revoke controls |
| `cchook/` | Claude Code hook interceptor (`interceptor.go`) with PID session isolation and traversal confinement |
| `run_claude_bap.bat` / `.ps1` | Self-contained transactional zero-trust launcher with state machine, settings custody, detached watchdog, and ConfigChange guard |
| `python-agent/` | `bap_sdk` package and agent examples |
| `tests/` | Python integration, MCP, gateway, SDK, sandbox, and `test_stop_revoke_lifecycle.py` |
| `scripts/` | certificate, acceptance, demo, benchmark, fleet, cleanup helpers, and `check_stale_copies.ps1` |
| `policy.cedar`, `schema.json` | root packaging copies of the Cedar policy/schema (72 copies across repo verified in sync) |
| `bap-config.json` | developer endpoint defaults, currently HTTPS control plane `:8443` and HTTP gateway `:9090` |
| `dist/` | generated multi-platform packages; ignored and safe to recreate |

There are three independent Go modules, not one workspace-level module:

- `bap-controlplane` requires Go 1.25.
- `bap-edge` and `bap-gateway` declare Go 1.24.
- Build scripts set `GOTOOLCHAIN=auto` and `CGO_ENABLED=0` for release builds.

The Python package supports Python 3.8+. Dashboard dependencies are locked in
`dashboard/package-lock.json`; use `npm ci` for a clean install.

## Current topology and ports

| Process | Code default | Repository launcher/default config | Notes |
| --- | --- | --- | --- |
| `bapcontrolplane` | HTTP `:8080` | HTTPS `:8443` | `-https` enables TLS and uses supplied/discovered certs or creates a dev cert |
| `bapdashboard` | HTTPS `:8444` | HTTPS `:8444` | UI at `/dashboard/`; health at `/dashboard-health` |
| `bapgateway` | HTTP `:9090` | HTTP `:9090` | health at `/health` and `/api/v1/health` |
| Vite dev server | `127.0.0.1:5173` | same | proxies API in development; allow its exact origin on the control plane |
| Envoy demo | `:10000` | `:10000` | optional demonstration path |

### UI Presence & Session Filtering
- Both the React Dashboard and `inspector_v2.html` filter out offline/stale agents (heartbeat older than 15s) from the primary active workload display.
- Only genuinely `active` running sessions and `revoked` agents (which require administrative visibility for restoration) are displayed.
- Workloads are distinguished by both user identity and Session ID / OS PID. Multiple Claude sessions from the same user/machine are reported as separate active agents rather than aggregated into one.

## Security and state model

- Control-plane admin calls use a bearer credential from `-admin-token`,
  `BAP_ADMIN_TOKEN`, or an auto-generated `.bap-admin-token` file.
- **Admin token durability**: `bapcontrolplane` inspects the filesystem for an existing valid `.bap-admin-token` on startup. If found, it reuses the token across restarts instead of generating an ephemeral new token. This preserves authentication continuity for dashboards, watchers, and CLI tools.
- Remote administrative access is disabled unless both the service boundary and
  valid credential allow it (`-allow-remote-admin` where applicable).
- `BAP_SECRET_KEY` should be stable and shared where local gateway verification is
  used. Without it, the control plane generates an ephemeral signing key.
- Gateway `-consume=true` is the safe default: grants are atomically consumed by
  the control plane. `-consume=false` requires a non-demo private signing secret.
- SQLite (`-db`, `BAP_DB_PATH`, default `bap-controlplane.db`) persists session
  data. Registry, policy/global kill state, grant state, and the in-memory audit
  store do not all gain durable restart semantics merely because SQLite is on.
- Offline policy cache permits continuity, but a disconnected edge cannot enforce
  a revocation it has not received.
- TLS is server authentication in the normal supplied setup. Optional dashboard
  client certificate flags support the dashboard-to-control-plane hop; native
  end-to-end mTLS is not implied everywhere.

### Multi-Session Isolation & Watcher Architecture
- Previous versions shared a single `.bap-session.json` marker in the workspace. When one session exited and deleted the file, other concurrent watchers incorrectly assumed their workloads ended and stopped pulsing heartbeats, causing active session counts to drop.
- **Isolated session markers**: Each session writes its marker to `.bap/sessions/<sha256(sessionID)>.json` and `.bap/sessions/pid-<pid>.json`.
- `bapedge watch`: The primary lifecycle sentinel is `isProcessAlive(watchPID)`. The watcher never terminates merely because `.bap-session.json` was deleted by another process.
- `cleanupSessionMarker`: Only deletes its own session marker, and only deletes `.bap-session.json` if the session ID inside matches its own.
- `cchook/interceptor.go`: Session ID resolution checks parent PID marker `.bap/sessions/pid-<ppid>.json` first, enabling multiple Claude Code instances to operate concurrently in the same project without session collision.

### Directory Traversal / Shell Box Confinement
- Agents (including Claude Code) must be permitted to navigate within project subdirectories (e.g. Maven submodules, child git repos, nested packages), but must be strictly prevented from escaping the workspace root.
- `bap-edge/internal/authz/workspace.go` provides:
  - `IsPathOutsideWorkspace(targetPath, workspaceRoot)`: Normalizes casing, drive letters, resolves symlinks and `..` segments, verifying path is strictly within the workspace.
  - `CheckCommandWorkspaceEscape(commandStr, workspaceRoot)`: Evaluates shell commands (`cd`, `chdir`, `pushd`, `popd`, paths in arguments, chaining `&&`, `;`, `|`). Internal subdirectory navigation is allowed; escape navigation sets `context.escapes_workspace = true`.
- Cedar policy (`policy.cedar`) enforces:
  ```cedar
  forbid (
      principal,
      action in [Action::"Bash", Action::"Read", Action::"Edit", Action::"Write", Action::"View"],
      resource
  )
  when {
      context.escapes_workspace == true
  };
  ```
- File hooks in `cchook/interceptor.go` intercept `Read`, `Edit`, `Write`, `View`, and `Bash` tool calls, asserting workspace containment before policy evaluation.

### Session Lifecycle (Stop, Revoke, Restore)
- **Stop** (`/api/v1/sessions/stop` or targeted kill with action `stop`): Graceful session shutdown. Marks session status as `closed`. Watcher notifies workload process to exit cleanly.
- **Revoke** (`/api/v1/sessions/revoke`): Security revocation. Marks session or user as `revoked`. Watcher immediately terminates the target workload process (`killProcessPID`) and synchronizes Cedar policies to block any further commands.
- **Restore** (`/api/v1/sessions/restore`): Re-enables a revoked user or session, clearing the revocation marker and restoring normal policy authorization.
- Automated end-to-end lifecycle verification: `python tests/test_stop_revoke_lifecycle.py`.

### Transactional Launcher & Settings Custody (`run_claude_bap.bat` / `.ps1`)
- Self-contained zero-trust launcher with state machine: `Prepared` -> `Swapping` -> `Swapped` -> `Guarded` -> `SessionActive` -> `ClaudeRunning` -> `Cleaning` -> `Complete`.
- Atomic recovery manifest (`.bap-recovery.json` + `.bap-lock.json` mirror) ensures reliable crash recovery on subsequent launches.
- Preserves original `.claude/settings.json` and `settings.local.json` via backup rename, replaces them with BAP-governed hooks (`SessionStart`, `UserPromptSubmit`, `PreToolUse` for `*`, and `ConfigChange` guard).
- Spawns a detached watchdog (`Start-BapWatchdog`) monitoring launcher process identity (PID + start ticks) to continuously repair tampered settings and restore originals if the launcher crashes or the terminal is closed.
- **Concurrency note**: The launcher uses a workspace-level mutex (`Acquire-BapMutex -TimeoutMs 0`) to prevent concurrent file swaps in the same folder. If multiple Claude sessions must run simultaneously in the same directory, run them with the pre-installed hook in `.claude/settings.json` or use separate workspace directories.

## Configuration precedence and important environment variables

For edge endpoints, environment variables override discovered config files. The
resolver searches the current directory and parents, the executable directory,
user config (`~/.bap` and legacy `~/.ltd`), then system config before defaults.

Common settings:

| Setting | Purpose |
| --- | --- |
| `BAP_SERVER_URL`, `BAP_CONTROL_PLANE_URL` | Control-plane endpoint override |
| `BAP_GATEWAY_URL`, `BAP_GATEWAY_PORT` | Gateway endpoint/port override |
| `BAP_CONFIG` | Explicit config path for components that support it |
| `BAP_CA_CERT` | CA used by edge, gateway/SDK calls, and dashboard launcher |
| `BAP_ADMIN_TOKEN` | Control-plane administrative bearer secret |
| `BAP_SECRET_KEY` | Grant signing/local verification secret |
| `BAP_DB_PATH` | SQLite location |
| `BAP_ALLOWED_ORIGINS` | Exact browser origin allowlist |
| `BAP_TRUST_DOMAIN` | SPIFFE trust domain |

The checked-in `bap-config.json` contains a machine-specific `config_source`
value. Treat that field as diagnostic metadata, not a portable path contract.

## Build commands

Run these from the repository root on Windows.

### Fast Windows build

```bat
build_binaries.bat
```

This ensures certificates, builds the Vite assets, then builds and copies:
`bapcontrolplane.exe`, `bapdashboard.exe`, `bapedge.exe`, `bapgateway.exe`, the
Claude hook, and the Copilot interceptor. It also creates compatibility copies
such as `bapmcp.exe` and `ltd-agent.exe`. If a root binary is locked, the script
may leave an `.old` backup. It synchronizes the legacy inspector copies too.

`build_binaries.bat --all` delegates to the full multi-platform build.

### Multi-platform release build

```powershell
.\build_all_platforms.ps1 -Archive
```

or:

```bat
build_all_platforms.bat
```

This builds six components for Windows AMD64, Linux AMD64/ARM64, and macOS
AMD64/ARM64, then creates complete-platform and role-specific archives under
`dist/` (control plane, dashboard, Claude client, and gateway).

Important build side effects:

- `scripts/ensure_certs.ps1` provisions/synchronizes local TLS material and
  embedded CA copies.
- `npm run build` refreshes
  `bap-controlplane/internal/dashboardui/web/`, which is embedded in the Go
  dashboard binary.
- Archive mode deletes stale archives and database/log files under the selected
  distribution directory before packaging.
- `-DistDir <path>` changes the output root. The repository's stray `windows/`
  tree appears to be one such generated output; canonical output is `dist/`.

### Component inner-loop builds

```powershell
Push-Location bap-controlplane
go build ./cmd/server
go build ./cmd/dashboard
Pop-Location

Push-Location bap-edge
go build .
Pop-Location

Push-Location bap-gateway
go build .
Pop-Location
```

For UI work:

```powershell
Push-Location dashboard
npm ci
npm run build
Pop-Location
```

Always rebuild the Vite assets before building `bapdashboard`; otherwise Go can
embed an older UI.

## Test commands and what they actually cover

### Fast, isolated checks (preferred during development)

```powershell
Push-Location bap-controlplane; go test ./...; Pop-Location
Push-Location bap-edge; go test ./...; Pop-Location
Push-Location bap-gateway; go test ./...; Pop-Location
Push-Location dashboard; npm test; Pop-Location
```

As of 2026-09-16 all four commands pass in this checkout. The dashboard command
runs Node presence-state tests. The Go suites cover API security/lifecycle,
TLS deployment cases, grants, OTC, policy stores, session persistence, dashboard
proxy behavior, audit integrity, authorization, sandbox helpers, directory
traversal containment (`TestWorkspaceContainment`), and gateway fail-closed behavior.

To verify workspace path confinement specifically:
```powershell
Push-Location bap-edge; go test -v ./internal/authz -run TestWorkspaceContainment; Pop-Location
```

To verify policy/schema synchronization across all 72 repository copies:
```powershell
powershell scripts/check_stale_copies.ps1
```

### Browser tests

```powershell
Push-Location dashboard
npx playwright install chromium
npm run test:browser
Pop-Location
```

Playwright exercises the built dashboard against a real Go fixture, including
prompt protection/admin actions, offline presence/XSS-safe rendering, and wide +
narrow screenshots. It is a separate release signal from `npm test`.

### Python/integration inventory

```powershell
python -m pytest --collect-only -q tests
```

Most are integration tests, not isolated unit tests: MCP cases launch binaries,
and SDK/gateway cases expect reachable services, credentials, and configured endpoints.
Use their targeted forms after needed binaries/services are ready:

```powershell
python tests/test_stop_revoke_lifecycle.py
python -m pytest tests/test_mcp_server.py -q
python -m pytest tests/test_python_agent.py -q
python -m pytest tests/test_gateway_pep.py -q
python tests/test_control_plane.py
```

`test_stop_revoke_lifecycle.py` launches an ephemeral control plane on port 62411
and exercises the complete lifecycle: session enrollment, heartbeat, graceful stop,
security revocation, and administrative restoration.

The network leak test skips or has different enforcement meaning on Windows; its
strong sandbox assertion belongs on Linux/WSL.

### Full Windows runner and manual acceptance

```bat
run_all_tests.bat
```

This is a heavy orchestration script. It rebuilds all platforms and archives,
runs policy/adversarial/interceptor/MCP/SDK/gateway checks, creates runtime logs,
starts local services, and terminates BAP processes/listeners during setup and
teardown. It may fall back from an unreachable configured remote control plane to
localhost, so a green result is not proof that the remote topology passed. Its
pass counter mixes individual assertions and whole suites; never describe it as a
fixed number of independent end-to-end cases.

Use `MANUAL_E2E_TESTING.md` plus `scripts/manual_acceptance.py` for isolated
acceptance evidence. Use `simple_local_test.bat` only as an interactive demo: it
starts control plane/gateway/watchers, opens a browser, uses demo credentials, and
cleans up processes.

Performance scripts under `tests/perf_test*.py` and `scripts/benchmark_ollama.py`
are benchmarks, not correctness or security release gates.

## Common developer shortcuts

```bat
rem Build/start the HTTPS control plane and separate dashboard, then open the UI
start_dashboard.bat

rem Start standalone Inspector V2 (served from control plane on 8443)
start_inspector.bat

rem Launch Claude Code with BAP hooks/watcher
run_claude_bap.bat

rem Configure endpoints interactively/centrally
configure_endpoints.bat

rem Stop the inspector/demo launcher
stop_inspector.bat
stop_demo.bat
```

`start_dashboard.bat` defaults to control plane `8443` and dashboard `8444`,
preserves the SQLite database, reuses `.bap-admin-token` if present, and
will only stop a listener when its executable name matches the expected BAP
process. Set `BAP_NO_BROWSER=1` to suppress browser launch and
`BAP_ALLOW_REMOTE_ADMIN=1` only when remote dashboard administration is intended.

Useful edge commands:

```powershell
.\bapedge.exe config
.\bapedge.exe register --server https://localhost:8443 --code <one-time-code>
.\bapedge.exe sync --server https://localhost:8443
.\bapedge.exe exec --json --source manual "git status"
.\bapedge.exe mcp
.\bapedge.exe verify-log
```

## Documentation map and source-of-truth order

Start with these documents rather than reading every long guide:

1. `README.md` - product entry point and supported flows.
2. `PROJECT_MEMORY.md` - maintainer commands, caveats, and repository hygiene.
3. `DEPLOYMENT_START_HERE.md` - choose a deployment topology.
4. `MANUAL_E2E_TESTING.md` - current acceptance workflow.
5. `DEPLOYMENT_REVIEW.md` - implemented security changes, remaining risks, and
   statements that must qualify production-readiness claims.

Focused references:

| Document | Use it for |
| --- | --- |
| `ARCHITECTURE.md` | design and component boundaries; verify old naming/ports against source |
| `API_GUIDE.md` | endpoint examples; source handlers remain authoritative |
| `INTEGRATIONS.md` | Claude, Copilot, Cursor, MCP, and SDK setup |
| `ENDPOINT_CONFIGURATION.md` | endpoint discovery/configuration details |
| `dashboard/README.md` | current React build, dev proxy, Node and Playwright commands |
| `bap-edge/README.md` | edge CLI and policy behavior |
| `python-agent/README.md` | SDK use and package build |
| `mcp/README.md` | MCP client configuration |
| `CIO_DEMO_GUIDE.md` | presentation/demo choreography only |

`FEATURES_AND_ARCHITECTURE.md`, `TESTING_GUIDE.md`, and older inspector sections
contain useful history but overlap the current guides and may describe superseded
ports, topology, counts, names, or production claims. Confirm important statements
in code/tests before repeating them.

## Guidelines for fast feature development

Maintainers and coding agents should follow these core conventions to build new features quickly without breaking existing architecture or re-introducing solved issues:

1. **Cedar Policy & Schema Synchronization**:
   - The root `policy.cedar` and `schema.json` are the authoritative source definitions.
   - 72 synchronized copies exist across `dist/`, `cchook/`, `copilot/`, and component folders.
   - Always run `powershell scripts/check_stale_copies.ps1` after editing Cedar policies or schemas to verify zero drift.
2. **Session Concurrency & Markers**:
   - Never rely on `.bap-session.json` alone for session lifecycle decisions.
   - Use per-session markers in `.bap/sessions/<sha256(sessionID)>.json` and `.bap/sessions/pid-<pid>.json`.
   - Never delete `.bap-session.json` on workload exit unless its stored `session_id` matches the exiting session.
   - For watchers, `isProcessAlive(watchPID)` is the primary sentinel; file deletion by another process must not kill a running watcher.
3. **Admin Authentication Durability**:
   - When communicating with administrative endpoints (`/api/v1/control/*`, `/api/v1/sessions/revoke`, etc.), read `.bap-admin-token` from the project root.
   - The control plane preserves and reuses this token across restarts.
4. **Shell & Directory Traversal Confinement**:
   - All tool hooks (`Read`, `Edit`, `Write`, `View`, `Bash`) pass through `bap-edge/internal/authz/workspace.go`.
   - Subdirectory navigation inside the workspace is permitted (e.g. child Maven submodules, `cd src`); any path resolving outside the workspace root triggers `context.escapes_workspace = true` and fails closed under Cedar `forbid`.
5. **Dashboard & UI Changes**:
   - If editing React UI (`dashboard/src/`), always run `npm run build` in `dashboard/` before compiling `bapdashboard.exe`. Otherwise, the Go binary embeds stale static assets.
   - Active workloads in UI are filtered by heartbeat (< 15s) and `active` status, with `revoked` instances highlighted for administrative restore.
6. **Inner-Loop Test Validation**:
   - Run the fast isolated suite: `go test ./...` in the modified Go module.
   - Run `python tests/test_stop_revoke_lifecycle.py` whenever modifying session lifecycle or control plane handlers.

## Worktree and generated-file rules

This repository is commonly used with a dirty worktree. Do not revert unrelated
changes, staged deletions, or regenerated dashboard assets. Inspect `git status
--short` before and after work.

Never commit:

- private keys, certificates, `.bap-admin-token`, credentials, or `.env` files;
- SQLite/WAL/SHM state, session markers, audit JSONL, or policy-state caches;
- root/component binaries, `.old` backups, archives, `dist/`, caches, test reports,
  Python build output, or `node_modules/`.

The `.gitignore` already covers most of these. Generated files that are already
tracked remain tracked until explicitly removed from the index; ignore rules do
not retroactively untrack them.

## Pruning candidates (proposal only; confirm before deletion)

No files are pruned as part of maintaining this document. Work through this list
in order and make cleanup a separate, reviewable change.

### P0 - safe generated clutter to remove locally

1. Remove the untracked `windows/` output tree (~587 MB in this checkout). It
   duplicates release packages and is not the canonical `dist/` target. Add
   `windows/` to `.gitignore` only if that `-DistDir windows` workflow is expected.
2. Remove `bap-controlplane/.build-staging-dashboard-fix/` (~25 MB, untracked).
3. Remove ignored `.pytest_cache/`, nested `.pytest_cache/`, `__pycache__/`,
   `python-agent/build/`, `python-agent/dist/`, and `*.egg-info/` outputs.
4. Remove ignored root/component `*.exe`, compatibility binary copies, and
   `*.old` backups when no BAP process is using them. Rebuild from source.
5. Remove/rebuild `dist/` (~440 MB here) whenever old release output is not needed.

Do not casually delete the active SQLite database/WAL, `.bap-admin-token`, or TLS
keys/certificates: they are ignored but contain local state/identity. Delete them
only as an intentional state reset or certificate rotation.

### P1 - finish removal of accidentally tracked generated files

The current worktree already stages deletion of Python wheels/build metadata,
Python bytecode, audit output, a session file, and accidental `[ALLOWED]` /
`[BLOCKED]` shell artifacts. Keep those deletions after confirming no release
process expects generated artifacts in source control. This aligns the index with
the existing ignore policy.

### P2 - consolidate duplicated source assets

1. Decide whether legacy `inspector.html` and `inspector_v2.html` are supported.
   If React is canonical, remove the dead proxy/package paths and duplicate root +
   `bap-controlplane/` copies. If compatibility is required, keep one canonical
   source and add a test that the routed page is actually served.
2. Make one Cedar policy/schema location canonical. Root, `bap-edge/`, `cchook/`,
   and `copilot/` copies can drift; generate/package required colocated copies.
3. Consolidate the identical `test_leak.py` copies under `tests/`, `bap-edge/tests/`,
   and `cchook/tests/`, or document why each execution context needs a copy.
4. Consider sharing the identical edge/gateway HTTP transport helper. Because the
   components are separate Go modules, weigh a small shared module against the
   dependency/packaging cost before changing it.
5. Choose `pyproject.toml` as the Python packaging authority and retain `setup.py`
   only if an older corporate build system demonstrably requires it.
6. Replace physical `bapmcp`/`ltd-agent` binary copies with packaging aliases or
   documented invocation where platform/install constraints allow it.

### P3 - reduce documentation overlap

Recommended target set:

- Keep `README.md` short as the landing page.
- Keep `ARCHITECTURE.md` as the single architecture narrative.
- Keep `API_GUIDE.md`, `INTEGRATIONS.md`, `DEPLOYMENT_START_HERE.md`,
  `MANUAL_E2E_TESTING.md`, and `DEPLOYMENT_REVIEW.md` as focused operational docs.
- Merge unique material from `FEATURES_AND_ARCHITECTURE.md` into `ARCHITECTURE.md`,
  then archive or remove the former.
- Merge still-valid test recipes from `TESTING_GUIDE.md` into
  `MANUAL_E2E_TESTING.md` or component READMEs; remove historical fixed test counts.
- Retire `REACT_ADMIN_INTEGRATION.md` after its still-current setup is captured in
  `dashboard/README.md` and deployment docs.
- Fold `ENDPOINT_CONFIGURATION.md` into deployment/integration docs if maintaining
  its separate matrix is not valuable.
- Keep `CIO_DEMO_GUIDE.md` and demo scripts only if the scripted sales/demo path is
  actively exercised; otherwise move them to `docs/archive/`.
- Remove `GEMINI.md.off` if it is no longer intentionally retained as an example.

Before pruning a document, search inbound links with `rg '<filename>'` and update
the README/documentation index in the same commit.

## Known gaps and claims to avoid

- Do not call the system comprehensively production-ready based only on the local
  automated runner. Native platform, browser, topology, restart, partition,
  certificate rotation, and failure-mode matrices remain release work.
- Do not call the current admin token an operator password or claim per-user SSO.
- Do not imply all control-plane state is durable.
- Do not imply a local/Windows sandbox test proves Linux namespace enforcement.
- Do not treat a successful localhost fallback as a remote deployment test.
- Do not quote a fixed test count without naming exactly what was counted.
