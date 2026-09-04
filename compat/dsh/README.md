# DSH compatibility baseline

The project supports exactly `dsh-v0.1.2-rc.1` at
`a66e4702047846cdaa10c66c9d3df3951f5ea70d`, installed with `pnpm@11.7.0` and
the frozen lockfile digest recorded in [`baseline.json`](./baseline.json).
Persistence is version-bound; passing a newer or older session format is not a
compatibility promise.

`make verify-dsh` performs a full upstream checkout and runs the upstream tests
covering browser token/cookie exchange, `settings/describe`, remote mux and ready
streams, Fetch GET/HEAD, Host/Origin and cross-site rejection, restart cookie
continuity, session-format rejection, SIGTERM, disposal, and shutdown. The local
Go suite separately exercises opaque proxying, redaction, cookie hardening,
signal forwarding, and failure cleanup, then drives the launcher with the exact
built DSH CLI for a real browser exchange.

## Access decision

| Candidate | Result |
| --- | --- |
| Direct DSH exposure | Rejected: the standard CLI intentionally binds loopback and the launch URL contains a bearer token. |
| Gateway configuration only | Rejected: it cannot own the process-memory token exchange or harden the returned cookie. |
| Independent sidecar | Rejected: 0.1.2 RC has no supported way to inject or retrieve the launch token across a process boundary. |
| Cell-local launcher | Selected: the parent process observes readiness, keeps the token in memory, and proxies DSH opaquely. |

The source checkout remains the behavioral baseline. The production Cell image
installs the official `@deepseek-ai/dsh@0.1.2-rc.1` npm artifact with the
tarball integrity recorded in `baseline.json`; the launcher is its PID 1.
The 0.1.2 RC web profile enables live patch watching, so the launcher invokes
its official CLI through `node --expose-internals`; this is required by DSH's
own HMR loader and does not expose the launch token.

The exact RC does not expose an application-flush acknowledgement. Its SIGTERM
path uses exit code zero after successful disposal, disposal rejection, and
timeout, so those outcomes are externally indistinguishable. Phase 3.1 therefore
removed `POST /quiesce`: snapshot consistency is established only by setting the
StatefulSet to zero, observing zero replicas, and proving that no Pod owned by
that StatefulSet UID remains. The resulting guarantee is writer-stopped and
crash-consistent, not application-consistent. Ordinary Pod termination still
uses the launcher's bounded HTTP/WebSocket drain and SIGTERM forwarding.

At the access boundary, the launcher removes identity headers and every default
credential cookie of pinned Envoy Gateway v1.9.1 before DSH. Current names are
`AccessToken`, `OauthHMAC`, `OauthExpires`, `IdToken`, `RefreshToken`,
`OauthNonce`, and `CodeVerifier`, each followed by the policy's eight-digit hex
suffix. Exact legacy names are reserved too. DSH and unrelated application
cookies remain intact.

## State ownership

| State | Owner | Data snapshot |
| --- | --- | --- |
| Sessions, attachments, storage domains | DSH on data PVC | Yes |
| Workspace | Cell data PVC | Yes |
| Configuration | DSH home, excluding secrets/signing records | Yes |
| Provider credentials | Kubernetes Secret / separate DSH credentials mount | No |
| Browser signing records | Separate DSH credentials mount | No |

This mapping is evidence for the exact release only. A DSH upgrade requires a
new baseline, compatibility run, and explicit persistence decision.
