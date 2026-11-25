#!/usr/bin/env node
// Dependency-free source checks: no package installation, builds or clusters.
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { readJson, record } from './json.ts'
const root = path.resolve(import.meta.dirname, '..')
process.chdir(root)
const files = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z'], {
  encoding: 'utf8',
}).split('\0').filter(Boolean)
const errors: string[] = []
const goVersion = fs.readFileSync('go.mod', 'utf8').match(/^go (\d+\.\d+\.\d+)$/m)?.[1]
const nodeVersion = fs.readFileSync('.node-version', 'utf8').trim()
const dockerfile = fs.readFileSync('images/environment/Dockerfile', 'utf8')
if (!goVersion || !dockerfile.includes(`golang:${goVersion}-bookworm@sha256:`))
  errors.push('The launcher builder must pin the go.mod Go version and an image digest')
const nodeImages = dockerfile.split('\n').filter(line => line.startsWith('FROM node:'))
const nodePrefix = `FROM node:${nodeVersion}-bookworm-slim@sha256:`
const hasNodePin = (line: string) =>
  line.startsWith(nodePrefix) && /^[a-f0-9]{64}(?: AS dsh)?$/.test(line.slice(nodePrefix.length))
if (nodeImages.length !== 2 || !nodeImages.every(hasNodePin))
  errors.push('Workload stages must pin the .node-version Node image and digest')
const rootEngines = record(record(readJson('package.json'), 'workspace manifest').engines, 'workspace engines')
for (const path of ['images/environment/package.json', 'packages/environment-connector/package.json']) {
  const manifest = record(readJson(path), 'runtime manifest')
  const engines = record(manifest.engines, 'runtime engines')
  if (engines.node !== rootEngines.node)
    errors.push(`${path}: Node engines differ from the workspace`)
}
for (const file of files) {
  if (!fs.existsSync(file) || !fs.lstatSync(file).isFile())
    continue
  const bytes = fs.readFileSync(file)
  if (bytes.includes(0))
    continue
  let source
  try {
    source = new TextDecoder('utf-8', {
      fatal: true,
    }).decode(bytes)
  }
  catch {
    errors.push(`${file}: invalid UTF-8`)
    continue
  }
  if (source.includes('\r'))
    errors.push(`${file}: use LF, not CR/CRLF`)
  if (source && !source.endsWith('\n'))
    errors.push(`${file}: missing final newline`)
  if (/[^\S\n]+$/m.test(source))
    errors.push(`${file}: trailing whitespace`)
  try {
    if (file.endsWith('.json'))
      JSON.parse(source)
    if (/\.(?:mjs|cjs|js)$/.test(file))
      execFileSync(process.execPath, ['--check', file], {
        stdio: 'pipe',
      })
    if (file.endsWith('.py'))
      execFileSync('python3', ['-c', 'import ast, pathlib, sys; ast.parse(pathlib.Path(sys.argv[1]).read_text(), filename=sys.argv[1])', file], {
        stdio: 'pipe',
      })
    if (file.endsWith('.sh') || /^#!.*\bbash\b/.test(source))
      execFileSync('bash', ['-n', file], {
        stdio: 'pipe',
      })
  }
  catch (error) {
    const stderr = error instanceof Error && 'stderr' in error ? error.stderr : undefined
    const details = Buffer.isBuffer(stderr) ? stderr.toString('utf8').trim() : typeof stderr === 'string' ? stderr.trim() : ''
    errors.push(`${file}: ${details || (error instanceof Error ? error.message : 'Source check failed')}`)
  }
}
const go = files.filter(file => file.endsWith('.go'))
if (go.length) {
  const unformatted = execFileSync('gofmt', ['-l', ...go], {
    encoding: 'utf8',
  }).trim()
  if (unformatted)
    errors.push(`Run gofmt on:\n${unformatted}`)
}
if (errors.length) {
  console.error(errors.join('\n'))
  process.exitCode = 1
}
else
  console.log(`Source standards passed (${files.length} tracked files)`)
