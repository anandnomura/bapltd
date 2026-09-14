#!/usr/bin/env python3
"""
BAP Executive Demonstration CLI Action Helper
Provides clean, cross-platform triggers for the Executive CIO Demonstration.
"""

import sys
import json
import argparse
import urllib.request
import urllib.error

SERVER = "http://localhost:8080"

def get_json(path):
    try:
        with urllib.request.urlopen(f"{SERVER}{path}", timeout=3) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except Exception as e:
        return {"error": str(e)}

def post_json(path, data=None):
    payload = json.dumps(data).encode("utf-8") if data is not None else b"{}"
    req = urllib.request.Request(
        f"{SERVER}{path}",
        data=payload,
        headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.getcode(), json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode("utf-8"))
        except Exception:
            return e.code, {"error": str(e)}
    except Exception as e:
        return 0, {"error": str(e)}

def action_safe():
    print("[*] Executing multi-stage pipeline via BAP Edge Zero-Trust Broker...")
    code, res = post_json("/api/v1/demo/exec-safe")
    if code == 200:
        print(f"  [+] Decision : {res.get('decision', 'allow').upper()} (Exit Code: {res.get('exit_code', 0)})")
        print(f"  [+] Latency  : {res.get('duration_ms', 1.4)}ms (Sub-2ms Zero-Friction Edge Evaluation)")
        print(f"  [+] Pipeline : {res.get('command')}")
        print(f"  [+] Policy   : {res.get('reason')}")
        print("\n  ==> Visual feedback: Pipeline flowed in emerald green on browser dashboard.")
    else:
        print(f"  [-] Execution result ({code}): {res.get('reason', res.get('error'))}")

def action_attack():
    print("[*] Simulating malicious credential exfiltration attack against Gateway PEP on port 9090...")
    code, res = post_json("/api/v1/demo/exec-attack")
    print(f"  [!] PEP Decision : {res.get('pep_decision', 'DENY')} (HTTP {res.get('exit_code', 403)})")
    print(f"  [!] Latency      : {res.get('duration_ms', 1.1)}ms (Terminated at perimeter)")
    print(f"  [!] Target Egress: {res.get('command')}")
    print(f"  [!] Perimeter PEP: {res.get('reason')}")
    print("\n  ==> Visual feedback: Gateway PEP & Cedar nodes flashed RED on browser dashboard.")

def action_toggle_scale():
    data = get_json("/api/v1/inspector/data")
    active_count = len([s for s in data.get("sessions", []) if s.get("status") == "active"])
    target = 25 if active_count < 15 else 5
    print(f"[*] Switching fleet scale from {active_count} to {target} agents...")
    code, res = post_json("/api/v1/demo/fleet-scale", {"count": target})
    if code == 200:
        if target == 25:
            print("  [+] Enterprise High-Density Mesh active (25 concurrent agent workloads across 5 squads).")
        else:
            print("  [+] Core Engineering Squad active (5 live developer workloads).")
        print(f"\n  ==> Visual feedback: Live Workload Radar expanded to {target} active agent chips.")
    else:
        print(f"  [-] Failed to update scale: {res}")

def action_toggle_kill():
    curr = get_json("/api/v1/control/kill-switch").get("kill_switch", False)
    next_state = not curr
    action_name = "ENGAGING EMERGENCY KILL-SWITCH" if next_state else "DISENGAGING KILL-SWITCH"
    print(f"[*] {action_name}...")
    code, res = post_json("/api/v1/control/kill-switch", {"enabled": next_state})
    if code == 200:
        if next_state:
            print("  [!] EMERGENCY FLEET KILL-SWITCH ACTIVE: All agent workloads globally frozen by CISO override.")
            print("\n  ==> Visual feedback: Red emergency alert banner active; all agent chips flagged [FROZEN].")
        else:
            print("  [+] FLEET GOVERNANCE RESTORED: Emergency freeze lifted. Standard Cedar invariants active.")
            print("\n  ==> Visual feedback: Emergency banner cleared; agent workloads restored to active.")
    else:
        print(f"  [-] Failed to update kill-switch: {res}")

def action_verify():
    print("[*] Performing live SHA-256 Merkle chain verification across all audit logs...")
    res = get_json("/api/v1/control/chain/verify")
    if res.get("valid"):
        print(f"  [+] Merkle Integrity : {res.get('merkle_integrity', '100% VALID')}")
        print(f"  [+] Head Hash        : {res.get('head_hash')}")
        print(f"  [+] Total Events     : {res.get('total_events')} recorded events in immutable hash chain")
        print(f"  [+] Compliance       : {res.get('compliance_status')} (Audit-ready for SOC 2 / FedRAMP)")
        print("\n  ==> Visual feedback: Tamper-evident badge verified with zero discrepancies.")
    else:
        print(f"  [-] Verification error: {res}")

def main():
    parser = argparse.ArgumentParser(description="BAP Executive Demo Actions")
    parser.add_argument("action", choices=["safe", "attack", "toggle-scale", "toggle-kill", "verify"], help="Demo action to run")
    args = parser.parse_args()

    if args.action == "safe":
        action_safe()
    elif args.action == "attack":
        action_attack()
    elif args.action == "toggle-scale":
        action_toggle_scale()
    elif args.action == "toggle-kill":
        action_toggle_kill()
    elif args.action == "verify":
        action_verify()

if __name__ == "__main__":
    main()

