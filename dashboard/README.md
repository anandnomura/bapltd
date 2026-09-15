# BAP React dashboard

React + Vite source for `/dashboard/`. Production assets are embedded into the
control-plane binary. The UI uses the real inspector API, never generates demo
agents, and requires a fresh credential entry for each administrative request.

```text
npm ci
npm run build
npm test
npm run test:browser
```

Install the browser once with `npx playwright install chromium`.
Build output is checked in under
`bap-controlplane/internal/api/web/dashboard-build`; rebuild it before a Go
release when frontend source changes. No Node runtime or external CDN is needed
to serve the built dashboard. Use a Node version supported by
[Vite](https://vite.dev/guide/) (20.19+ or 22.12+ for the installed major).

For development: run the control plane, then `npm run dev`. It proxies `/api`
to `http://localhost:8080`; set `BAP_DEV_SERVER` to change that upstream.
Browser Origin is preserved. If the origin differs from the API's accepted
origin, configure `-allowed-origins http://127.0.0.1:5173` on the control plane.
Do not put admin secrets in `VITE_*` variables: those values become browser code.

For a separately hosted production UI, deploy the build directory at
`/dashboard/` and reverse-proxy `/api/v1/` to BAP with TLS verification.
Do not auto-attach an admin token at a generic proxy. See
[the React/admin integration guide](../REACT_ADMIN_INTEGRATION.md) for the
recommended backend with individual login and per-action authorization.

The live view hides deregistered agents after 30 seconds using server time;
the history view retains them while the server registry retains them. Revoked
agents stay visible. A missing heartbeat becomes stale, not deregistered.

Prompt reveal fetches an authenticated snapshot and clears it after 60 seconds.
The browser does not retain the credential. This shared-token interface is not
an implementation of individual password accounts or SSO reauthentication.
