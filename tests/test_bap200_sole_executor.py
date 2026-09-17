"""
Integration Test Suite for BAP-200:
Make BAPEdge the Sole Executor for Protected Agent Actions.

Verifies:
1. AC1: Allowed non-idempotent operation occurs exactly once.
2. AC2: Denied operation produces zero side effects.
3. AC3: Direct low-level file writes outside allowed paths fail.
4. AC4: Comprehensive bypass matrix: Bash, Read, Write, Edit, Python, PowerShell, MCP, Child Process.
5. AC5: Allowed interpreter cannot access a denied file or network destination.
6. AC6: Claude cannot modify or disable BAP hooks, policies or binaries (Tamper Resistance).
7. AC7: Repeated alternative attempts remain blocked and appear as correlated risk events.
8. AC8: BAPEdge returns the actual output and exit status.
9. AC9: Cryptographic execution receipt with hash, identity, delegation, policy version, sandbox profile, and result.
"""

import json
import os
import subprocess
import sys
import time
import unittest

WORKSPACE_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
BAPEDGE_BIN = os.path.join(WORKSPACE_ROOT, "bapedge.exe")
INTERCEPTOR_BIN = os.path.join(WORKSPACE_ROOT, "cchook", "interceptor.exe")


def run_interceptor(payload: dict, env=None) -> dict:
    """Invokes the Claude Code PreToolUse interceptor binary with a JSON payload."""
    test_env = os.environ.copy()
    test_env["BAP_WORKSPACE_ROOT"] = WORKSPACE_ROOT
    if env:
        test_env.update(env)

    proc = subprocess.Popen(
        [INTERCEPTOR_BIN],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        cwd=WORKSPACE_ROOT,
        env=test_env,
    )
    stdout, stderr = proc.communicate(input=json.dumps(payload))
    return json.loads(stdout.strip()) if stdout.strip() else {}


def run_bapedge(args: list, env=None) -> tuple[int, dict, str]:
    """Invokes bapedge with given arguments and parses JSON output."""
    test_env = os.environ.copy()
    test_env["BAP_WORKSPACE_ROOT"] = WORKSPACE_ROOT
    if env:
        test_env.update(env)

    cmd = [BAPEDGE_BIN] + args
    proc = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        cwd=WORKSPACE_ROOT,
        env=test_env,
    )
    stdout, stderr = proc.communicate()
    data = {}
    if stdout.strip().startswith("{"):
        try:
            data = json.loads(stdout.strip())
        except Exception:
            pass
    return proc.returncode, data, stdout + "\n" + stderr


