# DSH compatibility baseline

Phase 0 supports exactly `dsh-v0.1.2-alpha.4` at
`4e84901e6471b79ec0338099867ebb4606d12bb5`, installed with `pnpm@11.7.0` and
the frozen lockfile digest recorded in [`baseline.json`](./baseline.json).
Persistence is version-bound; passing a newer or older session format is not a
compatibility promise.

`make verify-dsh` performs a full upstream checkout and runs the upstream tests
covering browser token/cookie exchange, `settings/describe`, remote mux and ready
streams, Fetch GET/HEAD, Host/Origin and cross-site rejection, restart cookie
continuity, session-format rejection, SIGTERM, flush, and shutdown. The local
Go suite separately exercises opaque proxying, redaction, cookie hardening,
signal forwarding, and failure cleanup, then drives the launcher with the exact
built DSH CLI for a real browser exchange.

## Access decision

| Candidate | Result |
| --- | --- |
| Direct DSH exposure | Rejected: the standard CLI intentionally binds loopback and the launch URL contains a bearer token. |
| Gateway configuration only | Rejected: it cannot own the process-memory token exchange or harden the returned cookie. |
| Independent sidecar | Rejected: alpha.4 has no supported way to inject or retrieve the launch token across a process boundary. |
| Cell-local launcher | Selected: the parent process observes readiness, keeps the token in memory, and proxies DSH opaquely. |

The source checkout remains the behavioral baseline. The production Cell image
installs the official `@deepseek-ai/dsh@0.1.2-alpha.4` npm artifact with the
tarball integrity recorded in `baseline.json`; the launcher is its PID 1.
The alpha.4 web profile enables live patch watching, so the launcher invokes
its official CLI through `node --expose-internals`; this is required by DSH's
own HMR loader and does not expose the launch token.

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
