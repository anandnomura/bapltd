"""Repeatable operator acceptance run. Standard-library Python, no pytest required.

Local: python scripts/manual_acceptance.py --local --https --hold
Remote: python scripts/manual_acceptance.py --server https://host:8443 --edge /path/bapedge
Never falls back to another server or stops a process it did not create.
"""
import argparse
import getpass
import hashlib
import json
import os
from pathlib import Path
import secrets
import socket
import ssl
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("Unexpected redirect: verify the exact API endpoint")


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    target = parser.add_mutually_exclusive_group(required=True)
    target.add_argument("--local", action="store_true", help="Build and launch disposable services")
    target.add_argument("--server", help="Exact URL of an existing test control plane")
    parser.add_argument("--edge", type=Path, help="Built bapedge binary (required for remote)")
    parser.add_argument("--gateway", help="Exact existing gateway URL; optional in remote mode")
    parser.add_argument("--https", action="store_true", help="Generate a local TLS certificate")
    parser.add_argument("--ca-cert", type=Path, help="Trusted PEM certificate/CA for remote HTTPS")
    parser.add_argument("--hold", action="store_true", help="Leave local services running until Enter for browser checks")
    parser.add_argument("--exercise-global-kill", action="store_true", help="Also test GLOBAL freeze/restore on the remote TEST server")
    parser.add_argument("--report", type=Path, help="Write a credential-free JSON result")
    args = parser.parse_args()
    if args.server and not args.edge:
        parser.error("--edge is required with --server")
    if args.server and not args.server.startswith("https://"):
        parser.error("Remote acceptance requires HTTPS; use --local for disposable HTTP testing")
    root = Path(__file__).resolve().parents[1]
    results, processes, logs = [], [], []
    suffix = ".exe" if os.name == "nt" else ""
    creationflags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
    server = args.server
    gateway = args.gateway
    admin = os.getenv("BAP_ADMIN_TOKEN") or (secrets.token_urlsafe(24) if args.local else getpass.getpass("Test server admin credential: "))
    test_env = os.environ.copy()
    for key in ["BAP_TEST_MODE", "LTD_TEST", "BAP_OFFLINE", "BAP_ENV"]:
        test_env.pop(key, None)
    failures = []

    def record(name, okay, detail=""):
        results.append({"case": name, "passed": okay, "detail": detail})
        print(f"[{'PASS' if okay else 'FAIL'}] {name}" + (f" — {detail}" if detail else ""), flush=True)
        if not okay:
            failures.append(name)
            raise AssertionError(name)

    with tempfile.TemporaryDirectory(prefix="bap-acceptance-") as directory:
        work = Path(directory)
        print(f"Disposable state: {work}", flush=True)
        try:
            if args.local:
                binaries = {}
                for module, output, package in [("bap-controlplane", "bapcontrolplane", "./cmd/server"), ("bap-edge", "bapedge", "."), ("bap-gateway", "bapgateway", ".")]:
                    print(f"[BUILD] {module}", flush=True)
                    destination = work / (output + suffix)
                    subprocess.run(["go", "build", "-o", str(destination), package], cwd=root / module, check=True, timeout=180)
                    binaries[output] = destination
                edge = binaries["bapedge"]
                port, gateway_port = free_port(), free_port()
                server = f"{'https' if args.https else 'http'}://localhost:{port}"
                gateway = f"http://localhost:{gateway_port}"
                test_env.update(BAP_ADMIN_TOKEN=admin, BAP_SECRET_KEY=secrets.token_urlsafe(32), BAP_SERVER_URL=server)
                command = [str(binaries["bapcontrolplane"]), "-port", str(port), "-db", str(work / "sessions.db"), "-policy", str(root / "bap-edge/policy.cedar"), "-schema", str(root / "bap-edge/schema.json")]
                if args.https:
                    command += ["-https"]
                log = open(work / "controlplane.log", "w", encoding="utf-8"); logs.append(log)
                processes.append(subprocess.Popen(command, cwd=work, env=test_env, stdout=log, stderr=log, creationflags=creationflags))
                if args.https:
                    args.ca_cert = work / "controlplane-cert.pem"
                    for _ in range(100):
                        if args.ca_cert.exists(): break
                        if processes[0].poll() is not None: raise RuntimeError("Control plane failed to start")
                        time.sleep(.1)
                test_env.update(BAP_SERVER_URL=server, BAP_GATEWAY_URL=gateway)
            else:
                edge = args.edge.resolve()
                test_env["BAP_SERVER_URL"] = server
            if args.ca_cert:
                test_env["BAP_CA_CERT"] = str(args.ca_cert.resolve())
            context = ssl.create_default_context(cafile=str(args.ca_cert) if args.ca_cert else None)
            opener = urllib.request.build_opener(urllib.request.HTTPSHandler(context=context), NoRedirect())

            def request(path, payload=None, token=None, expected=200, base=None, headers=None):
                hdr = {"Accept": "application/json", **(headers or {})}
                if token: hdr["Authorization"] = "Bearer " + token
                body = None if payload is None else json.dumps(payload).encode()
                if body is not None: hdr["Content-Type"] = "application/json"
                req = urllib.request.Request((base or server).rstrip("/") + path, data=body, headers=hdr)
                try:
                    with opener.open(req, timeout=10) as response:
                        status, raw = response.status, response.read()
                except urllib.error.HTTPError as error:
                    status, raw = error.code, error.read()
                if status != expected:
                    raise AssertionError(f"{path}: HTTP {status}, expected {expected}")
                return json.loads(raw) if raw else {}

            print(f"TESTING EXACT CONTROL PLANE: {server}", flush=True)
            for attempt in range(60 if args.local else 1):
                try:
                    health = request("/api/v1/health")
                    break
                except (OSError, urllib.error.URLError):
                    if attempt == (59 if args.local else 0): raise
                    time.sleep(.2)
            record("Control plane health", health.get("status") == "ok")
            request("/api/v1/control/kill-switch", token=admin)
            record("Admin credential accepted", True)
            request("/api/v1/control/kill-switch", {"enabled": True}, expected=401)
            record("Anonymous admin mutation denied", True)
            request("/api/v1/auth/inspector-handshake", expected=401)
            record("Handshake cannot disclose admin credential", True)
            request("/api/v1/health", headers={"Origin": "https://unapproved.example.test"}, expected=403)
            record("Unapproved browser origin rejected", True)

            if args.local:
                log = open(work / "gateway.log", "w", encoding="utf-8"); logs.append(log)
                processes.append(subprocess.Popen([str(binaries["bapgateway"]), "-port", str(gateway_port), "-controlplane", server], cwd=work, env=test_env, stdout=log, stderr=log, creationflags=creationflags))
                for _ in range(50):
                    try:
                        request("/health", base=gateway); break
                    except (OSError, urllib.error.URLError): time.sleep(.1)
            run_id = uuid.uuid4().hex[:10]
            app_id, instance = f"acceptance-{run_id}", f"worker-{run_id}"
            digest = hashlib.sha256(edge.read_bytes()).hexdigest()
            enrollment = request("/api/v1/agents/pre-register", {"app_id": app_id, "agent_name": "Acceptance worker", "owner_email": "operator@example.test", "env_profile": "production", "allowed_binary_hashes": [digest], "permitted_scopes": ["cli:exec", "api:read"]}, token=admin, expected=201)
            credentials = work / "credentials.json"
            registration = [str(edge), "register", "--server", server, "--code", enrollment["one_time_code"], "--instance-id", instance, "--config", str(credentials)]
            enrolled = subprocess.run(registration, cwd=work, env=test_env, capture_output=True, text=True, timeout=20)
            record("Real edge enrollment and binary attestation", enrolled.returncode == 0)
            replay = subprocess.run(registration, cwd=work, env=test_env, capture_output=True, text=True, timeout=20)
            record("Enrollment code replay denied", replay.returncode != 0)
            creds = json.loads(credentials.read_text())
            grant_payload = {"agent_id": creds["agent_id"], "binary_hash": digest, "scopes": ["api:read"]}
            request("/api/v1/grants/acquire", grant_payload, expected=401)
            record("Public agent ID/hash cannot mint grants", True)
            grant = request("/api/v1/grants/acquire", grant_payload, token=creds["session_token"])["token"]
            record("Enrolled credential acquires bounded grant", bool(grant))
            if gateway:
                request("/api/v1/financial-records", base=gateway, expected=401)
                request("/api/v1/financial-records", base=gateway, token=grant)
                request("/api/v1/financial-records", base=gateway, token=grant, expected=403)
                record("Gateway rejects anonymous access and grant replay", True)
            else:
                request("/api/v1/grants/consume", {"token": grant, "resource": "api:read"})
                request("/api/v1/grants/consume", {"token": grant, "resource": "api:read"}, expected=403)
                record("Direct grant consumption and replay", True, "Gateway not configured; gateway path not tested")

            cache = work / "policy"
            synced = subprocess.run([str(edge), "sync", "--server", server, "--agent-id", creds["agent_id"], "--policy-dir", str(cache)], cwd=work, env=test_env, capture_output=True, text=True, timeout=15)
            record("Policy sync is ONLINE (not cached fallback)", synced.returncode == 0 and "successful (Online)" in synced.stdout)
            session_id = f"sess-{run_id}"
            prompt = "List repository changes and explain the result."
            request("/api/v1/sessions/start", {"session_id": session_id, "app_id": app_id, "instance_id": instance, "user_email": "operator@example.test", "hostname": socket.gethostname(), "user_prompt": prompt})
            test_env.update(BAP_SESSION_ID=session_id, BAP_USER_PROMPT=prompt)
            def execute(command):
                return subprocess.run([str(edge), "exec", "--server", server, "--policy", str(cache / "policy.cedar"), "--audit-log", str(work / "audit.jsonl"), "--source", app_id, "--json", command], cwd=work, env=test_env, capture_output=True, text=True, timeout=20)
            record("Allowed client command executes", execute("git --version").returncode == 0)
            record("Forbidden client command denied", execute("cat .env").returncode != 0)
            public = request("/api/v1/inspector/data")
            private = request("/api/v1/admin/inspector/data", token=admin)
            record("Prompt protected in standard view", prompt not in json.dumps(public))
            record("Captured prompt reaches admin view", prompt in json.dumps(private))
            record("Real tool telemetry reaches server", any(e.get("session_id") == session_id and e.get("executable") == "git" for e in private["central_events"]))

            sdk_env = dict(test_env, BAP_CREDENTIALS=str(credentials))
            sdk_code = """import sys
sys.path.insert(0, sys.argv[1])
from bap_sdk import BAPSession
with BAPSession(app_id=sys.argv[2], instance_id=sys.argv[3], session_id=sys.argv[4], server_url=sys.argv[5], bapedge_path=sys.argv[6], user_prompt='Review repository changes.') as client:
    assert client.exec('git --version').is_allowed
    assert client.acquire_grant(['api:read'])
"""
            sdk = subprocess.run([sys.executable, "-c", sdk_code, str(root / "python-agent"), app_id, instance, session_id, server, str(edge)], cwd=cache, env=sdk_env, capture_output=True, text=True, timeout=30)
            record("Python SDK execution and enrolled grant acquisition", sdk.returncode == 0)
            request("/api/v1/sessions/start", {"session_id": session_id, "app_id": app_id, "instance_id": instance})
            request("/api/v1/control/agent/kill", {"target": session_id, "action": "revoke"}, token=admin)
            record("Targeted revoke blocks client", execute("git --version").returncode != 0)
            request("/api/v1/control/agent/kill", {"target": session_id, "action": "restore"}, token=admin)
            record("Targeted restore permits client", execute("git --version").returncode == 0)
            if args.local or args.exercise_global_kill:
                prior = request("/api/v1/control/kill-switch", token=admin)["kill_switch"]
                if prior:
                    raise RuntimeError("Fleet was already frozen; will not restore an existing emergency state")
                try:
                    request("/api/v1/control/kill-switch", {"enabled": True}, token=admin)
                    record("Global freeze blocks client", execute("git --version").returncode != 0)
                finally:
                    request("/api/v1/control/kill-switch", {"enabled": False}, token=admin)
                record("Global restore permits client", execute("git --version").returncode == 0)
            if args.hold:
                print(f"Open {server}/dashboard/ now. Active acceptance session: {session_id}")
                if args.local:
                    print(f"Admin credential file: {work / '.bap-admin-token'}")
                input("Press Enter to end the client and watch its 30-second departure countdown... ")
            request("/api/v1/sessions/end", {"session_id": session_id, "reason": "Acceptance run completed"})
            record("Deregistration is visible in registry", any(a.get("app_id") == app_id and a.get("status") == "deregistered" for a in request("/api/v1/agents")["agents"]))
            record("Audit chain verifies", request("/api/v1/control/chain/verify").get("valid") is True)
            if args.hold:
                print(f"\nOpen {server}/dashboard/ for the browser checklist.")
                if args.local:
                    print(f"Admin credential file (local only): {work / '.bap-admin-token'}")
                input("Press Enter when browser checks are complete to stop this run's services... ")
        except Exception as error:
            failures.append(type(error).__name__)
            print(f"[FAIL] {error}", file=sys.stderr)
        finally:
            for process in reversed(processes):
                if process.poll() is None:
                    process.terminate()
                    try: process.wait(timeout=8)
                    except subprocess.TimeoutExpired: process.kill(); process.wait(timeout=5)
            for handle in logs: handle.close()
    report = {"server": server, "gateway": gateway, "platform": sys.platform, "passed": not failures, "cases": results, "limitations": ["Browser checks require the manual checklist", "Only the host OS was executed", "Remote mode leaves test enrollment/history records", "No remote endpoint fallback"]}
    if args.report: args.report.write_text(json.dumps(report, indent=2), encoding="utf-8")
    print(f"\n{len(results)} checks; {'FAILED' if failures else 'PASSED'}. Browser and other OS checks remain separate.")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
