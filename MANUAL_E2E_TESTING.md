# BAP manual end-to-end acceptance

Use this guide for a new deployment, certificate change, client rollout or
release. A healthy API alone does not prove the client, gateway or browser works.
Start with [deployment choices](DEPLOYMENT_START_HERE.md).

## 1. One command for an isolated full run

Prerequisites: Python 3, Go, Git on PATH. Node is only needed if you changed the
React source. Run from the repository root in Windows PowerShell, macOS Terminal
or a Linux shell:

```text
python scripts/manual_acceptance.py --local --https --hold --report acceptance.json
```

Use `python3` if that is your Python executable. To test HTTP instead, omit
`--https`. The script builds **native** binaries, chooses free ports, creates
temporary state and a private signing key, launches only its own processes and
tests the exact URL it prints. It does not use release binaries from `dist/` or
overwrite your user enrollment/configuration. It terminates only its own
processes. Temporary state is removed when the run ends.

It exercises:

| Check | Required result |
| --- | --- |
| Health and admin access | Correct service; valid admin accepted |
| No admin credential | Kill request is 401; handshake cannot return the secret |
| Unapproved browser origin | 403, without reflected CORS permission |
| Real edge enrollment | Production binary hash accepted, credentials stored in the test directory |
| Enrollment replay | Same one-time code rejected |
| Grant acquisition | Public agent ID/hash alone rejected; enrolled bearer credential accepted |
| Gateway | Anonymous request rejected; valid grant accepted once; replay rejected |
| Policy sync | Explicit **Online** success, not an offline cache fallback |
| Client execution | `git --version` allowed; `cat .env` denied |
| Prompt and tool telemetry | Actual command and supplied prompt reach the control plane |
| Prompt privacy | Standard view redacted; authenticated admin view reveals it |
| Targeted control | Revoke denies the next client action; restore permits it |
| Global control | Freeze denies the next action; restore permits it |
| Deregistration | Registry reports the completed client as deregistered |
| Audit chain | Server reports `valid: true` |

Every `[PASS]` names its assertion. A failure stops dependent steps and returns
a nonzero exit code. `acceptance.json` contains results without credentials.
Do not substitute a historical “48 passed” count for these deployment results.

## 2. Check a remote server and client

Use a designated test environment: this creates an agent enrollment and session
and exercises targeted revocation. History records remain on the server.

Build the native edge binary on the **client machine**, then run there:

```text
python scripts/manual_acceptance.py --server https://bap.example.internal:8443 --edge /absolute/path/to/bapedge --gateway https://gateway.example.internal --ca-cert /path/to/company-ca.pem --report remote-acceptance.json
```

Windows example:

```powershell
python scripts/manual_acceptance.py --server https://bap.example.internal:8443 --edge C:\BAP\bapedge.exe --gateway https://gateway.example.internal --ca-cert C:\BAP\company-ca.pem --report remote-acceptance.json
```

Omit `--ca-cert` for certificates already trusted by the client OS/runtime.
The script securely prompts for the admin credential if `BAP_ADMIN_TOKEN` is
not set. On the server, enable `-allow-remote-admin` for this authenticated remote
administrative test. The script never changes that server setting.

If no gateway is supplied, the script checks direct grant consumption and
reports the gateway path as **not tested**. Remote global freeze is omitted
unless `--exercise-global-kill` is explicitly supplied. Use that option only
against a dedicated test fleet: it affects every workload on that control plane.

The script never substitutes localhost if the remote host fails. TLS trust,
wrong hostname, connection refusal and HTTP authorization failures are failures
of the selected deployment.

## 3. Test the React dashboard in the browser

Start `bapdashboard` separately and open its `/dashboard/` URL. For the default
local ports, open `https://localhost:8444/dashboard/`. The control-plane port is
API-only and does not serve a dashboard or diagnostic fallback.

For local generated HTTPS certificates, first establish trust in the browser's
trust store. The certificate is in the temporary directory printed by the run.
The Python script's CA setting does **not** install browser trust. A certificate
warning is not a passing production TLS test. Use an organization-issued local
test certificate if your browser will not trust the generated leaf certificate.

| Step | Action | Expected result |
| --- | --- | --- |
| B01 | Open DevTools Network and Console, then reload | HTML/CSS/JS and API load without mixed-content, certificate, CORS or script errors |
| B02 | Watch the connection indicator | Live after successful API responses; refresh timestamp advances |
| B03 | Start a real client session | Agent appears with instance, user, host and last-seen time; no sample agents appear automatically |
| B04 | Send a permitted command and a denied command | Both appear with actual tool/action and decision |
| B05 | Send a prompt through an instrumented client | Standard view says protected; absence of client prompt metadata says “Not captured” |
| B06 | Click Reveal prompts; enter a wrong credential | Rejected; no prompt disclosure; input cleared |
| B07 | Enter the correct credential | Captured prompts visible for 60 seconds; Hide prompts immediately removes them |
| B08 | Click Revoke and then Cancel/Escape | No mutation request; agent remains active |
| B09 | Click Revoke, enter a wrong credential | Error shown; no success claim or revoked state |
| B10 | Enter correct credential and confirm Revoke | Server confirms; row becomes revoked; next client command is blocked |
| B11 | Click Restore | Requests the credential again; successful restore permits client execution |
| B12 | Double-click Confirm during a slow request | One mutation; button disabled until response; a timeout is not displayed as success |
| B13 | End a client normally | Session closes; its durable agent identity remains visible and eventually shows offline |
| B14 | Switch to All agents & history | Agent and session history remain available; search finds them |
| B15 | Interrupt a client's reporting without sending session-end | Shows stale after 15 seconds and offline after 60 seconds without removing the row |
| B16 | Stop/disconnect the control plane | Connection changes to stale; last known data remains identifiable; action buttons disabled |
| B17 | Inspect Application storage and cookies | No admin token in localStorage/sessionStorage/cookies; no token in URLs |
| B18 | Keyboard-only navigation, 200% zoom, narrow screen | Dialog focus, Escape/cancel, labels and scrollable tables remain usable |

