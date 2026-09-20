import { isIP } from "node:net";
import { isDeepStrictEqual } from "node:util";
import { KubernetesReader, type KubernetesOptions } from "./kubernetes.js";
import {
  RuntimeAccessError,
  type AccessContext,
  type InstanceRef,
  type RuntimeAccess,
} from "./port.js";
import { connector } from "./proxy.js";

interface Meta {
  name?: string;
  namespace?: string;
  uid?: string;
  generation?: number;
  deletionTimestamp?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  ownerReferences?: Array<{
    apiVersion?: string;
    kind?: string;
    name?: string;
    uid?: string;
    controller?: boolean;
  }>;
}
interface Container {
  image?: string;
  name?: string;
  command?: string[];
  args?: string[];
  ports?: Array<{ name?: string; containerPort?: number; protocol?: string }>;
  env?: Array<{ name?: string; value?: string }>;
}
interface Condition {
  type?: string;
  status?: string;
  observedGeneration?: number;
}
interface Resource {
  metadata?: Meta;
  spec?: {
    [key: string]: unknown;
    image?: string;
    template?: { spec?: { containers?: Container[] } };
    containers?: Container[];
    selector?: Record<string, string>;
    type?: string;
    ports?: Array<{
      port?: number;
      targetPort?: number | string;
      protocol?: string;
    }>;
  };
  status?: {
    conditions?: Condition[];
    dshVersion?: string;
    imageDigest?: string;
    observedGeneration?: number;
    podIP?: string;
  };
}
interface EndpointSlice extends Resource {
  ports?: Array<{ port?: number; protocol?: string }>;
  endpoints?: Array<{
    conditions?: { ready?: boolean; terminating?: boolean };
    targetRef?: {
      kind?: string;
      uid?: string;
      name?: string;
      namespace?: string;
    };
    addresses?: string[];
  }>;
}
interface List {
  items?: EndpointSlice[];
}
export interface CellBinding {
  readonly ref: InstanceRef;
  readonly namespace: string;
  readonly name: string;
  readonly origin: string;
  readonly template: string;
  /** Exact defaulted Cell spec from the administrator's pinned fixture. No browser input. */
  readonly expectedSpec: Readonly<Record<string, unknown>>;
  readonly expectedPodSpec: Readonly<Record<string, unknown>>;
}
const uidPattern =
  /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/;
const namePattern = /^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$/;
const owner = (
  r: Resource,
  kind: string,
  version: string,
  name: string,
  uid: string,
) =>
  r.metadata?.ownerReferences?.some(
    (o) =>
      o.controller === true &&
      o.kind === kind &&
      o.apiVersion === version &&
      o.name === name &&
      o.uid === uid,
  );
const alive = (r: Resource) =>
  !!r.metadata?.uid && !r.metadata.deletionTimestamp;
const ready = (r: Resource) =>
  r.status?.conditions?.some(
    (c: { type?: string; status?: string; observedGeneration?: number }) =>
      c.type === "Ready" &&
      c.status === "True" &&
      c.observedGeneration === r.metadata?.generation,
  );

