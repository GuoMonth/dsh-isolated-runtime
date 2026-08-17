[简体中文](./CONTRIBUTING.zh-CN.md) | English

# Contributing

## Development model: spec-driven, then test-driven

This repository develops by **spec first, test second, implementation third**.

1. **Spec** — a capability starts as a written spec: a *boundary map* (surfaces
   + gaps), a contract sketch, or an *ADR* that records the decision. No
   implementation until the contract is written down.
2. **Test** — write the contract test *before* the implementation. A component's
   contract test is a **shared suite** that any implementation of that boundary
   must pass.
3. **Implement** — the smallest thing that satisfies the spec and the test.

## Bilingual documentation

Every user-facing document ships in English (default) and Simplified Chinese:

- English: `README.md`, `CONTRIBUTING.md`, `ROADMAP.md`, `docs/README.md`
- Chinese: `README.zh-CN.md`, `CONTRIBUTING.zh-CN.md`, `ROADMAP.zh-CN.md`,
  `docs/README.zh-CN.md`

Each file opens with a language toggle: the English file links the Chinese
translation, and vice versa. Keep the two in sync when either changes.

## Component boundaries (non-negotiable)

- **The isolation boundary is a runtime primitive** (Pod / container), never a
  rule inside a shared process. A change that moves isolation *into* shared code
  is out of scope — that is what `dsh-multi-tenant` does.
- **The control plane is runtime-agnostic.** Scheduler and Gateway talk to a
  container-runtime interface, not to Kubernetes directly. A pull request that
  hard-codes Kubernetes into the control plane is rejected.
- **Do not scaffold speculative components.** Create a component only when an
  independent install / replace / lifecycle boundary has been *demonstrated*,
  justified by a spec or ADR rather than by code size.

## Tests: contract vs isolation

Two test kinds prove different things:

- **Contract Test Suite** — for a component boundary (e.g. the Scheduler's
  placement interface). Any implementation must pass the same suite. This is
  what keeps "runtime-agnostic" honest: a Docker or Firecracker backend is
  proven by the contract, not by fiat.
- **Isolation Test** — an end-to-end proof that a tenant cannot escape its
  runtime boundary (network, filesystem, host resources). This is the project's
  "crown" test.

## License

By contributing, you agree that your contributions will be licensed under the
project's [MIT license](./LICENSE).
