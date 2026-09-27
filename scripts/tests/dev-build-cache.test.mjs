import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import { watch } from 'node:fs'
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

test('metadata-only directories do not treat Launcher probes or build outputs as Go source changes', () => {
  const root = path.resolve('fixture-workspace')
  const registry = createGoInputRegistry()
  const workspace = path.join(root, 'go.work')
  const module = path.join(root, 'shared', 'go.mod')
  const developmentWorkspace = path.join(root, '.tmp', 'plugin-dev', 'go.work')
  registry.update(root, [workspace, workspace + '.sum', module, developmentWorkspace])
  for (const event of ['rename', 'change']) {
    for (const input of [workspace, workspace + '.sum', module, developmentWorkspace]) {
      assert.equal(registry.isWatchEventRelevant(input, event), true)
    }
    for (const output of [
      path.join(root, '.launcher-write-test-1234'), path.join(root, 'notes.txt'),
      path.join(root, '.tmp'), path.join(root, '.tmp', 'plugin-dev', 'artifacts'),
      path.join(root, 'shared', '.launcher-write-test-5678'),
    ]) assert.equal(registry.isWatchEventRelevant(output, event), false)
  }
})

test('private plugin workspace metadata and external sources notify only their Go graph owners', () => {
  const registry = createGoInputRegistry()
  const server = path.resolve('server'), a = path.resolve('plugin-a'), b = path.resolve('plugin-b')
  const privateWork = path.resolve('.tmp/workspaces/a/go.work')
  const shared = path.resolve('library/shared.go')
  registry.update(server, [path.join(server, 'main.go')])
  registry.update(a, [privateWork, privateWork + '.sum', shared])
  registry.update(b, [shared])
  assert.deepEqual(registry.ownersForWatchEvent(privateWork + '.sum', 'change'), [a])
  assert.deepEqual(registry.ownersForWatchEvent(shared, 'change'), [a, b])
  assert.deepEqual(registry.ownersForWatchEvent(path.resolve('library/added.go'), 'rename'), [a, b])
  registry.retainRoots([server, b])
  assert.deepEqual(registry.ownersForWatchEvent(privateWork, 'rename'), [])
  assert.deepEqual(registry.ownersForWatchEvent(shared, 'change'), [b])
})

test('source and embedded-resource directories retain additions and deletions as their ownership changes', () => {
  const root = path.resolve('fixture-workspace')
  const source = path.join(root, 'shared', 'main.go')
  const metadata = path.join(root, 'shared', 'go.mod')
  const asset = path.join(root, 'shared', 'assets', 'first.json')
  const added = path.join(root, 'shared', 'extra.go')
  const addedAsset = path.join(root, 'shared', 'assets', 'second.json')
  const registry = createGoInputRegistry()
  registry.update('server', [source, metadata, asset])
  registry.update('plugin', [source, metadata, asset])
  for (const file of [source, metadata, asset, added, addedAsset]) {
    assert.equal(registry.isWatchEventRelevant(file, 'rename'), true)
  }
  assert.equal(registry.isWatchEventRelevant(source, 'change'), true)
  assert.equal(registry.isWatchEventRelevant(asset, 'change'), true)
  registry.retainRoots(['server'])
  assert.equal(registry.isWatchEventRelevant(addedAsset, 'rename'), true)
  registry.update('server', [metadata])
  assert.equal(registry.isWatchEventRelevant(added, 'rename'), false)
  assert.equal(registry.isWatchEventRelevant(addedAsset, 'rename'), false)
  assert.equal(registry.isWatchEventRelevant(metadata, 'change'), true)
})

test('real filesystem events from Launcher startup probes do not enqueue a reload', { timeout: 5000 }, async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-go-input-watch-'))
  const moduleRoot = path.join(root, 'shared')
  const assets = path.join(moduleRoot, 'assets')
  await fs.mkdir(assets, { recursive: true })
  const workspace = path.join(root, 'go.work'), source = path.join(moduleRoot, 'main.go')
  const firstAsset = path.join(assets, 'first.json'), addedAsset = path.join(assets, 'second.json')
  await fs.writeFile(workspace, 'go 1.26.6\n')
  await fs.writeFile(source, 'package shared\n')
  await fs.writeFile(firstAsset, '{}\n')
  const registry = createGoInputRegistry()
  registry.update('server', [workspace, source, firstAsset])
  const received = [], queued = [], errors = []
  const watchers = [root, moduleRoot, assets].map((directory) => {
    const watcher = watch(directory, (event, name) => {
      if (!name) return
      const file = path.join(directory, name.toString())
      received.push(file)
      if (registry.isWatchEventRelevant(file, event)) queued.push(file)
    })
    watcher.on('error', (error) => errors.push(error))
    return watcher
  })
  t.after(async () => { for (const watcher of watchers) watcher.close(); await fs.rm(root, { recursive: true, force: true }) })
  const probe = path.join(root, '.launcher-write-test-1234')
  await fs.writeFile(probe, 'probe')
  await fs.rm(probe)
  await fs.writeFile(workspace, 'go 1.26.6\nuse ./shared\n')
  await fs.writeFile(source, 'package shared\nvar Changed = true\n')
  await fs.writeFile(addedAsset, '{"added":true}\n')
  const deadline = Date.now() + 3000
  while (![workspace, source, addedAsset].every((file) => queued.includes(file)) && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 20))
  }
  await new Promise((resolve) => setTimeout(resolve, 100))
  assert.ok(received.includes(probe), 'the regression must exercise real probe notifications')
  assert.ok(!queued.includes(probe))
  for (const file of [workspace, source, addedAsset]) assert.ok(queued.includes(file), `missing source change: ${file}`)
  assert.deepEqual(errors, [])
})
