# Go development and lifecycle checks

Use Go 1.27.1: `dev-run go=1.27 -- make verify`. Make and the Docker launcher builder pin the same toolchain. Go supplies the workload launcher and native DSH compatibility tests; the Kubernetes API adapter is the Node Connector. No generated custom CRD or controller-runtime dependency remains.

`make verify` builds, runs Go race and Connector tests, vets Go and checks source standards. `make verify-dsh` tests the exact pinned upstream source seam. Real Kubernetes behavior requires the [environment harness](../test/environment/README.md); CI runs only source standards.

Every goroutine has an owner and exit condition. The launcher reaps its child, closes proxy transports and explicitly closes hijacked connections. Startup cancellation stops startup; after successful startup the owner calls Close. Graceful shutdown has a bounded escalation to SIGKILL, which must not count as successful environment stop. Launcher package tests use goleak and the race detector.

For optional local leak diagnostics, Go 1.27 provides the `goroutineleak` runtime/pprof profile. Keep captures private; this product exposes no pprof HTTP endpoint.
