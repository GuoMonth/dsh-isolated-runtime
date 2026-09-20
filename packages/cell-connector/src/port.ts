import type { IncomingMessage, ServerResponse } from "node:http";
import type { Duplex } from "node:stream";

/** Opaque runtime identity. Platform code must not derive resource addresses from it. */
export interface InstanceRef {
  readonly allocationKey: string;
  readonly identity: string;
}
export interface AccessContext {
  readonly signal: AbortSignal;
  readonly correlationId: string;
}
export interface InstanceView {
  readonly ref: InstanceRef;
  readonly origin: string;
  readonly template: string;
  readonly state: "Pending" | "Ready" | "Unavailable" | "Deleting";
  readonly reason?: string;
}
export interface Connector {
  readonly origin: string;
  forward(request: IncomingMessage, response: ServerResponse): Promise<void>;
  upgrade(
    request: IncomingMessage,
    socket: Duplex,
    head: Buffer,
  ): Promise<void>;
}
/** R2 deliberately has no create/delete/stop or hidden resource acquisition. */
export interface RuntimeAccess {
  inspect(ref: InstanceRef, context: AccessContext): Promise<InstanceView>;
  connect(ref: InstanceRef, context: AccessContext): Promise<Connector>;
}
export type ErrorCode =
  | "IntentConflict"
  | "CreateOutcomeUnknown"
  | "CreateRejected"
  | "StateUnavailable"
  | "AllocationUnresolved"
  | "InvalidConfiguration"
  | "Forbidden"
  | "RecordMissing"
  | "StaleInstance"
  | "TemplateMismatch"
  | "NotReady"
  | "ReadUnavailable"
  | "AccessRejected"
  | "ForwardOutcomeUnknown";
export class RuntimeAccessError extends Error {
  readonly effect: "not-submitted" | "accepted" | "unknown";
  readonly observedState: string;
  readonly retry: "never" | "read-first";
  readonly stage: "access" | "create" | "inspect" | "state";
  constructor(
    readonly code: ErrorCode,
    readonly correlationId: string,
    readonly nextAction: string,
    readonly target: Readonly<{ allocationKey?: string }> = {},
    detail: {
      stage?: "access" | "create" | "inspect" | "state";
      effect?: "not-submitted" | "accepted" | "unknown";
      observedState?: string;
    } = {},
  ) {
    super(code);
    this.name = "RuntimeAccessError";
    this.stage = detail.stage ?? "access";
    this.observedState = detail.observedState ?? "unverified";
    this.effect =
      detail.effect ??
      (code === "ForwardOutcomeUnknown" || code === "CreateOutcomeUnknown"
        ? "unknown"
        : "not-submitted");
    this.retry =
      code === "ReadUnavailable" ||
      code === "NotReady" ||
      code === "CreateOutcomeUnknown" ||
      code === "AllocationUnresolved"
        ? "read-first"
        : "never";
  }
  toJSON() {
    return {
      code: this.code,
      target: this.target,
      stage: this.stage,
      effect: this.effect,
      observedState: this.observedState,
      retry: this.retry,
      message: this.message,
      nextAction: this.nextAction,
      correlationId: this.correlationId,
    };
  }
}

/** Platform intent contains no backend coordinates. Tenant scope is resolved by the adapter. */
export interface AllocationIntent {
  readonly allocationKey: string;
  readonly owner: { readonly tenantId: string; readonly principalId: string };
  readonly template: string;
}
export interface AllocationRuntime extends RuntimeAccess {
  create(
    intent: AllocationIntent,
    context: AccessContext,
  ): Promise<InstanceView>;
  inspectAllocation(
    intent: AllocationIntent,
    expectedIdentity: string | undefined,
    context: AccessContext,
  ): Promise<InstanceView>;
}
