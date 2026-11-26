import test from 'node:test'
import assert from 'node:assert/strict'
import { assessAudit } from './audit-node.ts'

const vulnerabilities = { info: 0, low: 0, moderate: 0, high: 0, critical: 0 }
const workspace = { metadata: { vulnerabilities }, advisories: {} }
const workload = { auditReportVersion: 2, metadata: { vulnerabilities: { ...vulnerabilities, total: 0 } }, vulnerabilities: {} }

void test('pnpm and npm audits require complete clean reports', () => {
  const clean = { knownIssues: [], auditClean: true }
  assert.deepEqual(assessAudit(workspace, false), clean)
  assert.deepEqual(assessAudit(workload, true), clean)
  assert.throws(() => assessAudit(workspace, true))
  assert.throws(() => assessAudit(workload, false))
  assert.throws(() => assessAudit({ ...workspace, metadata: { vulnerabilities: {} } }, false))
  assert.throws(() => assessAudit({ ...workload, error: {} }, true))
})

void test('both displayed counts and returned findings must be empty', () => {
  assert.throws(() => assessAudit({ ...workspace, metadata: { vulnerabilities: { ...vulnerabilities, low: 1 } } }, false))
  assert.throws(() => assessAudit({ ...workspace, advisories: { advisory: {} } }, false))
  assert.throws(() => assessAudit({ ...workload, vulnerabilities: { dependency: {} } }, true))
})
