# Deployment, admin authentication and inspector review

Reviewed source on 2026-09-14. This records initial findings, fixes implemented during this review, and
remaining production architecture work.

## Implemented in this change

- React/Vite dashboard at `/dashboard/`, embedded in Go: live registry, 30-second
  departures, runtime history, stale-state distinction, searchable agents,
  actual tool actions and protected captured prompts.
- Admin actions and prompt reveal request a credential each time; no browser
  token persistence, automatic local login or secret-returning handshake.
- Explicit origin allowlist, cookie-free admin authentication, consistent prompt
  redaction, protected enrollment-code creation and fail-closed unconfigured auth.
- Exact action targets and explicit restore/revoke/global enabled values;
  client session start/end cannot remove administrative revocation.
- Grant acquisition requires enrolled proof or admin authority. Requested scopes
  cannot exceed permitted scopes; central consumption checks current registry
  status and global freeze even for previously issued grants.
- Central-consumption failure denies gateway access instead of bypassing replay
  protection. Published signing defaults removed; local default is a random key.
- `BAP_CA_CERT` configures trust across edge operations, gateway and Python SDK.
- Real prompt metadata from the SDK or `BAP_USER_PROMPT` reaches the dashboard;
  unsupported client integrations say Not captured.
- Isolated acceptance script, API/TLS/security regressions, presence boundary
  tests and browser tests against a real Go server.

## Recommendation

Use a React UI and a small backend on one HTTPS origin. That backend calls
`bapcontrolplane` over verified HTTPS and holds the BAP administrative secret.
Operators authenticate as individual users. Clicking **Freeze fleet**,
**Revoke**, or **Restore** opens a confirmation dialog and requires fresh
password verification or an identity-provider reauthentication challenge.

```mermaid
sequenceDiagram
    actor Operator
    participant UI as React browser UI
    participant BFF as UI backend
    participant IdP as Identity provider
    participant CP as BAP control plane
    Operator->>UI: Freeze fleet
    UI->>Operator: Show target, impact, reason; request reauthentication
    UI->>BFF: Action intent and reauthentication request
    BFF->>IdP: Verify fresh authentication
    IdP-->>BFF: Verified identity
    BFF->>BFF: Check role, CSRF, target and action; authorize once
    BFF->>CP: Exact action over HTTPS with server-held BAP credential
    CP-->>BFF: Result
    BFF-->>UI: Confirmed result or actionable error
```

SSO passwords should be entered only on the identity provider's page. If local
accounts are chosen, password verification, hashing and throttling belong on
the backend. The current BAP token is a shared API secret, not a password
database. Do not rename its input to “admin password” and imply otherwise.

The backend must bind authorization to the user, exact target, action and
payload, expire it quickly, and consume it atomically once. Rechecking a client
boolean or relying on a disabled button is insufficient. A normal logged-in
session must not authorize a kill without that action's fresh verification.
Keep passwords and BAP tokens out of URLs, browser storage, JavaScript bundles,
logs and telemetry. Use opaque server sessions with Secure/HttpOnly cookies,
CSRF validation and explicit origin checks. Record actor, target, reason,
request ID and outcome, without recording credentials.

