import { createAgentEnvironmentRuntime } from "/tmp/connector/index.js";
import { readFileSync, writeFileSync } from "node:fs";
import assert from "node:assert/strict";
const runtime = createAgentEnvironmentRuntime(
  JSON.parse(readFileSync("/tmp/options.json")),
);
const ctx = () => ({
  signal: AbortSignal.timeout(85000),
  correlationId: "b-real-" + process.argv[2],
});
const action = process.argv[2];
let v = JSON.parse(readFileSync("/tmp/binding.json"));
if (action === "missing" || action === "replaced") {
  for (const operation of [
    () => runtime.inspect(v.ref, ctx()),
    () => runtime.start(v.ref, v.revision, ctx()),
    () => runtime.delete(v.ref, v.revision, ctx()),
  ]) {
    await assert.rejects(
      operation,
      (e) => e.failure.code === "StorageMismatch",
    );
  }
  console.log(JSON.stringify({ name: action + "-pvc-rejected", ref: v.ref }));
} else {
  v = await runtime.inspect(v.ref, ctx());
  if (action === "start")
    v = (await runtime.start(v.ref, v.revision, ctx())).view;
  if (action === "stop")
    v = (await runtime.stop(v.ref, v.revision, ctx())).view;
  if (action === "race-start") {
    const results = await Promise.allSettled([
      runtime.start(v.ref, v.revision, ctx()),
      runtime.start(v.ref, v.revision, ctx()),
    ]);
    assert.equal(results.filter((r) => r.status === "fulfilled").length, 1);
    assert.equal(
      results.find((r) => r.status === "rejected").reason.failure.code,
      "IntentConflict",
    );
    console.log(
      JSON.stringify({ name: "race-start", accepted: 1, conflict: 1 }),
    );
  }
  if (["start", "race-start", "ready"].includes(action))
    for (let i = 0; i < 120; i++) {
      v = await runtime.inspect(v.ref, ctx());
      if (v.state === "Ready") break;
      if (i === 119) throw Error("Ready timeout");
      await new Promise((r) => setTimeout(r, 500));
    }
  writeFileSync("/tmp/binding.json", JSON.stringify(v));
  console.log(JSON.stringify({ name: action, view: v }));
}
