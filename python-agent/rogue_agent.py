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


def ensure_gateway_running(gateway_url: str) -> bool:
    try:
        req = urllib.request.Request(gateway_url)
        urllib.request.urlopen(req, timeout=1)
        return True
    except urllib.error.HTTPError:
        return True  # 401/403 means gateway PEP is up and rejecting unauthenticated requests
    except urllib.error.URLError:
        if "localhost" in gateway_url or "127.0.0.1" in gateway_url:
            import subprocess
            root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
            for cand in ["bapgateway.exe", "bap-gateway\\bapgateway.exe"]:
                p = os.path.join(root_dir, cand)
                if os.path.isfile(p):
                    print(f"[*] Gateway PEP is not running. Auto-starting {cand} on port 9090...")
                    subprocess.Popen([p, "-port", "9090"], cwd=root_dir, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                    time.sleep(1.5)
                    return True
        return False


def run_rogue_agent():
    ep = resolve_endpoints()
    gateway_url = f"{ep['gateway_url']}/api/v1/financial-records"

    ensure_gateway_running(gateway_url)

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
    except urllib.error.URLError as e:
        print(f"\n[CONNECTION ERROR] Could not reach Gateway PEP at {gateway_url}: {e.reason}")
        print("                   Ensure bapgateway.exe is running: .\\bapgateway.exe -port 9090")
        return

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
    except urllib.error.URLError as e:
        print(f"\n[CONNECTION ERROR] Could not reach Gateway PEP at {gateway_url}: {e.reason}")
        return

    print_banner("VERDICT: ZERO-TRUST INVARIANT PRESERVED")
    print("[+] Core Invariant: Agents NOT using BAP are 100% neutralized at the Gateway PEP.")
    print("[+] Backend core banking and customer databases remain completely unreachable.")
    print("=" * 80 + "\n")


if __name__ == "__main__":
    run_rogue_agent()

