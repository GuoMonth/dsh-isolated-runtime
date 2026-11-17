import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { createServer } from 'node:https'
import { API } from '../dist/api.js'
test('DELETE sends UID/RV preconditions as a framed body, preserving next keep-alive request', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'environment-api-'))
  execFileSync(
    'openssl',
    [
      'req',
      '-x509',
      '-newkey',
      'rsa:2048',
      '-nodes',
      '-keyout',
      join(dir, 'key'),
      '-out',
      join(dir, 'cert'),
      '-days',
      '1',
      '-subj',
      '/CN=localhost',
      '-addext',
      'subjectAltName=DNS:localhost',
    ],
    { stdio: 'ignore' },
  )
  writeFileSync(join(dir, 'token'), 'test-only-noncredential')
  let received
  const server = createServer(
    {
      key: readFileSync(join(dir, 'key')),
      cert: readFileSync(join(dir, 'cert')),
    },
    (req, res) => {
      const chunks = []
      req.on('data', c => chunks.push(c))
      req.on('end', () => {
        if (req.method === 'DELETE') {
          const body = Buffer.concat(chunks)
          assert.equal(Number(req.headers['content-length']), body.length)
          received = JSON.parse(body)
        }
        if (req.url === '/malformed') return res.end('{')
        if (req.url === '/watch') {
          return res.end(JSON.stringify({ type: 'MODIFIED', object: { metadata: {} } }) + '\n')
        }
        if (req.method === 'DELETE') return res.end(JSON.stringify({ apiVersion: 'v1', kind: 'Status', status: 'Success' }))
        const encoded = Buffer.from(JSON.stringify({ apiVersion: 'v1', kind: 'Pod', metadata: {
          name: 'test', uid: 'exact-writer', resourceVersion: '23', annotations: { text: '汉字' },
        } }))
        const offset = encoded.indexOf(Buffer.from('汉')) + 1
        res.write(encoded.subarray(0, offset))
        setImmediate(() => res.end(encoded.subarray(offset)))
      })
    },
  )
  await new Promise(r => server.listen(0, '127.0.0.1', r))
  try {
    const api = new API({
      server: `https://localhost:${server.address().port}`,
      caFile: join(dir, 'cert'),
      tokenFile: join(dir, 'token'),
    })
    const body = {
      apiVersion: 'v1',
      kind: 'DeleteOptions',
      preconditions: { uid: 'exact-writer', resourceVersion: '23' },
    }
    await api.remove('/resource', AbortSignal.timeout(3000), body)
    assert.deepEqual(received, body)
    assert.equal(
      (await api.call('GET', '/resource', AbortSignal.timeout(3000))).metadata
        .name,
      'test',
    )
    assert.equal((await api.call('GET', '/resource', AbortSignal.timeout(3000))).metadata.annotations.text, '汉字')
    await assert.rejects(api.call('POST', '/malformed', AbortSignal.timeout(3000), {}), (error) => {
      assert.equal(error.submitted, true)
      assert.ok(error.cause instanceof SyntaxError)
      return true
    })
    await assert.rejects(api.terminal('/watch', 'exact-writer', AbortSignal.timeout(3000)), (error) => {
      assert.equal(error.submitted, false)
      assert.ok(error.cause)
      return true
    })
  } finally {
    server.closeAllConnections()
    await new Promise(r => server.close(r))
    rmSync(dir, { recursive: true })
  }
})
