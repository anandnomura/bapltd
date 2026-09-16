#!/usr/bin/env python3
"""
BAP Live Agent Fleet Demonstration
====================================
Demonstrates dynamic agent lifecycle on the BAP Control Plane & UI:
1. Launches 5 governed agents (Python SDK and/or Claude Code).
2. Pauses so you can see all 5 active on the Inspector / Dashboard.
3. Randomly closes 2 agents and pauses so you can watch them disappear.
4. Launches a replacement agent and pauses so you can watch the count increase.
5. Cleanly shuts down all remaining sessions.
"""

import os
import sys
import time
import json
import ssl
import random
import urllib.request
import subprocess
from typing import List, Dict, Any

# Ensure python-agent is in python path
WORKSPACE_ROOT = os.path.abspath(os.path.dirname(__file__))
sys.path.insert(0, os.path.join(WORKSPACE_ROOT, "python-agent"))

from bap_sdk import BAPSession, resolve_endpoints

# TLS context for localhost dev certs
ssl_ctx = ssl.create_default_context()
ssl_ctx.check_hostname = False
ssl_ctx.verify_mode = ssl.CERT_NONE


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
            return json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        return {"error": str(e), "sessions": [], "agents": []}


def count_active_sessions(data: Dict[str, Any]) -> int:
    """Calculates active session count matching inspector_v2 logic (< 15s last active)."""
    sessions = data.get("sessions", [])
    now = time.time()
    active = 0
    for s in sessions:
        if s.get("status") == "active":
            last_ms = 0
            if s.get("last_active_at"):
                try:
                    import datetime
                    iso_str = s["last_active_at"].replace("Z", "+00:00")
                    t = datetime.datetime.fromisoformat(iso_str)
                    last_ms = t.timestamp()
                except Exception:
                    last_ms = now
            if (now - last_ms) <= 15:
                active += 1
    return active


