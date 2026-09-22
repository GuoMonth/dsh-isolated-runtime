import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import {
  KubernetesReader,
  createResource,
  deleteResource,
  type KubernetesOptions,
} from "./kubernetes.js";
import { createCellRuntime, type CellBinding } from "./cell.js";
import {
  bindCellTemplate,
  CELL_DSH_VERSION,
  CELL_TEMPLATE_VERSION,
  type CellTemplateInputs,
} from "./cell-template.js";
import {
  RuntimeAccessError,
  type AllocationIntent,
  type AllocationRuntime,
  type AccessContext,
  type InstanceView,
} from "./port.js";
export interface CellAllocationOptions {
  readonly template: typeof CELL_TEMPLATE_VERSION;
  readonly image: string;
  readonly storage: {
    readonly size: string;
    readonly storageClassName?: string;
    readonly retentionPolicy?: "Retain" | "Delete";
  };
  readonly resources: CellTemplateInputs["resources"];
  readonly credentialsSecret?: string;
  readonly namespaces: Readonly<Record<string, string>>;
  readonly domain: string;
}
const canonical = (value: unknown): string =>
  JSON.stringify(value, (_key, item: unknown) =>
    item && typeof item === "object" && !Array.isArray(item)
      ? Object.fromEntries(
          Object.entries(item).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)),
        )
      : item,
  );
const hash = (value: string) =>
  createHash("sha256").update(value).digest("hex");
const uidPattern =
  /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/;
const dnsLabel = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const dnsSubdomain = (value: string) =>
  typeof value === "string" &&
  value.length <= 253 &&
  value.split(".").every((label) => label.length <= 63 && dnsLabel.test(label));
