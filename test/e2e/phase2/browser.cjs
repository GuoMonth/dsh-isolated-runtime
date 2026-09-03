const fs = require("node:fs");
const { chromium } = require("playwright");

const mode = process.env.MODE;
const cellA = process.env.CELL_A_URL;
const cellB = process.env.CELL_B_URL;
const storageFile = "/work/storage.json";
const sessionFile = "/work/session.json";

function requireEnv(name) {
  if (!process.env[name]) throw new Error(`${name} is required`);
  return process.env[name];
}

async function cookieEvidence(page, target) {
  return (await page.context().cookies(target)).map(({ name, domain, path, secure, sameSite }) => ({
    name, domain, path, secure, sameSite,
  }));
}

async function rpc(page, method, args) {
  return page.evaluate(async ({ method, args }) => {
    const response = await fetch(`/api/${method}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        type: "client-request",
        rpcId: `phase2-${method.replaceAll("/", "-")}`,
        method,
        payload: { args },
      }),
    });
    if (!response.ok) throw new Error(`${method} status ${response.status}`);
    const body = await response.json();
    if (!body.result?.ok) throw new Error(`${method}: ${JSON.stringify(body.result?.error)}`);
    return body.result.value;
  }, { method, args });
}

async function follow(page, sessionId) {
  await page.evaluate((sessionId) => new Promise((resolve, reject) => {
    const socket = new WebSocket(`wss://${location.host}/api/remote.mux`);
    const timeout = setTimeout(() => reject(new Error("session/follow timeout")), 15000);
    socket.onopen = () => socket.send(JSON.stringify({
      type: "open",
      streamId: "phase2-follow",
      endpoint: "session/follow",
      payload: { args: { request: { address: { kind: "session", sessionId } } } },
    }));
    socket.onerror = () => reject(new Error("session/follow websocket failed"));
    socket.onmessage = (event) => {
      const frame = JSON.parse(event.data);
      if (frame.streamId !== "phase2-follow") return;
      if (frame.type === "item" && frame.value?.type === "snapshot") {
        clearTimeout(timeout);
        socket.close();
        resolve();
      } else if (frame.type === "error" || frame.type === "end") {
        reject(new Error(`session/follow ended: ${event.data}`));
      }
    };
  }), sessionId);
}

async function protocol(page, existingSession) {
  await rpc(page, "settings/describe", {});
  let sessionId = existingSession;
  if (!sessionId) {
    const created = await rpc(page, "session/create", { request: {} });
    sessionId = created.sessionId;
    if (!sessionId) throw new Error("session/create returned no id");
  }
  const listed = await rpc(page, "session/list", { _request: {} });
  if (!listed.items.some((item) => item.sessionId === sessionId)) throw new Error("durable session was not listed");
  await follow(page, sessionId);
  for (const method of ["HEAD", "GET"]) {
    const response = await page.evaluate(async ({ sessionId, method }) => {
      const result = await fetch(`/api/session.export?sessionId=${encodeURIComponent(sessionId)}`, { method });
      return result.status;
    }, { sessionId, method });
    if (response !== 200) throw new Error(`session.export ${method} status ${response}`);
  }
  return sessionId;
}

async function login(page, target, username) {
  const seen = [];
  const cellHops = [];
  const cellRequests = [];
  page.on("request", (request) => {
    seen.push(request.url());
    const url = new URL(request.url());
    if (!url.hostname.endsWith(".cells.test")) return;
    cellRequests.push(request.headerValue("cookie")
      .then((cookies) => ({
        host: url.hostname,
        path: url.pathname,
        cookieNames: (cookies || "").split(";").map((cookie) => cookie.split("=", 1)[0].trim()).filter(Boolean),
      }))
      .catch(() => ({ host: url.hostname, path: url.pathname, cookieNames: ["<unavailable>"] })));
  });
  page.on("response", async (response) => {
    try {
      const url = new URL(response.url());
      if (!url.hostname.endsWith(".cells.test")) return;
      const headers = await response.allHeaders();
      const location = headers.location ? new URL(headers.location, response.url()).pathname : "";
      const cookieNames = (headers["set-cookie"] || "")
        .split(/,(?=[^;,]+=)/)
        .map((cookie) => cookie.slice(0, cookie.indexOf("=")).trim())
        .filter(Boolean);
      cellHops.push({ host: url.hostname, path: url.pathname, status: response.status(), location, cookieNames });
    } catch {
      // Diagnostic collection must never perturb the browser contract test.
    }
  });
  let response;
  try {
    response = await page.goto(target, { waitUntil: "domcontentloaded", timeout: 90000 });
  } catch (error) {
    throw new Error(`${error.message}; Cell redirect evidence: ${JSON.stringify(cellHops.slice(-12))}; request cookies: ${JSON.stringify((await Promise.all(cellRequests)).slice(-12))}; stored cookies: ${JSON.stringify(await cookieEvidence(page, target))}`);
  }
  if (page.url().includes("dex.dsh-system.svc")) {
    await page.locator('input[name="login"]').fill(username);
    await page.locator('input[name="password"]').fill("password");
    try {
      await page.getByRole("button", { name: /login/i }).click();
      await page.waitForURL((url) => url.href.startsWith(target), { timeout: 90000 });
    } catch (error) {
      throw new Error(`${error.message}; Cell redirect evidence: ${JSON.stringify(cellHops.slice(-12))}; request cookies: ${JSON.stringify((await Promise.all(cellRequests)).slice(-12))}; stored cookies: ${JSON.stringify(await cookieEvidence(page, target))}`);
    }
  } else if (!response || response.status() >= 400) {
    throw new Error(`initial navigation status ${response?.status()}`);
  }
  await page.waitForLoadState("domcontentloaded");
  if (!page.url().startsWith(target)) throw new Error(`unexpected final URL ${page.url()}`);
  if (seen.some((url) => /[?&]token=/.test(url))) throw new Error("DSH launch token entered a browser-visible URL");
  const bodyLength = await page.evaluate(() => document.body.textContent.length);
  if (bodyLength === 0) throw new Error("DSH UI rendered an empty document");
}

