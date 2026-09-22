import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:https";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { createCellAllocationRuntime } from "../dist/index.js";
async function fixture(t) {
  const dir = mkdtempSync(join(tmpdir(), "dsh-r6-api-"));
  const key = join(dir, "key.pem"),
    cert = join(dir, "cert.pem"),
    token = join(dir, "token");
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-keyout",
      key,
      "-out",
      cert,
      "-days",
      "1",
      "-subj",
      "/CN=localhost",
      "-addext",
      "subjectAltName=IP:127.0.0.1",
    ],
    { stdio: "ignore" },
  );
  writeFileSync(token, "local-test-token");
  let cell;
  let status = 200;
  let deletes = [];
  let notifyDelete;
  const deleteSeen = new Promise((resolve) => {
    notifyDelete = resolve;
  });
  const server = createServer(
    { key: readFileSync(key), cert: readFileSync(cert) },
    async (req, res) => {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const raw = Buffer.concat(chunks).toString();
      assert.equal(req.headers.authorization, "Bearer local-test-token");
      res.setHeader("content-type", "application/json");
      if (req.method === "POST") {
        cell = JSON.parse(raw);
        cell.metadata.uid = randomUUID();
        cell.metadata.resourceVersion = "7";
        cell.metadata.generation = 1;
        res.writeHead(201);
        res.end(JSON.stringify(cell));
      } else if (req.method === "DELETE") {
        deletes.push(JSON.parse(raw));
        notifyDelete();
        if (status === 0) return;
        res.writeHead(status);
        res.end(
          JSON.stringify({
            kind: "Status",
            status: status < 300 ? "Success" : "Failure",
          }),
        );
      } else {
        res.writeHead(cell ? 200 : 404);
        res.end(JSON.stringify(cell ?? {}));
      }
    },
  );
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
    rmSync(dir, { recursive: true });
  });
  const runtime = createCellAllocationRuntime(
    {
      server: `https://127.0.0.1:${server.address().port}`,
      caFile: cert,
      tokenFile: token,
    },
    {
      template: "cell-mvp-v1",
      image: "example/dsh@sha256:" + "a".repeat(64),
      storage: { size: "20Gi" },
      resources: {
        requests: { cpu: "250m", memory: "512Mi" },
        limits: { cpu: "1", memory: "1Gi" },
      },
      namespaces: { tenant: "tenant-a" },
      domain: "dsh.example.com",
    },
  );
  const intent = {
    allocationKey: randomUUID(),
    owner: { tenantId: "tenant", principalId: "alice" },
    template: "cell-mvp-v1",
  };
  const context = () => ({
    signal: new AbortController().signal,
    correlationId: randomUUID(),
  });
  const view = await runtime.create(intent, context());
  assert.equal(cell.spec.securityClass, "standard");
  assert.equal(cell.spec.storage.retentionPolicy, "Retain");
  assert.equal(cell.spec.allocation.template, "cell-mvp-v1");
  return {
    runtime,
    intent,
    view,
    context,
    deletes,
    deleteSeen,
    get cell() {
      return cell;
    },
    status(value) {
      status = value;
    },
    missing() {
      cell = undefined;
    },
  };
}
test("Pending deletion uses UID/resourceVersion conditions; acceptance never claims stopped", async (t) => {
  const f = await fixture(t);
  assert.equal(f.view.state, "Pending");
  const result = await f.runtime.requestDelete(
    f.intent,
    f.view.ref.identity,
    f.context(),
  );
  assert.deepEqual(f.deletes, [
    {
      apiVersion: "v1",
      kind: "DeleteOptions",
      preconditions: { uid: f.view.ref.identity, resourceVersion: "7" },
      propagationPolicy: "Foreground",
    },
  ]);
  assert.equal(result.effect, "accepted");
  assert.equal(result.writerState, "unverified");
});
test("replacement identity, precondition conflict and missing record never trigger retries", async (t) => {
  const f = await fixture(t);
  const identity = f.view.ref.identity;
  f.cell.metadata.uid = randomUUID();
  await assert.rejects(
    f.runtime.requestDelete(f.intent, identity, f.context()),
    { code: "StaleInstance" },
  );
  assert.equal(f.deletes.length, 0);
  f.cell.metadata.uid = identity;
  f.status(409);
  await assert.rejects(
    f.runtime.requestDelete(f.intent, identity, f.context()),
    { code: "StaleInstance", effect: "not-submitted" },
  );
  assert.equal(f.deletes.length, 1);
  f.missing();
  const missing = await f.runtime.requestDelete(
    f.intent,
    identity,
    f.context(),
  );
  assert.equal(missing.observedState, "Missing");
  assert.equal(missing.writerState, "unverified");
  assert.equal(f.deletes.length, 1);
});
test("interrupted DELETE reports unknown and existing Deleting requires no additional write", async (t) => {
  const f = await fixture(t);
  f.status(0);
  const abort = new AbortController();
  const rejected = assert.rejects(
    f.runtime.requestDelete(f.intent, f.view.ref.identity, {
      ...f.context(),
      signal: abort.signal,
    }),
    { code: "DeleteOutcomeUnknown", effect: "unknown" },
  );
  await f.deleteSeen;
  abort.abort();
  await rejected;
  assert.equal(f.deletes.length, 1);
  f.cell.metadata.deletionTimestamp = new Date().toISOString();
  const observed = await f.runtime.requestDelete(
    f.intent,
    f.view.ref.identity,
    f.context(),
  );
  assert.equal(observed.effect, "accepted");
  assert.equal(f.deletes.length, 1);
});
