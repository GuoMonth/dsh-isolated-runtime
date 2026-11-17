import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { createRequire } from 'node:module'
import { test } from 'node:test'

const require = createRequire(new URL('../../images/environment/package.json', import.meta.url))
const { StreamableHTTPClientTransport } = require('@modelcontextprotocol/client')

async function listen(t, handler) {
  const server = createServer(handler)
  await new Promise((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', resolve)
  })
  t.after(() => new Promise((resolve, reject) => {
    server.closeAllConnections()
    server.close(error => error ? reject(error) : resolve())
  }))
  return new URL(`http://127.0.0.1:${server.address().port}/mcp`)
}

test('MCP SDK preserves same-origin RPC', async (t) => {
  const url = await listen(t, (request, response) => {
    assert.equal(request.headers['x-fixture-credential'], 'synthetic')
    response.writeHead(200, { 'content-type': 'application/json' })
    response.end(JSON.stringify({ jsonrpc: '2.0', id: 1, result: {} }))
  })
  const transport = new StreamableHTTPClientTransport(url, {
    requestInit: { headers: { 'x-fixture-credential': 'synthetic' } },
  })
  t.after(() => transport.close())
  const received = []
  transport.onmessage = message => received.push(message)
  await transport.start()
  await transport.send({ jsonrpc: '2.0', id: 1, method: 'ping' })
  assert.deepEqual(received, [{ jsonrpc: '2.0', id: 1, result: {} }])
})

test('MCP SDK rejects cross-origin redirect before forwarding headers or bodies', async (t) => {
  let foreignRequests = 0
  const foreign = await listen(t, (_request, response) => {
    foreignRequests++
    response.end()
  })
  let originRequests = 0
  const origin = await listen(t, (request, response) => {
    originRequests++
    assert.equal(request.headers['x-fixture-credential'], 'synthetic')
    response.writeHead(307, { location: foreign.href })
    response.end()
  })
  const transport = new StreamableHTTPClientTransport(origin, {
    requestInit: { headers: { 'x-fixture-credential': 'synthetic' } },
  })
  t.after(() => transport.close())
  await transport.start()
  await assert.rejects(transport.send({ jsonrpc: '2.0', id: 1, method: 'ping' }))
  assert.equal(originRequests, 1)
  assert.equal(foreignRequests, 0)
})
