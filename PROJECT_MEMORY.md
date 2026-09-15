# BAP project memory

> Working guide for maintainers and coding agents. Source inspection and the fast
> test layer were last checked on 2026-09-15. Prefer code and executable scripts
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

- `cchook/` handles Claude Code lifecycle/tool hooks.
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
| `bap-controlplane/cmd/server/` | Control-plane executable |
| `bap-controlplane/cmd/dashboard/` | Standalone dashboard executable |
| `bap-controlplane/internal/api/` | HTTP routes, auth/CORS, lifecycle and security tests |
| `bap-controlplane/internal/dashboardui/` | Embedded React assets and same-origin API proxy |
| `bap-gateway/` | Native Go gateway PEP and protected demo APIs |
| `dashboard/` | React 19 + Vite source, Node tests, Playwright tests |
| `python-agent/` | `bap_sdk` package and agent examples |
| `tests/` | Python integration, MCP, gateway, SDK, and sandbox tests |
| `scripts/` | certificate, acceptance, demo, benchmark, fleet, and cleanup helpers |
| `policy.cedar`, `schema.json` | root packaging copies of the Cedar policy/schema |
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

Do not repeat the old `:8082` dashboard assumption. The control plane no longer
hosts the dashboard and its lifecycle tests enforce that boundary. The standalone
dashboard proxies some legacy `/inspector*` paths, but the control-plane handler
does not currently serve those pages; treat the React `/dashboard/` as canonical.

## Security and state model

- Control-plane admin calls use a bearer credential from `-admin-token`,
  `BAP_ADMIN_TOKEN`, or an auto-generated `.bap-admin-token` file.
- Remote administrative access is disabled unless both the service boundary and
  valid credential allow it (`-allow-remote-admin` where applicable).
- `BAP_SECRET_KEY` should be stable and shared where local gateway verification is
  used. Without it, the control plane generates an ephemeral signing key.
- Gateway `-consume=true` is the safe default: grants are atomically consumed by
  the control plane. `-consume=false` requires a non-demo private signing secret.
- The dashboard is a proxy boundary, not a complete user identity system. The
  current credential dialog does not provide named accounts, SSO, or server-side
  step-up authentication. See `DEPLOYMENT_REVIEW.md` before production claims.
- SQLite (`-db`, `BAP_DB_PATH`, default `bap-controlplane.db`) persists session
  data. Registry, policy/global kill state, grant state, and the in-memory audit
  store do not all gain durable restart semantics merely because SQLite is on.
- Offline policy cache permits continuity, but a disconnected edge cannot enforce
  a revocation it has not received.
- TLS is server authentication in the normal supplied setup. Optional dashboard
  client certificate flags support the dashboard-to-control-plane hop; native
  end-to-end mTLS is not implied everywhere.

The Claude hook path combines lifecycle notifications, local revocation markers,
and `bapedge watch`. `SessionStart`, `UserPromptSubmit`, and `PreToolUse` are
configured in `.claude/settings.json`; denied hook operations fail closed. Avoid
documenting precise latency or kill timing as a contract unless benchmarked on the
target platform.

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

As of 2026-09-15 all four commands pass in this checkout. The dashboard command
runs three Node presence-state tests. The Go suites cover API security/lifecycle,
TLS deployment cases, grants, OTC, policy stores, session persistence, dashboard
proxy behavior, audit integrity, authorization, sandbox helpers, and gateway
fail-closed behavior.

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

This currently collects 24 cases. Most are integration tests, not isolated unit
tests: MCP cases launch binaries, and SDK/gateway cases expect reachable services,
credentials, and configured endpoints. Use their targeted forms only after the
needed binaries/services are ready:

```powershell
python -m pytest tests/test_mcp_server.py -q
python -m pytest tests/test_python_agent.py -q
python -m pytest tests/test_gateway_pep.py -q
python tests/test_control_plane.py
```

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

rem Launch Claude Code with BAP hooks/watcher
run_claude_bap.bat

rem Configure endpoints interactively/centrally
configure_endpoints.bat

rem Stop the old inspector/demo launcher
stop_inspector.bat
stop_demo.bat
```

`start_dashboard.bat` defaults to control plane `8443` and dashboard `8444`,
preserves the SQLite database, generates a fresh admin token for the launch, and
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
