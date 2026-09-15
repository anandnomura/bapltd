# Choose your BAP deployment

Start here, then run the [manual acceptance guide](MANUAL_E2E_TESTING.md).
The [deployment review](DEPLOYMENT_REVIEW.md) distinguishes current behavior from
the proposed React/admin design. Existing component guides remain references.

## What runs where?

| Component | Runs on | Purpose |
| --- | --- | --- |
| `bapcontrolplane` | Governance server | Enrollment, grants, policy, sessions, inspector API |
| `bapedge` | Each developer/worker machine | Local execution policy and client integration |
| Inspector or React UI | Browser, served by a web server | Displays telemetry and requests administrative actions |
| `bapgateway` | In front of protected APIs | Checks and consumes grants; optional for local CLI-only testing |

The browser does not need `bapedge`. A separate UI server does not automatically
proxy API calls: the supplied inspector uses relative `/api/v1/...` URLs, which
go to whichever server served its HTML.

## Pick one path

| Path | Addresses | Use it for | Important limitation |
| --- | --- | --- | --- |
| A: local HTTP | UI and API at `http://localhost:18080` | First disposable development test | Server binds all interfaces; restrict inbound access with the host firewall |
| B: local HTTPS | UI and API at `https://localhost:18443` | Certificate/trust testing | Generated certificate must be trusted; auto mode regenerates it on restart |
| C: remote HTTPS | UI and API at `https://bap.example.internal:8443` | Shared environment after review fixes | Supply a trusted certificate with the real DNS name; remote admin needs an explicit flag |
| D: separate React UI | Browser → UI backend → control plane | Recommended production architecture | Backend and per-action reauthentication are proposed, not implemented here |

Remote HTTP should remain an isolated diagnostic setup. Changing a URL to
`https://` does not enable TLS on a server. Native `bapgateway` currently serves
HTTP; use a TLS-terminating proxy for its externally exposed HTTPS address.

## Build on your current OS

Run each command from the indicated module directory. Go does not treat this
repository as a single module.

```text
cd bap-controlplane
go build -o bapcontrolplane ./cmd/server
cd ../bap-edge
go build -o bapedge .
cd ../bap-gateway
go build -o bapgateway .
```

On Windows, use output names `bapcontrolplane.exe`, `bapedge.exe`, and
`bapgateway.exe`, and invoke them with `./` or an absolute path. On macOS/Linux
use `./bapedge`. Do not run a Linux binary in Windows PowerShell: WSL is a
separate Linux client with its own paths, configuration and trust store.

Release packages target Windows AMD64, Linux AMD64/ARM64 and macOS AMD64/ARM64.
Cross-compilation verifies compilation, not runtime compatibility. Test each
package on its target OS before release.

## Configure a client

In a disposable client directory, using the path to your built edge binary:

```text
bapedge config set --server https://bap.example.internal:8443 --gateway https://gateway.example.internal
bapedge config show
```

The first command writes `bap-config.json` in the current directory. Substitute
your actual URLs and binary path. `--global` instead writes `~/.bap/config.json`.
Always inspect `config show` from the directory and OS account that runs the
real client, including service accounts and IDE processes.

Current edge lookup: first readable JSON file in the working directory/three
parents, executable directory/parent, user configuration, then system
configuration. If the resolved control-plane URL is still the default, stored
enrollment credentials can replace it. Environment overrides are applied last:
`BAP_SERVER_URL`, then `BAP_CONTROL_PLANE_URL`, then `LTD_SERVER_URL` (first set
value wins). A command's explicit `--server` overrides its default.

Do not assume the gateway, SDK and edge share an identical resolver. In
particular, the edge resolver currently does not read `BAP_CONFIG` even though
some existing documentation mentions it. Check each process separately.

## Server settings you actually need

| Setting | Meaning |
| --- | --- |
| `-port` | Listening port; default 8080; currently binds all interfaces |
| `BAP_ADMIN_TOKEN` | Shared administrative credential, **not** a user/password account |
| `-allow-remote-admin` | Allows token-authenticated remote admin API calls |
| `BAP_SECRET_KEY` | Grant-signing secret; gateway must agree when it verifies locally |
| `-https -tls-cert cert.pem -tls-key key.pem` | HTTPS with supplied certificate/key |
| `-https` alone | Generates a development certificate for localhost/127.0.0.1 |
| `-db path` | Session database location; not all control-plane state persists |
| `-policy path -schema path` | Explicit policy files, independent of launch directory |

The server does not derive its listening scheme and port from a client
`controlplane_url`. Certificates supplied without enabling TLS do not by
themselves turn on HTTPS. Do not combine `-tls-auto` with supplied files: auto
mode selects a generated certificate.

For remote HTTPS, use a certificate issued by the organization's trusted CA,
with the server DNS name in its Subject Alternative Names. Install CA trust in
each browser/runtime/OS that connects. `BAP_CA_CERT` configures edge, gateway and SDK trust across operations.
`register --ca-cert` remains a per-command override; set the environment setting
for later operations too. The browser's TLS exchange happens before any login API request.

## Next steps

1. Follow [manual testing](MANUAL_E2E_TESTING.md), including negative cases.
2. Review [admin/HTTPS findings and proposed UI](DEPLOYMENT_REVIEW.md).
3. Use [endpoint details](ENDPOINT_CONFIGURATION.md) and
   [agent integrations](INTEGRATIONS.md) for your selected client.
4. Use [testing reference](TESTING_GUIDE.md) for older scenarios; do not interpret
   a historical test count as deployment coverage.
