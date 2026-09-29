import type { IncomingMessage, ServerResponse } from "node:http";
import type { Duplex } from "node:stream";

export { environmentTemplate, ENVIRONMENT_TEMPLATE } from "./template.js";
export type { EnvironmentTemplateOptions } from "./template.js";

export interface EnvironmentOwner {
  readonly tenantId: string;
  readonly principalId: string;
}
/** Platform creates one allocation key per owner and never replaces an unknown write. */
export interface EnvironmentIntent {
  readonly allocationKey: string;
  readonly owner: EnvironmentOwner;
}
/** Exact bindings, opaque to platform routing. PVC has no Sandbox ownerReference. */
export interface EnvironmentRef extends EnvironmentIntent {
  readonly namespace: string;
  readonly sandbox: { readonly name: string; readonly uid: string };
  readonly data: { readonly name: string; readonly uid: string };
}
export interface EnvironmentContext {
  readonly signal: AbortSignal;
  readonly correlationId: string;
}
/** Administrator-only inputs. Runtime owns on-demand per-allocation namespaces,
 * fixed SA/network policy, PVC provisioning, and origin derivation. */
export interface EnvironmentRuntimeOptions {
  readonly kubernetes: { readonly server: string; readonly caFile: string; readonly tokenFile: string };
  readonly namespacePrefix: string;
  readonly domain: string;
  readonly image: string;
  readonly storage: { readonly size: string; readonly storageClassName: string };
  readonly resources: import("./template.js").EnvironmentTemplateOptions["resources"];
  /** Platform namespace; fixed workload ingress admits only its platform-labelled pods. */
  readonly platformNamespace: string;
}
export type EnvironmentState =
  | "Pending" | "Ready" | "Stopping" | "Stopped"
  | "StopUnverified" | "Unavailable" | "Deleting";
export interface EnvironmentView {
  readonly ref: EnvironmentRef;
  /** API-server resourceVersion. Mutations compare both UID and this revision. */
  readonly revision: string;
  readonly state: EnvironmentState;
  readonly origin: string;
  readonly reason?: string;
}
export interface EnvironmentConnection {
  readonly origin: string;
  forward(request: IncomingMessage, response: ServerResponse): Promise<void>;
  upgrade(request: IncomingMessage, socket: Duplex, head: Buffer): Promise<void>;
}
export interface EnvironmentMutation {
  readonly view: EnvironmentView;
  readonly effect: "accepted" | "not-submitted";
}
export interface EnvironmentDeletion {
  readonly ref: EnvironmentRef;
  readonly effect: "accepted" | "not-submitted";
  readonly state: "Deleting" | "Deleted";
  readonly dataRetained: true;
}
/** One in-process Kubernetes adapter; no public wire protocol or backend factory. */
export interface AgentEnvironmentRuntime {
  create(intent: EnvironmentIntent, context: EnvironmentContext): Promise<EnvironmentView>;
  /** Missing before binding is null; a missing known UID is StaleInstance, never create. */
  inspectAllocation(intent: EnvironmentIntent, expected: EnvironmentRef | undefined, context: EnvironmentContext): Promise<EnvironmentView | null>;
  inspect(ref: EnvironmentRef, context: EnvironmentContext): Promise<EnvironmentView>;
  /** signal owns the full connection lifetime: abort rejects new requests and closes
   * all established HTTP streams/WebSockets, including after connect resolves. */
  connect(ref: EnvironmentRef, context: EnvironmentContext): Promise<EnvironmentConnection>;
  /** Stops access first; Stopped requires positive termination proof for the exact writer. */
  stop(ref: EnvironmentRef, revision: string, context: EnvironmentContext): Promise<EnvironmentMutation>;
  /** Only a positively Stopped environment with the same PVC UID may start. */
  start(ref: EnvironmentRef, revision: string, context: EnvironmentContext): Promise<EnvironmentMutation>;
  /** Deletes runtime resources only, after verified stop; retains the sole data PVC. */
  delete(ref: EnvironmentRef, revision: string, context: EnvironmentContext): Promise<EnvironmentDeletion>;
}

export type EnvironmentErrorCode =
  | "InvalidConfiguration" | "Forbidden" | "OwnerMismatch" | "IntentConflict"
  | "StaleInstance" | "StorageMismatch" | "TemplateMismatch" | "NotReady"
  | "ReadUnavailable" | "AllocationUnresolved" | "CreateRejected"
  | "CreateOutcomeUnknown" | "StopRejected" | "StopUnverified"
  | "StartRejected" | "StartOutcomeUnknown" | "DeleteRejected"
  | "DeleteOutcomeUnknown" | "AccessRejected" | "ForwardOutcomeUnknown";
export interface EnvironmentFailure {
  readonly code: EnvironmentErrorCode;
  readonly stage: "configuration" | "create" | "inspect" | "access" | "stop" | "start" | "delete";
  readonly effect: "not-submitted" | "accepted" | "unknown";
  readonly observedState: EnvironmentState | "Missing" | "unverified";
  readonly retry: "never" | "read-first";
  readonly allocationKey: string;
  readonly correlationId: string;
  readonly nextAction: string;
}
/** Caller supplies redacted diagnostics, never raw Kubernetes responses or credentials. */
export class EnvironmentError extends Error {
  constructor(readonly failure: EnvironmentFailure) {
    super(failure.code);
    this.name = "EnvironmentError";
  }
  toJSON(): EnvironmentFailure { return { ...this.failure }; }
}
