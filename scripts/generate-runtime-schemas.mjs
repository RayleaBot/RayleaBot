#!/usr/bin/env node
// Sync runtime schema copies from contracts/ into the server module so Go can
// embed them via go:embed (embed cannot reference files outside the module).
// Default mode copies with LF normalization; --verify checks the copies match.
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const targetDir = 'server/internal/schemaassets/contracts'

const schemas = ['config.user.schema.json', 'plugin-info.schema.json']

const verifyMode = process.argv.includes('--verify')
let failed = false

for (const name of schemas) {
  const sourcePath = path.join(repoRoot, 'contracts', name)
  const targetPath = path.join(repoRoot, targetDir, name)
  const normalized = normalizeSchemaBytes(await fs.readFile(sourcePath))

  if (verifyMode) {
    let current = null
    try {
      current = normalizeSchemaBytes(await fs.readFile(targetPath))
    } catch {
      console.error(`missing embedded schema copy: ${targetDir}/${name}`)
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
  await fs.writeFile(targetPath, normalized)
}

if (failed) {
  process.exit(1)
}

function normalizeSchemaBytes(buffer) {
  return Buffer.from(buffer.toString('utf8').replace(/\r\n?/g, '\n'), 'utf8')
}
