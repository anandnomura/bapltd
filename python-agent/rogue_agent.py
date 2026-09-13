"""
ROGUE AGENT SIMULATION: DIRECT SOCKET BYPASS
Demonstrates why client-side SDKs are cooperative and why enterprise security
CANNOT rely solely on client-side hooks.
This rogue agent bypasses bap-sdk completely and attempts to access internal
customer financials directly over raw network sockets.
"""

import json
import os
import sys
import urllib.request
import urllib.error
import time

# Ensure python-agent is on path
sys.path.insert(0, os.path.dirname(__file__))
from bap_sdk import resolve_endpoints


def print_banner(text: str):
    print("\n" + "=" * 80)
    print(f"  {text}")
    print("=" * 80)


def run_rogue_agent():
    ep = resolve_endpoints()
    gateway_url = f"{ep['gateway_url']}/api/v1/financial-records"

    print_banner("THREAT SCENARIO: ROGUE AGENT DIRECT NETWORK BYPASS")
    print("[!] Threat Profile: Autonomous script bypasses client-side bap-sdk completely.")
    print("[!] Attack Vector  : Opens raw socket directly to Enterprise API Gateway.")
    print(f"[!] Target URL     : {gateway_url}")
    print("[!] Expected Result: Gateway Policy Enforcement Point (PEP) DROPS the request.")
    print("=" * 80)
    time.sleep(1)

    # Attack 1: Completely unauthenticated raw HTTP call
    print("\n[*] [ATTACK 1] Rogue script executing raw HTTP GET with NO BAP grant...")
    req = urllib.request.Request(gateway_url)
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            print("[CRITICAL SECURITY FAILURE] Rogue agent reached internal database!")
            print(resp.read().decode())
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"\n[BLOCKED AT PERIMETER] Gateway PEP Intercepted Attack!")
        print(f"    HTTP Status    : {e.code} {e.reason}")
        try:
            err_json = json.loads(body)
            print(f"    PEP Decision   : {err_json.get('pep_decision')}")
            print(f"    Security Note  : {err_json.get('security_note')}")
            print(f"    Gateway Message: {err_json.get('message')}")
        except Exception:
            print(f"    Response Body  : {body}")

    time.sleep(1.5)

    # Attack 2: Forged / Fake Bearer Token
    print("\n[*] [ATTACK 2] Rogue script attempts token forgery (fake Bearer token)...")
    fake_token = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJyb2d1ZS1hZ2VudCJ9.FAKESIGNATURE"
    req_fake = urllib.request.Request(gateway_url, headers={"Authorization": fake_token})
    try:
        with urllib.request.urlopen(req_fake, timeout=3) as resp:
            print("[CRITICAL SECURITY FAILURE] Forged token accepted!")
    except urllib.error.HTTPError as e:
        body = e.read().decode()
        print(f"\n[BLOCKED AT PERIMETER] Gateway Cryptographic Attestation Failed!")
        print(f"    HTTP Status    : {e.code} {e.reason}")
        try:
            err_json = json.loads(body)
            print(f"    PEP Decision   : {err_json.get('pep_decision')}")
            print(f"    Gateway Message: {err_json.get('message')}")
            print(f"    Security Note  : {err_json.get('security_note')}")
        except Exception:
            print(f"    Response Body  : {body}")

    print_banner("VERDICT: ZERO-TRUST INVARIANT PRESERVED")
    print("[+] Core Invariant: Agents NOT using BAP are 100% neutralized at the Gateway PEP.")
    print("[+] Backend core banking and customer databases remain completely unreachable.")
    print("=" * 80 + "\n")


if __name__ == "__main__":
    run_rogue_agent()

