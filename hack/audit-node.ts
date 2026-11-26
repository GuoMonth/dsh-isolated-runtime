import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parseJson, record } from './json.ts'

export function assessAudit(value: unknown, workload: boolean) {
  const report = record(value, 'audit report')
  assert.ok(!report.error, 'Audit service must return a complete report')
  const metadata = record(report.metadata, 'audit metadata')
  const counts = record(metadata.vulnerabilities, 'audit severity counts')
  const severities = ['info', 'low', 'moderate', 'high', 'critical']
  if (workload) {
    assert.equal(report.auditReportVersion, 2, 'Unknown npm audit report schema')
    severities.push('total')
  }
  for (const severity of severities)
    assert.equal(counts[severity], 0, `Expected zero ${severity} vulnerabilities`)
  const findings = record(workload ? report.vulnerabilities : report.advisories, 'audit findings')
  assert.equal(Object.keys(findings).length, 0, 'Audit reports vulnerabilities')
  return { knownIssues: [], auditClean: true }
}

function main() {
  const args = process.argv.slice(2)
  assert.ok(args.length === 0 || (args.length === 1 && args[0] === '--workload'))
  const workload = args.length === 1
  const result = spawnSync(workload ? 'npm' : 'pnpm', workload
    ? ['audit', '--prefix', 'images/environment', '--omit=dev', '--audit-level=low', '--json']
    : ['audit', '--audit-level', 'low', '--json'], { encoding: 'utf8', timeout: 90000, maxBuffer: 16 * 1024 * 1024 })
  process.stdout.write(result.stdout || '')
  process.stderr.write(result.stderr || '')
  if (result.error) throw result.error
  const report = parseJson(result.stdout)
  const assessment = assessAudit(report, workload)
  assert.equal(result.status, 0, 'Audit command failed')
  if (process.env.AUDIT_EVIDENCE_DIRECTORY) {
    const directory = resolve(process.env.AUDIT_EVIDENCE_DIRECTORY)
    mkdirSync(directory, { recursive: true })
    const name = workload ? 'workload' : 'workspace'
    writeFileSync(resolve(directory, `${name}-raw.json`), JSON.stringify(report, null, 2) + '\n')
    writeFileSync(resolve(directory, `${name}-assessment.json`), JSON.stringify(assessment, null, 2) + '\n')
  }
  console.log(JSON.stringify({ securityAssessment: assessment }))
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main()
