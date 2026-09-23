/*
 * Test-only verifier for the upstream agents.x-k8s.io/v1beta1 Sandbox path.
 *
 * Contract: verifyTarget receives JSON objects read from Kubernetes. `binding`
 * is the platform's immutable EnvironmentBinding projection and must contain:
 * { namespace, name, uid, pvcUids: {data, private},
 *   pvcNames: {data, private}, origin }. `origin` is the exact HTTPS origin
 * used by the Connector (scheme + authority only; no path, query, or hash).
 * This module returns the verified Pod IP. It performs no API calls and is
 * intentionally not used by the production Cell connector.
 */

import { isDeepStrictEqual } from "node:util";

const fail = (code, detail) => {
  const error = new Error(`${code}: ${detail}`);
  error.code = code;
  return error;
};

const object = (value, label) => {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw fail("InvalidTarget", `${label} must be an object`);
  return value;
};

const metadata = (resource, label) => object(resource?.metadata, `${label}.metadata`);

const alive = (resource, label) => {
  const meta = metadata(resource, label);
  if (!meta.uid || meta.deletionTimestamp)
    throw fail("StaleTarget", `${label} is missing UID or is terminating`);
  return meta;
};

const owner = (resource, binding, label) => {
  const refs = metadata(resource, label).ownerReferences;
  if (
    !Array.isArray(refs) ||
    !refs.some(
      (ref) =>
        ref?.controller === true &&
        ref.kind === "Sandbox" &&
        ref.apiVersion === "agents.x-k8s.io/v1beta1" &&
        ref.name === binding.name &&
        ref.uid === binding.uid,
    )
  )
    throw fail("OwnershipMismatch", `${label} is not controlled by the bound Sandbox`);
};

const readyCondition = (resource, label) => {
  const meta = metadata(resource, label);
  const generation = meta.generation;
  const conditions = resource?.status?.conditions;
  if (!Number.isSafeInteger(generation) || !Array.isArray(conditions))
    throw fail("NotReady", `${label} has no generation or conditions`);
  const condition = conditions.find((entry) => entry?.type === "Ready");
  if (
    condition?.status !== "True" ||
    condition.observedGeneration !== generation
  )
    throw fail("NotReady", `${label} Ready condition is not current`);
};

const same = (actual, expected, code, detail) => {
  if (!isDeepStrictEqual(actual, expected)) throw fail(code, detail);
};

