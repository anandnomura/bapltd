#!/usr/bin/env python3
"""
BAP Live Agent Fleet Demonstration
====================================
Demonstrates dynamic agent lifecycle on the BAP Control Plane & UI:
1. Launches N governed agents (Python SDK and/or Claude Code, up to 35+).
2. Pauses so you can see all active agents on the Inspector / Dashboard Radar.
3. Randomly closes agents and pauses so you can watch them disappear in real time.
4. Launches replacement agents and pauses so you can watch the count increase.
5. Cleanly shuts down all remaining sessions upon completion.

Usage:
  python demo_live_agents.py                   # Interactive scale selection (5 to 35)
  python demo_live_agents.py 35                # Launch 35 agents directly
  python demo_live_agents.py --count 35        # Scale up to 35 agents
  python demo_live_agents.py 35 --mode mixed   # 35 agents in Mixed squad mode
  run_agent_demo.bat 35                        # 1-click Windows launcher
"""

import os
import sys
import time
import json
import ssl
import random
import atexit
import argparse
import threading
import urllib.request
import subprocess
from typing import List, Dict, Any, Tuple

# Ensure python-agent is in python path
WORKSPACE_ROOT = os.path.abspath(os.path.dirname(__file__))
sys.path.insert(0, os.path.join(WORKSPACE_ROOT, "python-agent"))

from bap_sdk import BAPSession, resolve_endpoints

# TLS context for localhost dev certs
ssl_ctx = ssl.create_default_context()
ssl_ctx.check_hostname = False
ssl_ctx.verify_mode = ssl.CERT_NONE