const exactKeys = (
  value: unknown,
  required: readonly string[],
  optional: readonly string[] = [],
) => {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const keys = Object.keys(value);
  return (
    required.every((key) => Object.hasOwn(value, key)) &&
    keys.every((key) => required.includes(key) || optional.includes(key))
  );
};
const maxQuantity = 9_223_372_036_854_775_807n;
const positiveInt = (value: string) => /^[1-9][0-9]*$/.test(value);
const cpuMilli = (value: string): bigint | undefined => {
  if (typeof value !== "string") return undefined;
  if (positiveInt(value)) {
    const milli = BigInt(value) * 1000n;
    return milli <= maxQuantity ? milli : undefined;
  }
  const milli = /^([1-9][0-9]{0,2})m$/.exec(value);
  if (!milli) return undefined;
  const amount = BigInt(milli[1]!);
  return amount < 1000n ? amount : undefined;
};
const binaryBytes = (value: string): bigint | undefined => {
  if (typeof value !== "string") return undefined;
  const match = /^([1-9][0-9]*)(Ki|Mi|Gi|Ti)$/.exec(value);
  if (!match) return undefined;
  const amount = BigInt(match[1]!);
  if (amount % 1024n === 0n) return undefined;
  const power = { Ki: 1n, Mi: 2n, Gi: 3n, Ti: 4n }[match[2] as "Ki" | "Mi" | "Gi" | "Ti"];
  const bytes = amount * 1024n ** power;
  return bytes <= maxQuantity ? bytes : undefined;
};
function validateConfiguration(options: CellAllocationOptions) {
  if (
    !exactKeys(
      options,
      ["template", "image", "storage", "resources", "namespaces", "domain"],
      ["credentialsSecret"],
    )
  )
    throw new Error(
      "Invalid allocation fields; use template, image, storage, resources, credentialsSecret, namespaces and domain",
    );
  if (options.template !== CELL_TEMPLATE_VERSION)
    throw new Error("allocation.template: unsupported fixed Cell template version");
  if (
    typeof options.image !== "string" ||
    !/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(options.image)
  )
    throw new Error("allocation.image: expected a sha256-pinned image reference");
  if (
    typeof options.domain !== "string" ||
    !options.domain.includes(".") ||
    !dnsSubdomain(options.domain)
  )
    throw new Error("allocation.domain: expected a lowercase DNS domain without a port");
  if (!exactKeys(options.storage, ["size"], ["storageClassName", "retentionPolicy"]))
    throw new Error("allocation.storage: invalid or unknown field");
  if (!binaryBytes(options.storage.size))
    throw new Error("allocation.storage.size: expected a positive canonical binary quantity");
  if (
    options.storage.storageClassName !== undefined &&
    !dnsSubdomain(options.storage.storageClassName)
  )
    throw new Error("allocation.storage.storageClassName: invalid DNS name");
  if (
    options.storage.retentionPolicy !== undefined &&
    options.storage.retentionPolicy !== "Retain" &&
    options.storage.retentionPolicy !== "Delete"
  )
    throw new Error("allocation.storage.retentionPolicy: expected Retain or Delete");
  if (
    !exactKeys(options.resources, ["requests", "limits"]) ||
    !exactKeys(options.resources.requests, ["cpu", "memory"]) ||
    !exactKeys(options.resources.limits, ["cpu", "memory"])
  )
    throw new Error("allocation.resources: requests and limits must each contain only cpu and memory");
  const requestCPU = cpuMilli(options.resources.requests.cpu);
  const limitCPU = cpuMilli(options.resources.limits.cpu);
  const requestMemory = binaryBytes(options.resources.requests.memory);
  const limitMemory = binaryBytes(options.resources.limits.memory);
  if (
    requestCPU === undefined ||
    limitCPU === undefined ||
    requestMemory === undefined ||
    limitMemory === undefined ||
    requestCPU > limitCPU ||
    requestMemory > limitMemory
  )
    throw new Error(
      "allocation.resources: quantities must be positive, canonical and requests must not exceed limits",
    );
  if (
    options.credentialsSecret !== undefined &&
    !dnsSubdomain(options.credentialsSecret)
  )
    throw new Error("allocation.credentialsSecret: invalid Secret DNS name");
  if (
    !options.namespaces ||
    typeof options.namespaces !== "object" ||
    Array.isArray(options.namespaces) ||
    !Object.keys(options.namespaces).length
  )
    throw new Error("allocation.namespaces: expected a non-empty tenant-to-namespace map");
  const tenantIds = Object.keys(options.namespaces);
  const namespaces = Object.values(options.namespaces);
  if (
    tenantIds.some((id) => !id.trim() || id.length > 128 || /[\u0000-\u001f]/.test(id)) ||
    new Set(namespaces).size !== namespaces.length ||
    namespaces.some(
      (namespace) => typeof namespace !== "string" || !dnsLabel.test(namespace),
    )
  )
    throw new Error("allocation.namespaces: tenant IDs or namespace DNS labels are invalid or duplicated");
}
interface Cell {
  metadata?: {
    uid?: string;
    resourceVersion?: string;
    name?: string;
    namespace?: string;
    generation?: number;
    deletionTimestamp?: string;
  };
  spec?: Record<string, unknown>;
  status?: {
    observedGeneration?: number;
    dshVersion?: string;
    imageDigest?: string;
    conditions?: Array<{
      type?: string;
      status?: string;
      observedGeneration?: number;
      reason?: string;
    }>;
  };
}
export function createCellAllocationRuntime(
  kubernetes: KubernetesOptions,
  configuration: CellAllocationOptions,
): AllocationRuntime {
  const options = structuredClone(configuration);
  validateConfiguration(options);
  const templateInputs: CellTemplateInputs = {
    image: options.image,
    storage: {
      size: options.storage.size,
      ...(options.storage.storageClassName
        ? { storageClassName: options.storage.storageClassName }
        : {}),
      retentionPolicy: options.storage.retentionPolicy ?? "Retain",
    },
    resources: options.resources,
    ...(options.credentialsSecret
      ? { credentialsSecret: options.credentialsSecret }
      : {}),
  };
  const boundTemplate = bindCellTemplate(templateInputs);
  const templateDigest = hash(
    canonical({
      template: CELL_TEMPLATE_VERSION,
      spec: boundTemplate.spec,
      podTemplate: boundTemplate.podTemplate,
    }),
  );
  const reader = new KubernetesReader(kubernetes),
    bindings = new Map<string, CellBinding>();
  const access = createCellRuntime(kubernetes, () => [...bindings.values()]);
  function resolve(intent: AllocationIntent, context: AccessContext) {
    const namespace = Object.hasOwn(options.namespaces, intent.owner.tenantId)
        ? options.namespaces[intent.owner.tenantId]
        : undefined;
    if (
      !namespace ||
      intent.template !== CELL_TEMPLATE_VERSION ||
      !uidPattern.test(intent.allocationKey) ||
      !intent.owner.principalId ||
      intent.owner.principalId.length > 256
    )
      throw new RuntimeAccessError(
        "InvalidConfiguration",
        context.correlationId,
        "Check authorized owner, allocation key and fixed Cell template version",
        { allocationKey: intent.allocationKey },
      );
    const name = "allocation-" + hash(intent.allocationKey).slice(0, 48);
    const spec = {
      ...boundTemplate.spec,
      allocation: {
        key: intent.allocationKey,
        principal: intent.owner.principalId,
        template: intent.template,
        profileDigest: templateDigest,
      },
    };
    const path = `/apis/dsh.isolated.io/v1alpha1/namespaces/${encodeURIComponent(namespace)}/cells`;
    return { namespace, name, spec, path };
  }
  function verifyCell(
    intent: AllocationIntent,
    cell: Cell,
    expected: string | undefined,
    context: AccessContext,
  ) {
    const { namespace, name, spec } = resolve(intent, context);
    const fail = (
      code: "StaleInstance" | "IntentConflict" | "TemplateMismatch",
    ) =>
      new RuntimeAccessError(
        code,
        context.correlationId,
        "Inspect the original allocation and immutable template; do not adopt or recreate",
        { allocationKey: intent.allocationKey },
        { stage: "inspect" },
      );
    const identity = cell.metadata?.uid;
    if (
      !identity ||
      !uidPattern.test(identity) ||
      cell.metadata?.name !== name ||
      cell.metadata.namespace !== namespace ||
      (expected && identity !== expected)
    )
      throw fail("StaleInstance");
    if (!isDeepStrictEqual(cell.spec?.allocation, spec.allocation))
      throw fail("IntentConflict");
    if (!isDeepStrictEqual(cell.spec, spec)) throw fail("TemplateMismatch");
    return { namespace, name, spec, identity };
  }
  async function view(
    intent: AllocationIntent,
    cell: Cell,
    expected: string | undefined,
    context: AccessContext,
  ): Promise<InstanceView> {
    const { namespace, name, spec, identity } = verifyCell(
      intent,
      cell,
      expected,
      context,
    );
    const origin = `https://cell-${identity}.${options.domain}`;
    const base = {
      ref: { allocationKey: intent.allocationKey, identity },
      origin,
      template: intent.template,
    };
    const substituteIdentity = (value: unknown): unknown =>
      typeof value === "string"
        ? value
            .replaceAll("${INSTANCE_ID}", identity)
            .replaceAll("${CELL_NAME}", name)
            .replaceAll("${ORIGIN_HOST}", new URL(origin).host)
        : Array.isArray(value)
          ? value.map(substituteIdentity)
          : value && typeof value === "object"
            ? Object.fromEntries(
                Object.entries(value).map(([k, v]) => [k, substituteIdentity(v)]),
              )
            : value;
    const expectedPodTemplate = substituteIdentity(boundTemplate.podTemplate) as {
      metadata: Record<string, unknown>;
      spec: Record<string, unknown>;
    };
    bindings.set(intent.allocationKey, {
      ref: base.ref,
      namespace,
      name,
      origin,
      template: intent.template,
      expectedSpec: spec,
      expectedPodSpec: expectedPodTemplate.spec,
      expectedPodTemplateMetadata: expectedPodTemplate.metadata,
    });
    if (cell.metadata?.deletionTimestamp)
      return { ...base, state: "Deleting", reason: "DeletionRequested" };
    if (
      (cell.status?.dshVersion && cell.status.dshVersion !== CELL_DSH_VERSION) ||
      (cell.status?.imageDigest &&
        cell.status.imageDigest !==
          String(boundTemplate.spec.image).split("@")[1])
    )
      throw new RuntimeAccessError(
        "TemplateMismatch",
        context.correlationId,
        "Check the pinned DSH/image version",
        { allocationKey: intent.allocationKey },
      );
    const failureReasons = new Set([
      "OwnershipConflict",
      "ReconcileFailed",
      "UnsupportedSecurityClass",
      "AccessCheckFailed",
    ]);
    const failed = cell.status?.conditions?.find(
      (c) =>
        c.observedGeneration === cell.metadata?.generation &&
        c.status === "False" &&
        failureReasons.has(c.reason ?? ""),
    );
    if (failed)
      return { ...base, state: "Unavailable", reason: failed.reason! };
    const condition = cell.status?.conditions?.find(
      (c) =>
        c.type === "Ready" &&
        c.observedGeneration === cell.metadata?.generation,
    );
    if (
      cell.status?.observedGeneration !== cell.metadata?.generation ||
      condition?.status !== "True"
    )
      return { ...base, state: "Pending", reason: "AwaitingCurrentReady" };
    try {
      return await access.inspect(base.ref, context);
    } catch (error) {
      if (
        error instanceof RuntimeAccessError &&
        (error.code === "NotReady" || error.code === "RecordMissing")
      )
        return {
          ...base,
          state: "Pending",
          reason: "AwaitingVerifiedWorkload",
        };
      throw error;
    }
  }
  return {
    ...access,
    async requestDelete(raw, expectedIdentity, context) {
      const intent = structuredClone(raw);
      const resolved = resolve(intent, context);
      const signal = AbortSignal.any([
        context.signal,
        AbortSignal.timeout(10000),
      ]);
      const ref = {
        allocationKey: intent.allocationKey,
        identity: expectedIdentity,
      };
      const missing = {
        ref,
        effect: "not-submitted" as const,
        observedState: "Missing" as const,
        writerState: "unverified" as const,
      };
      if (!uidPattern.test(expectedIdentity))
        throw new RuntimeAccessError(
          "StaleInstance",
          context.correlationId,
          "Supply the persisted exact instance identity",
          { allocationKey: intent.allocationKey },
          { stage: "delete" },
        );
      try {
        let cell: Cell;
        try {
          cell = await reader.get<Cell>(
            resolved.path + "/" + resolved.name,
            signal,
            context.correlationId,
          );
        } catch (error) {
          if (
            error instanceof RuntimeAccessError &&
            error.code === "RecordMissing"
          )
            return missing;
          throw error;
        }
        // Deletion validates authority and intent, never workload readiness.
        verifyCell(intent, cell, expectedIdentity, context);
        if (cell.metadata?.deletionTimestamp)
          return {
            ref,
            effect: "accepted",
            observedState: "Deleting",
            writerState: "unverified",
          };
        if (!cell.metadata?.resourceVersion)
          throw new RuntimeAccessError(
            "DeleteRejected",
            context.correlationId,
            "Inspect the original Cell resource version",
          );
        const result = await deleteResource(
          kubernetes,
          resolved.path + "/" + resolved.name,
          expectedIdentity,
          cell.metadata.resourceVersion,
          signal,
          context.correlationId,
        );
        return result === "missing"
          ? missing
          : {
              ref,
              effect: "accepted",
              observedState: "Deleting",
              writerState: "unverified",
            };
      } catch (error) {
        throw new RuntimeAccessError(
          error instanceof RuntimeAccessError
            ? error.code
            : "DeleteOutcomeUnknown",
          context.correlationId,
          "Inspect the original instance; do not replay deletion, reopen access or reuse its storage",
          { allocationKey: intent.allocationKey },
          {
            stage: "delete",
            effect:
              error instanceof RuntimeAccessError ? error.effect : "unknown",
          },
        );
      }
    },
    async inspectAllocation(raw, expected, context) {
      const intent = structuredClone(raw),
        resolved = resolve(intent, context);
      const bounded = {
        ...context,
        signal: AbortSignal.any([context.signal, AbortSignal.timeout(5000)]),
      };
      try {
        const cell = await reader.get<Cell>(
          resolved.path + "/" + resolved.name,
          bounded.signal,
          context.correlationId,
        );
        return await view(intent, cell, expected, bounded);
      } catch (error) {
        throw new RuntimeAccessError(
          error instanceof RuntimeAccessError ? error.code : "ReadUnavailable",
          context.correlationId,
          "Query this original allocation only; missing or changed records must not be recreated or adopted",
          { allocationKey: intent.allocationKey },
          {
            stage: "inspect",
            observedState:
              error instanceof RuntimeAccessError &&
              error.code === "RecordMissing"
                ? "missing"
                : "unverified",
          },
        );
      }
    },
    async create(raw, context) {
      const intent = structuredClone(raw),
        resolved = resolve(intent, context);
      const bounded = {
        ...context,
        signal: AbortSignal.any([context.signal, AbortSignal.timeout(10000)]),
      };
      try {
        const existing = await reader.get<Cell>(
          resolved.path + "/" + resolved.name,
          bounded.signal,
          context.correlationId,
        );
        return await view(intent, existing, undefined, bounded);
      } catch (error) {
        if (
          !(error instanceof RuntimeAccessError) ||
          error.code !== "RecordMissing"
        )
          throw error;
      }
      const created = await createResource<Cell>(
        kubernetes,
        resolved.path,
        {
          apiVersion: "dsh.isolated.io/v1alpha1",
          kind: "Cell",
          metadata: { name: resolved.name, namespace: resolved.namespace },
          spec: resolved.spec,
        },
        bounded.signal,
        context.correlationId,
      );
      // A 409 is not adoption: read back and verify exactly the same immutable intent.
      try {
        const cell =
          created ??
          (await reader.get<Cell>(
            resolved.path + "/" + resolved.name,
            bounded.signal,
            context.correlationId,
          ));
        return await view(intent, cell, undefined, bounded);
      } catch (error) {
        throw new RuntimeAccessError(
          error instanceof RuntimeAccessError
            ? error.code
            : "CreateOutcomeUnknown",
          context.correlationId,
          "Creation may exist; inspect the original allocation before any further action",
          { allocationKey: intent.allocationKey },
          { stage: "create", effect: created ? "accepted" : "unknown" },
        );
      }
    },
  };
}
