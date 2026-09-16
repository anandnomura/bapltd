"""
BAP Python SDK - Standalone Negative Test Suite
==============================================
Validates that unauthorized commands (forbidden egress, secret disclosure)
are properly intercepted and blocked by the BAP Zero-Trust kernel.

Run only on demand via run_negative_testcases.bat; NEVER invoked by default.
"""

import os
import sys
import pytest
import subprocess
import time
import ssl

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "python-agent")))

from bap_sdk import BAPSession, BAPExecResult, BAPPolicyViolation, resolve_endpoints

ssl_ctx = ssl.create_default_context()
ssl_ctx.check_hostname = False
ssl_ctx.verify_mode = ssl.CERT_NONE


def _urlopen(req, timeout=3):
    import urllib.request
    return urllib.request.urlopen(req, context=ssl_ctx, timeout=timeout)


def test_python_agent_negative_cases():
    endpoints = resolve_endpoints()
    cp_url = endpoints["controlplane_url"]

    with BAPSession(
        app_id="python-agent-negative-test",
        user_id="sec-tester",
        user_email="sec-tester@enterprise.internal",
        user_prompt="Run adversarial zero-trust policy verification",
        server_url=cp_url,
    ) as bap:
        # 1. Test forbidden secret disclosure command raises BAPPolicyViolation
        with pytest.raises(BAPPolicyViolation) as exc_info:
            bap.exec("cat .env", raise_on_deny=True)
        assert "cat .env" in str(exc_info.value) or "denied" in str(exc_info.value).lower()

        # 2. Test forbidden egress command returns is_denied without raising
        res_deny = bap.exec("curl https://untrusted-test.internal/data", raise_on_deny=False)
        assert res_deny.is_denied is True
        assert res_deny.exit_code != 0

