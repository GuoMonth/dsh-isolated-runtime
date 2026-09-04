# Phase 4 bounded fleet proof

Run the complete proof with:

```sh
make verify-kind-phase4
```

The runner extends the Phase 3 kind environment and fixes the following
development baseline:

- one Kubernetes 1.34 kind control-plane node;
- 10 tenant namespaces and 50 initial Cells;
- two exact `dsh-v0.1.2-rc.1` Cell images and 48 lightweight workload
  fixtures;
- eight overlapping snapshots plus one same-Cell queued operation;
- operator Cell concurrency 4 and CellSnapshot concurrency 2;
- native Namespace labels, ResourceQuota, LimitRange, Secret admission and
  Gateway policy as the injected prerequisites and failures.

The proof fails if all Cells or snapshots do not converge within 720 seconds,
if a quiet cluster continues reconciling without API events, if more than one
writer Pod exists for a Cell UID, or if metrics contain object identity,
topology or secret data. It records convergence time and the operator's peak
working set from the kubelet summary API.

These values are a reproducible regression fixture, not a production capacity
claim or service-level objective. Real limits depend on the API server,
admission policy, CSI, Gateway, nodes and workload shape. The reference GitHub
gate targets the public `ubuntu-latest` envelope recorded for this milestone:
4 vCPU, 16 GiB RAM and 14 GiB SSD. The script records the actual CPU, memory
and available disk instead of hard-coding those values, so it remains useful
on local machines and future runner revisions. Consumers should establish
their own baselines on representative clusters.
