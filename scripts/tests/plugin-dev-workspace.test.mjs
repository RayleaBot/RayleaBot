import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import {
  collectWorkspaceSDKVersions,
  createDevelopmentReloadQueue,
  currentPluginPlatform,
  PLUGIN_DEV_OFF,
  PLUGIN_DEV_SYNC,
  PLUGIN_DEV_WATCH,
  mirrorVueSDK,
  renderDevelopmentGoWork,
  resolvePluginDevMode,
  selectWorkspacePlugins,
  watchPluginWorkspace,
} from '../plugin-dev-workspace.mjs'

test('plugin development mode defaults to sync only when a workspace exists', () => {
  assert.equal(resolvePluginDevMode({}, true), PLUGIN_DEV_SYNC)
  assert.equal(resolvePluginDevMode({}, false), PLUGIN_DEV_OFF)
  assert.equal(resolvePluginDevMode({ RAYLEA_PLUGIN_DEV: 'watch' }, false), PLUGIN_DEV_WATCH)
  assert.throws(() => resolvePluginDevMode({ RAYLEA_PLUGIN_DEV: 'build' }, true))
})

test('plugin platform projection uses artifact contract names', () => {
  assert.equal(currentPluginPlatform('win32', 'x64'), 'windows-x64')
  assert.equal(currentPluginPlatform('linux', 'x64'), 'linux-x64')
  assert.equal(currentPluginPlatform('darwin', 'arm64'), 'macos-arm64')
  assert.throws(() => currentPluginPlatform('darwin', 'x64'))
})

test('development go.work includes the SDK, plugin modules and SDK replacement once', () => {
  const rendered = renderDevelopmentGoWork({
    sdkGoPath: 'C:/workspace/RayleaBot/sdk/go',
    sdkGoVersions: ['v0.2.0', 'v0.2.0'],
    plugins: [
      { path: 'C:/workspace/plugins/echo' },
      { path: 'C:/workspace/plugins/echo' },
    ],
  })
  assert.match(rendered, /^go 1\.25\.12/m)
  assert.equal((rendered.match(/plugins(?:\\\\|\/)echo/g) ?? []).length, 1)
  assert.match(rendered, /RayleaBot(?:\\\\|\/)sdk(?:\\\\|\/)go/)
  assert.equal((rendered.match(/replace github\.com\/RayleaBot\/RayleaBot\/sdk\/go v0\.2\.0/g) ?? []).length, 1)
})

test('collects SDK versions declared by independent plugin modules', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'rayleabot-plugin-sdk-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const first = path.join(root, 'first')
  const second = path.join(root, 'second')
  await fs.mkdir(first, { recursive: true })
  await fs.mkdir(second, { recursive: true })
  await fs.writeFile(path.join(first, 'go.mod'), 'module example/first\n\nrequire github.com/RayleaBot/RayleaBot/sdk/go v0.2.0\n')
  await fs.writeFile(path.join(second, 'go.mod'), 'module example/second\n\nrequire (\n\tgithub.com/RayleaBot/RayleaBot/sdk/go v0.3.0-beta.1\n)\n')

  assert.deepEqual(await collectWorkspaceSDKVersions([{ path: first }, { path: second }]), ['v0.2.0', 'v0.3.0-beta.1'])
})

test('selects only changed development plugins in workspace order', () => {
  const plugins = [
    { id: 'raylea.echo' },
    { id: 'raylea.fortune' },
    { id: 'raylea.game-guide' },
  ]

  assert.equal(selectWorkspacePlugins(plugins), plugins)
  assert.deepEqual(
    selectWorkspacePlugins(plugins, ['raylea.game-guide', 'raylea.echo']),
    [plugins[0], plugins[2]],
  )
  assert.deepEqual(selectWorkspacePlugins(plugins, []), [])
  assert.throws(
    () => selectWorkspacePlugins(plugins, ['raylea.missing']),
    /Unknown development plugin id\(s\): raylea\.missing/,
  )
})

test('keeps plugin changes that arrive after the current reload batch is taken', () => {
  const queue = createDevelopmentReloadQueue()
  const echo = { id: 'raylea.echo' }
  const fortune = { id: 'raylea.fortune' }

  queue.addPlugin(echo, 'echo/main.go')
  queue.addPlugin(echo, 'echo/info.json')
  queue.addServer('server/internal/plugins/runtime.go')
  assert.deepEqual(queue.take(), {
    serverSourcePath: 'server/internal/plugins/runtime.go',
    pluginChanges: [{ plugin: echo, sourcePath: 'echo/info.json' }],
  })

  queue.addPlugin(fortune, 'fortune/ui/src/App.vue')
  assert.equal(queue.hasChanges(), true)
  assert.deepEqual(queue.take(), {
    serverSourcePath: '',
    pluginChanges: [{ plugin: fortune, sourcePath: 'fortune/ui/src/App.vue' }],
  })
  assert.equal(queue.hasChanges(), false)
})