const defaultTolerations = [
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

const permittedPodRuntimeDefaults = new Map([
  ["serviceAccount", undefined],
  ["nodeName", undefined],
  ["hostname", undefined],
  ["subdomain", undefined],
  ["priority", 0],
  ["preemptionPolicy", "PreemptLowerPriority"],
  ["dnsPolicy", "ClusterFirst"],
  ["restartPolicy", "Always"],
  ["schedulerName", "default-scheduler"],
  ["enableServiceLinks", true],
  ["terminationGracePeriodSeconds", 30],
  ["tolerations", defaultTolerations],
]);

const permittedContainerDefaults = new Map([
  ["imagePullPolicy", undefined],
  ["terminationMessagePath", "/dev/termination-log"],
  ["terminationMessagePolicy", "File"],
]);

const rejectHostEscapes = (value, path = "pod.spec") => {
  if (!value || typeof value !== "object") return;
  if (Array.isArray(value)) {
    value.forEach((entry, index) => rejectHostEscapes(entry, `${path}[${index}]`));
    return;
  }
  for (const [key, entry] of Object.entries(value)) {
    if (
      key === "hostNetwork" ||
      key === "hostPID" ||
      key === "hostIPC" ||
      key === "hostUsers" ||
      key === "hostPort" ||
      key === "hostPath" ||
      key === "ephemeralContainers" ||
      key === "initContainers"
    )
      throw fail("UnsafePodSpec", `${path}.${key} is not permitted`);
    rejectHostEscapes(entry, `${path}.${key}`);
  }
};

const verifySecurityBaseline = (spec) => {
  const podSecurity = spec.securityContext;
  if (
    podSecurity?.runAsNonRoot !== true ||
    podSecurity?.seccompProfile?.type !== "RuntimeDefault"
  )
    throw fail("UnsafePodSpec", "pod securityContext lacks non-root RuntimeDefault seccomp");
  if (spec.automountServiceAccountToken !== false)
    throw fail("UnsafePodSpec", "service account token automount must be false");
  const containers = spec.containers;
  if (!Array.isArray(containers) || containers.length !== 1)
    throw fail("UnsafePodSpec", "Sandbox must have exactly one container");
  const security = containers[0]?.securityContext;
  if (
    security?.runAsNonRoot !== true ||
    security.allowPrivilegeEscalation !== false ||
    security.readOnlyRootFilesystem !== true ||
    !Array.isArray(security.capabilities?.drop) ||
    !security.capabilities.drop.includes("ALL")
  )
    throw fail("UnsafePodSpec", "container securityContext is weaker than the frozen template");
};

const verifyPodSpec = (pod, expectedPodSpec) => {
  const actual = object(pod?.spec, "pod.spec");
  const expected = object(expectedPodSpec, "expectedPodSpec");
  rejectHostEscapes(actual);
  rejectHostEscapes(expected, "expectedPodSpec");
  verifySecurityBaseline(actual);
  verifySecurityBaseline(expected);

  const normalizedActual = structuredClone(actual);
  const normalizedExpected = structuredClone(expected);
  for (const [key, allowed] of permittedPodRuntimeDefaults) {
    if (key in normalizedExpected || !(key in normalizedActual)) continue;
    const valid =
      key === "serviceAccount"
        ? normalizedActual[key] === normalizedActual.serviceAccountName
        : allowed === undefined || isDeepStrictEqual(normalizedActual[key], allowed);
    if (valid) delete normalizedActual[key];
  }
  for (const key of ["containers"]) {
    if (Array.isArray(normalizedActual[key])) {
      normalizedActual[key] = normalizedActual[key].map((entry) =>
        structuredClone(entry),
      );
    }
    if (Array.isArray(normalizedExpected[key])) {
      normalizedExpected[key] = normalizedExpected[key].map((entry, index) => {
        const actualEntry = normalizedActual[key]?.[index] ?? {};
        const expectedEntry = structuredClone(entry);
        for (const [field, allowed] of permittedContainerDefaults) {
          if (field in expectedEntry || !(field in actualEntry)) continue;
          if (allowed === undefined || isDeepStrictEqual(actualEntry[field], allowed))
            delete normalizedActual[key][index][field];
        }
        return expectedEntry;
      });
    }
  }
  same(normalizedActual, normalizedExpected, "TemplateMismatch", "live Pod spec differs from frozen Sandbox template");
};

const asPvcMap = (pvcs) => {
  if (Array.isArray(pvcs)) {
    return new Map(
      pvcs.map((pvc) => [pvc?.metadata?.name, pvc]),
    );
  }
  const value = object(pvcs, "pvcs");
  return new Map([
    ["data", value.data],
    ["private", value.private],
  ]);
};

const verifyPvcs = (pvcs, binding, sandbox) => {
  const map = asPvcMap(pvcs);
  for (const role of ["data", "private"]) {
    const expectedName = binding.pvcNames?.[role];
    const expectedUid = binding.pvcUids?.[role];
    if (!expectedName || !expectedUid)
      throw fail("InvalidBinding", `missing ${role} PVC binding`);
    const pvc = map.get(expectedName) ?? map.get(role);
    const meta = alive(pvc, `${role} PVC`);
    if (
      meta.namespace !== binding.namespace ||
      meta.name !== expectedName ||
      meta.uid !== expectedUid
    )
      throw fail("StorageMismatch", `${role} PVC name, namespace, or UID differs from binding`);
    const labels = meta.labels ?? {};
    if (Object.keys(meta.ownerReferences ?? {}).length > 0 || labels["agents.dsh.io/environment-uid"] !== binding.uid)
      throw fail("StorageMismatch", `${role} PVC must be externally deleted and carry the trusted environment label`);
    if (pvc.status?.phase !== "Bound" || typeof pvc.spec?.volumeName !== "string" || !pvc.spec.volumeName)
      throw fail("StorageMismatch", `${role} PVC is not bound to a volume`);
  }
  if (sandbox.metadata.namespace !== binding.namespace)
    throw fail("OwnershipMismatch", "Sandbox and PVC namespace differ from binding");
};

const endpointItems = (endpoints) => {
  if (Array.isArray(endpoints)) return endpoints;
  if (Array.isArray(endpoints?.items)) return endpoints.items;
  return [endpoints];
};

const verifyServiceAndEndpoints = (service, endpoints, pod, sandbox, binding, podIP) => {
  const serviceMeta = alive(service, "Service");
  if (serviceMeta.namespace !== binding.namespace) throw fail("OwnershipMismatch", "Service namespace differs from binding");
  owner(service, binding, "Service");
  if (service.spec?.clusterIP !== "None" || service.spec?.type !== "ClusterIP")
    throw fail("EndpointMismatch", "Service must be the Sandbox headless ClusterIP service");
  const ports = service.spec?.ports;
  if (
    !Array.isArray(ports) ||
    ports.length !== 2 ||
    !ports.some((port) => port.name === "http" && port.port === 8080 && (port.targetPort === 8080 || port.targetPort === "http")) ||
    !ports.some((port) => port.name === "management" && port.port === 8081 && (port.targetPort === 8081 || port.targetPort === "management"))
  )
    throw fail("EndpointMismatch", "Service ports must be exactly http:8080 and management:8081");
  const selector = service.spec?.selector;
  for (const [key, value] of Object.entries(selector ?? {})) {
    if (pod.metadata?.labels?.[key] !== value)
      throw fail("EndpointMismatch", "Service selector does not select the verified Pod");
  }
  if (!selector || Object.keys(selector).length === 0)
    throw fail("EndpointMismatch", "Service has no selector");

  const resources = endpointItems(endpoints);
  for (const resource of resources) {
    const endpointMeta = metadata(resource, "Endpoints");
    const belongs =
      endpointMeta.labels?.["kubernetes.io/service-name"] === serviceMeta.name ||
      endpointMeta.name === serviceMeta.name ||
      endpointMeta.ownerReferences?.some((ref) => ref?.uid === serviceMeta.uid && ref.kind === "Service");
    if (!belongs) throw fail("EndpointMismatch", "Endpoint resource does not belong to the verified Service");
  }
  const candidates = resources.flatMap((resource) => {
    if (Array.isArray(resource?.endpoints)) return resource.endpoints;
    return (resource?.subsets ?? []).flatMap((subset) =>
      (subset.addresses ?? []).map((address) => ({
        addresses: [address.ip],
        targetRef: address.targetRef,
        conditions: { ready: true },
      })),
    );
  });
  const ready = candidates.filter(
    (entry) => entry?.conditions?.ready === true && entry.conditions?.terminating !== true,
  );
  const matching = ready.find(
    (entry) =>
      entry.targetRef?.kind === "Pod" &&
      entry.targetRef?.name === pod.metadata.name &&
      entry.targetRef?.namespace === binding.namespace &&
      entry.targetRef?.uid === pod.metadata.uid &&
      Array.isArray(entry.addresses) &&
      entry.addresses.includes(podIP),
  );
  if (!matching || ready.length !== 1)
    throw fail("EndpointMismatch", "Endpoints must contain exactly one ready target for the verified Pod IP and UID");
  if (sandbox.metadata.namespace !== serviceMeta.namespace)
    throw fail("OwnershipMismatch", "Service and Sandbox namespaces differ");
};

export function verifyTarget({ sandbox, pod, service, endpoints, pvcs, binding, expectedPodSpec }) {
  object(binding, "binding");
  for (const key of ["namespace", "name", "uid", "origin"]) {
    if (typeof binding[key] !== "string" || !binding[key])
      throw fail("InvalidBinding", `binding.${key} is required`);
  }
  let origin;
  try {
    origin = new URL(binding.origin);
  } catch {
    throw fail("InvalidBinding", "binding.origin is not a URL");
  }
  if (origin.protocol !== "https:" || origin.origin !== binding.origin || origin.pathname !== "/" || origin.search || origin.hash)
    throw fail("InvalidBinding", "binding.origin must be an exact HTTPS origin");

  const sandboxMeta = alive(sandbox, "Sandbox");
  if (
    sandboxMeta.namespace !== binding.namespace ||
    sandboxMeta.name !== binding.name ||
    sandboxMeta.uid !== binding.uid
  )
    throw fail("StaleTarget", "Sandbox identity differs from binding");
  readyCondition(sandbox, "Sandbox");
  if (sandbox.spec?.operatingMode && sandbox.spec.operatingMode !== "Running")
    throw fail("NotReady", "Sandbox is not in Running mode");
  if (sandbox.spec?.service !== true)
    throw fail("EndpointMismatch", "Sandbox service must be explicitly enabled");

  const podMeta = alive(pod, "Pod");
  if (podMeta.namespace !== binding.namespace) throw fail("OwnershipMismatch", "Pod namespace differs from binding");
  owner(pod, binding, "Pod");
  if (pod.status?.phase !== "Running") throw fail("NotReady", "Pod is not Running");
  if (!pod.status?.podIP) throw fail("NotReady", "Pod has no podIP");
  if (pod.status?.conditions?.find((entry) => entry?.type === "Ready")?.status !== "True")
    throw fail("NotReady", "Pod Ready condition is not True");
  verifyPodSpec(pod, expectedPodSpec);
  verifyPvcs(pvcs, binding, sandbox);
  verifyServiceAndEndpoints(service, endpoints, pod, sandbox, binding, pod.status.podIP);
  return pod.status.podIP;
}
