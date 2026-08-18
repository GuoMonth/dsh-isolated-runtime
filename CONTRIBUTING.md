[简体中文](./CONTRIBUTING.zh-CN.md) | English

# Contributing

## Development model: spec-driven, then test-driven

This repository develops by **spec first, test second, implementation third**.

1. **Spec** — write the boundary/invariant before implementation.
2. **Test** — reusable contract tests prove backend behavior; end-to-end
   isolation tests prove security properties.
3. **Implement** — add the smallest implementation that satisfies both.

## Component boundaries

- **Runtime ownership is non-negotiable.** Every Runtime has exactly one tenant
  owner; a tenant may own many Runtimes; cross-tenant reuse is invalid.
- **Do not reimplement infrastructure Kubernetes already owns.** Runtime
  allocation decides reuse/create/profile/resources/security posture. Kubernetes
  remains responsible for Pod-to-Node scheduling.
- **Kubernetes-specific code is allowed behind explicit adapters.** Kubernetes is
  the first real backend. Keep its API details in backend/controller adapters,
  but do not invent a lowest-common-denominator abstraction before a second
  backend exists.
- **Principal is trusted transport state.** Authentication establishes identity
  server-side. Public request payloads may name target resources but never
  establish caller identity.
- **Continuity is logical-state-first.** `pkg/checkpoint` owns persistence/
  restore. Do not add a second checkpoint authority to `pkg/runtime`.
- **Do not scaffold speculative components.** A new package/process needs a
  demonstrated install/replace/lifecycle/security boundary.

## Tests

- **Runtime Contract Suite** — shared suite under `pkg/runtime/runtimetest`; every
  backend must prove ownership immutability, foreign-tenant non-enumeration,
  conflict semantics, and lifecycle behavior.
- **Component tests** — allocation, admission, persistence, and control-plane
  contracts.
- **Isolation tests** — end-to-end cross-tenant filesystem/network/credential/
  routing checks. Claims about stronger host isolation must be scoped to the
  selected `SecurityClass` / `RuntimeClass`.

## Bilingual docs

User-facing architecture/docs changes update English and Simplified Chinese
together.

## License

By contributing, you agree that your contributions are licensed under MIT.
