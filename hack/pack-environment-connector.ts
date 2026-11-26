import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, resolve } from 'node:path'
import { list, parseJson, record, string } from './json.ts'

const root = resolve(import.meta.dirname, '..')
const output = process.argv[2]
if (!output)
  throw new Error('Usage: node hack/pack-environment-connector.ts OUTPUT_DIRECTORY')
const run = (command: string, args: readonly string[], cwd = root) => execFileSync(command, args, { cwd, encoding: 'utf8' })
const status = run('git', ['status', '--porcelain', '--untracked-files=all', '--', 'packages/environment-connector',
  'hack/pack-environment-connector.ts', 'hack/json.ts', 'LICENSE', 'package.json', 'pnpm-lock.yaml', 'pnpm-workspace.yaml'])
if (status.trim())
  throw new Error('Commit connector inputs before packing')

const source = { repository: 'GuoMonth/dsh-isolated-runtime', commit: run('git', ['rev-parse', 'HEAD']).trim() }
const directory = resolve(root, 'packages/environment-connector')
const destination = resolve(output)
run('pnpm', ['install', '--frozen-lockfile', '--ignore-scripts'])
run('pnpm', ['build'])
copyFileSync(resolve(root, 'LICENSE'), resolve(directory, 'LICENSE'))
writeFileSync(resolve(directory, 'source.json'), JSON.stringify(source, null, 2) + '\n')
mkdirSync(destination, { recursive: true })
const packed = run('npm', ['pack', '--ignore-scripts', '--pack-destination', destination, '--json'], directory)
const results = list(parseJson(packed), 'npm pack results')
assert.equal(results.length, 1, 'Expected exactly one Connector archive')
const metadata = record(results[0], 'Connector pack result')
const artifact = string(metadata.filename, 'Connector archive filename')
assert.ok(basename(artifact) === artifact && artifact.endsWith('.tgz'), 'Expected a local archive filename')
const bytes = readFileSync(resolve(destination, artifact))
const integrity = 'sha512-' + createHash('sha512').update(bytes).digest('base64')
assert.equal(integrity, metadata.integrity, 'Connector archive integrity differs from npm pack result')
const sha256 = createHash('sha256').update(bytes).digest('hex')
writeFileSync(resolve(destination, 'connector.json'), JSON.stringify({ ...source, artifact, integrity, sha256 }, null, 2) + '\n')
process.stdout.write(packed)
