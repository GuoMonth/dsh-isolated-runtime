import { request } from 'node:https'
import { readFile } from 'node:fs/promises'
import type { EnvironmentRuntimeOptions } from './index.ts'
import { parseResource, parseResourceList, parseDeletion, parseWatchEvent, type Resource } from './parse.ts'
export type { Resource } from './parse.ts'
export class APIError extends Error {
  readonly code = 'KubernetesRequestFailed'
  constructor(
    readonly status: number,
    readonly submitted: boolean,
    options?: ErrorOptions,
  ) {
    super(`Kubernetes ${status || 'unavailable'}`, options)
    this.name = 'KubernetesAPIError'
  }
}
export class API {
  constructor(
    private readonly options: EnvironmentRuntimeOptions['kubernetes'],
  ) {
    const server = new URL(options.server)
    if (
      server.protocol !== 'https:' ||
      server.username ||
      server.password ||
      server.pathname !== '/' ||
      server.search ||
      server.hash
    )
      throw new Error('Invalid Kubernetes HTTPS endpoint')
  }
  private async auth() {
    const [ca, token] = await Promise.all([
      readFile(this.options.caFile),
      readFile(this.options.tokenFile, 'utf8'),
    ])
    if (!token.trim() || /[\r\n]/.test(token.trim()))
      throw new APIError(0, false)
    return {
      ca,
      headers: {
        authorization: `Bearer ${token.trim()}`,
        accept: 'application/json',
      },
    }
  }
  call(method: string, path: string, signal: AbortSignal, body?: unknown): Promise<Resource>
  call<T>(method: string, path: string, signal: AbortSignal, body: unknown, decode: (value: unknown) => T): Promise<T>
  async call<T>(
    method: string,
    path: string,
    signal: AbortSignal,
    body?: unknown,
    decode?: (value: unknown) => T,
  ): Promise<Resource | T> {
    let submitted = false
    try {
      signal.throwIfAborted()
      const auth = await this.auth()
      const encoded = body === undefined ? undefined : JSON.stringify(body)
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
                  'content-length': Buffer.byteLength(encoded ?? ''),
                  'content-type':
                      method === 'PATCH'
                        ? 'application/json-patch+json'
                        : 'application/json',
                }),
            },
          },
          (res) => {
            const chunks: Buffer[] = []
            let bytes = 0
            res.on('data', (chunk: Buffer) => {
              bytes += chunk.length
              chunks.push(chunk)
              if (bytes > 4 * 1024 * 1024)
                res.destroy(new Error('Response too large'))
            })
            res.on('error', (cause) => { reject(new APIError(0, submitted, { cause })) })
            res.on('end', () => {
              if ((res.statusCode ?? 500) >= 300)
                return reject(new APIError(res.statusCode ?? 0, submitted))
              try {
                const value: unknown = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(Buffer.concat(chunks)))
                resolve(decode ? decode(value) : parseResource(value))
              } catch (cause) {
                reject(new APIError(0, submitted, { cause }))
              }
            })
          },
        )
        req.on('error', (cause) => { reject(new APIError(0, submitted, { cause })) })
        signal.throwIfAborted()
        submitted = method !== 'GET'
        req.end(encoded)
      })
    } catch (error) {
      if (error instanceof APIError) throw error
      throw new APIError(0, submitted, { cause: error })
    }
  }
  list(path: string, signal: AbortSignal) {
    return this.call('GET', path, signal, undefined, parseResourceList)
  }
  remove(path: string, signal: AbortSignal, body: unknown) {
    return this.call('DELETE', path, signal, body, parseDeletion)
  }
  async maybe(path: string, signal: AbortSignal) {
    try {
      return await this.call('GET', path, signal)
    } catch (e) {
      if (e instanceof APIError && e.status === 404) return null
      throw e
    }
  }
  /** A single bounded watch from the pre-stop Pod list RV, never an infinite reconnect. */
  async terminal(
    path: string,
    uid: string,
    signal: AbortSignal,
  ): Promise<void> {
    const auth = await this.auth()
    return new Promise((resolve, reject) => {
      const req = request(
        new URL(path, this.options.server),
        {
          ...auth,
          signal: AbortSignal.any([signal, AbortSignal.timeout(55_000)]),
        },
        (res) => {
          if (res.statusCode !== 200) {
            res.resume()
            reject(new APIError(res.statusCode ?? 0, false))
            return
          }
          let buffer = '',
            done = false
          const decoder = new TextDecoder('utf-8', { fatal: true })
          res.on('data', (chunk: Buffer) => {
            try {
              buffer += decoder.decode(chunk, { stream: true })
            } catch (cause) {
              res.destroy(new APIError(0, false, { cause }))
              return
            }
            if (buffer.length > 4 * 1024 * 1024) {
              res.destroy()
              return
            }
            for (
              let newline = buffer.indexOf('\n');
              newline >= 0;
              newline = buffer.indexOf('\n')
            ) {
              const line = buffer.slice(0, newline)
              buffer = buffer.slice(newline + 1)
              try {
                const value: unknown = JSON.parse(line)
                const event = parseWatchEvent(value)
                if (event.type === 'ERROR') throw new Error('Watch expired')
                if (isTerminal(event.object, uid)) {
                  done = true
                  resolve()
                  res.destroy()
                  return
                }
              } catch (cause) {
                res.destroy(new APIError(0, false, { cause }))
                return
              }
            }
          })
          const fail = (cause?: unknown) => {
            if (!done) reject(new APIError(0, false, { cause }))
          }
          res.on('error', fail)
          res.on('end', fail)
          res.on('close', fail)
        },
      )
      req.on('error', (cause) => { reject(new APIError(0, false, { cause })) })
      req.end()
    })
  }
}
export function isTerminal(pod: Resource | undefined, uid: string): boolean {
  const states = pod?.status?.containerStatuses
  return (
    pod?.metadata.uid === uid &&
    Array.isArray(states) &&
    states.length === 1 &&
    states[0]?.name === 'dsh' &&
    states[0].state.terminated?.exitCode === 0 &&
    pod.status?.phase === 'Succeeded'
  )
}
