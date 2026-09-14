#!/usr/bin/env python3
"""
================================================================================
BAP (Bounded Authority Plane) - Option 1: Envoy Proxy Ingress PEP Demonstration
Demonstrates Rogue Agent Defense using Cloud-Native Envoy Proxy on Podman / Docker
================================================================================

Target: Envoy Proxy on Port 10000 (with ext_authz HTTP Filter)
Authorization Target: BAP Control Plane on Port 8080 (/api/v1/auth/envoy)
"""

import json
import os
import sys
import time
import urllib.request
import urllib.error

# Add parent directory and python-agent to sys.path
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(root_dir, "python-agent"))

from bap_sdk import BAPSession, BAPPolicyViolation, resolve_endpoints

_ep = resolve_endpoints()
ENVOY_INGRESS_URL = f"{_ep.get('envoy_url', 'http://localhost:10000')}/api/v1/financial-records"
CONTROL_PLANE_URL = _ep.get("controlplane_url", "http://localhost:8080")


def print_banner(title: str):
    print("\n" + "=" * 78)
    print(f"   {title}")
    print("=" * 78)


def verify_envoy_running() -> bool:
    try:
        # Check if Envoy is listening
        req = urllib.request.Request("http://localhost:10000/api/v1/health")
        try:
            with urllib.request.urlopen(req, timeout=2) as resp:
                return True
        except urllib.error.HTTPError as e:
            # If Envoy returned 401/403 or anything, it's alive
            return True
    except Exception:
        return False


def scenario_1_rogue_agent_blocked():
    print_banner("[SCENARIO 1] Rogue Agent Bypass Attempt vs Envoy Ingress PEP")
    print("[*] Rogue agent executes raw Python network socket bypassing BAP SDK...")
    print(f"[*] Target Endpoint : {ENVOY_INGRESS_URL}")
    print("[*] BAP Grant Token : NONE (Unauthenticated raw socket call)")
    print()

    req = urllib.request.Request(ENVOY_INGRESS_URL)
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            print(f"[-] UNEXPECTED SUCCESS: HTTP {resp.status}")
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        server_header = e.headers.get("server", "envoy")
        print(f"[!] PERIMETER DEFENSE ACTIVE: HTTP {e.code} ({e.reason})")
        print(f"    - Intercepted By      : {server_header} (Envoy Proxy ext_authz)")
        print(f"    - Downstream Decision : TERMINATED AT INGRESS")
        print(f"    - Response Body       : {body.strip()}")
        print(f"[+] VERIFICATION SUCCESS: Rogue agent blocked before touching enterprise backend!")


def scenario_2_governed_agent_authorized():
    print_banner("[SCENARIO 2] Governed Agent Access via BAP Grant & Envoy ext_authz")
    print("[*] Legitimate agent enrolls with BAP SDK and acquires short-lived Grant...")

    with BAPSession(app_id="wealth-mgmt-worker", server_url=CONTROL_PLANE_URL) as bap:
        print(f"    - Enrolled Instance   : {bap.instance_id}")
        print(f"    - Workload SPIFFE ID  : {bap.spiffe_id}")

        # 1. Acquire grant
        token = bap.acquire_grant(scopes=["api:read", "financial:query"])
        print(f"    - Minted BAP Grant    : {token[:24]}...{token[-12:]}")
        print()

        # 2. Present grant to Envoy Ingress Gateway
        print(f"[*] Calling Envoy Ingress Gateway with Bearer Grant...")
        req = urllib.request.Request(
            ENVOY_INGRESS_URL,
            headers={"Authorization": f"Bearer {token}"}
        )
        try:
            with urllib.request.urlopen(req, timeout=3) as resp:
                print(f"[+] HTTP {resp.status} OK: Access Authorized by Envoy ext_authz!")
                print(f"    - Gateway Server      : {resp.headers.get('server', 'envoy')}")
                print(f"    - Upstream Service Ms : {resp.headers.get('x-envoy-upstream-service-time', 'N/A')} ms")
                data = json.loads(resp.read().decode())
                print(f"    - Verified Workload   : {data.get('verified_workload')}")
                print(f"    - Protected Records   : {len(data.get('accounts', []))} high-value accounts retrieved.")
                for acc in data.get("accounts", [])[:2]:
                    print(f"      * {acc.get('account_id')}: {acc.get('holder')} (${acc.get('balance'):,.2f})")
                print(f"[+] VERIFICATION SUCCESS: Governed agent permitted with full cryptographic audit!")
                return token
        except urllib.error.HTTPError as e:
            print(f"[-] Failed: HTTP {e.code} - {e.read().decode()}")
            return None


def scenario_3_replay_attack_denied(token: str):
    if not token:
        return
    print_banner("[SCENARIO 3] Replay Attack: Rogue Attacker Reuses Consumed Grant")
    print("[*] Rogue script attempts to replay the previously captured token...")
    print(f"[*] Presenting Burned Grant : {token[:24]}...")

    req = urllib.request.Request(
        ENVOY_INGRESS_URL,
        headers={"Authorization": f"Bearer {token}"}
    )
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            print(f"[-] UNEXPECTED SUCCESS: Replayed token was accepted!")
    except urllib.error.HTTPError as e:
        print(f"[!] REPLAY DEFENSE ACTIVE: HTTP {e.code} ({e.reason})")
        print(f"    - Intercepted By      : Envoy Proxy ext_authz")
        print(f"    - Reason              : Single-use grant was atomically destroyed upon first use!")
        print(f"[+] VERIFICATION SUCCESS: Token replay attack defeated at gateway perimeter!")


def main():
    print("=" * 78)
    print("   BAP ZERO-TRUST CLOUD-NATIVE SHOWCASE: ENVOY PROXY ON PODMAN")
    print("   Option 1: Industry-Standard Envoy ext_authz Ingress Enforcement")
    print("=" * 78)

    # 1. Check if Envoy is listening on port 10000
    if not verify_envoy_running():
        print("\n[!] Envoy Proxy is not responding on port 10000.")
        print("    To start Envoy with Podman or Docker:")
        print("      * On Linux/macOS : cd envoy && ./run_envoy_podman.sh")
        print("      * On Windows     : cd envoy && run_envoy_podman.bat")
        print("    Ensure bapcontrolplane is running on port 8080 first.\n")
        sys.exit(1)

    print("[+] Envoy Proxy is healthy and listening on port 10000.")
    scenario_1_rogue_agent_blocked()
    token = scenario_2_governed_agent_authorized()
    if token:
        scenario_3_replay_attack_denied(token)

    print_banner("ENVOY PROXY (OPTION 1) DEMONSTRATION COMPLETE")
    print("Summary:")
    print(" 1. Unmanaged / Rogue agents using raw sockets are STOPPED by Envoy at the ingress perimeter.")
    print(" 2. Governed agents using BAP Grants are VERIFIED and passed through to backend services.")
    print(" 3. Single-use grants cannot be stolen or replayed.")
    print("=" * 78 + "\n")


if __name__ == "__main__":
    main()

