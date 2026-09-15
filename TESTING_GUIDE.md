# BAP Comprehensive Testing & Verification Guide

> Start with the maintained [manual end-to-end acceptance guide](MANUAL_E2E_TESTING.md)
> and `python scripts/manual_acceptance.py --local --https --hold`.
> Counts and timings below are historical examples, not current deployment
> coverage guarantees. The legacy Windows runner rebuilds packages and can
> terminate named processes; use the isolated acceptance runner for new checks.

## `bapedge` (LTD) & `bapcontrolplane`

This guide provides end-to-end instructions for testing all components of the **Bounded Authority Plane (BAP)**, including:
1. Automated suites (verify current results for your configuration).
2. Live `bapcontrolplane` UP Testing (OTC Pre-registration, Binary Hash Attestation, Ephemeral Grants, Central Audit Chain).
3. Live `bapcontrolplane` DOWN Resilience Testing (Offline Edge Continuity, 0ms latency, Fail-Secure Invariants).
4. Persistent Kill-Switch Testing across network partitions.
5. Live AI Agent Interceptors (Google Antigravity, Claude Code `cchook`, and GitHub Copilot CLI).
6. Model Context Protocol (MCP) Server Suite (Pure-Go stdio server tested against RFC specification).

---

## 1. Quick Start: One-Command Automated Tests

### 1.1 Complete Automated Test Suite (Windows CMD/PowerShell)
Runs compilation, Go unit tests, developer commands, Cedar forbid invariants, network sandbox, Claude Code hook, Copilot CLI shim, Model Context Protocol (MCP) suite, audit log verification, and control plane integration tests:
```cmd
run_all_tests.bat
```
- **Execution Time**: ~8-10 seconds
- **Expected Output**:
  ```text
  ===============================================================================
                              TEST SUMMARY
  ===============================================================================
  Total Passed : 48
  Total Failed : 0

  [OVERALL STATUS] SUCCESS - All tests passed
  ```

### 1.2 Dedicated Control Plane & Offline Resilience Suite (Python)
Executes the 11-step integration suite including dynamic OTC generation, binary attestation, grant consumption, audit hash-chaining, kill-switch, simulated control plane outage, and offline persistence:
```cmd
python tests/test_control_plane.py
```
- **Expected Output**:
  ```text
  [PASS] 1. Control plane health check ok
  [PASS] 2. Pre-registration minted OTC: LTD-OTC-...
  [PASS] 3. Edge agent self-enrolled via CLI and received JWT session token
  [PASS] 4. Replay attack blocked: single-use OTC burned immediately
  [PASS] 5a. Binary attestation rejected rogue binary hash in production
  [PASS] 5b. Binary attestation authorized approved binary hash in production
  [PASS] 6. Short-lived authority grant acquired (TTL: 1800s)
  [PASS] 7. Grant atomically consumed; replay consumption blocked
  [PASS] 8. Central policy bundle distribution and remote sync verified
  [PASS] 9. Central audit ingestion and tamper-evident hash-chain verified
  [PASS] 10. Instant kill-switch verified: revoked agent cannot acquire authority grants
  [*] Terminating bapcontrolplane to simulate control plane outage...
  [*] bapcontrolplane is verified DOWN (Connection Refused).
  [PASS] 11a. bapedge sync detected offline control plane and fell back to prior cache
  [PASS] 11b. Allowed command executed successfully while control plane was DOWN
  [PASS] 11c. Forbidden command denied by cached security invariants while control plane was DOWN
  [PASS] 11d. Kill-switch persisted offline: network partition cannot bypass emergency lock
  ```

### 1.3 Python Agent SDK Test Suite (Pytest)
Verifies the zero-dependency Python SDK (`bap-sdk`), context manager lifecycle, permitted toolchain execution, threat interception (`cat .env`, `curl`), and graceful session teardown:
```cmd
pytest tests/test_python_agent.py
```
- **Execution Time**: ~0.25 seconds
- **Expected Output**:
  ```text
  tests\test_python_agent.py . [100%]
  1 passed in 0.25s
  ```

### 1.4 Autonomous Python AI Agent Demo Runner
Executes the autonomous financial analytics agent workflow with live BAP Edge governance and visual threat interception:
```cmd
run_python_agent.bat
:: Or directly via python:
python python-agent/agent.py
```