async function expectStatus(context, target, wanted) {
  const external = new URL(target);
  const direct = new URL(target);
  direct.hostname = "127.0.0.1";
  const cookies = await context.cookies(external.href);
  const response = await context.request.get(direct.href, {
    headers: {
      Host: external.host,
      Cookie: cookies.map(({ name, value }) => `${name}=${value}`).join("; "),
      "Sec-Fetch-Dest": "empty",
    },
    maxRedirects: 0,
    failOnStatusCode: false,
  });
  if (response.status() !== wanted) throw new Error(`${target} status ${response.status()}, want ${wanted}`);
}

async function main() {
  requireEnv("MODE");
  requireEnv("CELL_A_URL");
  requireEnv("CELL_B_URL");
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CHROME_EXECUTABLE || undefined,
    args: ["--host-resolver-rules=MAP *.cells.test 127.0.0.1,MAP dex.dsh-system.svc 127.0.0.1"],
  });
  try {
    const context = await browser.newContext({
      ignoreHTTPSErrors: true,
      storageState: fs.existsSync(storageFile) && mode !== "initial" && mode !== "group" ? storageFile : undefined,
    });
    const page = await context.newPage();
    if (mode === "initial") {
      await login(page, cellA, requireEnv("USERNAME"));
      const sessionId = await protocol(page);
      fs.writeFileSync(sessionFile, JSON.stringify({ sessionId }), { mode: 0o600 });
      await expectStatus(context, cellB, 403);
      const wrongOriginURL = new URL(`${cellA}/api/settings/describe`);
      const wrongOriginAuthority = wrongOriginURL.host;
      wrongOriginURL.hostname = "127.0.0.1";
      const cellACookies = await context.cookies(cellA);
      const wrongOrigin = await context.request.post(wrongOriginURL.href, {
        headers: {
          Host: wrongOriginAuthority,
          Origin: "https://evil.example",
          Cookie: cellACookies.map(({ name, value }) => `${name}=${value}`).join("; "),
          "content-type": "application/json",
        },
        data: { type: "client-request", rpcId: "wrong-origin", method: "settings/describe", payload: { args: {} } },
        failOnStatusCode: false,
      });
      if (wrongOrigin.status() < 400) throw new Error("wrong Origin was accepted");
      const anonymous = await browser.newContext({ ignoreHTTPSErrors: true });
      try {
        const external = new URL(cellA);
        const direct = new URL(`${cellA}/api/settings/describe`);
        direct.hostname = "127.0.0.1";
        const forged = await anonymous.request.get(direct.href, {
          headers: { Host: external.host, "X-Dsh-Oidc-Token": "forged" },
          maxRedirects: 0,
          failOnStatusCode: false,
        });
        if (forged.status() < 400) throw new Error("forged OIDC header was accepted");
      } finally {
        await anonymous.close();
      }
      await context.storageState({ path: storageFile });
    } else if (mode === "grant") {
      await login(page, cellB, requireEnv("USERNAME"));
      await rpc(page, "settings/describe", {});
      await context.storageState({ path: storageFile });
    } else if (mode === "deny") {
      await expectStatus(context, cellB, 403);
    } else if (mode === "resume") {
      await login(page, cellA, requireEnv("USERNAME"));
      const state = JSON.parse(fs.readFileSync(sessionFile, "utf8"));
      await protocol(page, state.sessionId);
    } else if (mode === "group") {
      await login(page, cellB, requireEnv("USERNAME"));
      await rpc(page, "settings/describe", {});
    } else if (mode === "status") {
      await expectStatus(context, requireEnv("EXPECT_URL"), Number(requireEnv("EXPECT_STATUS")));
    } else {
      throw new Error(`unknown MODE ${mode}`);
    }
    await context.close();
  } finally {
    await browser.close();
  }
  process.stdout.write(`Phase 2 browser mode ${mode} passed\n`);
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exit(1);
});
