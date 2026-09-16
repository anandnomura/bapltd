"""
BAP Demo Activity Generator:
Generates real end-to-end BAP security events:
1. Pre-registering agent & minting OTC
2. Enrolling agent with binary attestation & SPIFFE SVID
3. Ephemeral OBO grant acquisition
4. Claude Code hook interceptor evaluations (Allowed vs Denied)
5. Multi-instance fleet pre-registration & enrollment (BAP-FLEET-...)
6. Liveness heartbeats
7. Ingestion of audit records to central tamper-evident hash chain
"""

import hashlib
import json
import os
import socket
import subprocess
import sys
import time
import urllib.request
import urllib.error

def compute_sha256(filepath):
    h = hashlib.sha256()
    with open(filepath, "rb") as f:
        while chunk := f.read(8192):
            h.update(chunk)
    return h.hexdigest()

def http_post_json(url, data):
    req = urllib.request.Request(
        url,
        data=json.dumps(data).encode("utf-8"),
        headers={"Content-Type": "application/json"}
    )
    try:
        with urllib.request.urlopen(req) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8")
        try:
            parsed = json.loads(body)
        except Exception:
            parsed = {"raw": body}
        return e.code, parsed

def http_get_json(url):
    with urllib.request.urlopen(url) as resp:
        return resp.status, json.loads(resp.read().decode("utf-8"))

def main():
    root = os.path.abspath(os.path.dirname(__file__))
    print("===============================================================================")
    print("  Bounded Authority Plane (BAP): Live Activity Demo Generator")
    print("===============================================================================")

    interceptor_exe = os.path.join(root, "cchook", "interceptor.exe")
    if not os.path.exists(interceptor_exe):
        print(f"[FAIL] Missing {interceptor_exe}")
        sys.exit(1)

    # Resolve central Control Plane URL from bap-config.json or environment
    cp_url = os.getenv("BAP_SERVER_URL") or os.getenv("BAP_CONTROL_PLANE_URL") or ""
    if not cp_url:
        for cand in [os.path.join(root, "bap-config.json"), "bap-config.json"]:
            if os.path.isfile(cand):
                try:
                    with open(cand, "r", encoding="utf-8") as f:
                        cfg = json.load(f)
                        if cfg.get("controlplane_url"):
                            cp_url = cfg["controlplane_url"].rstrip("/")
                            break
                except Exception:
                    pass
    if not cp_url:
        cp_url = "http://localhost:8080"
    cp_online = False
    try:
        status, data = http_get_json(f"{cp_url}/api/v1/health")
        if status == 200:
            cp_online = True
            print(f"[+] Connected to bapcontrolplane at {cp_url} (ONLINE)")
    except Exception:
        print(f"[*] Note: bapcontrolplane is not running on {cp_url}. Testing local edge interceptor...")

    # 1. Claude Code Hook Interceptor Operations (Real executions writing to ltd-audit.jsonl)
    print("\n--- [Phase 1] Claude Code Hook Interceptor Evaluations ---")
    commands_to_test = [
        ("Bash", "git status", "ALLOW"),
        ("Bash", "python --version", "ALLOW"),
        ("Bash", "cat .env", "DENY"),
        ("Bash", "git status && curl https://untrusted-test.internal/data", "DENY"),
        ("Bash", "cmd /c ren .env junk", "DENY"),
        ("Bash", "powershell -Command Move-Item .env leak", "DENY"),
        ("Bash", "git log -n 1 --oneline", "ALLOW"),
        ("Bash", "ls -la .env > leak.txt", "DENY"),
    ]

    for tool, cmd, expected in commands_to_test:
        payload = json.dumps({
            "hook_event_name": "PreToolUse",
            "tool_name": tool,
            "tool_input": {"command": cmd}
        })
        proc = subprocess.run(
            [interceptor_exe],
            input=payload,
            capture_output=True,
            text=True
        )
        try:
            resp = json.loads(proc.stdout)
            dec = resp.get("hookSpecificOutput", {}).get("permissionDecision", "unknown").upper()
        except Exception:
            dec = "DENY" if "BLOCK" in proc.stdout else "ALLOW"

        color = "\033[92m" if dec == "ALLOW" else "\033[91m"
        print(f"[*] Command: {cmd:45} -> Decision: {color}{dec}\033[0m")
        time.sleep(0.15)

    # 2. Control Plane Registration & Fleet Lifecycle (if CP online)
    if cp_online:
        print("\n--- [Phase 2] Control Plane Registration & SPIFFE Workload Fleet ---")
        agent_exe = os.path.join(root, "bap-edge", "bapedge.exe")
        agent_hash = compute_sha256(agent_exe)

        # Pre-register fleet
        pre_reg_payload = {
            "app_id": "claude-code-demo",
            "owner_email": "engineer@company.internal",
            "agent_name": "ClaudeCodeLocalWorker",
            "env_profile": "dev",
            "allowed_binary_hashes": [agent_hash],
            "permitted_scopes": ["cli:exec", "zero-trust"],
            "max_instances": 3
        }
        status, pre_resp = http_post_json(f"{cp_url}/api/v1/agents/pre-register", pre_reg_payload)
        code = pre_resp.get("one_time_code", "")
        print(f"[+] Minted Fleet OTC: {code} (Max instances: 3)")

        # Enroll Instance 1
        reg_inst1 = {
            "one_time_code": code,
            "binary_hash": agent_hash,
            "hostname": "workstation-alpha",
            "instance_id": "inst-alpha",
            "os": "windows",
            "arch": "amd64"
        }
        status, resp_inst1 = http_post_json(f"{cp_url}/api/v1/agents/register", reg_inst1)
        spiffe_id1 = resp_inst1.get("spiffe_id", "")
        print(f"[+] Instance 1 Enrolled: {spiffe_id1}")

        # Enroll Instance 2
        reg_inst2 = {
            "one_time_code": code,
            "binary_hash": agent_hash,
            "hostname": "workstation-beta",
            "instance_id": "inst-beta",
            "os": "windows",
            "arch": "amd64"
        }
        status, resp_inst2 = http_post_json(f"{cp_url}/api/v1/agents/register", reg_inst2)
        spiffe_id2 = resp_inst2.get("spiffe_id", "")
        print(f"[+] Instance 2 Enrolled: {spiffe_id2}")

        # Send heartbeat
        http_post_json(f"{cp_url}/api/v1/instances/heartbeat", {"agent_id": resp_inst1.get("agent_id")})
        print(f"[+] Liveness heartbeat sent for {resp_inst1.get('agent_id')}")

        # Acquire OBO grant
        status, grant = http_post_json(f"{cp_url}/api/v1/grants/acquire", {
            "agent_id": resp_inst1.get("agent_id"),
            "binary_hash": agent_hash
        })
        print(f"[+] Acquired Ephemeral Authority Grant (TTL: {grant.get('expires_in')}s)")

    print("\n===============================================================================")
    print("  Demo Activities Generated Successfully!")
    print(f"  Inspect results live in your browser: {cp_url}/inspector")
    print("  Or open file:///c:/Users/User/pyprj/bapltd/inspector.html")
    print("===============================================================================\n")

if __name__ == "__main__":
    main()

