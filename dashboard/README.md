# BAP React dashboard

React + Vite source for the standalone `bapdashboard` service. Production assets
are embedded into that binary, never into `bapcontrolplane`. The UI uses the real inspector API, never generates demo
agents, and requires a fresh credential entry for each administrative request.

```text
npm ci
npm run build
npm test
npm run test:browser
```

Install the browser once with `npx playwright install chromium`.
Build output is checked in under
`bap-controlplane/internal/dashboardui/web`; rebuild it before a Go
release when frontend source changes. No Node runtime or external CDN is needed
to serve the built dashboard. Use a Node version supported by
[Vite](https://vite.dev/guide/) (20.19+ or 22.12+ for the installed major).

For development: run the control plane, then `npm run dev`. It proxies `/api`
to `http://localhost:8080`; set `BAP_DEV_SERVER` to change that upstream.
Browser Origin is preserved. If the origin differs from the API's accepted
origin, configure `-allowed-origins http://127.0.0.1:5173` on the control plane.
Do not put admin secrets in `VITE_*` variables: those values become browser code.

The packaged `bapdashboard` binary serves `/dashboard/` over HTTPS and
reverse-proxies `/api/v1/` to BAP with CA verification. Optional `-client-cert`
and `-client-key` flags support control planes that require mutual TLS.
Do not auto-attach an admin token at a generic proxy. See
[the React/admin integration guide](../REACT_ADMIN_INTEGRATION.md) for the
recommended backend with individual login and per-action authorization.

Agents remain visible when heartbeats are missed. Presence progresses from
active to stale to offline; revocation remains a separate administrative state.

Prompt reveal fetches an authenticated snapshot and clears it after 60 seconds.
The browser does not retain the credential. This shared-token interface is not
an implementation of individual password accounts or SSO reauthentication.