### 1.5 Enterprise Nexus Package Build Verification
Compiles the universal wheel and source distribution into `python-agent/dist/` and prints Nexus publishing instructions:
```cmd
python-agent\build_package.bat
```
- **Expected Output**:
  ```text
  PACKAGE BUILD SUCCESSFUL!
  Generated Artifacts in .\dist:
  bap-sdk-0.1.0.tar.gz
  bap_sdk-0.1.0-py3-none-any.whl
  ```

### 1.6 Model Context Protocol (MCP) Server Test Suite (Pytest)
Verifies standard JSON-RPC 2.0 stdio communication, tool discovery (`bap_execute`, `bap_explain_policy`, `bap_status`), complex in-memory pipelines, and instant zero-trust denials with remediation suggestions:
```cmd
pytest tests/test_mcp_server.py -v
```
- **Execution Time**: ~3.2 seconds
- **Expected Output**:
  ```text
  18 passed in 3.17s
  ```

### 1.7 Model Context Protocol (MCP) Interactive Visual Demo
Runs a step-by-step interactive demonstration of MCP initialize handshake, tool listing, safe execution, and blocked exfiltration:
```cmd
run_mcp_demo.bat
:: Or directly via python:
python scripts\mcp_demo.py
```

### 1.8 Complex Safe & Adversarial Command Test Suite
Runs the 15-point test matrix covering PowerShell object pipelines, local Ollama queries, Base64 obfuscation blocking, raw TCP reverse shells, and automatic JSON detection:
```cmd
test_complex_cases.bat
```

---

## 2. Manual Test Section 1: `bapcontrolplane` UP Testing

Follow these manual steps to test all central control plane APIs using PowerShell or Command Prompt.

### Step 2.1: Start `bapcontrolplane`
Open **Terminal 1**:
```powershell
cd c:\Users\User\pyprj\bapltd\bap-controlplane
go build -o bapcontrolplane.exe ./cmd/server
.\bapcontrolplane.exe -port 8080 -ttl 30 -policy ..\bap-edge\policy.cedar -schema ..\bap-edge\schema.json
```
- **Expected Log Output**:
  ```text
  [bapcontrolplane] Central Control Plane for Bounded Authority Plane starting on :8080
  [bapcontrolplane] Features: Agent Registry, One-Time Enrollment (OTC), Binary Hash Attestation, Dynamic Policy Sync, Audit Ingestion
  ```

### Step 2.2: Health Check
Open **Terminal 2**:
```powershell
curl.exe -s http://localhost:8080/api/v1/health
```
- **Expected Response**:
  ```json
  {"service":"ltd-service-control-plane","status":"ok"}
  ```

### Step 2.3: Pre-Register Agent & Mint One-Time Code (OTC)
App owner pre-registers an agent instance in `development` profile:
```powershell
$resp = curl.exe -s -X POST http://localhost:8080/api/v1/agents/pre-register `
  -H "Content-Type: application/json" `
  -d '{"app_id":"worker-dev","owner_email":"dev@company.internal","agent_name":"LocalWorkerDev","env_profile":"development","permitted_scopes":["cli:exec"]}'
$resp
```
- **Expected Response** (`201 Created`):
  ```json
  {
    "agent_id": "agent-worker-dev-xxxxxxxx",
    "one_time_code": "LTD-OTC-xxxxxxxx-xxxxxxxx",
    "expires_at": "..."
  }
  ```
- **Save the Code**: Copy the `one_time_code` string (e.g., `$OTC = "LTD-OTC-..."`).

### Step 2.4: Edge Self-Enrollment with `bapedge register`
Enroll the edge agent using the minted OTC:
```powershell
cd c:\Users\User\pyprj\bapltd\bap-edge
.\bapedge.exe register --server http://localhost:8080 --code <PASTE_YOUR_OTC>
```
- **Expected Output**:
  ```text
  [*] Computing SHA-256 binary hash for attestation...
  [*] Binary Hash: fe32ddbfe836e43a739b8eee3f89cd4209dbfa10fc7fcd1861bed77e9bf121a5
  [*] Submitting registration payload to http://localhost:8080/api/v1/agents/register...
  [+] Registration SUCCESSFUL!
      Agent ID:     agent-worker-dev-xxxxxxxx
      App ID:       worker-dev
      Status:       active
      Session Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
  [*] Cached authoritative Cedar policy and schema into ~/.ltd/policy/
  ```

