#!/usr/bin/env node
// Sync runtime schema copies from contracts/ into the server module so Go can
// embed them via go:embed (embed cannot reference files outside the module).
// Default mode copies with LF normalization; --verify checks the copies match.
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const targetDir = 'server/internal/config/contracts'

const schemas = [
  'backup-manifest.schema.json',
  'config.user.schema.json',
  'plugin-info.schema.json',
  'plugin-artifact.schema.json',
]
const verifyMode = process.argv.includes('--verify')
let failed = false

const schemaCopies = [
  ...schemas.map(name => ({ name, target: `${targetDir}/${name}` })),
  { name: 'deps-manifest.schema.json', target: 'server/internal/platform/deps/contracts/deps-manifest.schema.json' },
]

// These directories contain only schema copies owned by this generator.
const expectedPaths = new Set(schemaCopies.map(item => item.target))
for (const directory of new Set([...expectedPaths].map(target => path.posix.dirname(target)))) {
  const entries = await fs.readdir(path.join(repoRoot, directory), { withFileTypes: true }).catch(error => {
    if (error.code === 'ENOENT') return []
    throw error
  })
  for (const entry of entries) {
    if (!entry.isFile()) continue
    const target = `${directory}/${entry.name}`
    if (expectedPaths.has(target)) continue
    const fullPath = path.join(repoRoot, target)
    const schemaCopy = schemaCopies.some(item => path.posix.dirname(item.target) === directory) && entry.name.endsWith('.schema.json')
    if (!schemaCopy) continue
    if (verifyMode) { console.error(`stale generated runtime schema output: ${target}`); failed = true }
    else await fs.unlink(fullPath)
  }
}

for (const { name, target } of schemaCopies) {
  const sourcePath = path.join(repoRoot, 'contracts', name)
  const targetPath = path.join(repoRoot, target)
  const normalized = normalizeSchemaBytes(await fs.readFile(sourcePath))

  if (verifyMode) {
    let current = null
    try {
      current = normalizeSchemaBytes(await fs.readFile(targetPath))
    } catch {
      console.error(`missing embedded schema copy: ${target}`)
      failed = true
      continue
    }
    if (!normalized.equals(current)) {
      console.error(`embedded schema copy is out of sync with contracts/${name}; run node scripts/generate-runtime-schemas.mjs`)
      failed = true
    }
    continue
  }

  await fs.mkdir(path.dirname(targetPath), { recursive: true })
  await writeIfChanged(targetPath, normalized)
}

if (failed) {
  process.exit(1)
}

function normalizeSchemaBytes(buffer) {
  return Buffer.from(buffer.toString('utf8').replace(/\r\n?/g, '\n'), 'utf8')
}

async function writeIfChanged(target, data) {
  const current = await fs.readFile(target).catch(error => {
    if (error.code === 'ENOENT') return null
    throw error
  })
  if (!current?.equals(data)) await fs.writeFile(target, data)
}
