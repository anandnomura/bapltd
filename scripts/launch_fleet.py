#!/usr/bin/env python3
"""
BAP Enterprise Fleet Workload Simulator (Act 1: Fleet Visibility & Scale)
Enrolls 5 (or 25) simulated live Claude Code developer agent instances across enterprise squads.
Sends heartbeats to BAP Control Plane so the Live Workload Radar pulses in real time.
"""

import sys
import time
import json
import random
import argparse
import urllib.request
import urllib.error

SQUADS_5 = [
    {
        "name": "Alice Chen",
        "role": "Frontend Lead",
        "app_id": "claude-code",
        "instance_id": "laptop-alice",
        "user_id": "alice.chen",
        "user_email": "alice.chen@enterprise.internal",
        "spiffe_id": "spiffe://bap.internal/app/claude-code/instance/laptop-alice",
        "hostname": "MACBOOK-ALICE-PRO",
        "pid": 18420
    },
    {
        "name": "Bob Martinez",
        "role": "Backend Platform",
        "app_id": "claude-code",
        "instance_id": "laptop-bob",
        "user_id": "bob.martinez",
        "user_email": "bob.martinez@enterprise.internal",
        "spiffe_id": "spiffe://bap.internal/app/claude-code/instance/laptop-bob",
        "hostname": "DELL-XPS-BOB",
        "pid": 29104
    },
    {
        "name": "Carol Zhang",
        "role": "Cloud Infrastructure",
        "app_id": "claude-code",
        "instance_id": "laptop-carol",
        "user_id": "carol.zhang",
        "user_email": "carol.zhang@enterprise.internal",
        "spiffe_id": "spiffe://bap.internal/app/claude-code/instance/laptop-carol",
        "hostname": "THINKPAD-CAROL-SRE",
        "pid": 34152
    },
    {
        "name": "Dave Wilson",
        "role": "Security Engineering",
        "app_id": "claude-code",
        "instance_id": "laptop-dave",
        "user_id": "dave.wilson",
        "user_email": "dave.wilson@enterprise.internal",
        "spiffe_id": "spiffe://bap.internal/app/claude-code/instance/laptop-dave",
        "hostname": "SEC-WORKSTATION-DAVE",
        "pid": 41208
    },
    {
        "name": "Eve Patel",
        "role": "Data Intelligence",
        "app_id": "claude-code",
        "instance_id": "laptop-eve",
        "user_id": "eve.patel",
        "user_email": "eve.patel@enterprise.internal",
        "spiffe_id": "spiffe://bap.internal/app/claude-code/instance/laptop-eve",
        "hostname": "MACBOOK-EVE-DATA",
        "pid": 52816
    }
]

def generate_25_fleet():
    squad_names = [
        ("Frontend Squad", "claude-code", ["Alice", "Alex", "Amy", "Aaron", "Abby"]),
        ("Payments Platform", "copilot", ["Bob", "Brian", "Bella", "Ben", "Boris"]),
        ("Cloud Ops / SRE", "antigravity", ["Carol", "Chris", "Clara", "Cole", "Cynthia"]),
        ("Security Core", "claude-code", ["Dave", "Dan", "Diana", "Derek", "Daisy"]),
        ("Data & Analytics", "python-agent", ["Eve", "Ethan", "Emma", "Eric", "Elena"])
    ]
    agents = []
    base_pid = 11000
    for squad_title, tool, members in squad_names:
        for m in members:
            uid = f"{m.lower()}.dev"
            inst = f"node-{m.lower()}"
            spiffe = f"spiffe://bap.internal/app/{tool}/instance/{inst}"
            agents.append({
                "name": f"{m} ({squad_title})",
                "role": squad_title,
                "app_id": tool,
                "instance_id": inst,
                "user_id": uid,
                "user_email": f"{uid}@enterprise.internal",
                "spiffe_id": spiffe,
                "hostname": f"DEVHOST-{m.upper()}",
                "pid": base_pid + len(agents) * 142
            })
    return agents

def post_json(url, data):
    req = urllib.request.Request(
        url,
        data=json.dumps(data).encode("utf-8"),
        headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req, timeout=3) as resp:
            return resp.getcode(), json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        try:
            return e.code, json.loads(body)
        except Exception:
            return e.code, {"error": body}
    except Exception as e:
        return 0, {"error": str(e)}

def enroll_fleet(server_url, agents):
    print(f"[*] Enrolling {len(agents)} autonomous agent workloads into BAP Control Plane ({server_url})...")
    enrolled = 0
    for a in agents:
        payload = {
            "session_id": f"sess-{a['instance_id']}-{random.randint(1000, 9999)}",
            "app_id": a["app_id"],
            "instance_id": a["instance_id"],
            "user_id": a["user_id"],
            "user_email": a["user_email"],
            "spiffe_id": a["spiffe_id"],
            "client_pid": a["pid"],
            "hostname": a["hostname"]
        }
        code, resp = post_json(f"{server_url}/api/v1/sessions/start", payload)
        if code == 200:
            enrolled += 1
            print(f"  [+] {a['name']:<28} | {a['app_id']:<12} | PID: {a['pid']:<6} | {a['spiffe_id']}")
        else:
            print(f"  [-] Failed to enroll {a['name']}: {resp}")
    print(f"\n[+] Total Active Agents Online: {enrolled}/{len(agents)}")
    return enrolled

def send_heartbeats(server_url, agents):
    for a in agents:
        payload = {"agent_id": a["instance_id"]}
        post_json(f"{server_url}/api/v1/instances/heartbeat", payload)

def main():
    parser = argparse.ArgumentParser(description="BAP Enterprise Fleet Workload Simulator")
    parser.add_argument("--server", default="http://localhost:8080", help="BAP Control Plane URL")
    parser.add_argument("--count", type=int, default=5, choices=[5, 25], help="Fleet density: 5 or 25 agents")
    parser.add_argument("--once", action="store_true", help="Enroll sessions once and exit")
    parser.add_argument("--interval", type=int, default=10, help="Heartbeat interval in seconds")
    args = parser.parse_args()

    agents = SQUADS_5 if args.count == 5 else generate_25_fleet()

    enrolled = enroll_fleet(args.server, agents)
    if enrolled == 0:
        print("[-] Could not enroll agents. Is the control plane running?")
        sys.exit(1)

    if args.once:
        print("[*] One-time enrollment complete.")
        return

    print(f"\n[*] Heartbeat daemon active. Keeping {len(agents)} workloads alive on port 8080.")
    print("    Press Ctrl+C to terminate fleet.\n")
    try:
        while True:
            time.sleep(args.interval)
            send_heartbeats(args.server, agents)
    except KeyboardInterrupt:
        print("\n[*] Fleet simulator stopped.")

if __name__ == "__main__":
    main()