class DemoAgent:
    """Wrapper for either a Python SDK BAPSession or a simulated Claude Code session."""
    def __init__(self, name: str, app_id: str, operator: str, prompt: str, is_claude: bool = False, server_url: str = ""):
        self.name = name
        self.app_id = app_id
        self.operator = operator
        self.prompt = prompt
        self.is_claude = is_claude
        self.server_url = server_url
        self.session_id = f"sess-{'claude' if is_claude else 'py'}-{name.lower().replace(' ', '-')}-{random.randint(100, 999)}"
        self.py_session = None
        self.claude_proc = None
        self.is_alive = False

    def start(self):
        if self.is_claude:
            # Locate real claude.exe executable
            claude_bin = os.path.expanduser(r"~\.local\bin\claude.exe")
            if not os.path.isfile(claude_bin):
                import shutil
                claude_bin = shutil.which("claude") or shutil.which("claude.exe")
            
            # Spawn real claude.exe in its own console window (matching real developer behavior)
            creation_flags = 0
            if sys.platform == "win32":
                creation_flags = subprocess.CREATE_NEW_CONSOLE

            if claude_bin and os.path.isfile(claude_bin):
                self.claude_proc = subprocess.Popen(
                    [claude_bin],
                    creationflags=creation_flags
                )
            else:
                self.claude_proc = subprocess.Popen(
                    [sys.executable, "-c", "import time; time.sleep(3600)"],
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL
                )
            
            # Enroll session on control plane via bapedge session-start
            bapedge_bin = os.path.join(WORKSPACE_ROOT, "bapedge.exe")
            if not os.path.isfile(bapedge_bin):
                bapedge_bin = "bapedge.exe"
            
            start_args = [
                bapedge_bin, "session-start",
                "--server", self.server_url,
                "--session-id", self.session_id,
                "--pid", str(self.claude_proc.pid),
                "--app-id", self.app_id,
                "--prompt", self.prompt
            ]
            env = os.environ.copy()
            env["BAP_USER_PROMPT"] = self.prompt
            env["USERNAME"] = self.operator
            subprocess.run(start_args, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            self.is_alive = True
        else:
            # Python SDK BAPSession
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
        if self.is_claude:
            bapedge_bin = os.path.join(WORKSPACE_ROOT, "bapedge.exe")
            if not os.path.isfile(bapedge_bin):
                bapedge_bin = "bapedge.exe"
            # Gracefully notify control plane via bapedge session-end
            subprocess.run(
                [bapedge_bin, "session-end", "--server", self.server_url, "--session-id", self.session_id],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
            )
            if self.claude_proc:
                try:
                    self.claude_proc.terminate()
                    self.claude_proc.wait(timeout=2)
                except Exception:
                    pass
            self.is_alive = False
        else:
            if self.py_session:
                self.py_session.end(reason="demo termination")
            self.is_alive = False


def print_banner(title: str):
    print("\n" + "=" * 82)
    print(f"  {title}")
    print("=" * 82)


def print_agent_table(agents: List[DemoAgent]):
    print(f"{'#':<3} | {'Type':<12} | {'Name / Role':<24} | {'Operator':<14} | {'Session ID':<30} | {'Status'}")
    print("-" * 105)
    for idx, a in enumerate(agents, 1):
        atype = "Claude Code" if a.is_claude else "Python SDK"
        status = "[ACTIVE]" if a.is_alive else "[CLOSED]"
        print(f"{idx:<3} | {atype:<12} | {a.name:<24} | {a.operator:<14} | {a.session_id:<30} | {status}")
    print("-" * 105)


def main():
    endpoints = resolve_endpoints()
    server_url = endpoints["controlplane_url"]
    admin_token = read_admin_token()

    print_banner("BAP ZERO-TRUST: DYNAMIC AGENT FLEET LIFECYCLE DEMO")
    print(f"[*] Target Control Plane : {server_url}")
    print(f"[*] Admin Token Loaded   : {'YES (' + admin_token[:12] + '...)' if admin_token else 'NO (Viewing masked)'}")
    print(f"[*] Inspector V2 URL     : {server_url}/inspector_v2.html")
    print(f"[*] React Dashboard URL  : https://localhost:8444/dashboard/")
    print("=" * 82)

    # Check connection
    try:
        req = urllib.request.Request(f"{server_url}/api/v1/health")
        with urllib.request.urlopen(req, context=ssl_ctx, timeout=3):
            print("[+] Control Plane is ONLINE and reachable.\n")
    except Exception as e:
        print(f"[-] ERROR: Cannot reach {server_url} ({e}).")
        print("    Please start the control plane first via start_dashboard.bat or start_inspector.bat")
        sys.exit(1)

    print("Choose Fleet Mode to Demonstrate:")
    print("  [1] Python BAP SDK Agents (5 Python autonomous agents)")
    print("  [2] Claude Code Sessions (5 Governed Claude Code instances)")
    print("  [3] Mixed Squad (3 Python SDK Agents + 2 Claude Code Sessions)  [RECOMMENDED]")
    print("")
    choice = input("Select Mode [1/2/3, Enter = 3]: ").strip()
    if choice not in ("1", "2", "3"):
        choice = "3"

    agents_def = [
        ("Financial Analyst", "python-financial-analyst", "Carol Zhang", "Analyze Q3 portfolio volatility and fetch ledger records", False),
        ("Security Auditor", "python-sec-auditor", "Alice Vance", "Scan VPC endpoints for unauthorized egress", False),
        ("DevOps Deployer", "claude-code", "Bob Miller", "Deploy microservice canary to us-east-1 cluster", True),
        ("Cloud SRE Bot", "python-cloud-sre", "David Kim", "Monitor kubernetes cluster node memory pressure", False),
        ("Code Reviewer", "claude-code", "Elena Rostova", "Review PR #402 for path traversal and SQL injection vulnerabilities", True),
    ]

    if choice == "1":
        # All Python
        for i in range(len(agents_def)):
            n, a, op, p, _ = agents_def[i]
            agents_def[i] = (n, "python-agent", op, p, False)
    elif choice == "2":
        # All Claude
        for i in range(len(agents_def)):
            n, a, op, p, _ = agents_def[i]
            agents_def[i] = (n, "claude-code", op, p, True)

    agents: List[DemoAgent] = []
    for n, a, op, p, is_c in agents_def:
        agents.append(DemoAgent(name=n, app_id=a, operator=op, prompt=p, is_claude=is_c, server_url=server_url))

    try:
        # ---------------------------------------------------------------------
        # STAGE 1: Launch 5 Agents
        # ---------------------------------------------------------------------
        print_banner("STAGE 1: LAUNCHING 5 GOVERNED AGENTS INTO BAP CONTROL PLANE")
        for a in agents:
            print(f"  [+] Enrolling & starting: {a.name} ({a.session_id})...")
            a.start()
            time.sleep(0.3)

        time.sleep(1.5)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 82)
        print("  ACTIVE AGENT FLEET TABLE")
        print("=" * 82)
        print_agent_table(agents)
        print(f"\n[*] Total Active Sessions reported by Control Plane: {live_count}")
        print("\n" + "-" * 82)
        print(">>> [PAUSE] Open your browser now to see all 5 agents LIVE on the UI:")
        print(f"    Inspector V2 : {server_url}/inspector_v2.html")
        print(f"    Dashboard    : https://localhost:8444/dashboard/")
        print("-" * 82)
        input("\n>>> Press [ENTER] when ready to randomly CLOSE 2 agents and watch them disappear...")

        # ---------------------------------------------------------------------
        # STAGE 2: Close 2 Agents Randomly
        # ---------------------------------------------------------------------
        print_banner("STAGE 2: TERMINATING 2 AGENTS RANDOMLY")
        kill_indices = sorted(random.sample(range(len(agents)), 2))
        killed_agents = [agents[i] for i in kill_indices]

        for a in killed_agents:
            print(f"  [-] Closing session: {a.name} ({a.session_id})...")
            a.close()

        # Wait for control plane and watcher to update
        time.sleep(2.5)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 82)
        print("  UPDATED FLEET TABLE (2 CLOSED)")
        print("=" * 82)
        print_agent_table(agents)
        print(f"\n[*] Remaining Active Sessions reported by Control Plane: {live_count}")
        print("\n" + "-" * 82)
        print(f">>> [PAUSE] Notice the 2 closed agents have DISAPPEARED from the UI!")
        print(f"    Terminated: {killed_agents[0].name} and {killed_agents[1].name}")
        print(f"    Current Active Count on Cockpit/Dashboard: {live_count}")
        print("-" * 82)
        input("\n>>> Press [ENTER] when ready to launch a NEW replacement agent...")

        # ---------------------------------------------------------------------
        # STAGE 3: Launch 1 Replacement Agent
        # ---------------------------------------------------------------------
        print_banner("STAGE 3: LAUNCHING REPLACEMENT AGENT (COUNT INCREASING)")
        replacement_is_claude = (choice == "2" or (choice == "3" and random.choice([True, False])))
        new_agent = DemoAgent(
            name="Phoenix SRE Assistant",
            app_id="claude-code" if replacement_is_claude else "python-phoenix-sre",
            operator="Marcus Brody",
            prompt="Automated root cause analysis and canary recovery",
            is_claude=replacement_is_claude,
            server_url=server_url
        )
        print(f"  [+] Starting replacement agent: {new_agent.name} ({new_agent.session_id})...")
        new_agent.start()
        agents.append(new_agent)

        time.sleep(2)
        telemetry = get_inspector_telemetry(server_url, admin_token)
        live_count = count_active_sessions(telemetry)

        print("\n" + "=" * 82)
        print("  FLEET TABLE WITH REPLACEMENT AGENT")
        print("=" * 82)
        print_agent_table(agents)
        print(f"\n[*] Total Active Sessions reported by Control Plane: {live_count}")
        print("\n" + "-" * 82)
        print(f">>> [PAUSE] Notice the NEW agent appeared on the UI and the count INCREASED to {live_count}!")
        print("-" * 82)
        input("\n>>> Press [ENTER] to cleanly shut down all demo agents and finish...")

    finally:
        # ---------------------------------------------------------------------
        # STAGE 4: Clean Shutdown
        # ---------------------------------------------------------------------
        print_banner("STAGE 4: CLEAN SHUTDOWN OF ALL DEMO WORKLOADS")
        for a in agents:
            if a.is_alive:
                print(f"  [*] Teardown: {a.name} ({a.session_id})...")
                a.close()
        time.sleep(1.5)
        print("[+] All demo agent workloads have been cleanly closed.")
        print("[+] BAP Zero-Trust demonstration completed successfully!\n")


if __name__ == "__main__":
    main()
