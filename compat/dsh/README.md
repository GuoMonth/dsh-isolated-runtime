# DSH compatibility baseline

The project supports exactly `dsh-v0.1.3-alpha.1` at
`d347e703908d0406b7a7ef80e3a0e594d86b2215`, installed with `pnpm@11.7.0` and
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
| Independent sidecar | Rejected: 0.1.3-alpha.1 has no supported way to inject or retrieve the launch token across a process boundary. |
| Cell-local launcher | Selected: the parent process observes readiness, keeps the token in memory, and proxies DSH opaquely. |

The current GitHub release has not been published to npm. The Cell image builds
`build:official` from the exact source archive/checksum in baseline.json, deploys
the upstream runtime closure, and completes omitted transitive workspace peers
from that same source. It does not substitute older npm packages. The only source change is the hashed
Cell settings integration patch described below.
The launcher remains PID 1. The native runtime dependencies are built in the image.

Snapshot guarantees remain writer-stopped crash consistency. Upstream owns its
session format v2 and migrations; this project maintains no historical restore
or migration layer. Credential filtering retains the pinned Envoy cookie family
and forged/stale unsuffixed names as part of the access boundary.

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

## Cell settings integration patch

The pinned upstream UI only loads persistent settings on loopback hostnames.
`patches/cell-settings.patch` enables its existing settings mirror in the Cell
image, whose remote access is protected by Gateway OIDC and Cell authorization.
It adds no UI or API and changes no server-side authorization or proxy behavior.
The source archive and patch are separately hashed in `baseline.json`; both the
image build and compatibility gate verify and apply the same exact patch.
The MVP browser gate must configure a key through the native editor at a Cell
hostname and verify that credentials remain outside the data snapshot.
