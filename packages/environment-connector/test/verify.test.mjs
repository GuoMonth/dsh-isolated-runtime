import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  readyTarget,
  verifyBase,
  verifyPod,
  quantity,
} from "../dist/verify.js";
import { isTerminal } from "../dist/api.js";
const snapshot = JSON.parse(
  readFileSync(new URL("./fixtures/ready.json", import.meta.url)),
);
const get = (kind) => snapshot.resources.find((r) => r.kind === kind);
const sb = snapshot.sandbox,
  pod = get("Pod"),
  pvc = get("PersistentVolumeClaim"),
  service = get("Service"),
  slices = snapshot.resources.filter((r) => r.kind === "EndpointSlice");
const ref = {
  allocationKey: "b-lifecycle-1",
  owner: { tenantId: "b-fixture", principalId: "alice" },
  namespace: sb.metadata.namespace,
  sandbox: { name: "environment", uid: sb.metadata.uid },
  data: { name: "data", uid: pvc.metadata.uid },
};
const options = {
  namespacePrefix: "dsh-mvp-b",
  platformNamespace: "dsh-mvp-b-platform",
  domain: "environments.test",
  image: pod.spec.containers[0].image,
  storage: { size: "1Gi", storageClassName: "standard" },
  resources: {
    requests: { cpu: "100m", memory: "256Mi" },
    limits: { cpu: "1", memory: "1Gi" },
  },
};
const ns = {
  metadata: {
    name: ref.namespace,
    annotations: {
      ...sb.metadata.annotations,
      "environment.dsh.io/sandbox-uid": ref.sandbox.uid,
    },
  },
};
test("actual API defaults and canonical native quantities preserve ready identity", () => {
  verifyBase(ref, options, ns, sb, pvc);
  assert.equal(
    readyTarget(ref, options, sb, pvc, [pod], service, slices),
    pod.status.podIP,
  );
  assert.equal(quantity("1Gi"), quantity("1024Mi"));
  assert.equal(quantity("100m"), quantity("0.1"));
  assert.ok(Number.isNaN(quantity("1garbage")));
});
test("reject replaced data, owner, writer, unexpected privilege and ambiguous endpoints", () => {
  for (const mutate of [
    (p) => (p.metadata.uid = "replacement"),
    (p) => (p.metadata.annotations["environment.dsh.io/owner"] = "other"),
  ]) {
    const p = structuredClone(pvc);
    mutate(p);
    assert.throws(() => verifyBase(ref, options, ns, sb, p));
  }
  for (const mutate of [
    (p) => (p.metadata.ownerReferences[0].uid = "other"),
    (p) => (p.spec.containers[0].securityContext.privileged = true),
    (p) => p.spec.containers.push(p.spec.containers[0]),
  ]) {
    const p = structuredClone(pod);
    mutate(p);
    assert.throws(() => verifyPod(p, ref, options));
  }
  const es = structuredClone(slices);
  es[0].endpoints[0].targetRef.uid = "other";
  assert.throws(() => readyTarget(ref, options, sb, pvc, [pod], service, es));
  assert.throws(() =>
    readyTarget(ref, options, sb, pvc, [pod, pod], service, slices),
  );
});
test("absence, failure or a different writer is never positive stop evidence", () => {
  const terminal = {
    metadata: { uid: "writer" },
    status: {
      phase: "Succeeded",
      containerStatuses: [
        { name: "dsh", state: { terminated: { exitCode: 0 } } },
      ],
    },
  };
  assert.equal(isTerminal(terminal, "writer"), true);
  assert.equal(isTerminal(undefined, "writer"), false);
  assert.equal(isTerminal(terminal, "other"), false);
  terminal.status.containerStatuses[0].state.terminated.exitCode = 137;
  assert.equal(isTerminal(terminal, "writer"), false);
});

test("stop watches from collection RV, not the aged Pod item RV; observation failure remains gated", async (t) => {
  const { API } = await import("../dist/api.js");
  const { createAgentEnvironmentRuntime } = await import("../dist/index.js");
  const originalCall = API.prototype.call,
    originalTerminal = API.prototype.terminal;
  t.after(() => {
    API.prototype.call = originalCall;
    API.prototype.terminal = originalTerminal;
  });
  const sandbox = structuredClone(sb);
  let observed;
  API.prototype.call = async function (method, path, _signal, body) {
    if (method === "PATCH") {
      for (const op of body)
        if (op.op === "add" && op.path.startsWith("/metadata/annotations/"))
          sandbox.metadata.annotations[
            op.path.slice("/metadata/annotations/".length).replaceAll("~1", "/")
          ] = op.value;
      return sandbox;
    }
    if (path.startsWith("/api/v1/nodes/"))
      return {
        metadata: { uid: "node" },
        spec: {},
        status: { conditions: [{ type: "Ready", status: "True" }] },
      };
    if (path.includes("/leases/"))
      return {
        metadata: { ownerReferences: [{ kind: "Node", uid: "node" }] },
        spec: { renewTime: new Date().toISOString() },
      };
    if (path.includes("/sandboxes/")) return sandbox;
    if (path.endsWith("/persistentvolumeclaims/data")) return pvc;
    if (path.endsWith("/serviceaccounts/workload"))
      return get("ServiceAccount");
    if (path.includes("/networkpolicies/")) return get("NetworkPolicy");
    if (path.endsWith("/pods"))
      return {
        metadata: { resourceVersion: "current-list-cursor" },
        items: [pod],
      };
    if (path.includes("/services/")) return service;
    if (path.includes("/endpointslices?")) return { items: slices };
    return ns;
  };
  API.prototype.terminal = async function (path, uid) {
    observed = { path, uid };
    throw new Error("test-only failed observation");
  };
  const runtime = createAgentEnvironmentRuntime({
    ...options,
    kubernetes: {
      server: "https://kubernetes.test",
      caFile: "unused",
      tokenFile: "unused",
    },
  });
  await assert.rejects(
    runtime.stop(ref, sandbox.metadata.resourceVersion, {
      signal: AbortSignal.timeout(3000),
      correlationId: "cursor-regression",
    }),
    (e) =>
      e.failure.code === "StopUnverified" && e.failure.effect === "accepted",
  );
  assert.equal(observed.uid, pod.metadata.uid);
  assert.match(observed.path, /resourceVersion=current-list-cursor&/);
  assert.equal(
    sandbox.metadata.annotations["environment.dsh.io/phase"],
    "unverified",
  );
});
