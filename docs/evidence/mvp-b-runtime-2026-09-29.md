# B: production AgentEnvironment lifecycle, 2026-09-29

Scope: runtime #97/#98, following merged A/G1. Runtime PR [#106](https://github.com/GuoMonth/dsh-isolated-runtime/pull/106). Platform consumption/installation remains C/D/E; this report does not claim their integrated acceptance.

## Exact artifacts

- Connector source: `286a68d3ae592fc1ab6193297b79f5fdc9964a3b` (initial implementation `29530cbd2fec4da4b457efd8dfec64e239e7313d`, then current-list-RV stop fix).
- `@dsh/environment-connector-internal@0.0.0-rc.2`, tarball SHA256 `d2d22257c69f1f87e3ca982557530a1133cb7abfa7c94cc9719f84837222224c`.
- npm integrity `sha512-91yvF4rleoFOFcK+CCJwLDi/zeOOQPpcTr8ANb9dPsOnSgvMSUZjgxlblxlVCmQOGzkFz1jD9csTwI6fhWfLNA==`.
- Environment image source remains `29530cbd2fec4da4b457efd8dfec64e239e7313d`; the later Connector fix does not change image inputs. Linux/amd64 manifest: `docker.io/library/dsh-mvp-rc2@sha256:338d50f33680b8b1e10c6691596118e2273e48f084a609ea2734143c54a5feff`.
- Local Docker tag `dsh-mvp-rc2:b-runtime`; Docker index ID `sha256:233057cd5dc071aa6373a9d02431f5adc4272cc3048d489d7e4882ea18f29dac`, image config `sha256:bb3f49d3cd1a4f8420ed4d2a3965dd67cc50e3848122fa41123bc5a960a302f1`. Actual kind Pod imageID equals the manifest above.
- Upstream core v1.0.3 official asset SHA256 `725fafdabe6aac202a89dc57f1cfe0e2e92f3164c8c2bd343fffca52f7039d96`; linux/amd64 controller `registry.k8s.io/agent-sandbox/agent-sandbox-controller@sha256:b2160ee08dd4f2285b382d4b5073948891adfacbc9808a4ba9cf247766243c8c`.
- Platform ClusterRole source `config/runtime/cluster-role.yaml`, SHA256 `ccc79877cbca2e48da39b51252ee7de1e9157aa17641990acdc1fe6aae843723`. It grants no Secret reads, Pod writes, PVC deletion or namespace deletion; node/Lease get supports positive healthy-writer checks.

All images remain local; no npm publication, registry push or external Release occurred. DSH remains official npm 0.2.0-rc.2 / upstream source `639ed015397290b3745d163aafe02ffee4aa3f84`, with A's checksum-guarded minimal UI settings patch.

## Implementation and boundaries

`createAgentEnvironmentRuntime` supplies dedicated namespace creation, immutable owner/allocation linkage, one external PVC, workload SA/NetworkPolicy, exact template/Pod/Service/EndpointSlice verification, create/discover/inspect/connect/stop/start/delete. Namespace creation is the bounded initial provisioning operation, not a background reconciler. Existing incomplete allocations fail closed; they are not repaired or replaced automatically.

Core alone manages Pods/Services. Stop compares UID/revision, revokes connections, changes desired operatingMode, watches successful termination of the exact writer, checks the same Ready node and a Lease renewed within 40 seconds before/after observation, then verifies suspended generation and absence of writers/ready endpoints. The watch starts from the *collection* RV from the same pre-stop Pod list, not an aged item RV. Start requires positive stopped evidence and original PVC. Delete sends framed UID/RV-preconditioned DeleteOptions and retains PVC, namespace, workload SA and isolation policy.

`Stopping` and `StopUnverified` both deny start/delete. If the best-effort diagnostic annotation races core status, the durable `stopping` gate can remain; it is never interpreted as successful stop or automatically repaired. Node partitions, force deletion and lost observation do not produce a safe-to-reuse claim.

Removed executable product paths: custom Cell/CellSnapshot CRDs, operator/authorizer/StatefulSet/snapshot code, old CLI/Connector, standalone runtime files, legacy gates/publish workflows. Historical design documents are archived; active entrypoints point to the new contract. The shared native launcher/transport remains where it serves actual DSH behavior.

## Actual checks

