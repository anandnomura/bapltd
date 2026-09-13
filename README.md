# BAP: Bounded Authority Plane for AI Agents

**BAP** provides cryptographically bounded, zero-trust execution governance for AI agents such as **Google Antigravity**, **Claude Code**, **GitHub Copilot**, **Cursor/Windsurf**, and automated Python SDK workflow workers across Windows, Linux, WSL, and macOS.

The architecture formally converges into two core components:
1. **`bapedge`**: The **Local Trusted Daemon (LTD)** process running at the edge (developer laptops, CI/CD runners, and worker hosts). It acts as the local Zero-Trust Execution Broker and Policy Decision/Enforcement Point (PDP/PEP), evaluating Cedar policies, isolating processes, sanitizing outputs, injecting scoped OBO tokens, generating tamper-evident audit logs, and running as a native **Model Context Protocol (MCP)** server (`bapmcp.exe`). *(Legacy alias: `ltd-agent`)*.
2. **`bapcontrolplane`**: The central server-side control plane managing the Agent Registry, self-service One-Time Code (OTC) registration, binary image attestation, short-lived OBO JWT grants, dynamic Cedar policy distribution, and centralized tamper-evident audit ingestion with a real-time **Live Workload Radar**. *(Legacy alias: `ltd-service`)*.

> 📖 **Enterprise Integration Guide**: See [**`INTEGRATIONS.md`**](INTEGRATIONS.md) for complete, copy-pasteable setup guides for Google Antigravity, Claude Code, Claude Desktop, VS Code Copilot, Cursor, and Python SDK.

---

## Architecture Overview

```text
                           +-------------------------------------+
                           |      AI Agent (Claude / Copilot)    |
                           +------------------+------------------+
                                              | Tool Call ("Bash", "Terminal")
                                              v
                           +------------------+------------------+
                           |     Interceptor Hook / Wrapper      |
                           |    (cchook / copilot / VS Code)     |
                           +------------------+------------------+
                                              | Invokes bapedge exec --source <agent>
                                              v
                           +------------------+------------------+
                           |  bapedge (Local Trusted Daemon - LTD)|
                           |                                     |
                           |  1. Intelligent Normalization       |
                           |     - Strips shell artifacts        |
                           |     - Preserves quoted spaces       |
                           |  2. Cedar Policy Engine             |
                           |     - Whitelist Dev Tools           |
                           |     - Strict Anti-Evasion           |
                           +---------+-----------------+---------+
                                     |                 |
                             (Denied)|                 |(Allowed)
                                     v                 v
                          Exit 1 (JSON Error)     [Sandboxed Execution]
                                                  - Linux: Namespaces / cgroups
                                                  - Windows: Job Object Isolation
                                                    + 30-50ms fast cmd.exe runner
                                                  - Output sanitized (CRLF/spaces)
                                                  - Secret injected (CORP_OBO_TOKEN)
                                                  - Exit 0 (Output / JSON)
                                     |                 |
                                     +--------+--------+
                                              |
                                              v
                            +-------------------------------------+
                            |     Local Structured Audit Log      |
                            |         (ltd-audit.jsonl)           |
                            +------------------+------------------+
                                               | Non-Blocking HTTP Push (150ms)
                                               | (Smart Test Filter Active)
                                               v
                            +-------------------------------------+
                            |        bapcontrolplane              |
                            |  - Central Agent Registry & OTC     |
                            |  - Binary SHA-256 Attestation       |
                            |  - Ephemeral OBO JWT Grants         |
                            |  - Dynamic Cedar Policy Sync        |
                            |  - Session Lifecycle Engine         |
                            |  - Tamper-Evident SHA-256 Chain     |
                            +------------------+------------------+
                                               | Live Event & Presence Stream
                                               v
                            +-------------------------------------+
                            |       BAP Activity Inspector        |
                            |  - Live Workload Presence Radar     |
                            |  - Active Agent Chips & Live Toasts |
                            |  - Step-by-Step Scenario Replayer   |
                            +-------------------------------------+
```

---

## 1. Build Commands

Compile natively or cross-compile for all operating systems (pure Go, `CGO_ENABLED=0`):