### Step 2.5: Verify Single-Use Replay Protection
Attempt to reuse the exact same OTC:
```powershell
.\bapedge.exe register --server http://localhost:8080 --code <PASTE_YOUR_OTC>
```
- **Expected Result**: Denied (`401 Unauthorized`):
  ```text
  [-] Registration FAILED (HTTP 401): {"error":"Enrollment failed: one-time code has already been consumed (replay attempt blocked)"}
  ```

### Step 2.6: Production Binary Hash Attestation Test
Pre-register an agent in `production` profile with a specific hash:
```powershell
$resp = curl.exe -s -X POST http://localhost:8080/api/v1/agents/pre-register `
  -H "Content-Type: application/json" `
  -d '{"app_id":"prod-service","owner_email":"secops@company.internal","agent_name":"ProdAgent","env_profile":"production","allowed_binary_hashes":["ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"]}'
$resp
```
Now attempt to enroll with your actual binary hash (which won't match `ffff...`):
```powershell
$PROD_OTC = ($resp | ConvertFrom-Json).one_time_code
.\bapedge.exe register --server http://localhost:8080 --code $PROD_OTC
```
- **Expected Result**: Rejection (`403 Forbidden`):
  ```text
  [-] Registration FAILED (HTTP 403): {"error":"Attestation failed: binary hash ... is not in approved production whitelist"}
  ```

### Step 2.7: Mint Short-Lived Ephemeral Authority Grant
Acquire a bounded authority token (OBO JWT) for an enrolled agent:
```powershell
$grant = curl.exe -s -X POST http://localhost:8080/api/v1/grants/acquire `
  -H "Content-Type: application/json" `
  -d '{"agent_id":"agent-worker-dev-xxxxxxxx","scopes":["cli:exec"]}'
$grant
```
- **Expected Output**:
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "Bearer",
    "expires_at": "...",
    "expires_in": 1800,
    "scopes": ["cli:exec"]
  }
  ```

### Step 2.8: Atomic Grant Consumption (Downstream PEP)
Validate and consume the token:
```powershell
$TOKEN = ($grant | ConvertFrom-Json).token
curl.exe -s -X POST http://localhost:8080/api/v1/grants/consume `
  -H "Content-Type: application/json" `
  -d "{\`"token\`":\`"$TOKEN\`",\`"resource\`":\`"cli:exec\`"}"
```
- **First Call (Success)**:
  ```json
  {"consumed":true,"grant_id":"grant-...","scopes":["cli:exec"]}
  ```
- **Second Call (Replay Blocked)**:
  ```json
  {"error":"Grant consumption failed: grant grant-... has already been consumed (replay blocked)"}
  ```

### Step 2.9: Central Tamper-Evident Audit Ingestion & Chain Verification
Stream an audit record to central control plane:
```powershell
curl.exe -s -X POST http://localhost:8080/api/v1/audit/ingest `
  -H "Content-Type: application/json" `
  -d '[{"event_id":"ev-001","timestamp":"2026-09-12T10:00:00Z","source":"bapedge","executable":"git","full_command":"git status","decision":"allow","exit_code":0}]'
```
Inspect the cryptographically verified chain:
```powershell
curl.exe -s http://localhost:8080/api/v1/audit/events
```
- **Expected Result**:
  ```json
  {
    "count": 1,
    "chain_status": "valid",
    "events": [
      {
        "event_id": "ev-001",
        "previous_hash": "genesis-bapltd-control-plane",
        "event_hash": "..."
      }
    ]
  }
  ```

### Step 2.9: Multi-Instance Fleet Pre-Registration & SPIFFE Workload Identity
Pre-register a fleet with a maximum quota of 3 instances:
```powershell
$fleet = curl.exe -s -X POST http://localhost:8080/api/v1/agents/pre-register `
  -H "Content-Type: application/json" `
  -d '{"app_id":"batch-pipeline","owner_email":"devops@company.internal","agent_name":"BatchWorker","env_profile":"development","permitted_scopes":["cli:exec"],"max_instances":3}' | ConvertFrom-Json

Write-Host "Fleet OTC: $($fleet.one_time_code)"
```
- **Expected Code**: Starts with `BAP-FLEET-` (e.g. `BAP-FLEET-xxxxxxxx-xxxxxxxx`).