test('mirrors the Vue SDK without discarding installed workspace dependencies', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-plugin-vue-sdk-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const sdkVuePath = path.join(root, 'sdk-vue')
  const pluginPath = path.join(root, 'plugin')
  const target = path.join(pluginPath, '.rayleabot', 'sdk', 'vue')
  await fs.mkdir(path.join(pluginPath, 'ui'), { recursive: true })
  await fs.writeFile(path.join(pluginPath, 'ui', 'package.json'), '{}\n', 'utf8')
  await fs.mkdir(path.join(sdkVuePath, 'src'), { recursive: true })
  await fs.writeFile(path.join(sdkVuePath, 'src', 'index.ts'), 'export {}\n', 'utf8')
  await fs.mkdir(path.join(target, 'node_modules'), { recursive: true })
  await fs.writeFile(path.join(target, 'node_modules', 'installed.txt'), 'keep\n', 'utf8')
  await fs.writeFile(path.join(target, 'stale.txt'), 'remove\n', 'utf8')

  await mirrorVueSDK({ sdkVuePath, pluginPath })

  assert.equal(await fs.readFile(path.join(target, 'src', 'index.ts'), 'utf8'), 'export {}\n')
  assert.equal(await fs.readFile(path.join(target, 'node_modules', 'installed.txt'), 'utf8'), 'keep\n')
  await assert.rejects(fs.stat(path.join(target, 'stale.txt')), { code: 'ENOENT' })
})

test('forces a clean UI install when a new Vue SDK mirror has no dependencies', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-plugin-vue-sdk-install-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const sdkVuePath = path.join(root, 'sdk-vue')
  const pluginPath = path.join(root, 'plugin')
  await fs.mkdir(path.join(pluginPath, 'ui', 'node_modules'), { recursive: true })
  await fs.writeFile(path.join(pluginPath, 'ui', 'package.json'), '{}\n', 'utf8')
  await fs.writeFile(path.join(pluginPath, 'ui', 'node_modules', 'installed.txt'), 'stale\n', 'utf8')
  await fs.mkdir(path.join(sdkVuePath, 'src'), { recursive: true })
  await fs.writeFile(path.join(sdkVuePath, 'src', 'index.ts'), 'export {}\n', 'utf8')

  await mirrorVueSDK({ sdkVuePath, pluginPath })

  await assert.rejects(fs.stat(path.join(pluginPath, 'ui', 'node_modules')), { code: 'ENOENT' })
})

test('plugin watcher ignores generated trees and existing directory metadata events', { timeout: 5_000 }, async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-plugin-watch-'))
  const pluginPath = path.join(root, 'plugin')
  const sourceDir = path.join(pluginPath, 'ui', 'src')
  const outputDir = path.join(pluginPath, 'ui', 'dist')
  await fs.mkdir(sourceDir, { recursive: true })
  await fs.mkdir(outputDir, { recursive: true })
  const sourcePath = path.join(sourceDir, 'App.vue')
  await fs.writeFile(sourcePath, '<template />\n', 'utf8')
  const changes = []
  const stopWatching = await watchPluginWorkspace(
    [{ id: 'raylea.test', path: pluginPath }],
    (_plugin, sourcePath) => changes.push(path.relative(pluginPath, sourcePath)),
  )
  t.after(async () => {
    await stopWatching()
    await fs.rm(root, { recursive: true, force: true })
  })

  const now = new Date()
  await fs.utimes(path.join(pluginPath, 'ui'), now, now)
  await fs.readFile(sourcePath)
  await fs.writeFile(path.join(outputDir, 'index.js'), 'generated\n', 'utf8')
  await new Promise((resolve) => setTimeout(resolve, 200))
  assert.deepEqual(changes, [])

  await fs.writeFile(sourcePath, '<template><main /></template>\n', 'utf8')
  const deadline = Date.now() + 2_000
  while (!changes.includes(path.join('ui', 'src', 'App.vue')) && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 20))
  }
  assert.ok(changes.includes(path.join('ui', 'src', 'App.vue')))
})
