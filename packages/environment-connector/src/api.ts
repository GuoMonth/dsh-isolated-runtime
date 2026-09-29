import { request } from "node:https";
import { readFile } from "node:fs/promises";
import type { EnvironmentRuntimeOptions } from "./index.js";

// Internal Kubernetes JSON only. No untrusted object is returned as a product view.
export interface Resource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    uid: string;
    resourceVersion: string;
    generation?: number;
    deletionTimestamp?: string;
    annotations?: Record<string, string>;
    labels?: Record<string, string>;
    ownerReferences?: {
      apiVersion: string;
      kind: string;
      name: string;
      uid: string;
      controller?: boolean;
    }[];
  };
  spec?: any;
  status?: any;
  items?: Resource[];
  [key: string]: any;
}
export class APIError extends Error {
  constructor(
    readonly status: number,
    readonly submitted: boolean,
  ) {
    super(`Kubernetes ${status || "unavailable"}`);
  }
}
export class API {
  constructor(
    private readonly options: EnvironmentRuntimeOptions["kubernetes"],
  ) {
    const server = new URL(options.server);
    if (
      server.protocol !== "https:" ||
      server.username ||
      server.password ||
      server.pathname !== "/" ||
      server.search ||
      server.hash
    )
      throw new Error("Invalid Kubernetes HTTPS endpoint");
  }
  private async auth() {
    const [ca, token] = await Promise.all([
      readFile(this.options.caFile),
      readFile(this.options.tokenFile, "utf8"),
    ]);
    if (!token.trim() || /[\r\n]/.test(token.trim()))
      throw new APIError(0, false);
    return {
      ca,
      headers: {
        authorization: `Bearer ${token.trim()}`,
        accept: "application/json",
      },
    };
  }
  async call(
    method: string,
    path: string,
    signal: AbortSignal,
    body?: unknown,
  ): Promise<Resource> {
    let submitted = false;
    try {
      const auth = await this.auth();
      const encoded = body === undefined ? undefined : JSON.stringify(body);
      return await new Promise((resolve, reject) => {
        const req = request(
          new URL(path, this.options.server),
          {
            ...auth,
            method,
            signal: AbortSignal.any([signal, AbortSignal.timeout(20_000)]),
            headers: {
              ...auth.headers,
              ...(body === undefined
                ? {}
                : {
                    "content-length": Buffer.byteLength(encoded!),
                    "content-type":
                      method === "PATCH"
                        ? "application/json-patch+json"
                        : "application/json",
                  }),
            },
          },
          (res) => {
            let data = "";
            res.on("data", (chunk) => {
              data += chunk;
              if (data.length > 4 * 1024 * 1024)
                res.destroy(new Error("Response too large"));
            });
            res.on("error", () => reject(new APIError(0, submitted)));
            res.on("end", () => {
              if ((res.statusCode ?? 500) >= 300)
                return reject(new APIError(res.statusCode ?? 0, submitted));
              try {
                resolve(JSON.parse(data) as Resource);
              } catch {
                reject(new APIError(0, submitted));
              }
            });
          },
        );
        req.on("error", () => reject(new APIError(0, submitted)));
        signal.throwIfAborted();
        submitted = method !== "GET";
        req.end(encoded);
      });
    } catch (error) {
      if (error instanceof APIError) throw error;
      throw new APIError(0, submitted);
    }
  }
  async maybe(path: string, signal: AbortSignal) {
    try {
      return await this.call("GET", path, signal);
    } catch (e) {
      if (e instanceof APIError && e.status === 404) return null;
      throw e;
    }
  }
  /** A single bounded watch from the pre-stop Pod list RV, never an infinite reconnect. */
  async terminal(
    path: string,
    uid: string,
    signal: AbortSignal,
  ): Promise<void> {
    const auth = await this.auth();
    return new Promise((resolve, reject) => {
      const req = request(
        new URL(path, this.options.server),
        {
          ...auth,
          signal: AbortSignal.any([signal, AbortSignal.timeout(55_000)]),
        },
        (res) => {
          if (res.statusCode !== 200) {
            res.resume();
            reject(new APIError(res.statusCode ?? 0, false));
            return;
          }
          let buffer = "",
            done = false;
          res.on("data", (chunk) => {
            buffer += chunk;
            if (buffer.length > 4 * 1024 * 1024) {
              res.destroy();
              return;
            }
            for (
              let newline = buffer.indexOf("\n");
              newline >= 0;
              newline = buffer.indexOf("\n")
            ) {
              const line = buffer.slice(0, newline);
              buffer = buffer.slice(newline + 1);
              try {
                const event = JSON.parse(line);
                if (event.type === "ERROR") throw new Error("Watch expired");
                if (isTerminal(event.object, uid)) {
                  done = true;
                  resolve();
                  res.destroy();
                  return;
                }
              } catch {
                res.destroy();
                return;
              }
            }
          });
          const fail = () => {
            if (!done) reject(new APIError(0, false));
          };
          res.on("error", fail);
          res.on("end", fail);
          res.on("close", fail);
        },
      );
      req.on("error", () => reject(new APIError(0, false)));
      req.end();
    });
  }
}
export function isTerminal(pod: Resource | undefined, uid: string): boolean {
  const states = pod?.status?.containerStatuses;
  return (
    pod?.metadata?.uid === uid &&
    Array.isArray(states) &&
    states.length === 1 &&
    states[0]?.name === "dsh" &&
    states[0]?.state?.terminated?.exitCode === 0 &&
    pod?.status?.phase === "Succeeded"
  );
}