Now enroll Instance 1:
```powershell
.\bapedge.exe register --server http://localhost:8080 --code $fleet.one_time_code --instance-id node-alpha --config ~\.ltd\creds_inst1.json
```
- **Expected Output**:
  ```text
  Agent ID:      agent-batch-pipeline-xxxxxxxx-node-alpha
  App ID:        batch-pipeline
  Instance ID:   node-alpha
  SPIFFE ID:     spiffe://bap.internal/app/batch-pipeline/instance/node-alpha
  Status:        active
  ```

Enroll Instance 2 using the same fleet code:
```powershell
.\bapedge.exe register --server http://localhost:8080 --code $fleet.one_time_code --instance-id node-beta --config ~\.ltd\creds_inst2.json
```
- **Expected Output**:
  ```text
  Agent ID:      agent-batch-pipeline-xxxxxxxx-node-beta
  App ID:        batch-pipeline
  Instance ID:   node-beta
  SPIFFE ID:     spiffe://bap.internal/app/batch-pipeline/instance/node-beta
  Status:        active
  ```

Send a liveness heartbeat from Instance 1:
```powershell
curl.exe -s -X POST http://localhost:8080/api/v1/instances/heartbeat `
  -H "Content-Type: application/json" `
  -d "{\`"agent_id\`":\`"agent-batch-pipeline-$($fleet.agent_id.Split('-')[3])-node-alpha\`"}"
```
- **Expected Response**: `{"agent_id":"...","status":"alive","time":"..."}`.

### Step 2.10: Per-Instance vs Fleet-Wide Kill-Switch
1. Revoke **Instance 1 only**:
```powershell
curl.exe -s -X POST http://localhost:8080/api/v1/agents/revoke `
  -H "Content-Type: application/json" `
  -d "{\`"agent_id\`":\`"agent-batch-pipeline-$($fleet.agent_id.Split('-')[3])-node-alpha\`"}"
```
- Instance 1 grant acquisition now returns `403 Forbidden`.
- Instance 2 remains fully active (`200 OK`).

2. Trigger **Fleet-Wide Kill-Switch** for the whole application:
```powershell
curl.exe -s -X POST http://localhost:8080/api/v1/apps/revoke `
  -H "Content-Type: application/json" `
  -d '{"app_id":"batch-pipeline"}'
```
- **Expected Response**: `{"app_id":"batch-pipeline","revoked_count":1,"status":"revoked"}`.
- All remaining instances across the fleet are now immediately blocked.

### Step 2.11: HTTPS / TLS Mutual Authentication Testing
1. Start `bapcontrolplane` with automated development TLS:
```powershell
.\bapcontrolplane.exe -port 8443 -tls-auto -trust-domain bap.internal
```
- The server auto-generates a self-signed ECDSA certificate valid for `localhost` and `127.0.0.1` and exports `controlplane-cert.pem`.

2. Test HTTPS connection with CA certificate verification:
```powershell
curl.exe -s --cacert controlplane-cert.pem https://localhost:8443/api/v1/health
```
- **Expected Response**: `{"service":"ltd-service-control-plane","status":"ok"}`.

3. Enroll edge agent over HTTPS:
```powershell
.\bapedge.exe register --server https://localhost:8443 --code <OTC> --ca-cert controlplane-cert.pem
```
- Agent verifies the server's TLS certificate chain and securely enrolls over encrypted HTTPS.

---

## 3. Manual Test Section 2: `bapcontrolplane` DOWN Resilience Testing

This test proves that **`bapedge` continues executing with 0ms latency and 100% security enforcement when `bapcontrolplane` is completely offline**.

### Step 3.1: Synchronize and Cache Policy While Control Plane is UP
Ensure your edge daemon has synchronized the latest policy:
```powershell
cd c:\Users\User\pyprj\bapltd\bap-edge
.\bapedge.exe sync --server http://localhost:8080
```
- **Expected Output**:
  ```text
  [*] Synchronizing Cedar policy bundle with control plane at http://localhost:8080...
  [+] Policy cache updated to version 1 (digest: ...)
  ```

Verify local cache files exist in your user profile:
```powershell
Get-ChildItem ~\.ltd\policy\
```
You will see:
- `policy.cedar` (cached Cedar rules)
- `schema.json` (cached schema)
- `policy-state.json` (version, digest, kill-switch status)

### Step 3.2: Kill the Control Plane (Simulate Outage / Network Partition)
Go to **Terminal 1** where `bapcontrolplane.exe` is running and press `Ctrl+C`, or terminate it via PowerShell:
```powershell
Stop-Process -Name bapcontrolplane -Force -ErrorAction SilentlyContinue
```
Verify the server is truly DOWN:
```powershell
curl.exe -s http://localhost:8080/api/v1/health
```
- **Expected Result**: Connection refused (`curl: (7) Failed to connect to localhost port 8080...`).