# Master roster of realistic enterprise agent personas (40 distinct workloads)
MASTER_AGENT_ROSTER: List[Tuple[str, str, str, str, bool]] = [
    # Format: (Name/Role, app_id, Operator Name, Prompt/Task, is_claude_default)
    ("Financial Portfolio Analyst", "python-financial-analyst", "Carol Zhang", "Analyze Q3 portfolio volatility and fetch ledger records", False),
    ("Security Boundary Auditor", "python-sec-auditor", "Alice Vance", "Scan VPC endpoints for unauthorized egress rules", False),
    ("DevOps Blue-Green Deployer", "claude-code", "Bob Miller", "Deploy microservice canary to us-east-1 cluster", True),
    ("Cloud SRE Memory Sentry", "python-cloud-sre", "David Kim", "Monitor kubernetes cluster node memory pressure", False),
    ("Application Code Reviewer", "claude-code", "Elena Rostova", "Review PR #402 for path traversal and SQL injection vulnerabilities", True),
    ("Kubernetes Pod Autoscaler", "python-k8s-scaler", "Marcus Brody", "Evaluate HPA metrics and scale worker deployment nodes", False),
    ("IAM Privilege Minimizer", "python-iam-guard", "Sarah Connor", "Audit over-privileged IAM roles and revoke unused wildcards", False),
    ("Database Query Optimizer", "python-db-optimizer", "Alex Rivera", "Analyze slow query logs and generate missing index recommendations", False),
    ("API Contract Validator", "claude-code", "Priya Patel", "Verify OpenAPI 3.1 schema adherence against production routes", True),
    ("Zero-Trust Mesh Sentinel", "python-zt-sentinel", "Jordan Lee", "Validate mTLS certificate rotation and SPIFFE trust bundles", False),
    ("Frontend Asset Builder", "claude-code", "Mei Lin", "Bundle Vite production assets and verify Subresource Integrity hashes", True),
    ("Kafka Stream Rebalancer", "python-kafka-sentry", "Carlos Gomez", "Rebalance partition consumer groups on event streaming cluster", False),
    ("Secret Lease Rotator", "python-secret-rotator", "Nina Patel", "Rotate ephemeral database credentials and invalidate stale leases", False),
    ("Terraform Drift Detector", "claude-code", "Liam O'Connor", "Detect and reconcile out-of-band cloud infrastructure changes", True),
    ("Container CVE Inspector", "python-cve-scanner", "Fatima Al-Mansoor", "Inspect base container layers for critical glibc vulnerabilities", False),
    ("Chaos Engineering Probe", "python-chaos-bot", "Kenji Sato", "Simulate availability zone network partition in staging cluster", False),
    ("Log Anomaly Classifier", "python-log-analyst", "Rachel Green", "Apply statistical outlier detection to edge gateway access logs", False),
    ("Prompt Injection Firewall", "python-llm-firewall", "Victor Vance", "Evaluate incoming tool call payloads against adversarial jailbreak signatures", False),
    ("CI/CD Gatekeeper Pilot", "claude-code", "Chloe Dubois", "Orchestrate multi-stage blue-green deployment pipelines", True),
    ("Envoy Sidecar Monitor", "python-mesh-guard", "Sam Wilson", "Monitor Istio Envoy sidecar egress and route latency spikes", False),
    ("Global DNS Health Probe", "python-dns-probe", "Tariq Hadid", "Verify geo-routed DNS resolution latency across global regions", False),
    ("SOC2 Compliance Notary", "python-compliance-bot", "Beatrice Webb", "Generate SOC2 automated evidence manifests from audit logs", False),
    ("Redis Memory Evictor", "python-cache-manager", "Lucas Silva", "Prune stale cache keys and monitor memory fragmentation", False),
    ("Backup Integrity Validator", "python-dr-probe", "Yasmine Benali", "Verify automated point-in-time PostgreSQL backup snapshot integrity", False),
    ("Release Notes Synthesizer", "claude-code", "Oliver Twist", "Extract semantic commit changes and author markdown release notes", True),
    ("Egress Firewall Sentinel", "python-egress-guard", "Devante Washington", "Block unexpected outbound UDP packets on worker instances", False),
    ("GraphQL Schema Linter", "claude-code", "Sophia Martinez", "Enforce deprecation tags and query depth limits on GraphQL gateway", True),
    ("Vault AppRole Renewer", "python-vault-renewer", "Hakeem Olajuwon", "Renew short-lived AppRole tokens for worker microservices", False),
    ("ALB Ingress Traffic Tuner", "python-alb-tuner", "Ingrid Lindholm", "Optimize weighted round-robin distribution across healthy target groups", False),
    ("Static Semgrep Inspector", "claude-code", "Ryan Chang", "Run Semgrep rules on untrusted PR submissions before merge", True),
    ("Elasticsearch Shard Sizer", "python-es-reindex", "Aisha Bello", "Monitor index shard sizes and split oversized indices", False),
    ("WAF ModSecurity Sync", "python-waf-sentinel", "Diego Rodriguez", "Synchronize OWASP Top 10 ModSecurity CRS rule updates", False),
    ("Cloud FinOps Cost Hunter", "python-finops-bot", "Hannah Schmidt", "Flag unexpected cloud cost spikes in Athena and Snowflake queries", False),
    ("Wildcard ACME Renewer", "python-cert-renewer", "Gabriel Dupont", "Validate ACME DNS-01 challenge completion for wildcard certs", False),
    ("Release SHA Attestor", "python-spiffe-verifier", "Natalie Portman", "Audit binary SHA-256 digests against authoritative release manifest", False),
    ("OpenTelemetry Forwarder", "python-otel-collector", "Zachary Taylor", "Batch trace spans and export to Jaeger backend collector", False),
    ("Deadlock Graph Watcher", "python-db-lockwatch", "Uma Thurman", "Identify circular lock graph chains on primary database engine", False),
    ("Mock Service Fabricator", "claude-code", "Walter White", "Generate wiremock stubs for third-party payment provider endpoints", True),
    ("GitOps Sync Controller", "python-gitops-sync", "Xavier Woods", "Reconcile ArgoCD application state with git main branch", False),
    ("Hardware Enclave Guard", "python-hsm-guard", "Yvonne Strahovski", "Verify TPM 2.0 PCR registers for enclave workstation enrollment", False),
]


