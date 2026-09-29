// Runs inside a labelled platform probe against the real Kubernetes API.
import { createAgentEnvironmentRuntime } from "/tmp/connector/index.js";
import { readFileSync, writeFileSync } from "node:fs";
import { createServer, request } from "node:https";
import assert from "node:assert/strict";
const options = JSON.parse(readFileSync("/tmp/options.json"));
const runtime = createAgentEnvironmentRuntime(options);
const ctx = () => ({
  signal: AbortSignal.timeout(85_000),
  correlationId: "real-kind-scenarios",
});
const report = (name, value) => console.log(JSON.stringify({ name, ...value }));
const wait = (ms) => new Promise((r) => setTimeout(r, ms));
const ready = async (ref) => {
  for (let i = 0; i < 120; i++) {
    const v = await runtime.inspect(ref, ctx());
    if (v.state === "Ready") return v;
    await wait(500);
  }
  throw Error("Ready timeout");
};
const code = async (p, expected) => {
  try {
    await p;
    assert.fail("Expected " + expected);
  } catch (e) {
    assert.ok(
      [expected].flat().includes(e.failure?.code ?? e.code),
      JSON.stringify(e),
    );
    report(e.failure?.code ?? e.code, { failure: e.failure ?? e });
  }
};
let v = await runtime.create(
  {
    allocationKey: "b-scenarios-" + Date.now(),
    owner: { tenantId: "b-fixture", principalId: "alice" },
  },
  ctx(),
);
v = await ready(v.ref);
await code(
  runtime.inspect(
    { ...v.ref, owner: { ...v.ref.owner, principalId: "mallory" } },
    ctx(),
  ),
  "OwnerMismatch",
);
await code(
  runtime.inspect(
    { ...v.ref, sandbox: { ...v.ref.sandbox, uid: "wrong" } },
    ctx(),
  ),
  "StaleInstance",
);
await code(
  runtime.inspect({ ...v.ref, data: { ...v.ref.data, uid: "wrong" } }, ctx()),
  "StorageMismatch",
);
const discover = await runtime.inspectAllocation(
  {
    allocationKey: v.ref.allocationKey,
    owner: { principalId: "alice", tenantId: "b-fixture" },
  },
  v.ref,
  ctx(),
);
assert.equal(discover.ref.sandbox.uid, v.ref.sandbox.uid);
const raced = await Promise.allSettled([
  runtime.stop(v.ref, v.revision, ctx()),
  runtime.stop(v.ref, v.revision, ctx()),
]);
assert.equal(raced.filter((x) => x.status === "fulfilled").length, 1);
assert.equal(
  raced.find((x) => x.status === "rejected").reason.failure.code,
  "IntentConflict",
);
v = await runtime.inspect(v.ref, ctx());
assert.equal(v.state, "Stopped");
report("concurrent-stop", {
  results: raced.map((x) =>
    x.status === "fulfilled" ? x.value : x.reason.failure,
  ),
  view: v,
});
const deleting = await runtime.delete(v.ref, v.revision, ctx());
assert.equal(deleting.dataRetained, true);
for (let i = 0; i < 50; i++) {
  const d = await runtime.delete(v.ref, v.revision, ctx());
  if (d.state === "Deleted") {
    report("deleted-retained", d);
    break;
  }
  await wait(200);
  if (i === 49) throw Error("Delete timeout");
}
await code(
  runtime.create(
    { allocationKey: v.ref.allocationKey, owner: v.ref.owner },
    ctx(),
  ),
  "StaleInstance",
);
// TLS fault proxy forwards genuine requests to kube-apiserver, only loses the
// successful activation response or rejects observation. It never fabricates resources.
let fault = "drop-activation";
let cancellation;
const server = createServer(
  { key: readFileSync("/tmp/fault.key"), cert: readFileSync("/tmp/fault.crt") },
  (req, res) => {
    if (fault === "cancel-next-read" && req.method === "GET") {
      cancellation.abort();
      res.destroy();
      return;
    }
    if (fault === "deny-watch" && req.url.includes("watch=true")) {
      res.writeHead(403);
      res.end("{}");
      return;
    }
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      const body = Buffer.concat(chunks);
      const drop =
        (fault === "drop-activation" &&
          req.method === "PATCH" &&
          body.includes(Buffer.from('"value":"Running"'))) ||
        (fault === "drop-delete" && req.method === "DELETE");
      const upstream = request(
        new URL(req.url, options.kubernetes.server),
        {
          method: req.method,
          ca: readFileSync(options.kubernetes.caFile),
          headers: {
            ...req.headers,
            host: new URL(options.kubernetes.server).host,
          },
        },
        (received) => {
          if (drop && received.statusCode >= 200 && received.statusCode < 300) {
            received.resume();
            received.on("end", () => {
              fault = "none";
              res.destroy();
            });
            return;
          }
          if (
            fault === "cancel-after-activation" &&
            req.method === "PATCH" &&
            body.includes(Buffer.from('"value":"Running"')) &&
            received.statusCode === 200
          )
            fault = "cancel-next-read";
          res.writeHead(received.statusCode, received.headers);
          received.pipe(res);
        },
      );
      res.on("close", () => upstream.destroy());
      upstream.on("error", () => res.destroy());
      upstream.end(body);
    });
  },
);
await new Promise((r) => server.listen(9443, "127.0.0.1", r));
try {
  const faultRuntime = createAgentEnvironmentRuntime({
    ...options,
    kubernetes: {
      ...options.kubernetes,
      server: "https://localhost:9443",
      caFile: "/tmp/fault.crt",
    },
  });
  const intent = {
    allocationKey: "b-lost-response-" + Date.now(),
    owner: { tenantId: "b-fixture", principalId: "alice" },
  };
  await code(faultRuntime.create(intent, ctx()), "CreateOutcomeUnknown");
  v = await faultRuntime.inspectAllocation(intent, undefined, ctx());
  assert.ok(v?.ref);
  v = await ready(v.ref);
  report("read-first-same-allocation", { view: v });
  fault = "deny-watch";
  await code(faultRuntime.stop(v.ref, v.revision, ctx()), "StopUnverified");
  await wait(2000);
  v = await runtime.inspect(v.ref, ctx());
  assert.ok(["StopUnverified", "Stopping"].includes(v.state));
  await code(
    createAgentEnvironmentRuntime(options).start(v.ref, v.revision, ctx()),
    ["StartRejected", "IntentConflict"],
  );
  await code(runtime.delete(v.ref, v.revision, ctx()), [
    "DeleteRejected",
    "IntentConflict",
  ]);
  writeFileSync("/tmp/unverified.json", JSON.stringify(v));
  report("durable-stop-unverified", { view: v });
  fault = "none";
  v = await runtime.create(
    {
      allocationKey: "b-cancel-after-write-" + Date.now(),
      owner: { tenantId: "b-fixture", principalId: "alice" },
    },
    ctx(),
  );
  v = await ready(v.ref);
  v = (await runtime.stop(v.ref, v.revision, ctx())).view;
  fault = "cancel-after-activation";
  cancellation = new AbortController();
  try {
    await faultRuntime.start(v.ref, v.revision, {
      signal: cancellation.signal,
      correlationId: "cancel-after-write",
    });
    assert.fail("Expected cancelled read");
  } catch (e) {
    assert.equal(e.failure.effect, "accepted");
    assert.equal(e.failure.retry, "read-first");
    report("cancel-after-accepted-write", { failure: e.failure });
  }
  fault = "none";
  v = await ready(v.ref);
  report("accepted-write-observed", { view: v });
  writeFileSync("/tmp/binding.json", JSON.stringify(v));
  let deleted = await runtime.create(
    {
      allocationKey: "b-delete-lost-" + Date.now(),
      owner: { tenantId: "b-fixture", principalId: "alice" },
    },
    ctx(),
  );
  deleted = await ready(deleted.ref);
  deleted = (await runtime.stop(deleted.ref, deleted.revision, ctx())).view;
  fault = "drop-delete";
  await code(
    faultRuntime.delete(deleted.ref, deleted.revision, ctx()),
    "DeleteOutcomeUnknown",
  );
  fault = "none";
  for (let i = 0; i < 50; i++) {
    const observed = await runtime.delete(deleted.ref, deleted.revision, ctx());
    if (observed.state === "Deleted") {
      report("delete-read-first-same-uid", observed);
      break;
    }
    if (i === 49) throw Error("Delete observation timeout");
    await wait(200);
  }
} finally {
  server.closeAllConnections();
  await new Promise((r) => server.close(r));
}