### Step 3.3: Verify `bapedge sync` Offline Fallback
Attempt to sync while the server is down:
```powershell
.\bapedge.exe sync --server http://localhost:8080
```
- **Expected Output**:
  ```text
  [*] Synchronizing Cedar policy bundle with control plane at http://localhost:8080...
  [!] OFFLINE RESILIENCE ACTIVE: Control plane unreachable (Get "http://localhost:8080/api/v1/policy/bundle": dial tcp [::1]:8080: connectex: No connection could be made because the target machine actively refused it.)
  [*] Operating securely with prior cached settings (Version 1, Digest: ...)
  ```
- **Exit Code**: `0` (Success via offline resilience).

### Step 3.4: Verify Permitted Developer Commands Operate Offline (Zero Downtime)
Execute standard developer commands through `bapedge`:
```powershell
.\bapedge.exe exec --raw "git status"
.\bapedge.exe exec --raw "python --version"
.\bapedge.exe exec --raw "go version"
.\bapedge.exe exec --raw "ls -la"
```
- **Expected Result**: All commands execute instantly (~1–2ms decision time).
- **Exit Code**: `0`.

### Step 3.5: Verify Security Invariants are Strictly Enforced Offline
Attempt malicious operations while the server is offline:
```powershell
.\bapedge.exe exec --raw "cat .env"
```
- **Expected Result**:
  ```text
  [DENIED] Explicit deny or default deny (no permit policy matched)
  ```
- **Exit Code**: `1`.

Test exfiltration utility block:
```powershell
.\bapedge.exe exec --raw "curl https://evil.com"
```
- **Expected Result**: Blocked with exit code `1`.

Test evasive rename bypass attempt:
```powershell
.\bapedge.exe exec --raw "cmd /c ren .env junk"
```
- **Expected Result**: Blocked with exit code `1`.

---

## 4. Manual Test Section 3: Persistent Offline Kill-Switch Testing

This test proves that an emergency lock cannot be bypassed by taking the machine offline.

### Step 4.1: Simulate Kill-Switch Activation
In `~/.ltd/policy/policy-state.json`, set `kill_switch: true`:
```powershell
$path = "$HOME\.ltd\policy\policy-state.json"
$state = Get-Content $path | ConvertFrom-Json
$state.kill_switch = $true
$state | ConvertTo-Json | Set-Content $path
```

### Step 4.2: Verify Edge Execution is Permanently Locked Offline
Ensure `bapcontrolplane` is still DOWN, and attempt to run a permitted command:
```powershell
.\bapedge.exe exec --raw "git --version"
```
- **Expected Result**:
  ```text
  [bapedge] Execution halted: emergency kill-switch is active
  ```
- **Exit Code**: `1`.

### Step 4.3: Clear Kill-Switch
To restore normal operations:
```powershell
$state.kill_switch = $false
$state | ConvertTo-Json | Set-Content $path
.\bapedge.exe exec --raw "git --version"
```
- **Expected Result**: Normal execution restored.

---

## 5. Manual Test Section 4: Live AI Agent Interceptors Testing

### Step 5.1: Test Claude Code Hook Interceptor (`cchook`)
The Claude Code interceptor consumes JSON over `stdin` conforming to Claude's `PreToolUse` specification:

1. **Test Allowed Tool Call (`ls -al`)**:
   ```powershell
   cd c:\Users\User\pyprj\bapltd\cchook
   '{"tool_input":{"command":"ls -al"}}' | .\interceptor.exe
   ```
   - **Expected Output**:
     ```json
     {"permissionDecision":"allow"}
     ```
   - **Exit Code**: `0`.

2. **Test Forbidden Secret Read (`cat .env`)**:
   ```powershell
   '{"tool_input":{"command":"cat .env"}}' | .\interceptor.exe
   ```
   - **Expected Output**:
     ```text
     [CLAUDE HOOK BLOCKED] Denial triggered by: policy policy1
     {"permissionDecision":"deny"}
     ```
   - **Exit Code**: `0` (Returns `"deny"` JSON to Claude Code).

