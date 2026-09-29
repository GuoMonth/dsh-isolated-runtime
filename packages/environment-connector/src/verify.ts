import { isDeepStrictEqual as same } from "node:util";
import { isIP } from "node:net";
import { environmentTemplate } from "./template.js";
import {
  annotation,
  coordinates,
  group,
  networkPolicy,
  ownership,
  serviceAccount,
} from "./resources.js";
import type { Resource } from "./api.js";
import type { EnvironmentRef, EnvironmentRuntimeOptions } from "./index.js";
export class Mismatch extends Error {
  constructor(
    readonly code:
      | "OwnerMismatch"
      | "StaleInstance"
      | "StorageMismatch"
      | "TemplateMismatch",
  ) {
    super(code);
  }
}
const fail = (code: Mismatch["code"]): never => {
  throw new Mismatch(code);
};
export function own(
  resource: Resource,
  ref: Pick<EnvironmentRef, "allocationKey" | "owner">,
) {
  for (const [key, value] of Object.entries(ownership(ref)))
    if (resource.metadata.annotations?.[key] !== value) fail("OwnerMismatch");
}
export function controlled(
  resource: Resource,
  kind: string,
  name: string,
  uid: string,
  apiVersion: string,
): boolean {
  const refs = resource.metadata.ownerReferences ?? [];
  return (
    refs.length === 1 &&
    refs[0]?.controller === true &&
    refs[0].kind === kind &&
    refs[0].name === name &&
    refs[0].uid === uid &&
    refs[0].apiVersion === apiVersion
  );
}
// Compare Kubernetes quantities by value, not the API server's canonical spelling.
export function quantity(value: string): number {
  const m =
    /^([+-]?(?:\d+(?:\.\d*)?|\.\d+))([eE][+-]?\d+|[numkKMGTPE]|[KMGTPE]i)?$/.exec(
      value,
    );
  if (!m) return NaN;
  const units: Record<string, number> = {
    n: 1e-9,
    u: 1e-6,
    m: 1e-3,
    k: 1e3,
    K: 1e3,
    M: 1e6,
    G: 1e9,
    T: 1e12,
    P: 1e15,
    E: 1e18,
    Ki: 1024,
    Mi: 1024 ** 2,
    Gi: 1024 ** 3,
    Ti: 1024 ** 4,
    Pi: 1024 ** 5,
    Ei: 1024 ** 6,
  };
  const suffix = m[2] ?? "";
  return (
    Number(m[1]) *
    (suffix === "" ? 1 : (units[suffix] ?? 10 ** Number(suffix.slice(1))))
  );
}
function normalized(spec: any, live: boolean) {
  const result = structuredClone(spec);
  const defaults: any = {
    dnsPolicy: "ClusterFirst",
    restartPolicy: "Always",
    schedulerName: "default-scheduler",
    priority: 0,
    preemptionPolicy: "PreemptLowerPriority",
  };
  for (const [key, value] of Object.entries(defaults))
    if (same(result[key], value)) delete result[key];
  if (live) {
    if (typeof result.nodeName === "string") delete result.nodeName;
    if (result.serviceAccount === result.serviceAccountName)
      delete result.serviceAccount;
    const tolerations = [
      {
        key: "node.kubernetes.io/not-ready",
        operator: "Exists",
        effect: "NoExecute",
        tolerationSeconds: 300,
      },
      {
        key: "node.kubernetes.io/unreachable",
        operator: "Exists",
        effect: "NoExecute",
        tolerationSeconds: 300,
      },
    ];
    if (same(result.tolerations, tolerations)) delete result.tolerations;
  }
  for (const c of result.containers ?? []) {
    if (c.terminationMessagePath === "/dev/termination-log")
      delete c.terminationMessagePath;
    if (c.terminationMessagePolicy === "File")
      delete c.terminationMessagePolicy;
    for (const port of c.ports ?? [])
      if (port.protocol === "TCP") delete port.protocol;
    for (const key of ["startupProbe", "readinessProbe", "livenessProbe"])
      if (c[key]) {
        const p = c[key];
        p.timeoutSeconds ??= 1;
        p.successThreshold ??= 1;
        p.failureThreshold ??= 3;
        p.httpGet.scheme ??= "HTTP";
      }
    for (const resource of Object.values(c.resources ?? {}) as Record<
      string,
      string
    >[])
      for (const key of ["cpu", "memory"])
        if (resource[key]) resource[key] = String(quantity(resource[key]));
  }
  return result;
}
export function templateFor(
  ref: EnvironmentRef,
  options: EnvironmentRuntimeOptions,
) {
  return environmentTemplate({
    image: options.image,
    authority: new URL(coordinates(ref, options).origin).host,
    dataClaim: ref.data.name,
    serviceAccount,
    resources: options.resources,
  });
}
export function verifyBase(
  ref: EnvironmentRef,
  options: EnvironmentRuntimeOptions,
  namespace: Resource,
  sandbox: Resource,
  pvc: Resource,
) {
  own(namespace, ref);
  own(sandbox, ref);
  own(pvc, ref);
  if (
    namespace.metadata.deletionTimestamp ||
    namespace.metadata.name !== ref.namespace ||
    namespace.metadata.annotations?.[annotation + "sandbox-uid"] !==
      ref.sandbox.uid
  )
    fail("StaleInstance");
  if (
    sandbox.metadata.uid !== ref.sandbox.uid ||
    sandbox.metadata.name !== ref.sandbox.name ||
    sandbox.metadata.namespace !== ref.namespace
  )
    fail("StaleInstance");
  if (
    pvc.metadata.uid !== ref.data.uid ||
    pvc.metadata.name !== ref.data.name ||
    pvc.metadata.namespace !== ref.namespace ||
    pvc.metadata.deletionTimestamp ||
    pvc.metadata.annotations?.[annotation + "sandbox-uid"] !==
      ref.sandbox.uid ||
    (pvc.metadata.ownerReferences ?? []).length ||
    sandbox.metadata.annotations?.[annotation + "data-uid"] !== ref.data.uid
  )
    fail("StorageMismatch");
  if (
    pvc.spec?.storageClassName !== options.storage.storageClassName ||
    !same(pvc.spec?.accessModes, ["ReadWriteOnce"]) ||
    quantity(pvc.spec?.resources?.requests?.storage) !==
      quantity(options.storage.size)
  )
    fail("StorageMismatch");
  const expected = templateFor(ref, options);
  if (
    sandbox.apiVersion !== group ||
    sandbox.kind !== "Sandbox" ||
    Object.keys(sandbox.spec ?? {}).some(
      (k) =>
        !["operatingMode", "podTemplate", "service", "shutdownPolicy"].includes(
          k,
        ),
    ) ||
    sandbox.spec.service !== true ||
    sandbox.spec.shutdownPolicy !== "Retain" ||
    !["Running", "Suspended"].includes(sandbox.spec.operatingMode) ||
    !same(
      normalized(sandbox.spec.podTemplate?.spec, false),
      normalized(expected.spec, false),
    ) ||
    !same(sandbox.spec.podTemplate?.metadata, expected.metadata)
  )
    fail("TemplateMismatch");
}
export function verifyAncillary(
  ref: EnvironmentRef,
  options: EnvironmentRuntimeOptions,
  sa: Resource,
  policy: Resource,
) {
  own(sa, ref);
  own(policy, ref);
  if (
    sa.metadata.deletionTimestamp ||
    policy.metadata.deletionTimestamp ||
    sa.automountServiceAccountToken !== false ||
    sa.metadata.annotations?.[annotation + "sandbox-uid"] !== ref.sandbox.uid ||
    policy.metadata.annotations?.[annotation + "sandbox-uid"] !==
      ref.sandbox.uid ||
    !same(
      policy.spec,
      networkPolicy(ref.namespace, options.platformNamespace).spec,
    )
  )
    fail("TemplateMismatch");
}
export function verifyPod(
  pod: Resource,
  ref: EnvironmentRef,
  options: EnvironmentRuntimeOptions,
) {
  if (
    !controlled(pod, "Sandbox", ref.sandbox.name, ref.sandbox.uid, group) ||
    pod.metadata.namespace !== ref.namespace ||
    !same(
      normalized(pod.spec, true),
      normalized(templateFor(ref, options).spec, false),
    )
  )
    fail("TemplateMismatch");
}
export function readyTarget(
  ref: EnvironmentRef,
  options: EnvironmentRuntimeOptions,
  sandbox: Resource,
  pvc: Resource,
  pods: Resource[],
  service: Resource | null,
  slices: Resource[],
): string | null {
  for (const pod of pods) verifyPod(pod, ref, options);
  if (pods.length > 1) fail("TemplateMismatch");
  const pod = pods[0];
  if (
    !pod ||
    pod.metadata.deletionTimestamp ||
    pvc.status?.phase !== "Bound" ||
    !pvc.spec.volumeName
  )
    return null;
  const ready = (r: Resource, current = false) =>
    r.status?.conditions?.some(
      (c: any) =>
        c.type === "Ready" &&
        c.status === "True" &&
        (!current || c.observedGeneration === r.metadata.generation),
    );
  if (!ready(sandbox, true) || !ready(pod) || !isIP(pod.status?.podIP ?? ""))
    return null;
  if (!service) return null;
  if (
    service.metadata.deletionTimestamp ||
    !controlled(service, "Sandbox", ref.sandbox.name, ref.sandbox.uid, group) ||
    service.spec?.type !== "ClusterIP" ||
    service.spec?.clusterIP !== "None" ||
    service.spec?.publishNotReadyAddresses === true ||
    !same(service.spec?.selector, {
      "agents.x-k8s.io/sandbox-name-hash":
        pod.metadata.labels?.["agents.x-k8s.io/sandbox-name-hash"],
    }) ||
    !same(service.spec?.ports, [
      { name: "http", port: 8080, protocol: "TCP", targetPort: 8080 },
      { name: "management", port: 8081, protocol: "TCP", targetPort: 8081 },
    ])
  )
    fail("TemplateMismatch");
  const endpoints: any[] = [];
  for (const slice of slices) {
    if (
      !controlled(
        slice,
        "Service",
        service.metadata.name,
        service.metadata.uid,
        "v1",
      ) ||
      !same(slice.ports, [
        { name: "http", port: 8080, protocol: "TCP" },
        { name: "management", port: 8081, protocol: "TCP" },
      ])
    )
      fail("TemplateMismatch");
    endpoints.push(...(slice.endpoints ?? []));
  }
  if (!endpoints.length) return null;
  if (endpoints.length !== 1) fail("TemplateMismatch");
  const e = endpoints[0];
  if (
    e.targetRef?.uid !== pod.metadata.uid ||
    e.targetRef?.name !== pod.metadata.name ||
    e.targetRef?.namespace !== ref.namespace ||
    e.targetRef?.kind !== "Pod" ||
    !same(e.addresses, [pod.status.podIP])
  )
    fail("TemplateMismatch");
  return e.conditions?.ready === true && e.conditions?.terminating !== true
    ? pod.status.podIP
    : null;
}