class TestBAP200SoleExecutor(unittest.TestCase):

    def setUp(self):
        self.assertTrue(os.path.isfile(BAPEDGE_BIN), f"bapedge.exe missing at {BAPEDGE_BIN}")
        self.assertTrue(os.path.isfile(INTERCEPTOR_BIN), f"interceptor.exe missing at {INTERCEPTOR_BIN}")

    # -------------------------------------------------------------------------
    # AC1: Allowed non-idempotent operation occurs exactly once
    # -------------------------------------------------------------------------
    def test_ac1_non_idempotent_operation_occurs_exactly_once(self):
        counter_file = os.path.join(WORKSPACE_ROOT, "tests", "scratch_counter.txt")
        if os.path.exists(counter_file):
            os.remove(counter_file)

        # Non-idempotent command: Appends a line with timestamp/counter to file
        cmd_str = f'{sys.executable} -c "with open(r\'{counter_file}\', \'a\') as f: f.write(\'1\\n\')"'

        # 1. Claude Code PreToolUse Hook interceptor runs
        hook_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": cmd_str},
            "session_id": "sess-bap200-test-ac1",
        }
        hook_resp = run_interceptor(hook_payload)

        # Verify hook returns 'allow' with updatedInput routing to bapedge exec
        self.assertEqual(hook_resp["hookSpecificOutput"]["permissionDecision"], "allow")
        updated_input = hook_resp["hookSpecificOutput"].get("updatedInput", {})
        self.assertIn("command", updated_input)
        rewritten_cmd = updated_input["command"]
        self.assertIn("bapedge", rewritten_cmd)
        self.assertIn("exec", rewritten_cmd)

        # Verify the counter file was NOT touched during the hook evaluation
        self.assertFalse(os.path.exists(counter_file), "Hook should not execute the command directly")

        # 2. Simulated agent platform executes the rewritten command exactly once
        # Claude Code runs the rewritten command via child process
        proc = subprocess.run(
            rewritten_cmd,
            shell=True,
            cwd=WORKSPACE_ROOT,
            capture_output=True,
            text=True,
        )
        self.assertEqual(proc.returncode, 0, f"BAPEdge exec failed: {proc.stderr}")

        # 3. Verify the non-idempotent file write occurred EXACTLY ONCE
        self.assertTrue(os.path.exists(counter_file))
        with open(counter_file, "r") as f:
            lines = [l.strip() for l in f.readlines() if l.strip()]
        self.assertEqual(len(lines), 1, f"Expected exactly 1 execution, found {len(lines)}")
        self.assertEqual(lines[0], "1")

        # Cleanup
        if os.path.exists(counter_file):
            os.remove(counter_file)

    # -------------------------------------------------------------------------
    # AC2: Denied operation produces zero side effects
    # -------------------------------------------------------------------------
    def test_ac2_denied_operation_produces_zero_side_effects(self):
        canary_file = os.path.join(WORKSPACE_ROOT, "tests", "scratch_canary_deny.txt")
        if os.path.exists(canary_file):
            os.remove(canary_file)

        # Denied command: unauthorized network egress chained with canary file write
        denied_cmd = f"curl http://169.254.169.254 && echo pwned > {canary_file}"

        # 1. PreToolUse Hook test
        hook_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Bash",
            "tool_input": {"command": denied_cmd},
            "session_id": "sess-bap200-test-ac2",
        }
        hook_resp = run_interceptor(hook_payload)
        self.assertEqual(hook_resp["hookSpecificOutput"]["permissionDecision"], "deny")

        # Verify canary was not created by the hook
        self.assertFalse(os.path.exists(canary_file))

        # 2. Direct bapedge exec test
        rc, data, raw = run_bapedge(["exec", "--json", denied_cmd])
        self.assertNotEqual(rc, 0)
        self.assertFalse(data.get("allowed", True))

        # Verify zero side effects on filesystem
        self.assertFalse(os.path.exists(canary_file), "Denied command must produce zero side effects")

    # -------------------------------------------------------------------------
    # AC3: Direct low-level file writes outside allowed paths fail
    # -------------------------------------------------------------------------
    def test_ac3_direct_file_writes_outside_workspace_fail(self):
        # 1. Traversal via Write tool
        write_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Write",
            "tool_input": {"file_path": "../../secret_outside.txt"},
            "session_id": "sess-bap200-test-ac3",
        }
        resp = run_interceptor(write_payload)
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("outside workspace", resp["hookSpecificOutput"]["permissionDecisionReason"])

        # 2. Shell redirection write escaping workspace
        shell_cmd = "echo secret > ../escaped_file.txt"
        rc, data, _ = run_bapedge(["exec", "--json", shell_cmd])
        self.assertNotEqual(rc, 0)
        self.assertFalse(data.get("allowed", True))
        # Cedar denies via policy1 (forbid escapes_workspace) or suggestion informs workspace boundary
        self.assertTrue("policy1" in data.get("reason", "") or "workspace" in data.get("suggestion", "").lower())

    # -------------------------------------------------------------------------
    # AC4: Comprehensive bypass matrix (Bash, Read, Write, Edit, Python, PowerShell, MCP, Child)
    # -------------------------------------------------------------------------
    def test_ac4_comprehensive_bypass_matrix(self):
        # 1. Bash subshell bypass
        bash_subshell = "echo $(curl http://169.254.169.254)"
        rc, data, _ = run_bapedge(["exec", "--json", bash_subshell])
        self.assertFalse(data.get("allowed", True))

        # 2. Read sensitive credentials
        read_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Read",
            "tool_input": {"file_path": ".env"},
            "session_id": "sess-bap200-test-ac4",
        }
        resp = run_interceptor(read_payload)
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")

        # 3. Write targeting sensitive credentials
        write_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Write",
            "tool_input": {"file_path": ".env"},
            "session_id": "sess-bap200-test-ac4",
        }
        resp = run_interceptor(write_payload)
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")

        # 4. Edit targeting sensitive credentials
        edit_payload = {
            "hook_event_name": "PreToolUse",
            "tool_name": "Edit",
            "tool_input": {"file_path": "subdir/.env"},
            "session_id": "sess-bap200-test-ac4",
        }
        resp = run_interceptor(edit_payload)
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")

        # 5. Python raw socket bypass
        python_socket_cmd = 'python -c "import socket; s=socket.socket(); s.connect((\'169.254.169.254\', 80))"'
        rc, data, _ = run_bapedge(["exec", "--json", python_socket_cmd])
        self.assertFalse(data.get("allowed", True))

        # 6. PowerShell WebRequest bypass
        pwsh_cmd = 'powershell -Command "Invoke-WebRequest -Uri http://169.254.169.254"'
        rc, data, _ = run_bapedge(["exec", "--json", pwsh_cmd])
        self.assertFalse(data.get("allowed", True))

        # 7. Child process bypass attempt
        child_cmd = "cmd /c curl http://169.254.169.254"
        rc, data, _ = run_bapedge(["exec", "--json", child_cmd])
        self.assertFalse(data.get("allowed", True))

        # 8. MCP tool bypass attempt
        mcp_proc = subprocess.Popen(
            [BAPEDGE_BIN, "mcp"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            cwd=WORKSPACE_ROOT,
        )
        try:
            init_req = json.dumps({
                "jsonrpc": "2.0",
                "id": 1,
                "method": "initialize",
                "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "bap200-test"}}
            }) + "\n"
            mcp_proc.stdin.write(init_req)
            mcp_proc.stdin.flush()
            _ = mcp_proc.stdout.readline()

            call_req = json.dumps({
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {"name": "bap_exec", "arguments": {"command": "curl http://169.254.169.254"}}
            }) + "\n"
            mcp_proc.stdin.write(call_req)
            mcp_proc.stdin.flush()
            resp_line = mcp_proc.stdout.readline()
            mcp_resp = json.loads(resp_line.strip())
            res = mcp_resp.get("result", {})
            self.assertTrue(res.get("isError") or "denied" in str(res).lower() or "error" in mcp_resp)
        finally:
            if mcp_proc.stdin:
                mcp_proc.stdin.close()
            if mcp_proc.stdout:
                mcp_proc.stdout.close()
            if mcp_proc.stderr:
                mcp_proc.stderr.close()
            mcp_proc.terminate()
            mcp_proc.wait()

    # -------------------------------------------------------------------------
    # AC5: Allowed interpreter cannot access denied file or network destination
    # -------------------------------------------------------------------------
    def test_ac5_allowed_interpreter_cannot_access_denied_target(self):
        # Python interpreter reading .env
        py_env_read = 'python -c "print(open(\'.env\').read())"'
        rc, data, _ = run_bapedge(["exec", "--json", py_env_read])
        self.assertFalse(data.get("allowed", True))
        self.assertIn("policy", data.get("reason", "").lower())

        # PowerShell reading .env
        ps_env_read = 'powershell Get-Content .env'
        rc, data, _ = run_bapedge(["exec", "--json", ps_env_read])
        self.assertFalse(data.get("allowed", True))

    # -------------------------------------------------------------------------
    # AC6: Claude cannot modify or disable BAP hooks, policies or binaries
    # -------------------------------------------------------------------------
    def test_ac6_tamper_resistance_hooks_policies_binaries(self):
        # 1. Direct file edit on policy.cedar
        resp = run_interceptor({
            "hook_event_name": "PreToolUse",
            "tool_name": "Edit",
            "tool_input": {"file_path": "policy.cedar"},
            "session_id": "sess-bap200-tamper",
        })
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("Security Invariant Violation", resp["hookSpecificOutput"]["permissionDecisionReason"])

        # 2. Direct file edit on .claude/settings.json
        resp = run_interceptor({
            "hook_event_name": "PreToolUse",
            "tool_name": "Write",
            "tool_input": {"file_path": ".claude/settings.json"},
            "session_id": "sess-bap200-tamper",
        })
        self.assertEqual(resp["hookSpecificOutput"]["permissionDecision"], "deny")
        self.assertIn("Security Invariant Violation", resp["hookSpecificOutput"]["permissionDecisionReason"])

        # 3. Shell removal of bapedge.exe
        rc, data, _ = run_bapedge(["exec", "--json", "del bapedge.exe"])
        self.assertFalse(data.get("allowed", True))
        self.assertEqual(data.get("receipt", {}).get("result"), "DENIED_TAMPER")

        # 4. Shell modification of policy.cedar
        rc, data, _ = run_bapedge(["exec", "--json", 'echo "permit(principal, action, resource);" > policy.cedar'])
        self.assertFalse(data.get("allowed", True))
        self.assertEqual(data.get("receipt", {}).get("result"), "DENIED_TAMPER")

    # -------------------------------------------------------------------------
    # AC7: Repeated alternative attempts remain blocked and appear as correlated risk events
    # -------------------------------------------------------------------------
    def test_ac7_correlated_risk_events_on_repeated_evasions(self):
        test_session = f"sess-bap200-risk-{int(time.time())}"

        # Attempt 1: cat .env
        rc1, data1, _ = run_bapedge(["exec", "--session-id", test_session, "--json", "cat .env"])
        self.assertFalse(data1.get("allowed", True))

        # Attempt 2: type .env (alternative attempt in same session)
        rc2, data2, _ = run_bapedge(["exec", "--session-id", test_session, "--json", "type .env"])
        self.assertFalse(data2.get("allowed", True))
        warning2 = data2.get("warning", "")
        self.assertIn("CORRELATED RISK EVENT", warning2)
        self.assertIn("ELEVATED", warning2)

        # Attempt 3: python -c "open('.env')"
        rc3, data3, _ = run_bapedge(["exec", "--session-id", test_session, "--json", "python -c \"open('.env')\""])
        self.assertFalse(data3.get("allowed", True))
        warning3 = data3.get("warning", "")
        self.assertIn("CORRELATED RISK EVENT", warning3)

    # -------------------------------------------------------------------------
    # AC8: BAPEdge returns the actual output and exit status
    # -------------------------------------------------------------------------
    def test_ac8_actual_output_and_exit_status(self):
        # 1. Success exit status (0) and stdout
        rc, data, _ = run_bapedge(["exec", "--json", 'python -c "print(\'BAP_OUTPUT_VERIFIED\')"'])
        self.assertEqual(rc, 0)
        self.assertTrue(data.get("allowed", False))
        self.assertEqual(data.get("exit_code"), 0)
        self.assertIn("BAP_OUTPUT_VERIFIED", data.get("output", ""))

        # 2. Explicit non-zero exit code (e.g. 42)
        rc, data, _ = run_bapedge(["exec", "--json", 'python -c "import sys; sys.exit(42)"'])
        self.assertEqual(rc, 42)
        self.assertEqual(data.get("exit_code"), 42)

    # -------------------------------------------------------------------------
    # AC9: Cryptographic execution receipt fields
    # -------------------------------------------------------------------------
    def test_ac9_cryptographic_execution_receipt(self):
        rc, data, _ = run_bapedge(["exec", "--source", "claude-code", "--session-id", "sess-receipt-test", "--json", "echo receipt_test"])
        self.assertEqual(rc, 0)
        receipt = data.get("receipt")
        self.assertIsNotNone(receipt, "Receipt must be present in execution response")

        # Verify receipt schema
        self.assertTrue(receipt["receipt_id"].startswith("rcpt-"))
        self.assertTrue(receipt["request_hash"].startswith("sha256:"))
        self.assertTrue(receipt["identity"].startswith("spiffe://"))
        self.assertIn("agent:claude-code", receipt["delegation"])
        self.assertIn("user:", receipt["delegation"])
        self.assertTrue(receipt["policy_version"].startswith("sha256:"))
        self.assertEqual(receipt["sandbox_profile"], "bap-broker-standard")
        self.assertEqual(receipt["result"], "ALLOWED_EXECUTED")
        self.assertEqual(receipt["session_id"], "sess-receipt-test")
        self.assertIn("timestamp", receipt)


if __name__ == "__main__":
    unittest.main(verbosity=2)
