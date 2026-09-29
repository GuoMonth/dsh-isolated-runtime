// Real HTTP and WebSocket through the production Connector; no mock application.
import { createAgentEnvironmentRuntime } from "/tmp/connector/index.js";
import { readFileSync, writeFileSync } from "node:fs";
import { createServer, request } from "node:http";
import WebSocket from "/opt/dsh/node_modules/ws/wrapper.mjs";
import assert from "node:assert/strict";
const runtime = createAgentEnvironmentRuntime(
  JSON.parse(readFileSync("/tmp/options.json")),
);
let view = await runtime.inspect(
  JSON.parse(readFileSync("/tmp/binding.json")).ref,
  { signal: AbortSignal.timeout(10000), correlationId: "access" },
);
const origin = view.origin,
  host = new URL(origin).host;
let grant = new AbortController();
const context = () => ({
  signal: grant.signal,
  correlationId: "grant-lifetime",
});
const server = createServer(async (req, res) => {
  try {
    const c = await runtime.connect(view.ref, context());
    await c.forward(req, res);
  } catch (e) {
    res.writeHead(503);
    res.end(e.failure?.code ?? "error");
  }
});
server.on("upgrade", async (req, socket, head) => {
  try {
    const c = await runtime.connect(view.ref, context());
    await c.upgrade(req, socket, head);
  } catch {
    socket.destroy();
  }
});
await new Promise((r) => server.listen(9090, "127.0.0.1", r));
let cookie = "";
const call = (path, method = "GET", body) =>
  new Promise((resolve, reject) => {
    const encoded = body ? JSON.stringify(body) : undefined;
    const req = request(
      {
        hostname: "127.0.0.1",
        port: 9090,
        path,
        method,
        headers: {
          host,
          origin,
          cookie,
          ...(encoded
            ? {
                "content-type": "application/json",
                "content-length": Buffer.byteLength(encoded),
              }
            : {}),
        },
      },
      (res) => {
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () =>
          resolve({
            status: res.statusCode,
            headers: res.headers,
            body: Buffer.concat(chunks).toString(),
          }),
        );
      },
    );
    req.on("error", reject);
    req.end(encoded);
  });
const rpc = async (method, args) => {
  const res = await call("/api/" + method, "POST", {
    type: "client-request",
    rpcId: "b-" + Date.now(),
    method,
    payload: { args },
  });
  assert.equal(res.status, 200, res.body);
  const b = JSON.parse(res.body);
  assert.equal(b.result.ok, true, JSON.stringify(b.result.error));
  return b.result.value;
};
async function follow(sessionId) {
  const ws = new WebSocket("ws://127.0.0.1:9090/api/remote.mux", {
    headers: { host, origin, cookie },
  });
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(Error("WS timeout")), 10000);
    ws.on("error", reject);
    ws.on("open", () =>
      ws.send(
        JSON.stringify({
          type: "open",
          streamId: "b",
          endpoint: "session/follow",
          payload: {
            args: { request: { address: { kind: "session", sessionId } } },
          },
        }),
      ),
    );
    ws.on("message", (raw) => {
      const e = JSON.parse(raw);
      if (e.type === "item" && e.value?.type === "snapshot") {
        clearTimeout(timer);
        resolve();
      }
    });
  });
  return ws;
}
try {
  const bootstrap = await call("/");
  assert.equal(bootstrap.status, 303);
  assert.equal(bootstrap.headers["set-cookie"].length, 1);
  cookie = bootstrap.headers["set-cookie"][0].split(";")[0];
  const settings = await rpc("settings/describe", {});
  assert.equal(settings.writable, true);
  let sessionId;
  if (process.env.RESUME_SESSION === "1") {
    sessionId = JSON.parse(readFileSync("/tmp/session.json")).sessionId;
    const listed = await rpc("session/list", { _request: {} });
    assert.ok(listed.items.some((s) => s.sessionId === sessionId));
  } else {
    ({ sessionId } = await rpc("session/create", { request: {} }));
    writeFileSync("/tmp/session.json", JSON.stringify({ sessionId }));
  }
  let ws = await follow(sessionId);
  let closed = new Promise((r) => ws.once("close", r));
  grant.abort();
  await Promise.race([
    closed,
    new Promise((_, reject) =>
      setTimeout(() => reject(Error("revocation did not close WS")), 3000),
    ),
  ]);
  const denied = await call("/");
  assert.equal(denied.status, 503);
  console.log(
    JSON.stringify({
      name: "native-http-ws-revocation",
      sessionId,
      resumed: process.env.RESUME_SESSION === "1",
      closed: true,
    }),
  );
  grant = new AbortController();
  ws = await follow(sessionId);
  closed = new Promise((r) => ws.once("close", r));
  view = await runtime.inspect(view.ref, {
    signal: AbortSignal.timeout(10000),
    correlationId: "stop-access",
  });
  const stopped = await runtime.stop(view.ref, view.revision, {
    signal: AbortSignal.timeout(85000),
    correlationId: "stop-access",
  });
  assert.equal(stopped.view.state, "Stopped");
  await closed;
  console.log(
    JSON.stringify({ name: "stop-closes-native-ws", view: stopped.view }),
  );
  writeFileSync("/tmp/binding.json", JSON.stringify(stopped.view));
} finally {
  grant.abort();
  server.closeAllConnections();
  await new Promise((r) => server.close(r));
}