def safe_input(prompt_text: str, default: str = "") -> str:
    """Reads user input safely, returning default on EOFError or KeyboardInterrupt."""
    try:
        val = input(prompt_text).strip()
        return val if val else default
    except (EOFError, KeyboardInterrupt):
        print("")
        return default


def read_admin_token() -> str:
    """Reads administrative bearer token from .bap-admin-token if present."""
    token_file = os.path.join(WORKSPACE_ROOT, ".bap-admin-token")
    if os.path.isfile(token_file):
        try:
            with open(token_file, "r", encoding="utf-8") as f:
                return f.read().strip()
        except Exception:
            pass
    return os.getenv("BAP_ADMIN_TOKEN", "")


def get_inspector_telemetry(server_url: str, admin_token: str) -> Dict[str, Any]:
    """Fetches telemetry from /api/v1/inspector/data."""
    req = urllib.request.Request(f"{server_url}/api/v1/inspector/data")
    if admin_token:
        req.add_header("Authorization", f"Bearer {admin_token}")
    try:
        with urllib.request.urlopen(req, context=ssl_ctx, timeout=4) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            if isinstance(data, dict):
                return data
            return {"sessions": [], "agents": []}
    except Exception as e:
        return {"error": str(e), "sessions": [], "agents": []}


def count_active_sessions(data: Dict[str, Any]) -> int:
    """Calculates active session count matching inspector_v2 logic (< 15s last active)."""
    if not isinstance(data, dict):
        return 0
    sessions = data.get("sessions") or []
    now = time.time()
    active = 0
    for s in sessions:
        if isinstance(s, dict) and s.get("status") == "active":
            last_ms = 0
            if s.get("last_active_at"):
                try:
                    import datetime
                    iso_str = s["last_active_at"].replace("Z", "+00:00")
                    t = datetime.datetime.fromisoformat(iso_str)
                    last_ms = t.timestamp()
                except Exception:
                    last_ms = now
_ALL_SPAWNED_AGENTS: List["DemoAgent"] = []


def _cleanup_all():
    """Guarantees all background workloads and heartbeats are torn down upon exit."""
    for a in list(_ALL_SPAWNED_AGENTS):
        if a.is_alive:
            try:
                a.close()
            except Exception:
                pass


atexit.register(_cleanup_all)


