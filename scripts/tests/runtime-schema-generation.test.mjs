import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { spawnSync } from 'node:child_process'

test('runtime generation detects missing, changed and stale outputs deterministically', async t => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'rayleabot-schema-generator-'))
  t.after(() => fs.rm(root, { recursive: true, force: true }))
  await fs.mkdir(path.join(root, 'scripts'))
  await fs.copyFile(new URL('../generate-runtime-schemas.mjs', import.meta.url), path.join(root, 'scripts/generate-runtime-schemas.mjs'))
  await fs.cp(new URL('../../contracts', import.meta.url), path.join(root, 'contracts'), { recursive: true })
  const run = (...args) => spawnSync(process.execPath, [path.join(root, 'scripts/generate-runtime-schemas.mjs'), ...args], { encoding: 'utf8' })
  assert.notEqual(run('--verify').status, 0)
  assert.equal(run().status, 0)
  const target = path.join(root, 'sdk/vue/src/contract.generated.ts')
  const original = await fs.readFile(target, 'utf8')
  assert.equal(run().status, 0)
  assert.equal(await fs.readFile(target, 'utf8'), original)
  assert.equal(run('--verify').status, 0)
  await fs.writeFile(target, '// incorrect\n')
  assert.notEqual(run('--verify').status, 0)
  assert.equal(run().status, 0)
  const stale = path.join(root, 'server/internal/config/contracts/removed.schema.json')
  await fs.writeFile(stale, '{}')
  assert.notEqual(run('--verify').status, 0)
  assert.equal(run().status, 0)
  await assert.rejects(fs.stat(stale), { code: 'ENOENT' })
  await fs.unlink(target)
  assert.notEqual(run('--verify').status, 0)
  assert.equal(run().status, 0)
  assert.equal(run('--verify').status, 0)
})