Environment: dedicated `dsh-mvp-rc2`, Kubernetes 1.37, Calico 3.32.2, local-path RWO, one upstream core controller. Tests invoke the production Connector; the fault proxy forwards real API operations and only disrupts responses/observation. It does not fabricate Kubernetes resources.

| Check | Observed result |
| --- | --- |
| `dev-run go=1.27 -- make verify` | Build, Go race tests, vet, Connector tests and source standards passed |
| `dev-run go=1.27 -- make verify-dsh` | Exact official npm closure, checksum patch and real launcher seam tests passed |
| Connector unit tests | 12 pass: fixed template, identity/endpoint mutation rejection, terminal proof, framed DELETE, stream deadlines/cancellation, list-RV regression |
| Native create/stop/new-factory start | Ready → Stopped → Ready, same Sandbox/PVC UID, new Pod UID, final manifest actually executed |
| Pod rebuild | Original Pod `a1a06fa2-4dc8-445f-9176-bfd428458857` replaced by `bb05c012-913a-49a5-979e-650a72cef8a4`; PVC `26fd134a-fc42-4d40-94d9-c124400d2b98` unchanged; workspace/home/dsh markers and native session survive |
| Native HTTP/WS | Real settings/session RPC and session-follow snapshot; grant abort and explicit stop close established WS; revoked subsequent request rejected |
| Configuration/background process | Native model baseURL survives stop/start; a test background writer stops and does not resume after restart (file remains 80 bytes after a further wait) |
| Missing/replaced PVC | Disposable `b-volume-fault-1` first positively stopped; missing original and replacement UID both reject inspect/start/delete; no replacement writer appears |
| Concurrent operations | Same-revision stop and start each accept exactly one mutation and reject the other with IntentConflict |
| Unknown create/delete | Successful activation/DELETE responses dropped; caller sees unknown and confirms the same original UID by read-first; no replacement allocation |
| Cancellation after accepted write | Cancelled post-start read returns effect=accepted/retry=read-first; original environment subsequently Ready |
| Failed stop observation | Fault-proxy rejection returns StopUnverified; new factory cannot start/delete, including when core has removed the Pod |
| Real aged writer regression | Unchanged writer `b78d2a61-3a21-4a0a-b1b4-fc1611f6bc58`, item RV `13152`, actual Expired/410 (minimum `13639`); final package watches from list RV `13829` and positively stops the same UID. Cache advancement used 128 metadata events on the B observer Pod only |
| Network boundary | Labelled platform ingress succeeds; unlabelled platform-namespace probe cannot reach workload8080; workload cannot reach Kubernetes API |

Most broad checks first used artifact `29530cb`; final image checks and the final full scenario run use the final image above. The list-RV correction is covered by the cursor-specific unit test, real create/stop/start and a separate actual watch-cache-expiry regression recorded in `aged-writer.json`.

## Failures found and corrected

Initial DELETE transport lacked explicit Content-Length: Node did not frame its DELETE body, invalidating preconditions and contaminating the following keep-alive request. Fixed before artifact `29530cb`, with a TLS transport regression and real retained-data deletion.

A final negative test incorrectly demanded DeleteRejected while core was advancing resourceVersion. Correct behavior was IntentConflict with no submission; the fixture now permits those two refusal outcomes and the complete final scenario run passes.

During cleanup, a genuinely aged Pod RV had fallen out of Kubernetes history (actual ERROR/Expired/410). The old implementation correctly blocked unsafe reuse but unnecessarily lost normal stop evidence. `286a68d` starts the one bounded watch from the current list RV while retaining exact Pod UID verification; it adds no retry/recovery path. Existing failed-observation fixtures remain gated with data retained.

## Evidence and remaining work

Durable local evidence directory: `/home/aigs/projects/runtime/dsh-mvp-rc2/evidence/b/`. It contains `pack-list-rv.json`, `final-image.jsonl`, `final-scenarios.jsonl`, `final-access.jsonl`, `resumed-access.jsonl`, `model-config-resumed.jsonl`, `rebuild-final.json`, `volume-identity.json`, `network.json`, `background.json`, `aged-writer.json`, build/check logs and exact tarballs. Reproduction scripts are in `test/environment`; fixture inventory is retained in the local handoff.

No real model call/tool approval, full OIDC/browser flow, installed chart combination, node-partition experiment, data erasure product operation, migration or public release is claimed. E owns the combined platform/install acceptance. Coordinator owns merge order and transfer of the sole cluster write permission; B does not merge.
