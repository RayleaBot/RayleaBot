import { createHash } from 'node:crypto'
import fs from 'node:fs/promises'
import path from 'node:path'

const ignored = new Set(['.git', '.github', '.rayleabot', 'node_modules', 'dist', 'tmp', '.tmp', 'coverage', 'testdata', '__tests__'])

export async function writeIfChanged(file, content) {
  try {
    if ((await fs.readFile(file)).equals(Buffer.from(content))) return false
  } catch (error) { if (error.code !== 'ENOENT') throw error }
  await fs.mkdir(path.dirname(file), { recursive: true })
  await fs.writeFile(file, content)
  return true
}

// Trees are for UI/resources. Go inputs come from go list, including build tags and embeds.
export async function treeInputs(directory, { all = false } = {}) {
  const result = []
  async function visit(dir) {
    let entries
    try { entries = await fs.readdir(dir, { withFileTypes: true }) }
    catch (error) { if (error.code === 'ENOENT') return; throw error }
    for (const entry of entries.sort((a, b) => a.name.localeCompare(b.name))) {
      if (!all && (ignored.has(entry.name) || /(?:_test\.go|\.(?:test|spec)\.[cm]?[jt]sx?|\.md)$/i.test(entry.name))) continue
      const file = path.join(dir, entry.name)
      if (entry.isDirectory()) await visit(file)
      else result.push(file)
    }
  }
  await visit(directory)
  return result
}

export async function fingerprint(files, identity = {}) {
  const hash = createHash('sha256').update(JSON.stringify(identity))
  for (const file of [...new Set(files.map((file) => path.resolve(file)))].sort()) {
    hash.update('\0').update(file).update('\0')
    try {
      const stat = await fs.stat(file)
      hash.update(String(stat.mode & 0o111)).update('\0').update(await fs.readFile(file))
    } catch (error) {
      if (error.code !== 'ENOENT') throw error
      hash.update('<missing>')
    }
  }
  return hash.digest('hex')
}

async function outputFingerprint(outputs) {
  const files = []
  for (const output of outputs) {
    let stat
    try { stat = await fs.stat(output) } catch (error) { if (error.code === 'ENOENT') return ''; throw error }
    if (stat.isDirectory()) {
      const entries = await treeInputs(output, { all: true })
      if (!entries.length) return ''
      files.push(...entries)
    } else files.push(output)
  }
  return files.length ? fingerprint(files) : ''
}

export function createBuildCache(directory, log = () => {}) {
  return {
    async run(name, { inputs, identity = {}, outputs, build }) {
      const stampPath = path.join(directory, `${name}.json`)
      const inputFiles = await inputs()
      const key = await fingerprint(inputFiles, identity)
      let stamp
      try { stamp = JSON.parse(await fs.readFile(stampPath, 'utf8')) }
      catch (error) { if (error.code !== 'ENOENT' && !(error instanceof SyntaxError)) throw error }
      const output = await outputFingerprint(outputs)
      if (stamp?.version === 1 && stamp.key === key && output && stamp.output === output) {
        log(`${name}: cache hit`)
        return false
      }
      log(`${name}: ${stamp?.key === key ? 'output missing or changed' : 'inputs changed'}`)
      await build()
      // A successful command is not sufficient; the inputs must still describe its outputs.
      if (await fingerprint(await inputs(), identity) !== key) {
        throw Object.assign(new Error(`${name}: sources changed during build; retrying on the next reconciliation`), { code: 'DEV_INPUT_CHANGED' })
      }
      const builtOutput = await outputFingerprint(outputs)
      if (!builtOutput) throw new Error(`${name}: build did not produce its expected output`)
      await writeIfChanged(stampPath, JSON.stringify({ version: 1, key, output: builtOutput }))
      return true
    },
  }
}

export const goInputTemplate = '{{.Dir}}{{range .GoFiles}}\nF\t{{.}}{{end}}{{range .CgoFiles}}\nF\t{{.}}{{end}}{{range .CFiles}}\nF\t{{.}}{{end}}{{range .CXXFiles}}\nF\t{{.}}{{end}}{{range .HFiles}}\nF\t{{.}}{{end}}{{range .SFiles}}\nF\t{{.}}{{end}}{{range .SysoFiles}}\nF\t{{.}}{{end}}{{range .EmbedFiles}}\nF\t{{.}}{{end}}{{with .Module}}{{if .GoMod}}\nM\t{{.GoMod}}{{end}}{{end}}'

export function createGoInputRegistry() {
  const graphs = new Map(), files = new Set()
  const refresh = () => {
    files.clear()
    for (const inputs of graphs.values()) for (const file of inputs) files.add(file)
  }
  return {
    files,
    update(root, inputs) {
      graphs.set(path.resolve(root), inputs)
      refresh()
    },
    retainRoots(roots) {
      const activeRoots = new Set(roots.map((root) => path.resolve(root)))
      for (const root of graphs.keys()) if (!activeRoots.has(root)) graphs.delete(root)
      refresh()
    },
  }
}

export function parseGoInputs(output) {
  const files = new Set()
  let directory = ''
  for (const line of output.split(/\r?\n/)) {
    if (line.startsWith('F\t')) files.add(path.join(directory, line.slice(2)))
    else if (line.startsWith('M\t')) {
      files.add(line.slice(2))
      files.add(path.join(path.dirname(line.slice(2)), 'go.sum'))
    } else if (line) directory = line
  }
  return [...files]
}
