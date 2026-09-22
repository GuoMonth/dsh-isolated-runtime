import fixedTemplate from "./templates/cell-mvp-v1.json" with { type: "json" };

export const CELL_TEMPLATE_VERSION = "cell-mvp-v1" as const;
type JsonObject = Record<string, unknown>;
interface FixedTemplate {
  version: string;
  cellSpec: JsonObject;
  podTemplate: {
    metadata: JsonObject;
    spec: JsonObject;
  };
}

const template = fixedTemplate as unknown as FixedTemplate;
if (template.version !== CELL_TEMPLATE_VERSION)
  throw new Error("Cell connector template version does not match its contract");

export interface CellTemplateInputs {
  readonly image: string;
  readonly storage: {
    readonly size: string;
    readonly storageClassName?: string;
    readonly retentionPolicy: "Retain" | "Delete";
  };
  readonly resources: {
    readonly requests: { readonly cpu: string; readonly memory: string };
    readonly limits: { readonly cpu: string; readonly memory: string };
  };
  readonly credentialsSecret?: string;
}

export interface BoundCellTemplate {
  readonly spec: JsonObject;
  readonly podTemplate: { readonly metadata: JsonObject; readonly spec: JsonObject };
}

const substitute = (value: unknown, inputs: CellTemplateInputs): unknown => {
  if (typeof value === "string") {
    let result = value;
    const values: Record<string, string> = {
      __IMAGE__: inputs.image,
      __CPU_REQUEST__: inputs.resources.requests.cpu,
      __MEMORY_REQUEST__: inputs.resources.requests.memory,
      __CPU_LIMIT__: inputs.resources.limits.cpu,
      __MEMORY_LIMIT__: inputs.resources.limits.memory,
      __STORAGE_SIZE__: inputs.storage.size,
      __STORAGE_CLASS__: inputs.storage.storageClassName ?? "",
      __RETENTION_POLICY__: inputs.storage.retentionPolicy,
      __CREDENTIALS_SECRET__: inputs.credentialsSecret ?? "",
    };
    for (const [token, replacement] of Object.entries(values))
      result = result.replaceAll(token, replacement);
    return result
      .replaceAll("__ORIGIN_HOST__", "${ORIGIN_HOST}")
      .replaceAll("__CELL_UID__", "${INSTANCE_ID}")
      .replaceAll("__CELL_NAME__", "${CELL_NAME}");
  }
  if (Array.isArray(value)) return value.map((entry) => substitute(entry, inputs));
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.entries(value).map(([key, entry]) => [key, substitute(entry, inputs)]),
    );
  return value;
};

const objectAt = (value: unknown, key: string): JsonObject => {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error(`Invalid generated Cell template at ${key}`);
  return value as JsonObject;
};

export function bindCellTemplate(inputs: CellTemplateInputs): BoundCellTemplate {
  const cellSpec = structuredClone(template.cellSpec) as JsonObject;
  const podTemplate = structuredClone(template.podTemplate) as {
    metadata: JsonObject;
    spec: JsonObject;
  };
  const spec = substitute(cellSpec, inputs) as JsonObject;
  const boundPodTemplate = substitute(podTemplate, inputs) as {
    metadata: JsonObject;
    spec: JsonObject;
  };

  if (!inputs.storage.storageClassName)
    delete objectAt(spec.storage, "Cell storage").storageClassName;
  if (!inputs.credentialsSecret) {
    delete spec.credentialsRef;
    const containers = boundPodTemplate.spec.containers;
    if (!Array.isArray(containers) || !containers[0])
      throw new Error("Invalid generated Cell template containers");
    delete (containers[0] as JsonObject).envFrom;
  }
  return { spec, podTemplate: boundPodTemplate };
}