This follows [OWASP's recommendation to reauthenticate for sensitive operations](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).

## Initial findings (before the fixes above)

| Priority | Evidence | Impact / recommended correction |
| --- | --- | --- |
| P0 | `internal/api/handlers.go`: `handleInspectorHandshake`, `handleAdminLogin`, `isAdminCaller` | Loopback grants privileged treatment; handshake returns the master secret, empty local login succeeds, and local telemetry callers are treated as admins. Remove address-based authentication. A reverse proxy on loopback can make remote traffic look local. |
| P0 | `Server.Handler` | Reflects every supplied Origin with credential permission. Together with local token disclosure this creates a dangerous browser access path where network/browser policy allows the request. Use a strict origin allowlist; reject unapproved state-changing origins; CORS is not authentication. |
| P0 | Handshake/login cookie construction | Master token is returned in JSON and a JavaScript-readable cookie, without Secure or expiry. Replace with short-lived opaque sessions at the UI backend; never distribute the master secret to browsers. |
| P1 | `registerRoutes` and `handlePreRegister` | Enrollment-code creation is public. Define authorized enrollment operators and authenticate code creation before treating it as an enterprise enrollment boundary. Agent enrollment and monitoring also need an explicit API authorization review. |
| P1 | `bap-edge/cmd/register.go`, `cmd/sync.go`, `internal/policystore/store.go` | Custom CA option exists for registration only. Enrollment may succeed while subsequent sync falls back to cache after TLS failure. Centralize HTTP transport/trust settings across enrollment, sync, telemetry and grant calls. |
| P1 | `cmd/server/main.go` | Auto cert contains only localhost/127.0.0.1 and is regenerated each start. Remote DNS, IPv6 and restart trust can fail. Prefer stable supplied certs; validate conflicting/partial TLS flags at startup. |
| P1 | `inspector.html`, agent chip and session card click handlers | Several actions ignore HTTP status and immediately display success. Use one API client that checks status, shows pending/error states, prevents duplicate submits and refreshes confirmed server state. |
| P1 | `handleTargetedKillAgent` | Empty target defaults to a demo user; omitted action can toggle state. Require an exact stable ID and explicit action; reject malformed/unknown targets; retries must not restore something accidentally. |
| P1 | `cmd/server/main.go` | Default signing secret remains accepted; generated admin token is logged and written to disk. Production startup should require configured secrets and fail safely on missing/unreadable configuration. |
| P1 | `cmd/server/main.go`, in-memory registry/policy/audit stores | Session SQLite persistence does not imply persistence of registry, global kill state, grants or audit. Test restart semantics before documenting persistent centralized revocation. |
| P2 | `handleInspectorUI`; root and module `inspector.html` | File selected depends on working directory. Root/module copies currently match, but can drift from release copies. Maintain one canonical asset and embed or package it deterministically. |
| P2 | Inspector stylesheet and markup | Dark full-page background, tiny text, pulsing glows and demo controls compete with operations. Adopt the lighter layout below and move demo/replay to a separate view. |

Paths without a module prefix above refer to `bap-controlplane`.
These findings are based on source inspection; an end-to-end malicious browser
exploit was not executed. Existing protected mutation routes do require a
token when configured; the problem includes how that token is obtained and
reused, not only whether the mutation route has middleware.

## Why the browser connection feels fragile

Treat these as three separate checks, in order:

1. **Transport:** DNS, TCP reachability, certificate name, validity and CA trust.
2. **Browser access:** same origin or an approved cross-origin response/preflight.
3. **Identity and authorization:** who may view data and who may perform this action.

A login endpoint cannot fix an untrusted certificate. HTTPS pages generally
cannot fetch remote HTTP APIs; loopback has special browser treatment and must
not be used to extrapolate remote behavior. See
[MDN mixed content](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Mixed_content).
Cross-origin authorization headers trigger preflight, and credentialed cookie
requests add cookie-policy constraints. See [MDN CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS).

| Option | Benefit | Cost / decision |
| --- | --- | --- |
| Same-origin UI backend (recommended) | Browser has one trusted origin; master token stays server-side; per-user audit and reauthentication | Requires backend sessions and action authorization |
| Direct React → control plane | Fewer services | Requires native user sessions, strict CORS, CSRF strategy and step-up endpoints in BAP; today's shared-token login is insufficient |
| Token entered for each action | Small interim change for isolated admin tools | Browser still sees a reusable master secret; a backend cannot prove the operator typed it freshly. Does not meet strong per-action reauthentication |

A generic reverse proxy that attaches the admin token to every browser request
is not a UI backend authorization layer. Allow only specific routes/methods,
validate input and permissions, and restrict direct control-plane access.
Native BAP TLS currently provides server authentication; client-certificate
authentication (mTLS) is not configured by the supplied server flags.

## A lighter inspector

Open `/dashboard/` on the running server. The [React implementation](dashboard/README.md)
replaces the initial layout proposal. The inspector remains a diagnostic fallback.

- Use a pale gray page, white cards, dark slate text and restrained blue accents.
- Show endpoint, connection state, last successful refresh and viewing role first.
- Make sessions a readable table with search/filter controls and a details pane.
- Keep global fleet actions separate from row actions; replace ambiguous “×” with “Revoke”.
- Use a masked password input or SSO challenge only in the action dialog; show target and impact before confirmation. Clear input on close and after every attempt.
- Use at least 14px body text, visible keyboard focus and status text alongside color; verify contrast and 200% zoom. Disable nonessential motion when reduced motion is requested.
- Mark stale data visibly. Never show “revoked” merely because a request was sent.
- Bundle styles locally so an internal deployment does not depend on a public development CDN.

## Test coverage plan

The Windows runner increments a counter for commands and entire suites. Its
summary is not a count of independent deployment cases. It can also fall back
from an unreachable remote server to localhost. A passing run therefore cannot
prove that the remote deployment worked. The existing API lifecycle fixture
does not configure admin security.

Added in this review: `internal/api/deployment_test.go` covers eight configured
admin/network combinations, missing credentials across ten protected routes,
and four real HTTPS trust/name cases. These are 22 subcases, not 22 new
end-to-end deployments. They intentionally do not bless the unsafe handshake
behavior as an expected security contract.

| Layer | Required additions / release gate |
| --- | --- |
| Unit/API, every PR | No secret disclosure from handshake/login; loopback cannot bypass login; origin allow/deny/null and preflight; invalid JSON; auth expiry/revocation; permission checks for each route; explicit target/action validation |
| Client config, every PR | File/env/flag precedence for edge, gateway and SDK; malformed config; service-account working directory; IPv6 URLs; unreachable remote must not silently become a local passing test |
| TLS integration, every PR | Supplied/missing/mismatched cert/key flags; expired cert; stable cert after restart; CA configuration reused through registration → sync → execution → audit → gateway |
| Browser, every PR | Real Chromium/Firefox/WebKit tests for password dialog, cancellation, bad password, expired/replayed action authorization, role denial, 401/403/5xx, timeout, duplicate submit, stale data and secrets absent from storage |
| Deployment, nightly | Local HTTP; local trusted HTTPS; remote HTTPS; separate origins; same-origin backend; TLS termination proxy; denied origin; certificates untrusted/expired/wrong name |
| Platform, release | Run native binaries on Windows AMD64, Linux AMD64/ARM64, macOS AMD64/ARM64; WSL separately; enrollment, allowed/denied commands, offline behavior and gateway replay on each |
| State and failure, release | Two agents, targeted/global revoke and restore, partition before/after revoke, process restart, SQLite failure, policy rollback, certificate rotation, telemetry delivery failure |

Use pairwise coverage for routine combinations, but run every supported
production topology through enrollment, execution, telemetry and administrative
revocation. Label unsupported and untested configurations explicitly. Keep test
state, ports, credentials and output directories isolated; terminate only
processes started by that test. Save results by commit, OS, topology and case ID.

## Suggested implementation order

1. Remove secret-disclosing local authentication and unrestricted origins; add regression tests for both before wiring a proxy.
2. Choose the UI backend contract, implement per-user sessions and one-action reauthentication, then remove browser master-token storage.
3. Unify TLS/config behavior across all clients and validate startup flags.
4. Replace inspector fetch calls with a shared error-aware client and apply the lighter layout to one canonical UI asset.
5. Add native platform and browser CI, and use the manual guide as release acceptance evidence.

## Remaining limits

The credential dialogs do not implement named user accounts, server-enforced
password step-up or SSO. Session and audit writes remain cooperative telemetry,
not verified human identity: a displayed email is not cryptographic evidence of
who submitted it. Workload-authenticated telemetry remains production work.

SQLite persistence remains limited: registry, global policy state and central
audit are not durable across restart. Disconnected clients cannot enforce a
revocation they have not received. Explicit gateway `-consume=false` lacks
central one-time consumption/revocation guarantees and needs a configured secret.
Native mTLS and browser/OS coverage beyond the executed environments remain
follow-up work. These changes do not constitute a comprehensive security audit.
