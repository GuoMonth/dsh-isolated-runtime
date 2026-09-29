import { createAgentEnvironmentRuntime } from "/tmp/connector/index.js";
import { writeFileSync, readFileSync } from "node:fs";
import assert from "node:assert/strict";
const options = JSON.parse(readFileSync("/tmp/options.json"));
const runtime = createAgentEnvironmentRuntime(options);
const context = () => ({
  signal: AbortSignal.timeout(85_000),
  correlationId: "b-real-lifecycle",
});
const intent = {
  allocationKey: process.env.ALLOCATION || "b-lifecycle-1",
  owner: { tenantId: "b-fixture", principalId: "alice" },
};
async function ready(ref) {
  for (let i = 0; i < 90; i++) {
    const v = await runtime.inspect(ref, context());
    if (v.state === "Ready") return v;
    await new Promise((r) => setTimeout(r, 500));
  }
  throw Error("Ready deadline");
}
let current;
if (process.env.RESUME === "1")
  current = await runtime.inspect(
    JSON.parse(readFileSync("/tmp/binding.json")).ref,
    context(),
  );
else current = await runtime.create(intent, context());
writeFileSync("/tmp/binding.json", JSON.stringify(current));
console.log(JSON.stringify({ phase: "created", view: current }));
current = await ready(current.ref);
console.log(JSON.stringify({ phase: "ready", view: current }));
const stop = await runtime.stop(current.ref, current.revision, context());
assert.equal(stop.view.state, "Stopped");
console.log(JSON.stringify({ phase: "stopped", view: stop.view }));
const restarted = await createAgentEnvironmentRuntime(options).start(
  stop.view.ref,
  stop.view.revision,
  context(),
);
current = await ready(restarted.view.ref);
console.log(JSON.stringify({ phase: "restarted", view: current }));
writeFileSync("/tmp/binding.json", JSON.stringify(current));