### Windows (AMD64)
```cmd
:: Build bap-edge broker (LTD)
cd bap-edge
go build -o bapedge.exe .
copy /y bapedge.exe ltd-agent.exe >nul
cd ..

:: Build bap-controlplane
cd bap-controlplane
go build -o bapcontrolplane.exe ./cmd/server
cd ..

:: Build Claude Code interceptor
cd cchook
go build -o interceptor.exe interceptor.go
cd ..

:: Build GitHub Copilot interceptor
cd copilot
go build -o copilot_interceptor.exe copilot_interceptor.go
cd ..

:: Build Gateway Policy Enforcement Point (Envoy ext_authz emulator)
cd bap-gateway
go build -o bapgateway.exe .
cd ..
```

### Linux / WSL (AMD64)
```bash
# Build bap-edge broker
cd bap-edge
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bapedge .
chmod +x bapedge
cp bapedge ltd-agent
cd ..

# Build bap-controlplane
cd bap-controlplane
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bapcontrolplane ./cmd/server
chmod +x bapcontrolplane
cd ..

# Build Claude Code interceptor
cd cchook
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o interceptor interceptor.go
chmod +x interceptor
cd ..

# Build GitHub Copilot interceptor
cd copilot
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o copilot_interceptor copilot_interceptor.go
chmod +x copilot_interceptor
cd ..
```

### macOS (Darwin AMD64 / Apple Silicon ARM64)
```bash
# Build bap-edge broker for macOS
cd bap-edge
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bapedge-darwin .
cd ..
```

---

## 2. Run Commands

### Interactive CLI Execution (Direct Output)
```cmd
:: Standard developer commands (permitted)
bap-edge\bapedge.exe exec "ls -al"
bap-edge\bapedge.exe exec "git status"
bap-edge\bapedge.exe exec "go version"
bap-edge\bapedge.exe exec "java -version"
bap-edge\bapedge.exe exec "python --version"

:: Commands with arguments containing spaces (automatically encoded)
bap-edge\bapedge.exe exec python -c "import sys; print(sys.argv[1])" "hello world"
bap-edge\bapedge.exe exec 'java -Dapp.name="My Custom App" -version'

:: Chaining and pipes
bap-edge\bapedge.exe exec "echo hello && echo world"
bap-edge\bapedge.exe exec "ls -al | grep -i README"
```

### Output Format Flags
- `--raw`: Outputs raw text regardless of whether stdout is connected to a terminal.
- `--json`: Formats response as a JSON object (`{"allowed": true, "output": "..."}`).
- `--source <name>`: Tags the calling agent in audit logs (`claude-code`, `copilot`, `cli`). Defaults to `LTD_SOURCE` or `cli`.
- `--audit-log <path>`: Specifies audit log file location (defaults to `LTD_AUDIT_LOG` or `ltd-audit.jsonl`, `'off'` to disable).

---

## 3. Test Commands

### Windows Automated 1-Click Test Suite (41 Checks)
Run the root test suite to recompile, test unit code, verify policies, check anti-evasion, validate hooks, and verify control plane offline resilience:
```cmd
run_all_tests.bat
```
Output:
```text
Total Passed : 41
Total Failed : 0
[OVERALL STATUS] SUCCESS - All tests passed!
```

### Go Unit Tests
```cmd
cd bap-edge
go test -v ./...
cd ..
cd bap-controlplane
go test -v ./...
cd ..
```

### Linux / WSL End-to-End Suite
```bash
./test_e2e.sh
```

---

## 4. Claude Code Integration & Managed Settings

Claude Code executes bash commands via tool use. You can secure Claude Code using project-level configuration or tamper-proof **Managed Settings**.

### Step 1: Project-Level Hook (`.claude/settings.json`)
In your project repository, create `.claude/settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "cchook/interceptor.exe"
      }
    ]
  }
}
```
*(On Linux/macOS, use `"cchook/interceptor"` or `"cchook/interceptor.sh"`)*.

### Step 2: Enterprise Managed Settings (Tamper-Proof)
To enforce `ltd-agent` globally so agents or users cannot disable the hook or bypass policies, configure Claude Code **Managed Settings**:

