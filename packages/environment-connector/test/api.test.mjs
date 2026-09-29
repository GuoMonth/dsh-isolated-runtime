import test from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
import { createServer } from "node:https";
import { API } from "../dist/api.js";
test("DELETE sends UID/RV preconditions as a framed body, preserving next keep-alive request", async () => {
  const dir = mkdtempSync(join(tmpdir(), "environment-api-"));
  execFileSync(
    "openssl",
    [
      "req",
      "-x509",
      "-newkey",
      "rsa:2048",
      "-nodes",
      "-keyout",
      join(dir, "key"),
      "-out",
      join(dir, "cert"),
      "-days",
      "1",
      "-subj",
      "/CN=localhost",
      "-addext",
      "subjectAltName=DNS:localhost",
    ],
    { stdio: "ignore" },
  );
  writeFileSync(join(dir, "token"), "test-only-noncredential");
  let received;
  const server = createServer(
    {
      key: readFileSync(join(dir, "key")),
      cert: readFileSync(join(dir, "cert")),
    },
    (req, res) => {
      const chunks = [];
      req.on("data", (c) => chunks.push(c));
      req.on("end", () => {
        if (req.method === "DELETE") {
          const body = Buffer.concat(chunks);
          assert.equal(Number(req.headers["content-length"]), body.length);
          received = JSON.parse(body);
        }
        res.end(JSON.stringify({ metadata: { name: "test" } }));
      });
    },
  );
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  try {
    const api = new API({
      server: `https://localhost:${server.address().port}`,
      caFile: join(dir, "cert"),
      tokenFile: join(dir, "token"),
    });
    const body = {
      apiVersion: "v1",
      kind: "DeleteOptions",
      preconditions: { uid: "exact-writer", resourceVersion: "23" },
    };
    await api.call("DELETE", "/resource", AbortSignal.timeout(3000), body);
    assert.deepEqual(received, body);
    assert.equal(
      (await api.call("GET", "/resource", AbortSignal.timeout(3000))).metadata
        .name,
      "test",
    );
  } finally {
    server.closeAllConnections();
    await new Promise((r) => server.close(r));
    rmSync(dir, { recursive: true });
  }
});
