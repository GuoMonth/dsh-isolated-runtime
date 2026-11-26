# DSH 0.2.2-rc.5 compatibility

[简体中文](README.zh-CN.md)

The workload installs the official npm release at source [9c2670d](https://github.com/deepseek-ai/deepseek-harness/tree/9c2670d3bce5275e0d1610272958121af87787bb). [baseline.json](baseline.json) records its version, package integrity, transport and data paths; the committed npm lockfile fixes the full closure. The launcher `/version` response and image label carry that pin. The runtime 0.5.0-rc.4 candidate targets this pin. The published runtime 0.5.0-rc.3 and platform 0.12.0-rc.2 pair uses DSH 0.2.2-rc.3.

## Reuse official capabilities

Follow the upstream [architecture](https://github.com/deepseek-ai/deepseek-harness/blob/9c2670d3bce5275e0d1610272958121af87787bb/docs/architecture.md): supported applications start through profiles, and plugins contribute capabilities through Cordis. Consume published artifacts rather than forking the application or rebuilding its plugin tree here.

| Capability | Choice |
| --- | --- |
| Launch and composition | Plain Node launches the official `dsh web` profile. Profiles, bundles, plugin installation and config reload stay upstream. The old `--expose-internals` requirement is removed after a real-package startup check. |
| UI, worktrees, subagents and teams | Use capabilities delivered by the official profile and Plugin Manager; experimental capabilities keep their upstream status. The platform does not run a second agent loop or enable extra plugins automatically. |
| Settings, credentials and Sessions | Use the native Host APIs and storage. No settings-client patch, alternate Session store or data converter. |
| Unary RPC and streaming | Forward native HTTP and `/api/remote.mux` WebSockets without implementing model/tool protocols, codecs, stream recovery or heartbeat policy. |
| Organization identity | OIDC membership and access revocation remain platform responsibilities; a DSH account or browser cookie does not authorize access to another environment. |
| Resource lifecycle | Runtime verifies exact owner, Sandbox/PVC/Pod identities and positive writer termination. These Kubernetes deployment obligations are separate from DSH's application lifecycle. |

The only deployment overlay sets `workspace-controller.documentsDirectory` to the persistent working directory. Headless Linux has no desktop Documents lookup; the official controller still owns workspace layout and persistence.

The launcher retains a narrow cookie adapter: official browser authentication still emits `SameSite=Strict` and exposes no SameSite configuration. An external OIDC redirect chain needs `Lax` for safe top-level navigation. The proxy adds `Secure` for the platform's HTTPS entry, while cookie values, signatures, authority checks and authentication remain native. It does not reconstruct upstream authentication. Keep the native token process-local, preserve Host/Origin, and reject unauthorized or foreign-authority requests.

## SDK security updates

All official DSH packages and their source/integrity pin remain at RC5. The workload manifest adds three exact transitive security overrides, reproduced by its npm lockfile: MCP client 2.4.0 ([cross-origin redirect credentials](https://github.com/advisories/GHSA-6prh-2h8m-c8cw)), Undici 8.12.4 ([TLS validation and related fixes](https://github.com/advisories/GHSA-2m5v-pch6-74hx)) and fflate 0.8.3 ([malformed ZIP64 loop](https://github.com/advisories/GHSA-px8p-9vwx-vf98)). This updates dependencies without patching DSH source or reimplementing their protocols. Reassess these overrides when adopting a newer official release.

`make security` audits both Node closures and Go application/test dependencies; network/database errors fail the gate. The native gate checks every nested SDK copy against the security pins, then runs same-origin MCP RPC and a real two-origin redirect rejection fixture with synthetic headers. Browser and volume checks validate this modified workload closure separately from upstream package identity.

## Verify the exact distribution

```sh
make verify
make verify-dsh
```

The native gate consumes the committed npm closure and starts the real web profile without model calls. PR CI runs it on Node 24.22.3 and 26.13.1 when DSH integration inputs change. The Go probe verifies Session RPC, WebSocket snapshots, export and recovery across a launcher restart. Extended browser checks use [test/rc2](../../test/rc2/README.md); Linux workload checks use [test/environment](../../test/environment/README.md). Platform authorization and real provider acceptance have separate harnesses.

Fresh installation and same-version restart are distinct from old-data migration. Keep upstream Session generations and storage unchanged; this candidate does not establish an in-place upgrade or downgrade path. A successful SIGTERM exit is not a durable flush acknowledgement. The [runtime contract](../../docs/en/contract.md) retains those boundaries.
