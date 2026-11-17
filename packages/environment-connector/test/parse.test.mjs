import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { parseEnvironmentOwner, parseEnvironmentIntent, parseEnvironmentRef } from '../dist/index.js'
import { parseResource, parseResourceList, parseWatchEvent } from '../dist/parse.js'
import { verifyPod } from '../dist/verify.js'
import { API, APIError } from '../dist/api.js'
import { createAgentEnvironmentRuntime } from '../dist/index.js'

test('admitted identities preserve exact strings and reject empty, coerced or oversized values', () => {
  const owner = { tenantId: ' tenant 汉 ', principalId: 'é\u0301' }
  assert.deepEqual(parseEnvironmentOwner(owner), owner)
  for (const tenantId of ['', 1, null, '😀'.repeat(129)])
    assert.throws(() => parseEnvironmentOwner({ ...owner, tenantId }))
  assert.throws(() => parseEnvironmentOwner({ ...owner, admin: true }))
  const intent = parseEnvironmentIntent({ allocationKey: 'key', owner })
  assert.deepEqual(parseEnvironmentRef({ ...intent, namespace: 'ns', sandbox: { name: 'environment', uid: 'sb' }, data: { name: 'data', uid: 'pvc' } }).owner, owner)
})

test('malformed submitted write retains cause and unknown outcome without replay', async (t) => {
  const original = API.prototype.call
  t.after(() => { API.prototype.call = original })
  const cause = new SyntaxError('private upstream data')
  let writes = 0
  API.prototype.call = async function (method) {
    if (method === 'GET') throw new APIError(404, false)
    writes++
    throw new APIError(0, true, { cause })
  }
  const runtime = createAgentEnvironmentRuntime({ kubernetes: { server: 'https://cluster.test', caFile: 'unused', tokenFile: 'unused' },
    namespacePrefix: 'env', platformNamespace: 'platform', domain: 'env.test', image: `env@sha256:${'a'.repeat(64)}`,
    storage: { storageClassName: 'standard', size: '1Gi' },
    resources: { requests: { cpu: '100m', memory: '256Mi' }, limits: { cpu: '1', memory: '1Gi' } } })
  await assert.rejects(runtime.create(parseEnvironmentIntent({ allocationKey: 'key', owner: { tenantId: 'tenant', principalId: 'alice' } }),
    { correlationId: 'create', signal: AbortSignal.timeout(3000) }), (error) => {
    assert.equal(error.failure.code, 'CreateOutcomeUnknown')
    assert.equal(error.failure.effect, 'unknown')
    assert.equal(error.cause.cause, cause)
    assert.equal(JSON.stringify(error).includes('private upstream data'), false)
    return true
  })
  assert.equal(writes, 1)
  for (const invalid of [null, { allocationKey: '', owner: { tenantId: 'tenant', principalId: 'alice' } },
    { allocationKey: 'key', owner: { tenantId: 42, principalId: 'alice' } }]) {
    await assert.rejects(runtime.create(invalid, { correlationId: 'invalid', signal: AbortSignal.timeout(3000) }), (error) => {
      assert.equal(error.failure.code, 'InvalidConfiguration')
      assert.equal(error.failure.effect, 'not-submitted')
      assert.equal(error.failure.correlationId, 'invalid')
      assert.ok(error.cause)
      return true
    })
  }
  assert.equal(writes, 1)
})

test('external resource parsing preserves unexpected privileges for template rejection', () => {
  const snapshot = JSON.parse(readFileSync(new URL('./fixtures/ready.json', import.meta.url)))
  const pod = snapshot.resources.find(resource => resource.kind === 'Pod')
  pod.spec.hostPID = true
  const parsed = parseResource(pod)
  assert.equal(parsed.spec.hostPID, true)
  const pvc = snapshot.resources.find(resource => resource.kind === 'PersistentVolumeClaim')
  const ref = parseEnvironmentRef({ allocationKey: 'b-lifecycle-1', owner: { tenantId: 'b-fixture', principalId: 'alice' }, namespace: pod.metadata.namespace,
    sandbox: { name: 'environment', uid: snapshot.sandbox.metadata.uid }, data: { name: 'data', uid: pvc.metadata.uid } })
  assert.throws(() => verifyPod(parsed, ref, { domain: 'environments.test', namespacePrefix: 'dsh-mvp-b', image: pod.spec.containers[0].image,
    resources: { requests: { cpu: '100m', memory: '256Mi' }, limits: { cpu: '1', memory: '1Gi' } } }))
  assert.throws(() => parseResource({ ...pod, metadata: { ...pod.metadata, name: '../other' } }))
  assert.throws(() => parseResourceList({ metadata: { resourceVersion: '42', continue: 'another-page' }, items: [] }))
  assert.throws(() => parseResourceList({ metadata: { resourceVersion: '42' } }))
  assert.throws(() => parseWatchEvent({ type: 'MODIFIED', object: { metadata: { uid: pod.metadata.uid }, status: { phase: 'Succeeded' } } }))
})
