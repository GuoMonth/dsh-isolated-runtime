import { request } from "node:https";
import { readFile } from "node:fs/promises";
import { RuntimeAccessError } from "./port.js";

export interface KubernetesOptions {
  readonly server: string;
  readonly caFile: string;
  readonly tokenFile: string;
}
/** GET-only, TLS-verified client; no kubeconfig exec plugins or write credentials exposed to callers. */
export class KubernetesReader {
  private readonly server: URL;
  constructor(private readonly options: KubernetesOptions) {
    this.server = new URL(options.server);
    if (
      this.server.protocol !== "https:" ||
      this.server.username ||
      this.server.password ||
      this.server.pathname !== "/" ||
      this.server.search ||
      this.server.hash
    )
      throw new Error("Invalid Kubernetes HTTPS server");
  }
  async get<T>(
    path: string,
    signal: AbortSignal,
    correlationId: string,
  ): Promise<T> {
    const fail = (code: "Forbidden" | "RecordMissing" | "ReadUnavailable") =>
      new RuntimeAccessError(
        code,
        correlationId,
        "Inspect the original binding and Kubernetes API availability; do not recreate the instance",
      );
    try {
      const [ca, token] = await Promise.all([
        readFile(this.options.caFile),
        readFile(this.options.tokenFile, "utf8"),
      ]);
      if (!token.trim() || /[\r\n]/.test(token.trim()))
        throw fail("ReadUnavailable");
      return await new Promise<T>((resolve, reject) => {
        const req = request(
          new URL(path, this.server),
          {
            method: "GET",
            ca,
            rejectUnauthorized: true,
            signal,
            headers: {
              authorization: `Bearer ${token.trim()}`,
              accept: "application/json",
            },
          },
          (res) => {
            if (res.statusCode !== 200) {
              res.resume();
              reject(
                fail(
                  res.statusCode === 404
                    ? "RecordMissing"
                    : res.statusCode === 401 || res.statusCode === 403
                      ? "Forbidden"
                      : "ReadUnavailable",
                ),
              );
              return;
            }
            const chunks: Buffer[] = [];
            let size = 0;
            res.on("data", (chunk: Buffer) => {
              size += chunk.length;
              if (size > 4 * 1024 * 1024) {
                res.destroy();
                reject(fail("ReadUnavailable"));
              } else chunks.push(chunk);
            });
            res.on("error", () => reject(fail("ReadUnavailable")));
            res.on("end", () => {
              try {
                resolve(JSON.parse(Buffer.concat(chunks).toString()) as T);
              } catch {
                reject(fail("ReadUnavailable"));
              }
            });
          },
        );
        req.on("error", () => reject(fail("ReadUnavailable")));
        req.end();
      });
    } catch (error) {
      if (error instanceof RuntimeAccessError) throw error;
      throw fail("ReadUnavailable");
    }
  }
}

/** A single bounded create. Anything after dispatch without a definitive rejection is unknown. */
export async function createResource<T>(
  options: KubernetesOptions,
  path: string,
  body: unknown,
  signal: AbortSignal,
  correlationId: string,
): Promise<T | undefined> {
  let dispatched = false;
  const fail = (
    code: "CreateOutcomeUnknown" | "CreateRejected" | "Forbidden",
  ) =>
    new RuntimeAccessError(
      code,
      correlationId,
      "Inspect the original allocation; never change its key or replay an unresolved create",
      {},
      {
        stage: "create",
        effect:
          dispatched && code === "CreateOutcomeUnknown"
            ? "unknown"
            : "not-submitted",
      },
    );
  try {
    const [ca, raw] = await Promise.all([
      readFile(options.caFile),
      readFile(options.tokenFile, "utf8"),
    ]);
    const token = raw.trim();
    if (!token || /[\r\n]/.test(token)) throw fail("CreateRejected");
    const data = JSON.stringify(body);
    signal.throwIfAborted();
    return await new Promise<T | undefined>((resolve, reject) => {
      const req = request(
        new URL(path, options.server),
        {
          method: "POST",
          ca,
          rejectUnauthorized: true,
          signal,
          headers: {
            authorization: `Bearer ${token}`,
            accept: "application/json",
            "content-type": "application/json",
            "content-length": Buffer.byteLength(data),
          },
        },
        (res) => {
          if (res.statusCode === 409) {
            res.resume();
            resolve(undefined);
            return;
          }
          if (res.statusCode !== 201) {
            res.resume();
            reject(
              fail(
                res.statusCode === 401 || res.statusCode === 403
                  ? "Forbidden"
                  : [400, 404, 405, 415, 422].includes(res.statusCode ?? 0)
                    ? "CreateRejected"
                    : "CreateOutcomeUnknown",
              ),
            );
            return;
          }
          const chunks: Buffer[] = [];
          let size = 0;
          res.on("data", (chunk: Buffer) => {
            size += chunk.length;
            if (size > 4 * 1024 * 1024) {
              res.destroy();
              reject(fail("CreateOutcomeUnknown"));
            } else chunks.push(chunk);
          });
          res.on("error", () => reject(fail("CreateOutcomeUnknown")));
          res.on("end", () => {
            try {
              resolve(JSON.parse(Buffer.concat(chunks).toString()) as T);
            } catch {
              reject(fail("CreateOutcomeUnknown"));
            }
          });
        },
      );
      req.on("error", () =>
        reject(fail(dispatched ? "CreateOutcomeUnknown" : "CreateRejected")),
      );
      dispatched = true;
      req.end(data);
    });
  } catch (error) {
    if (error instanceof RuntimeAccessError) throw error;
    throw fail(dispatched ? "CreateOutcomeUnknown" : "CreateRejected");
  }
}
