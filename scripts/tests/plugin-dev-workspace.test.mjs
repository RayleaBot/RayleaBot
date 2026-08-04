import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'

import {
  collectWorkspaceSDKVersions,
  currentPluginPlatform,
  PLUGIN_DEV_OFF,
  PLUGIN_DEV_SYNC,
  PLUGIN_DEV_WATCH,
  renderDevelopmentGoWork,
  resolvePluginDevMode,
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
