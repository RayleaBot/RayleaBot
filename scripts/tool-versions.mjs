import fs from 'node:fs'

export function readToolVersions(url = new URL('../.tool-versions', import.meta.url)) {
  const versions = {}
  for (const line of fs.readFileSync(url, 'utf8').split(/\r?\n/)) {
    const fields = line.split('#', 1)[0].trim().split(/\s+/)
    if (!fields[0]) continue
    if (fields.length !== 2 || Object.hasOwn(versions, fields[0]) || !/^[a-z]+$/.test(fields[0]) || !/^\d+\.\d+\.\d+$/.test(fields[1])) {
      throw new Error('.tool-versions requires one fixed version per tool')
    }
    versions[fields[0]] = fields[1]
  }
  for (const tool of ['golang', 'nodejs', 'python', 'pnpm', 'npm', 'corepack', 'sqlc']) {
    if (!versions[tool]) throw new Error(`Missing tool version: ${tool}`)
  }
  return Object.freeze(versions)
}

export const toolVersions = readToolVersions()
