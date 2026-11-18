import { z } from 'zod'
import { identities } from './identity.ts'

// Identity limits are UTF-16 code units, as in the original allocation contract.
const opaque = z.string().min(1).refine(value => value.length <= 256, 'Identity exceeds 256 code units')
export const ownerSchema = z.strictObject({
  tenantId: opaque.transform(identities.tenantId),
  principalId: opaque.transform(identities.principalId),
})
export const intentSchema = z.strictObject({
  allocationKey: opaque.transform(identities.allocationKey),
  owner: ownerSchema,
})
export const refSchema = z.strictObject({
  ...intentSchema.shape,
  namespace: opaque.transform(identities.namespace),
  sandbox: z.strictObject({ name: opaque, uid: opaque.transform(identities.sandboxUID) }),
  data: z.strictObject({ name: opaque, uid: opaque.transform(identities.pvcUID) }),
})
const quantities = z.strictObject({ cpu: z.string().min(1), memory: z.string().min(1) })
export const runtimeOptionsSchema = z.strictObject({
  kubernetes: z.strictObject({ server: z.string().min(1), caFile: z.string().min(1), tokenFile: z.string().min(1) }),
  namespacePrefix: z.string().min(1),
  domain: z.string().min(1),
  image: z.string().regex(/^\S+@sha256:[a-f0-9]{64}$/u),
  storage: z.strictObject({ size: z.string().min(1), storageClassName: z.string().min(1) }),
  resources: z.strictObject({ requests: quantities, limits: quantities }),
  platformNamespace: z.string().min(1),
})

export const parseEnvironmentOwner = (value: unknown) => ownerSchema.parse(value)
export const parseEnvironmentIntent = (value: unknown) => intentSchema.parse(value)
// Runtime operations also receive a full ref/binding; admit only its allocation identity here.
export const parseAllocationIdentity = (value: unknown) => z.object(intentSchema.shape).parse(value)
export const parseEnvironmentRef = (value: unknown) => refSchema.parse(value)
export const parseEnvironmentRuntimeOptions = (value: unknown) => runtimeOptionsSchema.parse(value)

