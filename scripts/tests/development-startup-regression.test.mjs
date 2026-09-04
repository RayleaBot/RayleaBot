import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { promisify } from 'node:util'
import { createTrustedChildEnvironment } from '../start-dev-support.mjs'
import { currentPluginPlatform, renderDevelopmentGoWork, watchPluginWorkspace } from '../plugin-dev-workspace.mjs'

const execute = promisify(execFile)
const repository = path.resolve(import.meta.dirname, '../..')

test('standalone plugin builder can invoke Go inside the sanitized development environment', { timeout: 180_000 }, async (t) => {
  const temporary = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-startup-build-'))
  t.after(() => fs.rm(temporary, { recursive: true, force: true }))
  const goRoot = (await execute('go', ['env', 'GOROOT'])).stdout.trim()
  const goExecutable = path.join(goRoot, 'bin', process.platform === 'win32' ? 'go.exe' : 'go')
  const environment = { ...createTrustedChildEnvironment({ nodeExecutablePath: process.execPath, goExecutablePath: goExecutable }), CGO_ENABLED: '0' }
  const executableSuffix = process.platform === 'win32' ? '.exe' : ''
  const builder = path.join(temporary, 'raylea-plugin' + executableSuffix)
  await execute(goExecutable, ['build', '-o', builder, './cmd/raylea-plugin'], {
    cwd: path.join(repository, 'sdk/go'), env: { ...environment, GOWORK: 'off' },
  })
  const plugin = path.join(temporary, 'plugin')
  await fs.mkdir(plugin)
  await fs.writeFile(path.join(plugin, 'go.mod'), 'module fixture.local/startup\n\ngo 1.26.6\n')
  await fs.writeFile(path.join(plugin, 'main.go'), 'package main\nfunc main() {}\n')
  await fs.writeFile(path.join(plugin, 'LICENSE'), 'fixture license\n')
  await fs.writeFile(path.join(plugin, 'info.json'), JSON.stringify({
    id: 'startup.fixture', name: 'Startup fixture', version: '0.4.0', manifest_version: '3', min_core_version: '0.4.0', license: 'MIT',
  }))
  const workspace = path.join(temporary, 'go.work')
  await fs.writeFile(workspace, renderDevelopmentGoWork({ sdkGoPath: path.join(repository, 'sdk/go'), plugins: [{ path: plugin }] }))
  const developmentEnvironment = { ...environment, GOWORK: workspace, RAYLEA_PLUGIN_BUILD_USE_WORKSPACE: '1' }
  const binary = path.join(temporary, 'backend' + executableSuffix)
  await execute(goExecutable, ['build', '-o', binary, '.'], { cwd: plugin, env: developmentEnvironment })
  const output = path.join(temporary, 'artifacts')
  await execute(builder, ['build-go', '--plugin', plugin, '--backend-binary', binary, '--skip-ui-build', '--expanded=true', '--archive=false', '--target', currentPluginPlatform(), '--out', output], {
    cwd: repository, env: developmentEnvironment,
  })
  const artifact = path.join(output, currentPluginPlatform(), 'startup.fixture')
  for (const name of ['artifact.json', 'THIRD_PARTY_NOTICES.md', 'sbom.spdx.json']) {
    assert.ok((await fs.stat(path.join(artifact, name))).size > 0)
  }
  const inspection = JSON.parse((await execute(builder, ['inspect', '--artifact', artifact, '--target', currentPluginPlatform()], { env: developmentEnvironment })).stdout)
  assert.equal(inspection.PluginID, 'startup.fixture')
})

test('Server watcher excludes existing and newly created runtime outputs but observes source edits', { timeout: 10_000 }, async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-startup-watch-'))
  const source = path.join(root, 'main.go')
  await fs.writeFile(source, 'package main\n')
  await fs.mkdir(path.join(root, 'tmp'))
  await fs.writeFile(path.join(root, 'tmp', 'running.exe'), 'previous executable')
  const changes = [], errors = []
  const stop = await watchPluginWorkspace([
    { id: 'server', path: root, hasGoModule: true, ignoredDirectories: ['tmp', '.tmp', '.cache', '.gocache', 'logs'] },
  ], (_entry, file) => changes.push(path.relative(root, file)), (error) => errors.push(error))
  t.after(async () => { await stop(); await fs.rm(root, { recursive: true, force: true }) })
  await fs.writeFile(path.join(root, 'tmp', 'running.exe'), 'candidate executable')
  await fs.mkdir(path.join(root, 'logs'))
  await fs.writeFile(path.join(root, 'logs', 'server.log'), 'runtime output')
  await fs.writeFile(source, 'package main\nfunc main() {}\n')
  const deadline = Date.now() + 5_000
  while (!changes.includes('main.go') && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 20))
  await new Promise((resolve) => setTimeout(resolve, 100))
  assert.ok(changes.includes('main.go'))
  assert.deepEqual(changes.filter((file) => file !== 'main.go'), [])
  assert.deepEqual(errors, [])
})
