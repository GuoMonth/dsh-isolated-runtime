import test from "node:test";
import assert from "node:assert/strict";
import { createServer, get } from "node:http";
import { connect } from "node:net";
import { once } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { connector } from "../dist/proxy.js";

const origin = "https://cell.cells.test";
const upstreamAddress = "127.0.0.29";
const limits = { connectMs: 100, headersMs: 100 };
async function fixture(t, handler, upgrade) {
  const controller = new AbortController();
  const sockets = new Set();
  const upstream = createServer(handler);
  const track = (socket) => {
    sockets.add(socket);
    socket.on("error", () => {});
    socket.once("close", () => sockets.delete(socket));
  };
  upstream.on("connection", track);
  if (upgrade) upstream.on("upgrade", (req, socket, head) => {
    socket.resume();
    socket.once("end", () => socket.end());
    upgrade(req, socket, head);
  });
  const ingress = createServer((req, res) => {
    void channel().forward(req, res).catch(() => res.destroy());
  });
  function channel() {
    return connector(origin, { signal: controller.signal, correlationId: "deadline-proof" }, async () => upstreamAddress, limits);
  }
  ingress.on("connection", track);
  ingress.on("upgrade", (req, socket, head) => {
    void channel().upgrade(req, socket, head).catch(() => socket.destroy());
  });
  t.after(() => {
    controller.abort();
    for (const socket of sockets) socket.destroy();
    ingress.close();
    upstream.close();
  });
  upstream.listen(8080, upstreamAddress);
  await once(upstream, "listening");
  ingress.listen(0, "127.0.0.1");
  await once(ingress, "listening");
  return { port: ingress.address().port, controller };
}
function request(port) {
  return get({ hostname: "127.0.0.1", port, path: "/", headers: { host: "cell.cells.test" } });
}
async function ws(port) {
  const socket = connect(port, "127.0.0.1");
  socket.on("error", () => {});
  await once(socket, "connect");
  socket.write("GET / HTTP/1.1\r\nHost: cell.cells.test\r\nOrigin: https://cell.cells.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n");
  return socket;
}

test("silent HTTP upstream has a bounded header wait and unknown write outcome", { timeout: 3000 }, async (t) => {
  let closed;
  const close = new Promise(resolve => { closed = resolve; });
  const { port } = await fixture(t, (req) => { req.socket.once("close", closed); });
  const [response] = await once(request(port), "response");
  let body = "";
  for await (const chunk of response) body += chunk;
  assert.equal(response.statusCode, 502);
  const error = JSON.parse(body);
  assert.equal(error.code, "ForwardOutcomeUnknown");
  assert.equal(error.effect, "unknown");
  assert.equal(error.correlationId, "deadline-proof");
  await close;
});

test("late Upgrade is closed without a handshake or leaked upstream socket", { timeout: 3000 }, async (t) => {
  let target;
  let accepted;
  const ready = new Promise(resolve => { accepted = resolve; });
  const { port } = await fixture(t, undefined, (_req, socket) => { target = socket; accepted(); });
  const client = await ws(port);
  let received = "";
  client.on("data", chunk => { received += chunk; });
  const closed = once(client, "close");
  await ready;
  const targetClosed = once(target, "close");
  await Promise.all([closed, targetClosed]);
  assert.equal(received, "");
  assert.equal(target.destroyed, true);
});

test("HTTP model stream remains open beyond the handshake limit and cancels both ends", { timeout: 3000 }, async (t) => {
  let closed;
  const close = new Promise(resolve => { closed = resolve; });
  const { port, controller } = await fixture(t, (req, res) => {
    req.socket.once("close", closed);
    res.writeHead(200, { "content-type": "text/event-stream" });
    res.write("data: ready\n\n");
  });
  const [response] = await once(request(port), "response");
  response.on("error", () => {});
  response.resume();
  await delay(limits.headersMs * 3);
  assert.equal(response.destroyed, false);
  const aborted = once(response, "aborted");
  controller.abort();
  await Promise.all([aborted, close]);
});

test("established Upgrade stays open past the handshake limit and revocation closes it", { timeout: 3000 }, async (t) => {
  let target;
  const { port, controller } = await fixture(t, undefined, (_req, socket) => {
    target = socket;
    socket.write("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n");
  });
  const client = await ws(port);
  const [handshake] = await once(client, "data");
  assert.match(handshake.toString(), /101 Switching Protocols/);
  await delay(limits.headersMs * 3);
  assert.equal(client.destroyed, false);
  assert.equal(target.destroyed, false);
  const closed = Promise.all([once(client, "close"), once(target, "close")]);
  controller.abort();
  await closed;
});