const strings = z.record(z.string(), z.string())
const namespaceName = z.string().regex(/^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/u).max(63)
const resourceName = z.string().regex(/^[a-z0-9](?:[-a-z0-9.]*[a-z0-9])?$/u).max(253)
const reference = z.looseObject({
  apiVersion: z.string().optional(), kind: z.string(), name: z.string(),
  uid: opaque, namespace: z.string().optional(), controller: z.boolean().optional(),
})
const condition = z.looseObject({
  type: z.string(), status: z.string(), observedGeneration: z.number().int().optional(),
})
const probe = z.looseObject({
  httpGet: z.looseObject({ path: z.string(), port: z.union([z.string(), z.number()]), scheme: z.string().optional() }),
  timeoutSeconds: z.number().optional(), successThreshold: z.number().optional(), failureThreshold: z.number().optional(),
})
const port = z.looseObject({
  name: z.string().optional(), port: z.number().optional(), targetPort: z.union([z.string(), z.number()]).optional(),
  containerPort: z.number().optional(), protocol: z.string().optional(),
})
const container = z.looseObject({
  name: z.string(), image: z.string(),
  terminationMessagePath: z.string().optional(), terminationMessagePolicy: z.string().optional(),
  ports: z.array(port).optional(),
  resources: z.looseObject({ limits: strings.optional(), requests: strings.optional() }).optional(),
  startupProbe: probe.optional(), readinessProbe: probe.optional(), livenessProbe: probe.optional(),
})
/** Preserve every unknown field: dropping one could hide an unexpected privilege from the exact-template comparison. */
export const podSpecSchema = z.looseObject({
  containers: z.array(container), nodeName: z.string().optional(),
  serviceAccount: z.string().optional(), serviceAccountName: z.string().optional(),
  tolerations: z.array(z.unknown()).optional(),
})
export type PodSpec = z.infer<typeof podSpecSchema>
const endpoint = z.looseObject({
  addresses: z.array(z.string()), targetRef: reference.optional(),
  conditions: z.looseObject({ ready: z.boolean().optional(), terminating: z.boolean().optional() }).optional(),
})
export type Endpoint = z.infer<typeof endpoint>
const metadata = z.looseObject({
  name: resourceName, uid: opaque, resourceVersion: opaque.transform(identities.revision),
  namespace: namespaceName.optional(), generation: z.number().int().optional(), deletionTimestamp: z.string().optional(),
  annotations: strings.optional(), labels: strings.optional(), ownerReferences: z.array(reference).optional(),
})
const spec = z.looseObject({
  operatingMode: z.string().optional(), service: z.boolean().optional(), shutdownPolicy: z.string().optional(),
  podTemplate: z.looseObject({ metadata: z.unknown(), spec: podSpecSchema }).optional(),
  storageClassName: z.string().optional(), accessModes: z.array(z.string()).optional(), volumeName: z.string().optional(),
  resources: z.looseObject({ requests: strings.optional(), limits: strings.optional() }).optional(),
  nodeName: z.string().optional(), taints: z.array(z.looseObject({ key: z.string() })).optional(),
  renewTime: z.string().optional(),
  type: z.string().optional(), clusterIP: z.string().optional(), publishNotReadyAddresses: z.boolean().optional(),
  selector: strings.optional(), ports: z.array(port).optional(),
  // Pod fields are also admitted here; the Pod parser below gives them their complete typed form.
  containers: z.array(container).optional(), serviceAccount: z.string().optional(), serviceAccountName: z.string().optional(),
  tolerations: z.array(z.unknown()).optional(),
})
const status = z.looseObject({
  conditions: z.array(condition).optional(), phase: z.string().optional(), podIP: z.string().optional(),
  containerStatuses: z.array(z.looseObject({
    name: z.string(), state: z.looseObject({ terminated: z.looseObject({ exitCode: z.number().int() }).optional() }),
  })).optional(),
})
const resource = z.looseObject({
  apiVersion: z.string().min(1), kind: z.string().min(1), metadata,
  spec: spec.optional(), status: status.optional(),
  automountServiceAccountToken: z.boolean().optional(),
  endpoints: z.array(endpoint).nullable().optional(), ports: z.array(port).nullable().optional(),
})
export type Resource = z.infer<typeof resource>
const listMetadata = z.looseObject({ resourceVersion: opaque.transform(identities.revision), continue: z.literal('').optional() })
const listedResource = resource.extend({ apiVersion: z.string().min(1).optional(), kind: z.string().min(1).optional() })
const list = z.discriminatedUnion('kind', [
  z.looseObject({ apiVersion: z.literal('v1'), kind: z.literal('PodList'), metadata: listMetadata, items: z.array(listedResource) }),
  z.looseObject({ apiVersion: z.literal('discovery.k8s.io/v1'), kind: z.literal('EndpointSliceList'), metadata: listMetadata, items: z.array(listedResource) }),
]).superRefine((value, ctx) => {
  const kind = value.kind === 'PodList' ? 'Pod' : 'EndpointSlice'
  for (const [index, item] of value.items.entries()) {
    if (item.apiVersion !== undefined && item.apiVersion !== value.apiVersion)
      ctx.addIssue({ code: 'custom', path: ['items', index, 'apiVersion'], message: 'Item version differs from its typed list' })
    if (item.kind !== undefined && item.kind !== kind)
      ctx.addIssue({ code: 'custom', path: ['items', index, 'kind'], message: 'Item kind differs from its typed list' })
  }
}).transform(value => ({
  ...value,
  // Kubernetes typed lists omit TypeMeta from items; kubectl's printer adds it.
  items: value.items.map(item => ({ ...item, apiVersion: value.apiVersion, kind: value.kind === 'PodList' ? 'Pod' : 'EndpointSlice' })),
}))
export type ResourceList = z.infer<typeof list>
export const parseResource = (value: unknown): Resource => resource.parse(value)
export const parseResourceList = (value: unknown): ResourceList => list.parse(value)
const deletion = z.union([resource, z.looseObject({ apiVersion: z.literal('v1'), kind: z.literal('Status'), status: z.literal('Success') })])
export function parseDeletion(value: unknown): void {
  deletion.parse(value)
}
const watch = z.discriminatedUnion('type', [
  z.object({ type: z.enum(['ADDED', 'MODIFIED', 'DELETED']), object: resource }),
  z.object({ type: z.literal('ERROR') }),
])
export const parseWatchEvent = (value: unknown) => watch.parse(value)
