import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { promisify } from 'node:util'
import { execFile } from 'node:child_process'
import test from 'node:test'
import { createBuildCache, createGoInputRegistry, fingerprint, goInputTemplate, parseGoInputs } from '../dev-build-cache.mjs'

test('persistent cache checks content, tool identity and real output, not timestamps', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-dev-cache-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const source = path.join(root, 'source'), output = path.join(root, 'output')
  await fs.writeFile(source, 'first')
  let builds = 0
  const options = { inputs: async () => [source], outputs: [output], identity: { tool: '1' }, build: async () => { builds++; await fs.copyFile(source, output) } }
  const run = () => createBuildCache(path.join(root, 'cache')).run('backend', options)
  assert.equal(await run(), true)
  assert.equal(await run(), false)
  const old = (await fs.stat(source)).mtime
  await fs.writeFile(source, 'other')
  await fs.utimes(source, old, old)
  assert.equal(await run(), true)
  await fs.writeFile(output, 'broken')
  assert.equal(await run(), true)
  await fs.rm(output)
  assert.equal(await run(), true)
  options.identity.tool = '2'
  assert.equal(await run(), true)
  assert.equal(builds, 5)
})

test('failed and superseded builds cannot become reusable cache entries', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-dev-stale-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const source = path.join(root, 'source'), output = path.join(root, 'output')
  await fs.writeFile(source, 'first')
  const cache = createBuildCache(path.join(root, 'cache'))
  const options = { inputs: async () => [source], outputs: [output], build: async () => { await fs.copyFile(source, output); await fs.writeFile(source, 'newer') } }
  await assert.rejects(cache.run('plugin', options), { code: 'DEV_INPUT_CHANGED' })
  options.build = async () => { throw new Error('compiler failure') }
  await assert.rejects(cache.run('plugin', options), /compiler failure/)
  options.build = () => fs.copyFile(source, output)
  assert.equal(await cache.run('plugin', options), true)
  assert.equal(await fs.readFile(output, 'utf8'), 'newer')
})

test('Go input graph includes embeds and local dependencies, excludes tests and foreign platforms', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-dev-inputs-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const write = (name, text) => fs.writeFile(path.join(root, name), text)
  await write('go.mod', 'module fixture.local/development\n\ngo 1.26.6\n')
  await write('main.go', 'package main\nimport _ "embed"\n//go:embed payload.json\nvar payload string\nfunc main() {}\n')
  await write('main_test.go', 'package main\n')
  await write('foreign.go', '//go:build linux\n\npackage main\n')
  await write('payload.json', '{}')
  const list = async () => parseGoInputs((await promisify(execFile)('go', ['list', '-deps', '-f', goInputTemplate, '.'], { cwd: root, env: { ...process.env, GOWORK: 'off', GOOS: 'windows', GOARCH: 'amd64', CGO_ENABLED: '0' }, maxBuffer: 8 * 1024 * 1024 })).stdout)
  const files = await list()
  assert.ok(files.includes(path.join(root, 'payload.json')))
  assert.ok(!files.includes(path.join(root, 'foreign.go')))
  assert.ok(!files.includes(path.join(root, 'main_test.go')))
  const before = await fingerprint(files)
  await write('foreign.go', '//go:build linux\n\npackage main\nvar unused = 1\n')
  assert.equal(await fingerprint(await list()), before)
  await write('payload.json', '{"changed":true}')
  assert.notEqual(await fingerprint(await list()), before)
})

test('Go input ownership drops removed projects and obsolete dependencies while retaining shared inputs', () => {
  const registry = createGoInputRegistry()
  registry.update('server', ['server/main.go', 'shared/old.go'])
  registry.update('plugin', ['plugin/main.go', 'shared/old.go'])
  registry.retainRoots(['server'])
  assert.equal(registry.files.has('plugin/main.go'), false)
  assert.equal(registry.files.has('shared/old.go'), true)
  registry.update('server', ['server/main.go', 'shared/new.go'])
  assert.equal(registry.files.has('shared/old.go'), false)
  assert.equal(registry.files.has('shared/new.go'), true)
})
