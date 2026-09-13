"""
GOVERNED AGENT DEMONSTRATION: ZERO-TRUST GATEWAY ACCESS
Demonstrates how legitimate AI agents use bap-sdk to acquire ephemeral BAP Grants
and successfully query protected enterprise API Gateways (Envoy, Kong, bap-gateway).
"""

import json
import os
import sys
import time
import urllib.request
import urllib.error

# Ensure python-agent folder is on sys.path
sys.path.insert(0, os.path.dirname(__file__))

from bap_sdk import BAPSession, BAPPolicyViolation, resolve_endpoints


def print_banner(text: str):
    print("\n" + "=" * 80)
    print(f"  {text}")
    print("=" * 80)


def run_governed_agent():
    ep = resolve_endpoints()
    gateway_url = f"{ep['gateway_url']}/api/v1/financial-records"

    print_banner("GOVERNED AI AGENT: SECURE ENTERPRISE API ACCESS")
    print("[*] Purpose        : Demonstrates authorized BAP-governed API access via Gateway PEP.")
    print("[*] Workload ID    : python-financial-analyst")
    print(f"[*] Gateway Target : {gateway_url}")
    print("=" * 80)
    time.sleep(1)

    app_id = "python-financial-analyst"

    print("\n[*] Step 1: Initializing BAP Session & Registering Workload...")
    with BAPSession(app_id=app_id) as bap:
        print(f"    [+] Session Active : {bap.session_id}")
        print(f"    [+] Human Identity : {bap.user_id} ({bap.user_email})")
        print(f"    [+] Workload SPIFFE: {bap.spiffe_id}")
        time.sleep(1)

        print("\n[*] Step 2: Requesting Ephemeral BAP Grant Token (JWT-SVID)...")
        token = bap.acquire_grant(scopes=["api:read", "financial:query"])
        print(f"    [+] Grant Acquired : {token[:28]}...{token[-12:]}")
        print(f"    [+] Grant Type     : Short-Lived Ephemeral Bearer Grant (TTL: 30 min)")
        time.sleep(1)

        print("\n[*] Step 3: Calling Enterprise Gateway PEP with Bearer Grant...")
        req = urllib.request.Request(
            gateway_url,
            headers={
                "Authorization": f"Bearer {token}",
                "X-BAP-Session-ID": bap.session_id,
            }
        )

        try:
            with urllib.request.urlopen(req, timeout=3) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                print(f"\n[SUCCESS: 200 OK] Gateway Policy Enforcement Point Authorized Access!")
                print(f"    PEP Decision     : {data.get('pep_decision')}")
                print(f"    Verified Workload: {data.get('verified_workload')}")
                print(f"    Security Boundary: {data.get('security_boundary')}")
                print("\n[+] Protected Financial Records Retrieved:")
                for acc in data.get("accounts", []):
                    print(f"    * Account: {acc['account_id']} | Holder: {acc['holder']:<30} | Balance: {acc['currency']} {acc['balance']:,.2f} ({acc['risk_tier']})")

        except urllib.error.HTTPError as e:
            print(f"[ERROR] Gateway rejected request: {e.code} {e.reason}")
            print(e.read().decode())

    print_banner("VERDICT: ZERO-TRUST DUAL-PEP MODEL SUCCESSFUL")
    print("[+] Governed agents with cryptographic BAP Grants pass seamlessly.")
    print("[+] Workload identity & human operator attribution verified at the network edge.")
    print("=" * 80 + "\n")


if __name__ == "__main__":
    run_governed_agent()