export function createCellRuntime(
  options: KubernetesOptions,
  input: readonly CellBinding[] | (() => readonly CellBinding[]),
): RuntimeAccess {
  const reader = new KubernetesReader(options);
  function loadBindings() {
    const bindings = new Map<string, CellBinding>();
    for (const raw of typeof input === "function" ? input() : input) {
      const b: CellBinding = structuredClone(raw);
      const url = new URL(b.origin);
      if (
        !b.ref.allocationKey ||
        !uidPattern.test(b.ref.identity) ||
        !namePattern.test(b.namespace) ||
        !namePattern.test(b.name) ||
        url.protocol !== "https:" ||
        url.origin !== b.origin ||
        !url.hostname.startsWith(`cell-${b.ref.identity}.`) ||
        !b.template ||
        !/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(String(b.expectedSpec.image)) ||
        !b.expectedPodSpec ||
        bindings.has(b.ref.allocationKey)
      )
        throw new Error("Invalid or duplicate prebuilt Cell binding");
      bindings.set(b.ref.allocationKey, b);
    }
    return bindings;
  }
  loadBindings();
  async function verify(ref: InstanceRef, context: AccessContext) {
    ref = { ...ref };
    const fail = (code: ConstructorParameters<typeof RuntimeAccessError>[0]) =>
      new RuntimeAccessError(
        code,
        context.correlationId,
        "Check the original prebuilt binding, template and Ready conditions; do not adopt another instance",
        { allocationKey: ref.allocationKey },
      );
    context.signal.throwIfAborted();
    const b = loadBindings().get(ref.allocationKey);
    if (!b || b.ref.identity !== ref.identity) throw fail("StaleInstance");
    const signal = AbortSignal.any([context.signal, AbortSignal.timeout(5000)]);
    const get = (path: string) =>
      reader.get<Resource>(path, signal, context.correlationId);
    const ns = encodeURIComponent(b.namespace),
      base = `cell-${b.ref.identity}`;
    const core = `/api/v1/namespaces/${ns}`;
    const cell = await get(
      `/apis/dsh.isolated.io/v1alpha1/namespaces/${ns}/cells/${encodeURIComponent(b.name)}`,
    );
    if (!alive(cell) || cell.metadata?.uid !== ref.identity)
      throw fail("StaleInstance");
    if (
      !isDeepStrictEqual(cell.spec, b.expectedSpec) ||
      cell.status?.dshVersion !== "0.1.5-rc.2" ||
      cell.status?.imageDigest !== String(b.expectedSpec.image).split("@")[1]
    )
      throw fail("TemplateMismatch");
    if (
      !ready(cell) ||
      cell.status?.observedGeneration !== cell.metadata?.generation
    )
      throw fail("NotReady");
    const [workload, service, pod] = await Promise.all([
      get(`/apis/apps/v1/namespaces/${ns}/statefulsets/${base}`),
      get(`${core}/services/${base}`),
      get(`${core}/pods/${base}-0`),
    ]);
    for (const resource of [workload, service]) {
      if (
        !alive(resource) ||
        !owner(
          resource,
          "Cell",
          "dsh.isolated.io/v1alpha1",
          b.name,
          ref.identity,
        ) ||
        resource.metadata?.annotations?.["dsh.isolated.io/cell-uid"] !==
          ref.identity ||
        resource.metadata.annotations["dsh.isolated.io/cell-name"] !== b.name
      )
        throw fail("StaleInstance");
    }
    if (
      workload.metadata?.annotations?.["dsh.isolated.io/access-mode"] !==
      "platform"
    )
      throw fail("AccessRejected");
    if (
      !isDeepStrictEqual(workload.spec?.template?.spec, b.expectedPodSpec) ||
      Object.entries(b.expectedPodSpec).some(
        ([key, value]) => !isDeepStrictEqual(pod.spec?.[key], value),
      )
    )
      throw fail("TemplateMismatch");
    const containers = workload.spec?.template?.spec?.containers;
    const podContainers = pod.spec?.containers;
    if (
      !Array.isArray(containers) ||
      containers.length !== 1 ||
      !Array.isArray(podContainers) ||
      podContainers.length !== 1 ||
      containers[0]?.image !== b.expectedSpec.image ||
      podContainers[0]?.image !== b.expectedSpec.image ||
      !containers[0]?.env?.some(
        (e: { name?: string; value?: string }) =>
          e.name === "CELL_AUTHORITY" && e.value === new URL(b.origin).host,
      ) ||
      !isDeepStrictEqual(podContainers[0]?.env, containers[0]?.env) ||
      !isDeepStrictEqual(podContainers[0]?.command, containers[0]?.command) ||
      !isDeepStrictEqual(podContainers[0]?.args, containers[0]?.args)
    )
      throw fail("TemplateMismatch");
    if (
      !podContainers[0]?.ports?.some(
        (p) =>
          p.name === "http" && p.containerPort === 8080 && p.protocol === "TCP",
      )
    )
      throw fail("TemplateMismatch");
    if (
      !alive(pod) ||
      !owner(pod, "StatefulSet", "apps/v1", base, workload.metadata!.uid!) ||
      pod.metadata?.annotations?.["dsh.isolated.io/cell-uid"] !==
        ref.identity ||
      !pod.status?.conditions?.some(
        (c: { type?: string; status?: string }) =>
          c.type === "Ready" && c.status === "True",
      )
    )
      throw fail("NotReady");
    const selector = service.spec?.selector;
    if (
      !selector ||
      selector["dsh.isolated.io/cell-uid"] !== ref.identity ||
      Object.entries(selector).some(
        ([key, value]) => pod.metadata?.labels?.[key] !== value,
      ) ||
      service.spec?.type !== "ClusterIP" ||
      !service.spec?.ports?.some(
        (p) =>
          p.port === 80 &&
          (p.targetPort === 8080 || p.targetPort === "http") &&
          p.protocol === "TCP",
      )
    )
      throw fail("StaleInstance");
    const slices = await reader.get<List>(
      `/apis/discovery.k8s.io/v1/namespaces/${ns}/endpointslices?labelSelector=${encodeURIComponent(`kubernetes.io/service-name=${base}`)}`,
      signal,
      context.correlationId,
    );
    const address = pod.status?.podIP;
    if (
      typeof address !== "string" ||
      !isIP(address) ||
      !slices.items?.some(
        (slice: EndpointSlice) =>
          alive(slice) &&
          owner(slice, "Service", "v1", base, service.metadata!.uid!) &&
          slice.ports?.some((p) => p.port === 8080 && p.protocol === "TCP") &&
          slice.endpoints?.some(
            (e) =>
              e.conditions?.ready === true &&
              e.conditions?.terminating !== true &&
              e.targetRef?.kind === "Pod" &&
              e.targetRef?.uid === pod.metadata?.uid &&
              e.targetRef?.name === `${base}-0` &&
              e.targetRef?.namespace === b.namespace &&
              e.addresses?.includes(address),
          ),
      )
    )
      throw fail("NotReady");
    context.signal.throwIfAborted();
    return {
      view: {
        ref: { ...b.ref },
        origin: b.origin,
        template: b.template,
        state: "Ready" as const,
      },
      address,
    };
  }
  return {
    async inspect(ref, context) {
      return (await verify(ref, context)).view;
    },
    async connect(ref, context) {
      ref = { ...ref };
      const { view } = await verify(ref, context);
      return connector(
        view.origin,
        context,
        async () => (await verify(ref, context)).address,
      );
    },
  };
}
