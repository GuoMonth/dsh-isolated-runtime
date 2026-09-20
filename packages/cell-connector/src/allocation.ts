import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import {
  KubernetesReader,
  createResource,
  type KubernetesOptions,
} from "./kubernetes.js";
import { createCellRuntime, type CellBinding } from "./cell.js";
import {
  RuntimeAccessError,
  type AllocationIntent,
  type AllocationRuntime,
  type AccessContext,
  type InstanceView,
} from "./port.js";
export interface CellProfile {
  readonly template: string;
  readonly expectedSpec: Readonly<Record<string, unknown>>;
  /** Exact defaulted workload spec; only ${INSTANCE_ID} and ${ORIGIN_HOST} substitutions. */
  readonly expectedPodSpec: Readonly<Record<string, unknown>>;
}
export interface CellAllocationOptions {
  readonly namespaces: Readonly<Record<string, string>>;
  readonly domain: string;
  readonly profiles: readonly CellProfile[];
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
interface Cell {
  metadata?: {
    uid?: string;
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
  if (!/^[a-z0-9.-]+\.[a-z0-9-]+$/.test(options.domain))
    throw new Error("Invalid Cell domain");
  const namespaces = Object.values(options.namespaces);
  if (
    !namespaces.length ||
    new Set(namespaces).size !== namespaces.length ||
    namespaces.some((n) => !/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(n))
  )
    throw new Error("Invalid tenant namespaces");
  const profiles = new Map<string, CellProfile>();
  for (const p of options.profiles) {
    if (
      !p.template ||
      p.template.length > 128 ||
      profiles.has(p.template) ||
      !p.expectedPodSpec ||
      !/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(String(p.expectedSpec.image)) ||
      "allocation" in p.expectedSpec ||
      JSON.stringify(p.expectedSpec).includes("${") ||
      JSON.stringify(p.expectedSpec).includes("restoreFrom")
    )
      throw new Error("Invalid allocation profile");
    profiles.set(p.template, p);
  }
  const reader = new KubernetesReader(kubernetes),
    bindings = new Map<string, CellBinding>();
  const access = createCellRuntime(kubernetes, () => [...bindings.values()]);
  function resolve(intent: AllocationIntent, context: AccessContext) {
    const namespace = Object.hasOwn(options.namespaces, intent.owner.tenantId)
        ? options.namespaces[intent.owner.tenantId]
        : undefined,
      profile = profiles.get(intent.template);
    if (
      !namespace ||
      !profile ||
      !uidPattern.test(intent.allocationKey) ||
      !intent.owner.principalId ||
      intent.owner.principalId.length > 256
    )
      throw new RuntimeAccessError(
        "InvalidConfiguration",
        context.correlationId,
        "Check authorized owner, allocation key and pinned template",
        { allocationKey: intent.allocationKey },
      );
    const name = "allocation-" + hash(intent.allocationKey).slice(0, 48);
    const spec = {
      ...profile.expectedSpec,
      allocation: {
        key: intent.allocationKey,
        principal: intent.owner.principalId,
        template: intent.template,
        profileDigest: hash(canonical(profile)),
      },
    };
    const path = `/apis/dsh.isolated.io/v1alpha1/namespaces/${encodeURIComponent(namespace)}/cells`;
    return { namespace, profile, name, spec, path };
  }
  async function view(
    intent: AllocationIntent,
    cell: Cell,
    expected: string | undefined,
    context: AccessContext,
  ): Promise<InstanceView> {
    const { namespace, profile, name, spec } = resolve(intent, context);
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
    const origin = `https://cell-${identity}.${options.domain}`;
    const base = {
      ref: { allocationKey: intent.allocationKey, identity },
      origin,
      template: intent.template,
    };
    const substitute = (value: unknown): unknown =>
      typeof value === "string"
        ? value
            .replaceAll("${INSTANCE_ID}", identity)
            .replaceAll("${ORIGIN_HOST}", new URL(origin).host)
        : Array.isArray(value)
          ? value.map(substitute)
          : value && typeof value === "object"
            ? Object.fromEntries(
                Object.entries(value).map(([k, v]) => [k, substitute(v)]),
              )
            : value;
    bindings.set(intent.allocationKey, {
      ref: base.ref,
      namespace,
      name,
      origin,
      template: intent.template,
      expectedSpec: spec,
      expectedPodSpec: substitute(profile.expectedPodSpec) as Record<
        string,
        unknown
      >,
    });
    if (cell.metadata.deletionTimestamp)
      return { ...base, state: "Deleting", reason: "DeletionRequested" };
    if (
      (cell.status?.dshVersion && cell.status.dshVersion !== "0.1.5-rc.2") ||
      (cell.status?.imageDigest &&
        cell.status.imageDigest !==
          String(profile.expectedSpec.image).split("@")[1])
    )
      throw fail("TemplateMismatch");
    const failureReasons = new Set([
      "OwnershipConflict",
      "ReconcileFailed",
      "SandboxRuntimeClassUnconfigured",
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
      cell.status?.observedGeneration !== cell.metadata.generation ||
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
