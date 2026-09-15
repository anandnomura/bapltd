#!/usr/bin/env python3
"""
Gracefully close and deregister all active BAP sessions.
Used by simple_local_test.bat to ensure the Live Radar transitions to 0 Active Agents.
"""

import argparse
import json
import os
import sys
import time
import urllib.request
import urllib.error

# Ensure python-agent is discoverable for _urlopen and ssl context
ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
PYTHON_AGENT_DIR = os.path.join(ROOT_DIR, "python-agent")
if PYTHON_AGENT_DIR not in sys.path:
    sys.path.insert(0, PYTHON_AGENT_DIR)

from bap_sdk.client import _urlopen


def close_sessions(server_url: str, cert_file: str = "", sessions: list = None) -> int:
    if cert_file:
        os.environ["BAP_CA_CERT"] = cert_file

    server_url = server_url.rstrip("/")

    # If specific sessions are not passed, fetch all active sessions from inspector data
    target_sessions = sessions or []
    if not target_sessions:
        try:
            req = urllib.request.Request(f"{server_url}/api/v1/inspector/data")
            with _urlopen(req, timeout=3) as resp:
                data = json.loads(resp.read().decode())
                all_sessions = data.get("sessions", [])
                target_sessions = [s.get("session_id") for s in all_sessions if s.get("status") == "active"]
        except Exception as e:
            print(f"[-] Could not query active sessions from control plane: {e}")
            return 0

    if not target_sessions:
        print(" [*] No active sessions found. Active agents count is already 0.")
        return 0

    closed_count = 0
    for sess_id in target_sessions:
        if not sess_id:
            continue
        try:
            payload = json.dumps({"session_id": sess_id, "reason": "workload_completed"}).encode()
            req = urllib.request.Request(
                f"{server_url}/api/v1/sessions/end",
                data=payload,
                headers={"Content-Type": "application/json"}
            )
            with _urlopen(req, timeout=3) as resp:
                print(f" [*] Deregistered session {sess_id} -> CLOSED")
                closed_count += 1
        except Exception as e:
            print(f" [-] Warning: Failed to close session {sess_id}: {e}")

    # Verify final count
    try:
        req = urllib.request.Request(f"{server_url}/api/v1/inspector/data")
        with _urlopen(req, timeout=3) as resp:
            data = json.loads(resp.read().decode())
            active = [s for s in data.get("sessions", []) if s.get("status") == "active"]
            print(f" [+] Active agents remaining on Control Plane: {len(active)}")
    except Exception:
        pass

    return closed_count


def main():
    parser = argparse.ArgumentParser(description="Close active BAP sessions")
    parser.add_argument("--server-url", default=os.getenv("BAP_SERVER_URL", "https://localhost:8443"))
    parser.add_argument("--cert-file", default=os.getenv("BAP_CA_CERT", ""))
    parser.add_argument("--sessions", help="Comma-separated list of session IDs to close", default="")
    args = parser.parse_args()

    session_list = [s.strip() for s in args.sessions.split(",") if s.strip()] if args.sessions else None
    close_sessions(args.server_url, args.cert_file, session_list)


if __name__ == "__main__":
    main()