#### Windows:
Create or edit `%PROGRAMDATA%\Anthropic\ClaudeCode\managed-settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "C:\\ProgramData\\Anthropic\\ClaudeCode\\hooks\\interceptor.exe"
      }
    ]
  }
}
```

#### Linux / macOS:
Create or edit `/etc/claude-code/managed-settings.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "command": "/etc/claude-code/hooks/interceptor"
      }
    ]
  }
}
```

### Testing Claude Code Hook (Simulated & Live)
- **Simulated Tool Call**:
  ```powershell
  # Allowed command
  '{"tool_input": {"command": "ls -al"}}' | .\cchook\interceptor.exe

  # Blocked command (cat .env)
  '{"tool_input": {"command": "cat .env"}}' | .\cchook\interceptor.exe
  ```
- **Live Interactive Claude Session**:
  Launch `claude` in your terminal and prompt:
  - *Permitted*: `"Can you run git status?"` -> Allowed and executed.
  - *Forbidden*: `"Can you print the secrets in .env?"` -> Blocked by hook; Claude explains permission was denied.

---

## 5. GitHub Copilot Integration

GitHub Copilot executes commands in the integrated terminal (VS Code Agent Mode) or via GitHub Copilot CLI.

### Step 1: VS Code Terminal Integration (`.vscode/settings.json`)
Pre-configured in `.vscode/settings.json` to route terminal command executions through `copilot-wrap.bat`:
```json
{
  "terminal.integrated.profiles.windows": {
    "CopilotZeroTrust": {
      "path": "${workspaceFolder}\\copilot\\copilot-wrap.bat",
      "overrideName": true
    }
  },
  "terminal.integrated.defaultProfile.windows": "CopilotZeroTrust"
}
```

### Step 2: Live Testing with GitHub Copilot
- **In VS Code Copilot Chat (Agent Mode / `@workspace`)**:
  - Prompt: `@workspace show git status in terminal` -> Passes through `copilot-wrap.bat`, executes safely.
  - Prompt: `@workspace print .env in terminal` -> Blocked by policy with `[COPILOT BLOCKED BY POLICY]`.
- **In GitHub Copilot CLI (`gh copilot`)**:
  ```cmd
  copilot\copilot-wrap.bat "git status"
  copilot\copilot-wrap.bat "cat .env"
  ```
*(For complete live test scripts, evasion tests, and log verification, see [TESTING_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/TESTING_GUIDE.md)).*

---

## 6. Audit Logging & Learning Telemetry

Every execution is logged to `ltd-audit.jsonl` in JSON Lines format:
```json
{"timestamp":"2026-09-12T01:14:04.589Z","source":"copilot","client_pid":12352,"executable":"git","arguments":"status","full_command":"git status","decision":"allow","duration_ms":125,"exit_code":0}
{"timestamp":"2026-09-12T01:14:08.870Z","source":"claude-code","client_pid":10932,"executable":"cat","arguments":".env","full_command":"cat .env","decision":"deny","reason":"Denial triggered by: policy policy1","duration_ms":2,"exit_code":1}
```

### How to Learn from Audit Logs
1. **Find Unexpected Denials**:
   Filter the audit log for unexpected denials to refine policy rules:
   ```powershell
   Get-Content ltd-audit.jsonl | ConvertFrom-Json | Where-Object { $_.decision -eq 'deny' }
   ```
2. **Review Command Patterns**:
   Inspect what commands Claude or Copilot ran during a session:
   ```powershell
   Get-Content ltd-audit.jsonl | ConvertFrom-Json | Select-Object timestamp, source, executable, decision, duration_ms
   ```
3. **Detect Evasion Attempts**:
   Identify attempts to rename, copy, or redirect `.env` or exfiltrate credentials.

---

## 7. BAP Control Plane (`bapcontrolplane`) & Edge LTD Self-Registration

While `bapedge` operates as the Local Trusted Daemon (LTD) on the edge, the central **BAP Control Plane** (`bapcontrolplane`) manages the **Agent Registry**, **One-Time Code (OTC) Enrollment**, **Binary Image Attestation**, **Short-Lived Authority Grants**, and **Dynamic Cedar Policy Synchronization**.