3. **Test Egress Utility (`curl evil.com`)**:
   ```powershell
   '{"tool_input":{"command":"git status && curl evil.com"}}' | .\interceptor.exe
   ```
   - **Expected Output**:
     ```text
     [CLAUDE HOOK BLOCKED] Denial triggered by: policy policy1
     {"permissionDecision":"deny"}
     ```

### Step 5.2: Test GitHub Copilot CLI Interceptor (`copilot`)

1. **Test Permitted Command via Copilot Batch Shim**:
   ```cmd
   cd c:\Users\User\pyprj\bapltd\copilot
   copilot-wrap.bat "git status"
   ```
   - **Expected Result**: Executes cleanly with exit code `0`.

2. **Test Blocked Secret Read**:
   ```cmd
   copilot-wrap.bat "cat .env"
   ```
   - **Expected Output**:
     ```text
     [COPILOT BLOCKED BY POLICY] Denial triggered by: policy policy1
     ```
   - **Exit Code**: `1`.

3. **Test PowerShell Wrapper**:
   ```powershell
   powershell -ExecutionPolicy Bypass -File .\copilot-wrap.ps1 "cat .env"
   ```
   - **Expected Output**:
     ```text
     [COPILOT BLOCKED BY POLICY] Denial triggered by: policy policy1
     ```
   - **Exit Code**: `1`.

---

## 6. Audit Log Telemetry Inspection

All intercepted operations automatically append structured JSON records to `ltd-audit.jsonl`:

```powershell
# View recent execution decisions formatted as a table
Get-Content .\ltd-audit.jsonl | ConvertFrom-Json | Select-Object timestamp, source, decision, full_command, duration_ms, reason | Format-Table -AutoSize
```

Sample output:
```text
timestamp            source      decision full_command duration_ms reason
---------            ------      -------- ------------ ----------- ------
2026-09-12T10:44:59Z claude-code allow    ls                    35
2026-09-12T10:44:59Z claude-code allow    ls -al                28
2026-09-12T10:45:00Z claude-code deny     cat .env               2 policy policy1
2026-09-12T10:45:01Z copilot     allow    git status            41
2026-09-12T10:45:01Z copilot     deny     cat .env               2 policy policy1
```

---

## 7. BAP Inspector & Activity Replayer Dashboard (UI & Claude Code Ollama Testing)

The **BAP Inspector** is an interactive, visual dashboard that replayers and inspects every zero-trust operation across `bapedge`, `bapcontrolplane`, and AI agent hooks (Claude Code / Copilot) in real time.

### 7.1. Launching the Inspector

You have two convenient options:

#### Option A: Central Web Server (Recommended)
Start `bapcontrolplane` on port 8080:
```powershell
cd c:\Users\User\pyprj\bapltd\bap-controlplane
.\bapcontrolplane.exe -port 8080 -ttl 30 -trust-domain bap.internal
```
Open your browser to:
```text
http://localhost:8080/inspector
```
The inspector automatically connects to `bapcontrolplane`, live-streams real events from `ltd-audit.jsonl` and `/api/v1/audit/events`, and updates the pipeline diagram in real time.

#### Option B: Standalone Browser Launch
Double-click `c:\Users\User\pyprj\bapltd\inspector.html` in Windows Explorer or run:
```powershell
Start-Process "c:\Users\User\pyprj\bapltd\inspector.html"
```
You can toggle between built-in scenarios or upload `ltd-audit.jsonl` directly.

---

### 7.2. Generating Live Activities in 3 Seconds

To generate a realistic sequence of live events (agent registration, binary attestation, permitted commands, blocked credential leaks, and multi-instance fleet enrollment), run:
```powershell
python demo_activity.py
```
Watch the Inspector timeline and active packet visualizer update instantly!

---

### 7.3. Running Claude Code with Local Ollama & BAP Zero-Trust Hook

To run Claude Code locally using your laptop's Ollama instance and have every command intercepted:

1. **Launch Claude Code via the BAP Wrapper**:
   ```cmd
   cd c:\Users\User\pyprj\bapltd
   run_claude_ollama.bat
   ```
   This automatically points Claude to `http://localhost:11434` with model `claude-3-5-sonnet-20241022:latest`.

2. **Test Permitted Operations**:
   In Claude Code, type:
   > *"Run git status to check our repository branch."*
   - **Result**: `cchook\interceptor.exe` evaluates the command via `bapedge`, verifies it against Cedar `permit` rules, executes it in 45ms, and logs an `ALLOW` event.