The dashboard currently uses a shared admin credential. It prompts for every
action, but this is not individual user/password authentication. For enforced
per-user reauthentication and role audit, use the backend design in
[React deployment](REACT_ADMIN_INTEGRATION.md).

## 4. Supply actual prompt metadata

The client integration must supply the user's prompt. BAP does not infer it
from a command or read an IDE transcript automatically.

Python SDK:

```python
from bap_sdk import BAPSession

with BAPSession(
    app_id="review-worker",
    user_prompt="Inspect the latest changes and summarize any risks.",
) as session:
    session.exec("git status")
```

For edge/CLI integrations, set `BAP_USER_PROMPT` in the calling process:

```powershell
$env:BAP_USER_PROMPT = 'Inspect the latest changes and summarize any risks.'
.\bapedge.exe exec --source review-worker 'git status'
Remove-Item Env:BAP_USER_PROMPT
```

```bash
BAP_USER_PROMPT='Inspect the latest changes and summarize any risks.' ./bapedge exec --source review-worker 'git status'
```

For gateway grants, enroll first with `bapedge register` and point the SDK at the
resulting file using `BAP_CREDENTIALS` if it is not `~/.ltd/credentials.json`.
The enrollment must include the requested scopes and belong to the SDK's server
URL. Session presence alone no longer grants authority to mint tokens.

## 5. Certificate and topology negative tests

Run these from each actual client OS and browser. Use disposable certificates
and restore the original test configuration afterwards.

| Variation | Expected |
| --- | --- |
| HTTPS server URL uses a DNS name absent from the certificate | TLS rejection before login |
| Remove private CA from test client's trust configuration | TLS rejection; not a passing offline-sync result |
| Expired certificate or mismatched key | Client rejection or server startup failure |
| HTTPS dashboard calls remote HTTP control plane | Browser rejects unsafe mixed-content request; fix routing/TLS |
| Separate UI origin not in allowlist | 403 / failed browser access |
| Exact allowed UI origin and correct credential | CORS and authenticated action succeed |
| Remote admin disabled with otherwise valid token | 403 |
| Proxy uses loopback upstream | No anonymous handshake/login or prompt bypass |
| Control plane unreachable during gateway consumption | Denied; no silent local verification fallback |
| Registration succeeds with `BAP_CA_CERT` | Confirm sync, telemetry, revocation polling and gateway consumption also work |
| Restart with supplied certificate | Same trusted identity; auto-generated dev certificates instead change per restart |

`BAP_CA_CERT` is now the common PEM trust setting for edge registration, sync,
watch, audit and revocation calls, the native gateway, and the Python SDK.
`register --ca-cert` remains a per-command override. Set the environment setting
for the later commands as well. Never use `--insecure` as acceptance evidence.

## 6. Offline and restart checks

Use a disposable client directory and explicit `--policy` / `--policy-dir` so
you do not accidentally exercise your normal user cache.

1. Successfully sync policy online. Record version and digest.
2. Stop the test control plane. Repeat sync: it must explicitly report offline
   fallback when using the existing cache. Confirm permitted commands still work
   and denied commands remain denied.
3. In a different **empty** policy cache, repeat sync with the server down:
   it must fail rather than invent a successful synchronization.
4. Restart the test server, engage global freeze and let the client receive it.
   Stop the server again. The previously informed client must stay blocked.
5. A client partitioned **before** the freeze cannot learn a new revocation.
   Record this limitation; do not report instant offline revocation.
6. Restart the server and check sessions, registry, audit and global kill state
   separately. SQLite currently persists sessions, not every store. Registry
   history in the UI is history retained by the running server, not a durable
   archival guarantee. This remains a production hardening item.

## 7. Record coverage honestly

Repeat the applicable scenario on Windows AMD64, Linux AMD64/ARM64 and macOS
AMD64/ARM64. Treat WSL as a separate Linux client. Repeat browser steps for each
supported browser and same-origin/separate-UI topology.

```text
Commit / binary hash:
Date / tester:
Server OS and exact URL:
Client OS / architecture:
Browser and version:
HTTP / HTTPS / proxy arrangement:
Certificate identity and expiry (no private keys):
Automated acceptance report:
Browser cases passed / failed / not tested:
Other deployment cases passed / failed / not tested:
```

Automated browser tests: `cd dashboard`, `npm ci`, `npx playwright install chromium`,
then `npm run test:browser`. These start a disposable real Go server and exercise
the built React UI. They currently run Chromium; they do not establish that
Firefox, Safari, every OS or corporate proxy works.
