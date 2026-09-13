"""
Automated pytest test suite for BAP Gateway Policy Enforcement Point (PEP).
Verifies:
1. Rogue agent with no BAP grant is blocked (HTTP 401 Unauthorized).
2. Rogue agent with forged/fake token is blocked (HTTP 403 Forbidden).
3. Governed agent using bap-sdk acquires grant and receives HTTP 200 OK.
4. Single-use grant replay attack is blocked (HTTP 403 Forbidden).
"""

import json
import os
import sys
import urllib.request
import urllib.error
import pytest

# Ensure python-agent is on path
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
sys.path.insert(0, os.path.join(root_dir, "python-agent"))

from bap_sdk import BAPSession, resolve_endpoints

_ep = resolve_endpoints()
GATEWAY_URL = f"{_ep['gateway_url']}/api/v1/financial-records"
CONTROL_PLANE_URL = _ep["controlplane_url"]


def test_gateway_rogue_unauthenticated_blocked():
    """Unauthenticated rogue call without Bearer grant must be dropped with 401."""
    req = urllib.request.Request(GATEWAY_URL)
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req, timeout=3)
    assert exc_info.value.code == 401
    body = json.loads(exc_info.value.read().decode())
    assert body.get("pep_decision") == "DENY"
    assert "Rogue agent" in body.get("message")


def test_gateway_rogue_forged_token_blocked():
    """Rogue call with forged token must be rejected with 403."""
    fake_token = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJyb2d1ZSJ9.INVALID"
    req = urllib.request.Request(GATEWAY_URL, headers={"Authorization": fake_token})
    with pytest.raises(urllib.error.HTTPError) as exc_info:
        urllib.request.urlopen(req, timeout=3)
    assert exc_info.value.code == 403
    body = json.loads(exc_info.value.read().decode())
    assert body.get("pep_decision") == "DENY"


def test_gateway_governed_agent_lifecycle():
    """Governed agent using bap-sdk acquires grant, passes gateway, and verifies single-use."""
    app_id = "test-governed-worker"
    with BAPSession(app_id=app_id, server_url=CONTROL_PLANE_URL) as bap:
        # 1. Acquire grant
        token = bap.acquire_grant(scopes=["api:read", "financial:query"])
        assert token and len(token) > 20

        # 2. First call with valid grant -> 200 OK
        req = urllib.request.Request(GATEWAY_URL, headers={"Authorization": f"Bearer {token}"})
        with urllib.request.urlopen(req, timeout=3) as resp:
            assert resp.status == 200
            data = json.loads(resp.read().decode())
            assert data.get("pep_decision") == "ALLOW"
            assert len(data.get("accounts", [])) > 0

        # 3. Replay attack: calling again with the consumed token must be blocked (HTTP 403)
        req_replay = urllib.request.Request(GATEWAY_URL, headers={"Authorization": f"Bearer {token}"})
        with pytest.raises(urllib.error.HTTPError) as exc_info:
            urllib.request.urlopen(req_replay, timeout=3)
        assert exc_info.value.code == 403
        body = json.loads(exc_info.value.read().decode())
        assert body.get("pep_decision") == "DENY"

    # 4. Verify post-session deregistration on Control Plane
    import time
    time.sleep(0.5)
    req_sess = urllib.request.Request(f"{CONTROL_PLANE_URL}/api/v1/sessions/{bap.session_id}")
    with urllib.request.urlopen(req_sess, timeout=3) as resp:
        assert resp.status == 200
        sess_data = json.loads(resp.read().decode())
        assert sess_data.get("status") == "closed", "Session was not marked closed"

    req_agents = urllib.request.Request(f"{CONTROL_PLANE_URL}/api/v1/agents")
    with urllib.request.urlopen(req_agents, timeout=3) as resp:
        assert resp.status == 200
        agents_data = json.loads(resp.read().decode())
        agents_list = agents_data.get("agents", []) if isinstance(agents_data, dict) else agents_data
        matching = [a for a in agents_list if a.get("app_id") == app_id]
        assert any(a.get("status") == "deregistered" for a in matching), "Agent was not marked deregistered in registry"


def test_gateway_pep_emits_audit_telemetry():
    """Verify that Gateway PEP forwards both blocked rogue calls and permitted governed calls to Control Plane audit store."""
    import time
    time.sleep(0.3)
    req_audit = urllib.request.Request(f"{CONTROL_PLANE_URL}/api/v1/audit/events")
    with urllib.request.urlopen(req_audit, timeout=3) as resp:
        assert resp.status == 200
        data = json.loads(resp.read().decode())
        events = data.get("events", [])
        gateway_events = [e for e in events if e.get("source") == "bap-gateway-pep"]
        assert len(gateway_events) > 0, "No audit events ingested from Gateway PEP"
        
        # Verify rogue denial exists
        has_deny = any(e.get("decision") == "deny" for e in gateway_events)
        assert has_deny, "Gateway PEP did not record rogue denial event in audit store"

        # Verify governed allow exists
        has_allow = any(e.get("decision") == "allow" for e in gateway_events)
        assert has_allow, "Gateway PEP did not record governed allow event in audit store"


