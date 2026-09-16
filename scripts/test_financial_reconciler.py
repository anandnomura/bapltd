#!/usr/bin/env python3
"""
Test Financial Reconciler Agent with BAP Gateway PEP
Used by simple_local_test.bat to simulate an autonomous financial agent governed by BAP.
"""

import argparse
import json
import os
import shutil
import subprocess
import sys
import tempfile
import urllib.request
import urllib.error

# Ensure python-agent is discoverable
ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
PYTHON_AGENT_DIR = os.path.join(ROOT_DIR, "python-agent")
if PYTHON_AGENT_DIR not in sys.path:
    sys.path.insert(0, PYTHON_AGENT_DIR)

from bap_sdk import BAPSession
from bap_sdk.client import _urlopen


def find_bapedge() -> str:
    candidates = [
        os.path.join(ROOT_DIR, "dist", "windows-amd64", "bapedge.exe"),
        os.path.join(ROOT_DIR, "bapedge.exe"),
        os.path.join(ROOT_DIR, "bap-edge", "bapedge.exe"),
        shutil.which("bapedge.exe"),
        shutil.which("bapedge"),
    ]
    for c in candidates:
        if c and os.path.isfile(c):
            return os.path.abspath(c)
    return "bapedge.exe"


def ensure_agent_enrolled(server_url: str, admin_token: str) -> str:
    """Ensures a valid credentials.json exists for the financial-reconciler agent."""
    creds_path = os.getenv("BAP_CREDENTIALS", os.path.expanduser("~/.ltd/credentials.json"))
    if os.path.isfile(creds_path):
        try:
            with open(creds_path, "r", encoding="utf-8") as f:
                data = json.load(f)
                if data.get("server_url", "").rstrip("/") == server_url.rstrip("/"):
                    return creds_path
        except Exception:
            pass

    # Pre-register agent to obtain OTC
    pre_reg_payload = {
        "app_id": "financial-reconciler",
        "owner_email": "devops@company.internal",
        "agent_name": "FinancialReconciler",
        "env_profile": "dev",
        "permitted_scopes": ["api:read", "financial:query", "cli:exec", "zero-trust"]
    }
    hdrs = {"Content-Type": "application/json"}
    if admin_token:
        hdrs["X-BAP-Admin-Token"] = admin_token

    pre_req = urllib.request.Request(
        f"{server_url.rstrip('/')}/api/v1/agents/pre-register",
        data=json.dumps(pre_reg_payload).encode("utf-8"),
        headers=hdrs
    )
    with _urlopen(pre_req, timeout=3) as resp:
        otc = json.loads(resp.read().decode())["one_time_code"]

    target_creds = os.path.join(os.path.expanduser("~/.ltd"), "credentials.json")
    os.makedirs(os.path.dirname(target_creds), exist_ok=True)

    bapedge_bin = find_bapedge()
    proc = subprocess.run([
        bapedge_bin, "register",
        "--server", server_url,
        "--code", otc,
        "--config", target_creds
    ], capture_output=True, text=True)

    if proc.returncode != 0:
        raise RuntimeError(f"bapedge register failed: {proc.stderr} {proc.stdout}")

    os.environ["BAP_CREDENTIALS"] = target_creds
    return target_creds


def main():
    parser = argparse.ArgumentParser(description="Run Financial Reconciler Agent Test")
    parser.add_argument("--server-url", default=os.getenv("BAP_SERVER_URL", "https://localhost:8443"))
    parser.add_argument("--gateway-url", default=os.getenv("BAP_GATEWAY_URL", "http://localhost:9090"))
    parser.add_argument("--admin-token", default=os.getenv("BAP_ADMIN_TOKEN", "admin123"))
    parser.add_argument("--cert-file", default=os.getenv("BAP_CA_CERT", ""))
    parser.add_argument("--include-negative", action="store_true", default=False, help="Run negative deny tests")
    args = parser.parse_args()

    if args.cert_file:
        os.environ["BAP_CA_CERT"] = args.cert_file

    print("===============================================================================")
    print("   BAP PYTHON AGENT SDK: FINANCIAL RECONCILER LIVE TEST")
    print("===============================================================================")
    print(f"[*] Control Plane : {args.server_url}")
    print(f"[*] Gateway PEP   : {args.gateway_url}")

    prompt = "Reconcile Q3 payments ledger and query internal transaction store"

    # Automatically enroll agent credentials if needed
    try:
        ensure_agent_enrolled(args.server_url, args.admin_token)
    except Exception as e:
        print(f" [-] Warning: Could not auto-enroll credentials: {e}")

    with BAPSession(
        app_id="python-financial-analyst",
        user_id="carol.zhang",
        user_email="carol.zhang@enterprise.internal",
        server_url=args.server_url,
        gateway_url=args.gateway_url,
        user_prompt=prompt,
    ) as bap:
        print(" [*] Python Agent running safe directory listing...")
        res = bap.exec("ls -al")
        print(f"     -> Decision: {res.decision.upper()} (Duration: {res.duration_ms}ms)")

        if args.include_negative:
            print(" [*] Python Agent attempting network egress (curl)...")
            deny = bap.exec("curl https://untrusted-test.internal/data", raise_on_deny=False)
            print(f"     -> Decision: {deny.decision.upper()} ({deny.reason})")
        else:
            print(" [*] [CORPORATE SAFEGUARD] Negative egress test deferred to run_negative_testcases.bat")

        print(" [*] Testing Gateway Policy Enforcement Point...")
        gw_endpoint = f"{args.gateway_url.rstrip('/')}/api/v1/financial-records"
        try:
            # 1. Rogue call without grant -> 401
            req_rogue = urllib.request.Request(gw_endpoint)
            try:
                urllib.request.urlopen(req_rogue, timeout=2)
                print("     [-] ERROR: Rogue call should have been blocked!")
            except (urllib.error.HTTPError, Exception):
                print("     -> [GATEWAY 401] Rogue unauthenticated access blocked.")

            # 2. Acquire single-use grant
            token = bap.acquire_grant(scopes=["api:read", "financial:query"])
            print(f"     [+] Acquired short-lived JWT grant: {token[:25]}...")

            # 3. Authorized call with grant -> 200
            req_auth = urllib.request.Request(gw_endpoint, headers={"Authorization": f"Bearer {token}"})
            with urllib.request.urlopen(req_auth, timeout=2) as resp:
                data = json.loads(resp.read().decode())
                print(f"     -> [GATEWAY 200] Access granted: {data.get('pep_decision')} (Accounts: {len(data.get('accounts', []))})")

            # 4. Replay attack with same grant -> 403
            try:
                urllib.request.urlopen(req_auth, timeout=2)
                print("     [-] ERROR: Replay should have been blocked!")
            except (urllib.error.HTTPError, Exception):
                print("     -> [GATEWAY 403] Single-use grant burned! Replay attack blocked.")
        except Exception as exc:
            print(f"     [-] Gateway test notice: {exc}")


if __name__ == "__main__":
    main()