```text
  [Central Infra: bapcontrolplane]
          |
          | 1. App Owner Pre-Registers Agent (AppID, Scope, Env Profile)
          v
    Generates Single-Use OTC ("LTD-OTC-XXXX-XXXX", TTL 15m)
          |
          | 2. App Owner hands OTC to edge LTD
          v
  [Edge: bapedge register --server <url> --code <OTC>]
          |
          | 3. Computes executing binary SHA-256 hash & host info
          v
  [Central Infra: bapcontrolplane]
          |
          | 4. Single-Use Verification: Burns OTC immediately (prevents replay)
          | 5. Binary Image Attestation:
          |    - Production: Strict SHA-256 whitelist matching CI/CD artifacts
          |    - Development: Trust-On-First-Use (TOFU) or dev whitelists
          v
  Enrolls Agent in Registry & Mints Initial JWT Session Token
          |
          | 6. Acquire Short-Lived Grants (/api/v1/grants/acquire)
          v
  Cryptographically signed OBO JWT tokens (15-60 min TTL)
          |
  [Remote Kill-Switch: /api/v1/agents/revoke] -> Instantly revokes agent access
```

### Running the BAP Control Plane
```powershell
cd bap-controlplane
go build -o bapcontrolplane.exe ./cmd/server
.\bapcontrolplane.exe -port 8080 -ttl 30 -policy ../bap-edge/policy.cedar -schema ../bap-edge/schema.json
```

### Step 1: Self-Registration by App Owner
The app owner calls the control plane to pre-register an agent:
```bash
curl -X POST http://localhost:8080/api/v1/agents/pre-register \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "payments-worker",
    "owner_email": "owner@company.internal",
    "agent_name": "SettlementAgent",
    "env_profile": "production",
    "allowed_binary_hashes": ["4e75b1590ef4ac3a871ba7ed2075f32d7a4cfbad773e700b583fcb4643d709fb"],
    "permitted_scopes": ["cli:exec", "payments:settle"],
    "ttl_minutes": 15
  }'
```
Response:
```json
{
  "agent_id": "agent-payments-worker-3fa8b10e",
  "one_time_code": "LTD-OTC-a1b2c3d4-e5f60718",
  "expires_at": "2026-09-12T11:45:00Z"
}
```

### Step 2: One-Time Edge Registration
On the edge host where the Local Trusted Daemon (`bapedge`) is deployed:
```bash
bapedge register --server http://control-plane.corp:8080 --code LTD-OTC-a1b2c3d4-e5f60718
```
The edge LTD broker:
1. Computes the cryptographic SHA-256 hash of its executing binary.
2. Sends the hash along with the OTC to the control plane.
3. Upon validation, the OTC is burned (replay blocked) and session credentials are stored in `~/.ltd/credentials.json`.

### Step 3: Acquiring Short-Lived Grants
Edge agents obtain short-lived (15–60 min) signed authority tokens:
```bash
curl -X POST http://localhost:8080/api/v1/grants/acquire \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "agent-payments-worker-3fa8b10e",
    "binary_hash": "4e75b1590ef4ac3a871ba7ed2075f32d7a4cfbad773e700b583fcb4643d709fb",
    "scopes": ["cli:exec"]
  }'
```

### Step 4: Central Kill-Switch (Instant Revocation)
If an agent is compromised or decommissioned, revoke it immediately:
```bash
curl -X POST http://localhost:8080/api/v1/agents/revoke \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "agent-payments-worker-3fa8b10e"}'
```
Once revoked, any subsequent request to acquire grants is immediately rejected with HTTP 403 Forbidden.

---

## 8. Offline Edge Resilience & Fail-Secure Design

A fundamental invariant of BAP is that **edge agents must continue operating normally if `bapcontrolplane` is down, without compromising security**:

1. **Local Policy Cache (`~/.ltd/policy/`)**:
   - `bapedge` caches the latest authoritative Cedar policy bundle (`policy.cedar`, `schema.json`, and `policy-state.json`).