class DemoAgent:
    """Wrapper for either a Python SDK BAPSession or a governed Claude Code session."""
    def __init__(
        self,
        name: str,
        app_id: str,
        operator: str,
        prompt: str,
        is_claude: bool = False,
        server_url: str = "",
        headless: bool = True
    ):
        self.name = name
        self.app_id = app_id
        self.operator = operator
        self.prompt = prompt
        self.is_claude = is_claude
        self.server_url = server_url.rstrip("/")
        self.headless = headless
        
        # Clean slug for session ID
        clean_slug = "".join(c if c.isalnum() else "-" for c in name.lower()).strip("-")[:20]
        prefix = "claude" if is_claude else "py"
        self.session_id = f"sess-{prefix}-{clean_slug}-{random.randint(100, 999)}"
        
        self.py_session = None
        self.proc = None
        self.is_alive = False
        self._hb_stop = None
        self._hb_thread = None
        _ALL_SPAWNED_AGENTS.append(self)

    def start(self):
        if self.is_claude:
            # Spawn background worker process with valid PID
            if not self.headless and sys.platform == "win32":
                creation_flags = subprocess.CREATE_NEW_CONSOLE
            else:
                creation_flags = subprocess.CREATE_NO_WINDOW if sys.platform == "win32" else 0

            try:
                self.proc = subprocess.Popen(
                    [sys.executable, "-c", "import time; time.sleep(3600)"],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                    creationflags=creation_flags
                )
            except Exception:
                self.proc = None

            # Register session directly with Control Plane REST endpoint
            payload = {
                "session_id": self.session_id,
                "user_prompt": self.prompt,
                "app_id": self.app_id,
                "instance_id": self.session_id,
                "user_id": self.operator,
                "user_email": f"{self.operator.lower().replace(' ', '.')}@enterprise.internal",
                "spiffe_id": f"spiffe://bap.internal/app/{self.app_id}/instance/{self.session_id}",
                "client_pid": self.proc.pid if self.proc else os.getpid(),
                "hostname": os.getenv("COMPUTERNAME", "localhost"),
                "client_type": "claude-code",
                "metadata": {
                    "source": "demo_fleet",
                    "role": self.name
                }
            }
            try:
                req = urllib.request.Request(
                    f"{self.server_url}/api/v1/sessions/start",
                    data=json.dumps(payload).encode("utf-8"),
                    headers={"Content-Type": "application/json"}
                )
                with urllib.request.urlopen(req, context=ssl_ctx, timeout=3):
                    pass
            except Exception:
                pass

            self.is_alive = True

            # Start automated heartbeat thread so Claude session remains active on radar
            self._hb_stop = threading.Event()
            def _claude_hb_worker():
                while self._hb_stop and not self._hb_stop.wait(4.0):
                    if not self.is_alive:
                        break
                    try:
                        hb_payload = {
                            "session_id": self.session_id,
                            "app_id": self.app_id,
                            "agent_id": self.session_id
                        }
                        req = urllib.request.Request(
                            f"{self.server_url}/api/v1/sessions/heartbeat",
                            data=json.dumps(hb_payload).encode("utf-8"),
                            headers={"Content-Type": "application/json"}
                        )
                        with urllib.request.urlopen(req, context=ssl_ctx, timeout=3):
                            pass
                    except Exception:
                        pass

            self._hb_thread = threading.Thread(
                target=_claude_hb_worker,
                daemon=True,
                name=f"bap-hb-claude-{self.session_id}"
            )
            self._hb_thread.start()

        else:
            # Python SDK BAPSession (includes built-in background heartbeat)
            self.py_session = BAPSession(
                app_id=self.app_id,
                session_id=self.session_id,
                user_id=self.operator,
                user_email=f"{self.operator.lower().replace(' ', '.')}@enterprise.internal",
                user_prompt=self.prompt,
                server_url=self.server_url
            )
            self.py_session.start()
            self.is_alive = True

    def close(self):
        if not self.is_alive:
            return

        if self._hb_stop:
            self._hb_stop.set()

        if self.is_claude:
            # Directly deregister via Control Plane
            try:
                end_payload = {
                    "session_id": self.session_id,
                    "reason": "demo termination"
                }
                req = urllib.request.Request(
                    f"{self.server_url}/api/v1/sessions/end",
                    data=json.dumps(end_payload).encode("utf-8"),
                    headers={"Content-Type": "application/json"}
                )
                with urllib.request.urlopen(req, context=ssl_ctx, timeout=3):
                    pass
            except Exception:
                pass

            if self.proc:
                try:
                    self.proc.terminate()
                    self.proc.wait(timeout=1)
                except Exception:
                    pass
            self.is_alive = False
        else:
            if self.py_session:
                try:
                    self.py_session.end(reason="demo termination")
                except Exception:
                    pass
            self.is_alive = False


def print_banner(title: str):
    print("\n" + "=" * 90)
    print(f"  {title}")
    print("=" * 90)


def print_agent_table(agents: List[DemoAgent]):
    print(f"{'#':<3} | {'Type':<12} | {'Name / Role':<30} | {'Operator':<18} | {'Session ID':<30} | {'Status'}")
    print("-" * 105)
    for idx, a in enumerate(agents, 1):
        atype = "Claude Code" if a.is_claude else "Python SDK"
        status = "[ACTIVE]" if a.is_alive else "[CLOSED]"
        print(f"{idx:<3} | {atype:<12} | {a.name:<30} | {a.operator:<18} | {a.session_id:<30} | {status}")
    print("-" * 105)


