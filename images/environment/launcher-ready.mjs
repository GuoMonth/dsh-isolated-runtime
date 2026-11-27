import { closeSync, writeFileSync } from 'node:fs'

export const name = 'environment-launcher-ready'
export const inject = ['webServer', 'connection']

export function apply(ctx) {
  if (process.env.DSH_LAUNCHER_READY_FD !== '3') return
  const fd = 3

  function report(token) {
    if (process.env.DSH_LAUNCHER_READY_FD !== '3') return
    delete process.env.DSH_LAUNCHER_READY_FD
    if (!token) throw new Error('DSH readiness requires a launch token')
    const listener = new URL(ctx.webServer.getOrigin())
    listener.searchParams.set('token', token)
    try {
      writeFileSync(fd, listener.href + '\n')
    } finally {
      closeSync(fd)
    }
  }

  // RC.5 provides the token through Connection. Newer Web profiles contribute it to the entry event.
  if (typeof ctx.connection.authenticatedUrl === 'function') {
    function ready() {
      const entry = new URL(ctx.connection.authenticatedUrl(ctx.webServer.getOrigin()))
      report(entry.searchParams.get('token'))
    }
    const settled = ctx.get('loader')?.await()
    if (settled === undefined) ready()
    else void settled.then(ready)
    return
  }

  ctx.on('webserver/entry-url', (_root, entries) => {
    queueMicrotask(() => {
      const entry = entries.find(candidate => candidate.label === 'dsh web')
      if (!entry) throw new Error('DSH readiness requires the native web entry')
      report(new URL(entry.url).searchParams.get('token'))
    })
  })
}