2. **Zero Downtime for Developers**:
   - Authorized developer toolchains (`git`, `go`, `python`, `npm`, `cargo`) execute locally in 1–2ms via in-process Cedar evaluation.
   - If `bapcontrolplane` is unreachable, `bapedge sync` logs `[!] OFFLINE RESILIENCE ACTIVE` and cleanly falls back to cached settings.
3. **Strict Retention of Security Invariants**:
   - Secrets protection (`cat .env`, `type .env`, evasive renames like `ren .env`), credential directory protection (`~/.aws`, `~/.ssh`), and egress blocks (`curl`, `wget`) are hard invariants compiled into the cached Cedar rules and remain **100% active offline**.
4. **Persistent Kill-Switch**:
   - If an emergency lock was triggered, `policy-state.json` retains `kill_switch: true`. Network disconnection **cannot bypass** the kill-switch.
5. **Fail-Secure Default**:
   - If no cached policy bundle exists and the central control plane cannot be contacted, `bapedge` strictly denies execution (`exit 1`). It never fails open.

---

## 9. Python Agent SDK (`bap-sdk`) & Corporate Nexus / Artifactory Distribution

For autonomous Python AI agents (e.g., built on **LangChain**, **CrewAI**, **AutoGen**, or **LlamaIndex**), BAP provides a zero-dependency SDK package (`bap-sdk`).

### 3-Line Integration
```python
from bap_sdk import BAPSession, BAPPolicyViolation

with BAPSession(app_id="python-analytics-worker") as bap:
    # 1. Permitted developer tool execution (<2ms)
    res = bap.exec("python --version")
    print("Runtime:", res.stdout)

    # 2. Threat interception: cat .env is blocked before shell creation!
    try:
        bap.exec("cat .env")
    except BAPPolicyViolation as e:
        print("[SHIELD ACTIVATED] BAP blocked credential disclosure:", e)
```

### Building & Publishing to Corporate Nexus
Corporate environments with air-gapped repositories or private PyPI mirrors can package and publish `bap-sdk` with one command:

```cmd
:: Build universal wheel (.whl) and source distribution (.tar.gz)
.\python-agent\build_package.bat

:: Publish to Enterprise Nexus / Artifactory via Twine
twine upload --repository-url https://nexus.internal.company.com/repository/pypi-internal/ python-agent\dist\*
```

Developer teams then simply install from the internal repository:
```bash
pip install bap-sdk --index-url https://nexus.internal.company.com/repository/pypi-internal/simple
```

---

## 10. Rogue & Non-Registered Agent Prevention (6-Layer Security Perimeter)

To guarantee that rogue insiders, compromised worker processes, or unauthorized scripts cannot execute actions outside BAP governance, BAP enforces a strict 6-layer defense model:

1. **Cryptographic Binary Hash Attestation**: Control plane verifies the SHA-256 digest of executing agent binaries against pre-registered whitelists (`/api/v1/agents/register`). Unregistered binaries return `403 Forbidden`.
2. **Single-Use OTC & Fleet Quota Enforcement**: Agent enrollment requires high-entropy One-Time Credentials (`LTD-OTC-xxxx-xxxx`) that burn atomically on first use, or Fleet Tokens (`BAP-FLEET-xxxx-xxxx`) strictly bounded by `max_instances`. Unregistered clones return `401 Unauthorized`.
3. **In-Process Cedar Default-Deny**: The edge policy broker (`bapedge`) evaluates all commands under strict **Default-Deny**. Any unpermitted tool invocation is intercepted before process spawn ($<2\text{ms}$).
4. **Atomic Authority Grant Burning**: Authority tokens (JWT-SVIDs) minted for elevated operations are destroyed upon first presentation via `/api/v1/grants/consume`, neutralizing replay attacks.
5. **Dynamic Central & Persistent Kill-Switch**: Compromised agents or entire application fleets are revoked instantly (`/api/v1/agents/revoke` or `/apps/revoke`). Revocation writes persistent state to edge disks, surviving restarts and network cut-offs.
6. **Dual-Identity Binding & Tamper-Evident Blockchain**: Telemetry binds the human user (`apiKeyHelper` token or authenticated OS user) to the workload SPIFFE ID, with every event chained into a sequential SHA-256 hash log. Rogue actions cannot be wiped or hidden from auditors.

