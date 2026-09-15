# React dashboard and administrative actions

The new React/Vite dashboard is served at **`/dashboard/`**. It is built into
`bapcontrolplane`; the server does not need Node at runtime. The diagnostic
inspector remains at `/inspector?mode=live`.

## Recommended deployment today

Serve the UI and API on one trusted HTTPS origin. Use a supplied certificate:

```bash
export BAP_ADMIN_TOKEN='your-private-random-admin-credential'
export BAP_SECRET_KEY='your-private-random-signing-key'
./bapcontrolplane -port 8443 -https -tls-cert server.pem -tls-key server-key.pem -allow-remote-admin
```

On Windows PowerShell, set variables with `$env:BAP_ADMIN_TOKEN = '...'` and
`$env:BAP_SECRET_KEY = '...'`, then run `.\bapcontrolplane.exe` with the same flags.
Use your secrets mechanism rather than the placeholder values above.

The browser validates TLS before any API call. `-https` without supplied files
creates a localhost development certificate, not a certificate for your remote
DNS name. See [deployment choices](DEPLOYMENT_START_HERE.md).

## Current authentication contract

- Standard telemetry is available with prompt text redacted. Prompt reveal uses
  `GET /api/v1/admin/inspector/data` with an explicit admin Bearer header.
- Reveal requires a fresh credential entry, keeps a snapshot for 60 seconds,
  and discards the credential. The operator can hide prompts immediately.
- Freeze, revoke and restore each require a separate credential entry. The
  password input is masked. Cancel sends nothing; rejected requests do not
  change the displayed state or produce a success message.
- Server authorization accepts `Authorization: Bearer ...` or
  `X-BAP-Admin-Token`. Cookies no longer authenticate admin APIs.
- `POST /api/v1/auth/admin-login` accepts `{"token":"..."}` and verifies it.
  It returns status/role only, creates no cookie and never returns the token.
- `/api/v1/auth/inspector-handshake` only verifies an explicitly supplied header.
  Loopback callers do not receive special authentication privileges.
- Missing credentials return 401; invalid credentials or disabled remote admin
  return 403. An unconfigured admin middleware returns 503.
- Enrollment-code creation (`/agents/pre-register`) now requires admin access.
- `/control/agent/kill` requires an **exact session ID or agent ID** and an
  explicit `revoke` or `restore` action. Prefix matching and implicit toggles
  are removed. `/control/kill-switch` requires an explicit boolean `enabled`.

This server uses a **shared administrative secret**, not named user accounts.
A person who possesses the secret can call the API directly. A frontend dialog
cannot prove to the server that a password was freshly typed. Implement the
backend model below for that stronger guarantee.

## Separate React server

The UI uses relative `/api/v1/...` requests. Host the Vite build at `/dashboard/`
and proxy those API paths to BAP, or develop with `npm run dev` in `dashboard/`.
The development proxy's default upstream is `http://localhost:8080`; override
with `BAP_DEV_SERVER`.

For an approved different browser origin, configure an exact allowlist:

```text
bapcontrolplane -allowed-origins https://dashboard.example.internal,http://127.0.0.1:5173
```

The equivalent environment variable is `BAP_ALLOWED_ORIGINS`. Scheme, host and
port must match; do not append paths or a trailing slash. Wildcard and `null`
origins are rejected. The default is same-origin only. CORS does not authenticate
requests, and the backend does not trust `X-Forwarded-For` to bypass admin checks.
TLS-terminating proxies should configure the external origin explicitly because
an HTTP upstream sees a different scheme.

Never place the admin secret in a `VITE_*` variable, static bundle, URL, browser
storage or proxy rule that injects it into every request. Never disable TLS
verification to connect the proxy to the control plane. See
[MDN CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CORS).

## Stronger production option: UI backend and individual login

Use a backend on the same origin as React. It holds BAP's admin credential and
calls the control plane over verified HTTPS. The browser receives only an
opaque Secure/HttpOnly session cookie. Authenticate named users with the
organization's identity provider and check their roles on the backend.

For every freeze/revoke/restore:

1. Show the exact target, effect and reason in a confirmation dialog.
2. Require fresh identity-provider reauthentication, or verify a local password
   on the backend if you intentionally operate a local account system.
3. Bind the result to the user, action, target and payload; expire it quickly
   and consume it once. Check CSRF and allowed origin.
4. Execute only that authorized action. Log actor, target, reason and outcome;
   never log the password or BAP secret.
5. Refresh confirmed server state and clear pending UI state.

This backend/SSO system is **not implemented** by the current shared-token UI.
It is the recommended next step for individual admin accounts and enforced
per-action reauthentication. See
[OWASP reauthentication guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html).

Native BAP TLS does not currently configure client-certificate authentication;
mTLS would need explicit implementation or a suitable trusted proxy.

## Verify the deployment

Use [the full manual guide](MANUAL_E2E_TESTING.md), including browser tests,
wrong credentials, unapproved origins, certificate failures, cancellation,
repeated clicks, revocation and restore. Unit tests alone do not exercise
browser TLS or CORS.
