# Go development and lifecycle checks

The module and supported build paths use Go 1.27.1. Make pins GOTOOLCHAIN to
that exact patch; Docker builders pin the matching multi-platform image digest.
Manual workflows and Source standards use the same Go patch. Kubernetes SDK
and test tooling follow the [Kubernetes baseline](archive/standalone-alpha1/kubernetes-baseline.md).

Run from the repository root:

```sh
make verify
make lint
make vuln
make test-envtest
```

`verify` checks formatting, reproducible code generation, vet, race tests and
builds. The controller-gen v0.22.0 is pinned and must reproduce
the committed API/CRD/RBAC files without drift. `lint` uses golangci-lint v2.13.2
with standard checks plus response-body closure analysis. `vuln` uses
govulncheck v1.8.0 and the current upstream vulnerability database; tool versions
are fixed, but vulnerability results can change as advisories are published.
govulncheck v1.1.4 cannot analyze the Go 1.27 standard library and must not be
used as evidence of a successful scan.

These are local behavioral checks, not new automatic CI gates. Source standards
remains the only automatic check. For proxy/authentication changes also run the
existing Phase 2 kind browser gate and exact DSH compatibility gate. Record the
commit, commands, results, prerequisite failures and unexecuted checks in the PR.

## Ownership of concurrent work

- Every goroutine needs an owner, an exit condition and, where teardown requires
  completion, a join. Propagate request/startup contexts rather than hiding them
  behind background contexts. Use a fresh bounded shutdown context after the
  request or process context has been canceled.
- HTTP Shutdown stops accepting work but does not forcibly close active requests
  on deadline. Close remaining connections after a failed drain. Hijacked proxy
  connections need explicit tracking and closure.
- A launcher owns its child until it is reaped. StartContext cancellation applies
  during startup; after successful startup the caller must Close the instance.
  Graceful termination precedes bounded escalation to SIGKILL. Scanner and proxy
  goroutines, as well as the private upstream transport, have explicit teardown.
- Controller-runtime owns controller/cache workers through Manager.Start's
  context. Do not replace watches and bounded queues with detached polling.
- A canceled OIDC discovery request must stop promptly. Startup signal handling
  must be installed before discovery or child readiness waits begin.

Package-level goleak checks run after test cleanup for launcher, authorizer,
authorizer command and controller tests, without broad goroutine ignore lists.
This permits parallel tests while catching residual work at package exit.
Race detection and leak detection prove different properties. Test cancellation,
deadline expiry, client disconnects, child failure and repeated startup/shutdown;
a passing happy-path test alone is insufficient.

## Optional leak diagnostics

Go 1.27 provides the `goroutineleak` runtime/pprof profile without an experimental
build flag. For a local diagnostic build or temporary test, collect it with
`pprof.Lookup("goroutineleak").WriteTo(privateFile, 1)` and compare with the
ordinary goroutine profile after reproducing the suspected failure. Keep the
output private and inspect it before sharing; stacks and labels may carry
sensitive information. No HTTP pprof endpoint is added or enabled by this change.

The profile identifies some permanently blocked goroutines using reachability;
it cannot detect all leaks, including some involving globally reachable state.
It complements, rather than replaces, teardown tests and investigation of open
connections, response bodies and child processes.

References: [Go 1.27 release notes](https://go.dev/doc/go1.27),
[goleak](https://github.com/uber-go/goleak),
[Go vulnerability management](https://go.dev/doc/security/vuln/).
