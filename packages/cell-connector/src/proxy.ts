import {
  request as httpRequest,
  type ClientRequest,
  type IncomingMessage,
  type OutgoingHttpHeaders,
  type ServerResponse,
} from "node:http";
import type { Duplex } from "node:stream";
import {
  RuntimeAccessError,
  type AccessContext,
  type Connector,
} from "./port.js";

const hop = new Set([
  "connection",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);
function clean(input: IncomingMessage["headers"]): OutgoingHttpHeaders {
  const removed = new Set([
    ...hop,
    ...String(input.connection ?? "")
      .split(",")
      .map((x) => x.trim().toLowerCase()),
  ]);
  return Object.fromEntries(
    Object.entries(input).filter(
      ([name, value]) => value !== undefined && !removed.has(name),
    ),
  );
}
const nativeCookie = (name: string) => /^dsh-auth-[A-Za-z0-9_-]+$/.test(name);
function nativeCookies(input: string | undefined): string {
  return (input ?? "")
    .split(";")
    .map((x) => x.trim())
    .filter((x) => {
      const i = x.indexOf("=");
      return i > 0 && nativeCookie(x.slice(0, i)) && !/[\r\n]/.test(x);
    })
    .join("; ");
}
function responseHeaders(
  input: IncomingMessage["headers"],
): OutgoingHttpHeaders {
  const result = clean(input);
  const cookies = (input["set-cookie"] ?? []).filter(
    (raw) =>
      nativeCookie(raw.split("=", 1)[0] ?? "") &&
      !/;\s*domain\s*=/i.test(raw) &&
      /;\s*secure(?:;|$)/i.test(raw) &&
      /;\s*httponly(?:;|$)/i.test(raw),
  );
  delete result["set-cookie"];
  if (cookies.length) result["set-cookie"] = cookies;
  result["cache-control"] = "no-store";
  result["content-security-policy"] = [
    result["content-security-policy"],
    "frame-ancestors 'self'",
  ]
    .filter(Boolean)
    .join(", ");
  result["x-frame-options"] = "SAMEORIGIN";
  return result;
}

interface HandshakeTimeouts {
  readonly connectMs: number;
  readonly headersMs: number;
}
const handshakeTimeouts: HandshakeTimeouts = {
  connectMs: 5_000,
  headersMs: 30_000,
};

// Only bound establishment. Never impose a total duration on an established
// model stream or WebSocket. A timed-out write still has an unknown outcome.
function boundHandshake(request: ClientRequest, limits: HandshakeTimeouts) {
  let connected: ReturnType<typeof setTimeout> | undefined;
  let headers: ReturnType<typeof setTimeout> | undefined;
  let completed = false;
  const stop = () => {
    completed = true;
    clearTimeout(connected);
    clearTimeout(headers);
  };
  request.once("socket", (socket) => {
    if (completed || !socket.connecting) return;
    connected = setTimeout(() => request.destroy(new Error("UpstreamConnectTimeout")), limits.connectMs);
    connected.unref();
    socket.once("connect", () => clearTimeout(connected));
  });
  request.once("finish", () => {
    if (completed) return;
    headers = setTimeout(() => request.destroy(new Error("UpstreamHeadersTimeout")), limits.headersMs);
    headers.unref();
  });
  request.once("response", stop);
  request.once("upgrade", stop);
  request.once("error", stop);
  request.once("close", stop);
}

/** All transport state stays inside runtime. No address, token or resource stop handle escapes. */
export function connector(
  origin: string,
  context: AccessContext,
  resolve: () => Promise<string>,
  limits: HandshakeTimeouts = handshakeTimeouts,
): Connector {
  if (![limits.connectMs, limits.headersMs].every((value) => Number.isSafeInteger(value) && value > 0))
    throw new Error("Invalid internal handshake timeout");
  let consumed = false;
  async function prepare(request: IncomingMessage, websocket: boolean) {
    if (consumed)
      throw new RuntimeAccessError(
        "AccessRejected",
        context.correlationId,
        "Acquire a new connection for each request",
      );
    consumed = true;
    context.signal.throwIfAborted();
    const external = new URL(origin);
    if (
      !request.url?.startsWith("/") ||
      request.url.startsWith("//") ||
      /[\r\n]/.test(request.url) ||
      request.headers.host !== external.host ||
      request.headers["sec-fetch-site"] === "cross-site" ||
      (request.headers.origin !== undefined &&
        request.headers.origin !== origin) ||
      ((websocket || !["GET", "HEAD"].includes(request.method ?? "")) &&
        request.headers.origin !== origin)
    )
      throw new RuntimeAccessError(
        "AccessRejected",
        context.correlationId,
        "Use the configured application origin and an authorized session",
      );
    const address = await resolve();
    context.signal.throwIfAborted();
    const headers = clean(request.headers);
    for (const name of Object.keys(headers)) {
      if (
        ["cookie", "authorization", "host", "origin", "forwarded"].includes(
          name,
        ) ||
        name.startsWith("x-")
      )
        delete headers[name];
    }
    headers.host = external.host;
    if (request.headers.origin) headers.origin = origin;
    const cookies = nativeCookies(request.headers.cookie);
    if (cookies) headers.cookie = cookies;
    if (websocket) {
      headers.connection = "Upgrade";
      headers.upgrade = "websocket";
    }
    return { address, headers };
  }
  return {
    origin,
    async forward(request: IncomingMessage, response: ServerResponse) {
      const { address, headers } = await prepare(request, false);
      context.signal.throwIfAborted();
      const upstream = httpRequest({
        hostname: address,
        port: 8080,
        path: request.url,
        method: request.method,
        headers,
        signal: context.signal,
      });
      boundHandshake(upstream, limits);
      const abort = () => {
        upstream.destroy();
        response.destroy();
      };
      context.signal.addEventListener("abort", abort, { once: true });
      response.once("close", () => {
        context.signal.removeEventListener("abort", abort);
        upstream.destroy();
      });
      upstream.once("response", (received) => {
        if (context.signal.aborted || response.destroyed) {
          received.destroy();
          return;
        }
        response.writeHead(
          received.statusCode ?? 502,
          responseHeaders(received.headers),
        );
        received.once("error", () => response.destroy());
        received.pipe(response);
      });
      upstream.once("error", () => {
        if (response.destroyed) return;
        if (response.headersSent) response.destroy();
        else {
          response.writeHead(502, {
            "cache-control": "no-store",
            "content-type": "application/json",
          });
          response.end(
            JSON.stringify(
              new RuntimeAccessError(
                "ForwardOutcomeUnknown",
                context.correlationId,
                "Inspect the original application operation; do not automatically replay it",
              ),
            ),
          );
        }
      });
      request.once("error", () => upstream.destroy());
      request.pipe(upstream);
    },
    async upgrade(request: IncomingMessage, socket: Duplex, head: Buffer) {
      const { address, headers } = await prepare(request, true);
      context.signal.throwIfAborted();
      const upstream = httpRequest({
        hostname: address,
        port: 8080,
        path: request.url,
        method: "GET",
        headers,
        signal: context.signal,
      });
      boundHandshake(upstream, limits);
      const abort = () => {
        upstream.destroy();
        socket.destroy();
      };
      context.signal.addEventListener("abort", abort, { once: true });
      socket.once("close", () => {
        context.signal.removeEventListener("abort", abort);
        upstream.destroy();
      });
      upstream.once("upgrade", (response, target, upstreamHead) => {
        if (context.signal.aborted || socket.destroyed) {
          target.destroy();
          return;
        }
        const outgoing = responseHeaders(response.headers);
        outgoing.connection = "Upgrade";
        outgoing.upgrade = "websocket";
        const lines = Object.entries(outgoing).flatMap(([name, value]) =>
          (Array.isArray(value) ? value : [value]).map(
            (v) => `${name}: ${String(v)}`,
          ),
        );
        socket.write(
          `HTTP/1.1 101 Switching Protocols\r\n${lines.join("\r\n")}\r\n\r\n`,
        );
        const cancelTarget = () => target.destroy();
        context.signal.addEventListener("abort", cancelTarget, { once: true });
        target.once("close", () => {
          context.signal.removeEventListener("abort", cancelTarget);
          socket.destroy();
        });
        target.on("error", () => socket.destroy());
        socket.once("close", () => target.destroy());
        if (upstreamHead.length) socket.write(upstreamHead);
        if (head.length) target.write(head);
        socket.pipe(target).pipe(socket);
      });
      upstream.once("response", (response) => {
        response.resume();
        socket.end(
          `HTTP/1.1 ${response.statusCode ?? 502} Rejected\r\nConnection: close\r\nContent-Length: 0\r\n\r\n`,
        );
      });
      upstream.once("error", () => socket.destroy());
      upstream.end();
    },
  };
}