def build_fleet_roster(count: int, mode: str) -> List[Tuple[str, str, str, str, bool]]:
    """Builds a fleet definition of exact length `count` matching the chosen mode."""
    roster = []
    pool_len = len(MASTER_AGENT_ROSTER)

    for i in range(count):
        base_name, base_app, base_op, base_prompt, base_is_claude = MASTER_AGENT_ROSTER[i % pool_len]
        iteration = i // pool_len

        name = f"{base_name} #{iteration + 1}" if iteration > 0 else base_name
        op = f"{base_op} #{iteration + 1}" if iteration > 0 else base_op

        if mode == "1":
            # All Python SDK
            app_id = base_app if "claude" not in base_app else "python-agent"
            is_claude = False
        elif mode == "2":
            # All Claude Code
            app_id = "claude-code"
            is_claude = True
        else:
            # Mixed Squad
            if mode == "3" and (i % 2 == 1):
                app_id = "claude-code"
                is_claude = True
            else:
                app_id = base_app
                is_claude = base_is_claude

        roster.append((name, app_id, op, base_prompt, is_claude))

    return roster


def parse_args():
    parser = argparse.ArgumentParser(
        description="BAP Live Agent Fleet Demonstration (Scale up to 35+ Agents)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python demo_live_agents.py                   # Interactive scale selection
  python demo_live_agents.py 35                # Launch 35 agents directly
  python demo_live_agents.py --count 35        # Scale up to 35 agents
  python demo_live_agents.py 35 --mode mixed   # 35 agents in Mixed squad mode
  run_agent_demo.bat 35                        # 1-click Windows launcher
        """
    )
    parser.add_argument(
        "fleet_size",
        nargs="?",
        type=int,
        default=None,
        help="Number of agents to simulate (e.g. 5 to 35)"
    )
    parser.add_argument(
        "--count", "-n",
        type=int,
        default=None,
        help="Number of agents to simulate (1 to 50, default: interactive or 5)"
    )
    parser.add_argument(
        "--mode", "-m",
        choices=["1", "2", "3", "python", "claude", "mixed"],
        default=None,
        help="Fleet mode: 1=Python SDK, 2=Claude Code, 3=Mixed Squad"
    )
    parser.add_argument(
        "--headless",
        action="store_true",
        default=True,
        help="Run without popping visible console windows (default: True)"
    )
    return parser.parse_args()


def main():
    args = parse_args()
    endpoints = resolve_endpoints()
    server_url = endpoints["controlplane_url"]
    admin_token = read_admin_token()

    print_banner("BAP ZERO-TRUST: DYNAMIC AGENT FLEET LIFECYCLE DEMO")
    print(f"[*] Target Control Plane : {server_url}")
    print(f"[*] Admin Token Loaded   : {'YES (' + admin_token[:12] + '...)' if admin_token else 'NO (Viewing masked)'}")
    print(f"[*] Inspector V2 URL     : {server_url}/inspector_v2.html")
    print(f"[*] React Dashboard URL  : https://localhost:8444/dashboard/")
    print("=" * 90)

    # Check connection
    try:
        req = urllib.request.Request(f"{server_url}/api/v1/health")
        with urllib.request.urlopen(req, context=ssl_ctx, timeout=3):
            print("[+] Control Plane is ONLINE and reachable.\n")
    except Exception as e:
        print(f"[-] ERROR: Cannot reach {server_url} ({e}).")
        print("    Please start the control plane first via start_controlplane_supervisor.bat or start_dashboard.bat")
        sys.exit(1)

    # Resolve target fleet scale
    target_count = args.count if args.count is not None else args.fleet_size
    if target_count is None:
        print("Select Fleet Concurrency Scale:")
        print("  [1] Standard Fleet  : 5  Agents  (Fast baseline demo)")
        print("  [2] High-Load Fleet : 35 Agents  (Demonstrates UI concurrency & Live Radar scale) [RECOMMENDED]")
        print("  [3] Custom Fleet    : Enter any number between 1 and 50")
        print("")
        scale_choice = safe_input("Select Concurrency Scale [1/2/number, Enter = 35]: ", default="35")
        if scale_choice in ("1", "5"):
            target_count = 5
        elif scale_choice in ("2", "35") or not scale_choice:
            target_count = 35
        elif scale_choice.isdigit():
            target_count = max(1, min(50, int(scale_choice)))
        else:
            target_count = 35

    # Resolve fleet mode
    mode = args.mode
    if mode in ("python", "1"):
        mode = "1"
    elif mode in ("claude", "2"):
        mode = "2"
    elif mode in ("mixed", "3"):
        mode = "3"
    else:
        print(f"\nChoose Agent Ecosystem Type (Scaling to {target_count} agents):")
        print(f"  [1] Python BAP SDK Autonomous Agents ({target_count} agents)")
        print(f"  [2] Claude Code Governed Sessions ({target_count} agents)")
        claude_half = target_count // 2
        py_half = target_count - claude_half
        print(f"  [3] Mixed Squad ({claude_half} Claude Code + {py_half} Python SDK)  [RECOMMENDED]")
        print("")
        mode_choice = safe_input("Select Mode [1/2/3, Enter = 3]: ", default="3")
        if mode_choice not in ("1", "2", "3"):
            mode = "3"
        else:
            mode = mode_choice

    # Build agent roster
    fleet_def = build_fleet_roster(target_count, mode)
    headless = args.headless or (target_count > 5)

    agents: List[DemoAgent] = []
    for n, a, op, p, is_c in fleet_def:
        agents.append(DemoAgent(
            name=n,
            app_id=a,
            operator=op,
            prompt=p,
            is_claude=is_c,
            server_url=server_url,
            headless=headless
        ))

    try:
        # ---------------------------------------------------------------------
        # STAGE 1: Launch N Agents
        # ---------------------------------------------------------------------
        print_banner(f"STAGE 1: LAUNCHING {target_count} GOVERNED AGENTS INTO BAP CONTROL PLANE")
        for idx, a in enumerate(agents, 1):
            atype = "Claude Code" if a.is_claude else "Python SDK"
            print(f"  [{idx:02d}/{target_count}] Enrolling & starting {atype}: {a.name} ({a.session_id})...")
            a.start()
            # Stagger launch slightly to avoid HTTP storm while keeping speed high
            time.sleep(0.08 if target_count > 15 else 0.2)

        time.sleep(2.0)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 90)
        print(f"  ACTIVE AGENT FLEET TABLE ({len(agents)} REGISTERED WORKLOADS)")
        print("=" * 90)
        print_agent_table(agents)
        print(f"\n[*] Total Active Sessions reported by Control Plane: {live_count} / {target_count}")
        print("\n" + "-" * 90)
        print(f">>> [PAUSE] Open your browser now to see all {live_count} agents LIVE on the UI Radar:")
        print(f"    Inspector V2 : {server_url}/inspector_v2.html")
        print(f"    Dashboard    : https://localhost:8444/dashboard/")
        print("-" * 90)

        # Dynamic kill count
        if target_count <= 5:
            kill_count = min(2, target_count)
        else:
            kill_count = max(2, int(target_count * 0.25))  # e.g. 8 for 35

        safe_input(f"\n>>> Press [ENTER] when ready to randomly CLOSE {kill_count} agents and watch them drop on the UI...")

        # ---------------------------------------------------------------------
        # STAGE 2: Terminate Subset of Agents Randomly
        # ---------------------------------------------------------------------
        print_banner(f"STAGE 2: TERMINATING {kill_count} AGENTS RANDOMLY (WATCHING LIVE RADAR DROP)")
        kill_indices = sorted(random.sample(range(len(agents)), kill_count))
        killed_agents = [agents[i] for i in kill_indices]

        for a in killed_agents:
            print(f"  [-] Closing session: {a.name} ({a.session_id})...")
            a.close()

        # Wait for control plane and watcher to update
        time.sleep(2.5)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 90)
        print(f"  UPDATED FLEET TABLE ({kill_count} CLOSED, {len(agents) - kill_count} REMAINING ACTIVE)")
        print("=" * 90)
        print_agent_table(agents)
        print(f"\n[*] Remaining Active Sessions reported by Control Plane: {live_count}")
        print("\n" + "-" * 90)
        print(f">>> [PAUSE] Notice the {kill_count} closed agents have DISAPPEARED from the UI Radar!")
        print(f"    Current Active Count on Cockpit/Dashboard: {live_count}")
        print("-" * 90)

        # Dynamic replacement count
        if target_count <= 5:
            repl_count = 1
        else:
            repl_count = max(2, kill_count // 2)  # e.g. 4 for 35

        safe_input(f"\n>>> Press [ENTER] when ready to launch {repl_count} NEW replacement agents...")

        # ---------------------------------------------------------------------
        # STAGE 3: Launch Replacement Agents
        # ---------------------------------------------------------------------
        print_banner(f"STAGE 3: LAUNCHING {repl_count} REPLACEMENT AGENTS (WATCHING COUNT INCREASE)")
        for r_idx in range(repl_count):
            repl_is_claude = (mode == "2" or (mode == "3" and random.choice([True, False])))
            new_agent = DemoAgent(
                name=f"Phoenix Auto-Recovery Sentry #{r_idx + 1}" if repl_count > 1 else "Phoenix SRE Assistant",
                app_id="claude-code" if repl_is_claude else f"python-phoenix-sre-{r_idx + 1}",
                operator=f"Marcus Brody #{r_idx + 1}" if repl_count > 1 else "Marcus Brody",
                prompt="Automated root cause analysis, telemetry surge recovery, and canary deployment",
                is_claude=repl_is_claude,
                server_url=server_url,
                headless=headless
            )
            print(f"  [+] Starting replacement agent: {new_agent.name} ({new_agent.session_id})...")
            new_agent.start()
            agents.append(new_agent)
            time.sleep(0.1)

        time.sleep(2.0)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 90)
        print(f"  FLEET TABLE WITH REPLACEMENT AGENTS ({len(agents)} TOTAL REGISTERED)")
        print("=" * 90)
        print_agent_table(agents)
        print(f"\n[*] Total Active Sessions reported by Control Plane: {live_count}")
        print("\n" + "-" * 90)
        print(f">>> [PAUSE] Notice {repl_count} NEW agents appeared on the UI and the count INCREASED to {live_count}!")
        print("-" * 90)
        safe_input("\n>>> Press [ENTER] to cleanly shut down all demo agents and finish...")

    finally:
        # ---------------------------------------------------------------------
        # STAGE 4: Clean Shutdown
        # ---------------------------------------------------------------------
        print_banner("STAGE 4: CLEAN SHUTDOWN OF ALL FLEET WORKLOADS")
        for a in agents:
            if a.is_alive:
                print(f"  [*] Teardown: {a.name} ({a.session_id})...")
                a.close()
        time.sleep(1.5)
        print("\n[+] All demo agent workloads have been cleanly closed.")
        print("[+] BAP Zero-Trust fleet demonstration completed successfully!\n")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n[-] Demonstration stopped by user. All workloads cleanly closed.\n")
        sys.exit(0)
    except Exception as e:
        print(f"\n[-] Unexpected error during demonstration: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