---

## 11. Zero-Trust Gateway PEP: Option 3 (Native Go) & Option 1 (Envoy on Podman)

Client-side hooks and SDKs are **cooperative PEPs**. If an autonomous rogue script or prompt injection refuses to use `bap-sdk` and opens raw HTTP sockets (`import requests; requests.get(...)`), client-side wrappers cannot prevent packets from leaving the network card.

Therefore, **real-world enterprise security requires an Ingress Gateway PEP** (Envoy Proxy, Istio Service Mesh, or `bap-gateway`):
- All backend microservices, core banking APIs, and customer databases are placed behind the Gateway.
- The Gateway intercepts every request via `ext_authz` and requires `Authorization: Bearer <BAP_GRANT>`.
- The Gateway calls `/api/v1/auth/envoy` or `/api/v1/grants/consume` to validate signature, verify SPIFFE claims, and atomically burn the token.
- **Rogue Neutralization**: Any unauthenticated or replayed request is terminated immediately with **HTTP 401/403**. The backend core API is **never reached**.

### Option 3: Pure-Go Native PEP (`bapgateway.exe`) - Integrated Demo
Best for developer laptops, Windows environments, and CIO demos without container runtime friction. Avoids the 2-month infosec approval delay for third-party executables.
```cmd
:: 1. Run the Gateway PEP (port 9090)
.\bapgateway.exe -port 9090 -controlplane http://localhost:8080

:: 2. Test Rogue Script Bypass (Blocked with HTTP 401)
python .\python-agent\rogue_agent.py

:: 3. Test Governed Agent (Authorized with HTTP 200)
python .\python-agent\governed_agent.py
```

### Option 1: Cloud-Native Envoy Proxy on Podman / Docker - Standalone Showcase
Best for Linux production servers, Red Hat OpenShift, and Kubernetes where Envoy is pre-approved:
```bash
# 1. Turn on Envoy Proxy with Podman (port 10000)
cd envoy
./run_envoy_podman.sh       # Linux / WSL / macOS
run_envoy_podman.bat        # Windows

# 2. Run standalone demonstration
python demo_envoy.py        # or demo_envoy_podman.bat / .sh

# 3. Stop Envoy container
./stop_envoy_podman.sh      # or stop_envoy_podman.bat
```
*(For complete details, see [envoy/ENVOY_PODMAN_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/envoy/ENVOY_PODMAN_GUIDE.md)).*

---

## 12. Comprehensive Documentation & Guides

- [ENDPOINT_CONFIGURATION.md](file:///c:/Users/User/pyprj/bapltd/ENDPOINT_CONFIGURATION.md): Complete guide for configuring central BAP Control Plane hosts and gateways for developer laptops via `bap-config.json` and `configure_endpoints.bat`.
- [FEATURES_AND_ARCHITECTURE.md](file:///c:/Users/User/pyprj/bapltd/FEATURES_AND_ARCHITECTURE.md): Master architectural reference, 34-feature matrix, and complete threat defense guide.
- [CIO_DEMO_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/CIO_DEMO_GUIDE.md): Complete executive walkthrough, interactive demo script, talking points, and FAQ for CIO/CISO presentations.
- [envoy/ENVOY_PODMAN_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/envoy/ENVOY_PODMAN_GUIDE.md): Guide for Option 1: Envoy Proxy on Podman/Docker, container commands, and troubleshooting.
- [python-agent/README.md](file:///c:/Users/User/pyprj/bapltd/python-agent/README.md): Developer guide for `bap-sdk`, LangChain/CrewAI integrations, and Nexus distribution.
- [API_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/API_GUIDE.md): Complete REST API specification and CLI reference for all control plane endpoints and edge commands.
- [TESTING_GUIDE.md](file:///c:/Users/User/pyprj/bapltd/TESTING_GUIDE.md): Step-by-step testing guide for automated and manual verification across offline resilience, kill-switch, and multi-agent workflows.