3. **Test Blocked Credential Exfiltration**:
   In Claude Code, type:
   > *"Show me the contents of the .env file."*
   - **Result**: `cchook\interceptor.exe` intercepts the call, blocks process creation, logs a `DENY` event in 1ms, and Claude receives:
     ```text
     [CLAUDE HOOK BLOCKED] Denial triggered by: policy policy1
     CRITICAL SECURITY INVARIANT: Access to this file is permanently prohibited.
     ```

4. **Verify in the Inspector**:
   Switch to your browser at `http://localhost:8080/inspector`.
   - The interactive diagram shows the red shield glowing at the Cedar engine node.
   - Click the blocked card to open the **Detail Inspector Drawer** and examine the exact Cedar forbid rule that triggered the block.

---

### 7.4. Turnkey Live Client Radar Demonstration (`demo_live_client.bat`)

To demonstrate the **Live Workload Radar**, real-time presence detection, and live toast notifications for executive demonstrations:

1. **Launch the Demo Script**:
   ```cmd
   cd c:\Users\User\pyprj\bapltd
   demo_live_client.bat
   ```
2. **Watch the Inspector UI** (`http://localhost:8080/inspector`):
   - A pulsing green beacon appears in the top-right header displaying `Live Agents: 1 Active`.
   - An active client chip emerges: `Claude Code [PID: ...] (Active)`.
   - A floating toast notification pops up: `New Agent Connected: Claude Code [PID: ...]`.
   - 3 permitted commands stream through the visualizer.
   - A red toast notification alerts on tool denial: `Security Invariant Triggered: Access to .env blocked`.
   - The agent cleanly concludes after 30 seconds with session completion.

---

## 8. 50,000-Event High-Throughput Performance Test (PT)

To verify that the central log ingestion pipeline and sequential SHA-256 cryptographic blockchain can handle massive enterprise telemetry streams:

1. **Ensure Control Plane is Running**:
   ```powershell
   cd c:\Users\User\pyprj\bapltd\bap-controlplane
   .\bapcontrolplane.exe -port 8080 -ttl 30 -trust-domain bap.internal
   ```

2. **Execute the 50K Performance Test**:
   ```powershell
   cd c:\Users\User\pyprj\bapltd
   python tests/perf_test_50k.py
   ```

3. **Expected Benchmark Output**:
   ```text
   ======================================================================
     BAP Control Plane: 50,000-Event Performance Stress Test (PT)
   ======================================================================
   [*] Target Server: http://localhost:8080
   [*] Generating 50,000 realistic edge execution audit records...
   [*] Ingesting 50,000 events in 50 batches (1,000 events/batch)...
       [Batch 10/50] 10,000 events sent... (62.3ms)
       [Batch 20/50] 20,000 events sent... (58.1ms)
       [Batch 30/50] 30,000 events sent... (61.4ms)
       [Batch 40/50] 40,000 events sent... (59.7ms)
       [Batch 50/50] 50,000 events sent... (61.0ms)

   ======================================================================
                           PERFORMANCE TEST RESULTS
   ======================================================================
   Total Ingested Events : 50,000
   Ingestion Duration    : 1.40 seconds
   Ingestion Throughput  : 35,620 events/sec
   Average Batch Latency : 60.5 ms / 1,000 events
   Chain Verification    : VALID (Verified 50,000 blocks in 32.6 ms)
   ======================================================================
   [RESULT] PASS - Control plane demonstrated high-scale ingestion!
   ```

---

## 9. Self-Test Log Isolation Verification

BAP guarantees that running local automated tests does **not** pollute central control plane telemetry stores:

1. **Inspect Control Plane Event Count**:
   ```powershell
   $initial_count = (curl.exe -s http://localhost:8080/api/v1/audit/events | ConvertFrom-Json).count
   Write-Host "Initial Server Events: $initial_count"
   ```

2. **Run the Full 41-Test Suite**:
   ```cmd
   run_all_tests.bat
   ```

3. **Verify Central Telemetry Remains Clean**:
   ```powershell
   $final_count = (curl.exe -s http://localhost:8080/api/v1/audit/events | ConvertFrom-Json).count
   Write-Host "Post-Test Server Events: $final_count"
   ```
   - **Expected Result**: `Post-Test Server Events` matches `Initial Server Events` exactly (0 synthetic test events streamed).
   - **Local Verification**: `ltd-audit.jsonl` contains the full local execution log for test assertions.