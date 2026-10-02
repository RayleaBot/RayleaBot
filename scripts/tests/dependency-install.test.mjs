import assert from 'node:assert/strict'
import { execFile } from 'node:child_process'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { promisify } from 'node:util'
import { createBuildCache } from '../dev-build-cache.mjs'
import { createDependencyInstallEnvironment, ensureDependencyInstall, isDependencyInstallComplete, resolveCorepackCliPath } from '../start-dev-support.mjs'
import { toolVersions } from '../tool-versions.mjs'

const execute = promisify(execFile)

test('real pnpm repairs deleted packages and command shims before caching the completed install', { timeout: 120_000 }, async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-pnpm-repair-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const projectDir = path.join(root, 'project'), cli = path.join(root, 'cli'), leaf = path.join(root, 'leaf')
  for (const dir of [projectDir, cli, leaf]) await fs.mkdir(dir)
  await fs.writeFile(path.join(leaf, 'package.json'), JSON.stringify({ name: 'fixture-leaf', version: '1.0.0', main: 'index.cjs' }))
  await fs.writeFile(path.join(leaf, 'index.cjs'), 'module.exports = "fixture-ready";\n')
  await fs.writeFile(path.join(cli, 'package.json'), JSON.stringify({ name: 'fixture-cli', version: '1.0.0', bin: { 'fixture-cli': 'cli.cjs' }, dependencies: { 'fixture-leaf': 'file:../leaf' } }))
  await fs.writeFile(path.join(cli, 'cli.cjs'), '#!/usr/bin/env node\nconsole.log(require("fixture-leaf"));\n')
  await fs.writeFile(path.join(projectDir, 'package.json'), JSON.stringify({ name: 'fixture-project', private: true, packageManager: `pnpm@${toolVersions.pnpm}`, devDependencies: { 'fixture-cli': 'file:../cli' } }))
  const corepack = resolveCorepackCliPath()
  const run = (args) => execute(process.execPath, [corepack, 'pnpm', '--config.verify-deps-before-run=false', ...args], {
    cwd: projectDir, env: createDependencyInstallEnvironment(process.env), timeout: 30_000,
  })
  await run(['install', '--lockfile-only', '--offline', '--ignore-scripts'])
  const inputFiles = [path.join(projectDir, 'package.json'), path.join(projectDir, 'pnpm-lock.yaml')]
  const cache = createBuildCache(path.join(root, 'cache'))
  let installs = 0, repairs = 0
  const ensure = () => ensureDependencyInstall({
    projectDir, cache, name: 'deps', inputs: async () => inputFiles, identity: { pnpm: toolVersions.pnpm },
    onRepair: () => { repairs++ },
    install: async () => {
      installs++
      await run(['install', '--frozen-lockfile', '--offline', '--ignore-scripts'])
    },
  })
  const checkCommand = async () => assert.equal((await run(['exec', 'fixture-cli'])).stdout.trim(), 'fixture-ready')
  await ensure()
  assert.equal(await isDependencyInstallComplete(projectDir), true)
  await checkCommand()
  const initialInstalls = installs
  await ensure()
  assert.equal(installs, initialInstalls)

  const modules = path.join(projectDir, 'node_modules')
  const shim = path.join(modules, '.bin', 'fixture-cli' + (process.platform === 'win32' ? '.cmd' : ''))
  for (const remove of [
    () => fs.rm(path.join(modules, '.pnpm'), { recursive: true, force: true }),
    async () => {
      const entries = JSON.parse(await fs.readFile(path.join(modules, '.package-map.json'), 'utf8')).packages
      const leafEntry = Object.entries(entries).find(([id]) => id.startsWith('fixture-leaf@'))[1]
      await fs.rm(path.resolve(modules, leafEntry.url, 'package.json'))
    },
    () => fs.rm(shim),
    () => fs.writeFile(path.join(modules, '.modules.yaml'), '{invalid'),
  ]) {
    await remove()
    assert.equal(await isDependencyInstallComplete(projectDir), false)
    await ensure()
    await checkCommand()
    const count = installs
    await ensure()
    assert.equal(installs, count, 'a repaired install must reuse its final cache stamp')
  }
  assert.ok(repairs > 0, 'at least one incomplete install must exercise state reset')
})

test('an installation that never becomes complete cannot create a reusable cache stamp', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'raylea-pnpm-failed-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  const cacheDir = path.join(root, 'cache'), cache = createBuildCache(cacheDir)
  const packageFile = path.join(root, 'package.json')
  await fs.writeFile(packageFile, '{}')
  let installs = 0
  await assert.rejects(ensureDependencyInstall({
    projectDir: root, cache, name: 'deps', inputs: async () => [packageFile], install: async () => { installs++ },
  }))
  assert.equal(installs, 2)
  await assert.rejects(fs.access(path.join(cacheDir, 'deps.json')), { code: 'ENOENT' })
})
