#!/usr/bin/env python3
"""
BAP Enterprise Fleet Workload Simulator (Act 1: Fleet Visibility & Scale)
Enrolls 5 (or 25) simulated live Claude Code developer agent instances across enterprise squads.
Sends heartbeats to BAP Control Plane so the Live Workload Radar pulses in real time.
"""

import os
import sys
import time
import json
import random
import signal
import atexit
import argparse
import urllib.request
import urllib.error

def is_pid_alive(pid):
    if not pid or pid <= 0:
        return True
    if sys.platform == "win32":
        try:
            import ctypes
            PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
            SYNCHRONIZE = 0x00100000
            kernel32 = ctypes.windll.kernel32
            handle = kernel32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE, False, pid)
            if not handle:
                return False
            exit_code = ctypes.c_ulong()
            kernel32.GetExitCodeProcess(handle, ctypes.byref(exit_code))
            kernel32.CloseHandle(handle)
            STILL_ACTIVE = 259
            return exit_code.value == STILL_ACTIVE
        except Exception:
            return True
    else:
        try:
            os.kill(pid, 0)
            return True
        except OSError:
            return False

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

def cleanup_fleet(server_url):
    print(f"\n[*] Deregistering all agent workloads from BAP Control Plane ({server_url})...")
    code, resp = post_json(f"{server_url}/api/v1/sessions/reset", {})
    if code == 200:
        print(f"  [+] Cleanly deregistered {resp.get('closed_sessions', 0)} active session(s). Dashboard updated.")
    else:
        print(f"  [-] Note on session reset: {resp}")

def main():
    parser = argparse.ArgumentParser(description="BAP Enterprise Fleet Workload Simulator")
    parser.add_argument("--server", default="http://localhost:8080", help="BAP Control Plane URL")
    parser.add_argument("--count", type=int, default=5, choices=[5, 25], help="Fleet density: 5 or 25 agents")
    parser.add_argument("--once", action="store_true", help="Enroll sessions once and exit")
    parser.add_argument("--cleanup", action="store_true", help="Deregister all agents from control plane and exit")
    parser.add_argument("--parent-pid", type=int, default=0, help="Parent process PID to monitor for automatic teardown")
    parser.add_argument("--interval", type=int, default=10, help="Heartbeat interval in seconds")
    args = parser.parse_args()

    if args.cleanup:
        cleanup_fleet(args.server)
        return

    agents = SQUADS_5 if args.count == 5 else generate_25_fleet()

    enrolled = enroll_fleet(args.server, agents)
    if enrolled == 0:
        print("[-] Could not enroll agents. Is the control plane running?")
        sys.exit(1)

    if args.once:
        print("[*] One-time enrollment complete.")
        return

    def handle_exit(signum=None, frame=None):
        cleanup_fleet(args.server)
        sys.exit(0)

    try:
        signal.signal(signal.SIGINT, handle_exit)
        signal.signal(signal.SIGTERM, handle_exit)
    except Exception:
        pass

    atexit.register(lambda: cleanup_fleet(args.server))

    print(f"\n[*] Heartbeat daemon active. Keeping {len(agents)} workloads alive on port 8080.")
    if args.parent_pid > 0:
        print(f"    Parent PID monitor active: PID {args.parent_pid} (Auto-deregisters when demo shell exits).")
    print("    Press Ctrl+C to terminate fleet.\n")

    try:
        while True:
            # Check parent PID liveness every second
            for _ in range(max(1, args.interval)):
                time.sleep(1)
                if args.parent_pid > 0 and not is_pid_alive(args.parent_pid):
                    print(f"\n[*] Parent shell (PID {args.parent_pid}) exited. Automatically deregistering agent workloads...")
                    cleanup_fleet(args.server)
                    sys.exit(0)
            send_heartbeats(args.server, agents)
    except KeyboardInterrupt:
        handle_exit()

if __name__ == "__main__":
    main()
