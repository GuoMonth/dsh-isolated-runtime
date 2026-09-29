import assert from "node:assert/strict";
import test from "node:test";
import { environmentTemplate, EnvironmentError } from "../dist/index.js";
const options = {
  image: `dsh-environment@sha256:${"a".repeat(64)}`, authority: "alice.example.test",
  dataClaim: "data", serviceAccount: "workload",
  resources: { requests: { cpu: "100m", memory: "256Mi" }, limits: { cpu: "1", memory: "1Gi" } },
};
test("single durable root, no credentials or Kubernetes token injection", () => {
  const { spec } = environmentTemplate(options);
  assert.equal(spec.automountServiceAccountToken, false);
  assert.equal(spec.volumes.filter(v => v.persistentVolumeClaim).length, 1);
  const container = spec.containers[0];
  assert.equal(container.envFrom, undefined);
  assert.equal(container.env.find(v => v.name === "DSH_HOME").value, "/var/lib/dsh/data/dsh");
  assert.equal(container.env.find(v => v.name === "HOME").value, "/var/lib/dsh/data/home");
  assert.deepEqual(container.resources, options.resources);
  container.resources.limits.cpu = "2";
  assert.equal(options.resources.limits.cpu, "1");
});
test("reject mutable images and invalid authority before submission", () => {
  assert.throws(() => environmentTemplate({ ...options, image: "dsh:latest" }));
  assert.throws(() => environmentTemplate({ ...options, authority: "https://alice.test" }));
});
test("unknown create is structured, does not imply safe retry", () => {
  const failure = { code: "CreateOutcomeUnknown", stage: "create", effect: "unknown", observedState: "unverified", retry: "read-first", allocationKey: "allocation", correlationId: "log-id", nextAction: "Inspect the same allocation; do not create another key" };
  assert.deepEqual(new EnvironmentError(failure).toJSON(), failure);
});
