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
