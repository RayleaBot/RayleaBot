import { createHash, randomUUID } from 'node:crypto'
import fs from 'node:fs/promises'
import path from 'node:path'
import { readFileContentSnapshot } from './file-content-tracker.mjs'

const ignored = new Set(['.git', '.github', '.rayleabot', 'node_modules', 'dist', 'tmp', '.tmp', 'coverage', 'testdata', '__tests__'])

export async function writeIfChanged(file, content, { atomic = false } = {}) {
  try {
    if ((await fs.readFile(file)).equals(Buffer.from(content))) return false
  } catch (error) { if (error.code !== 'ENOENT') throw error }
  await fs.mkdir(path.dirname(file), { recursive: true })
  if (atomic) {
    const temporary = `${file}.${process.pid}.${randomUUID()}.tmp`
    try { await fs.writeFile(temporary, content); await fs.rename(temporary, file) }
    finally { await fs.rm(temporary, { force: true }) }
  } else await fs.writeFile(file, content)
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

export async function fingerprint(files, identity = {}, { memo = new Map(), previous = {}, records = {} } = {}) {
  previous ??= {}
  const hash = createHash('sha256').update(JSON.stringify(identity))
  const sorted = [...new Set(files.map((file) => path.resolve(file)))].sort()
  const snapshots = new Array(sorted.length)
  let next = 0, failure
  // Bound open files and memory while overlapping filesystem latency. Each
  // entry is statted again, including entries shared by several build targets.
  await Promise.all(Array.from({ length: Math.min(8, sorted.length) }, async () => {
    while (!failure && next < sorted.length) {
      const index = next++, file = sorted[index]
      const snapshot = await readFileContentSnapshot(file, memo.get(file) ?? previous[file])
      snapshots[index] = snapshot
      if (snapshot) memo.set(file, snapshot)
      else memo.delete(file)
    }
  }).map((worker) => worker.catch((error) => { failure ??= error })))
  if (failure) throw failure
  for (const [index, file] of sorted.entries()) {
    hash.update('\0').update(file).update('\0')
    const snapshot = snapshots[index]
    if (snapshot) { records[file] = snapshot; hash.update(String(snapshot.mode)).update('\0').update(snapshot.digest) }
    else hash.update('<missing>')
  }
  return hash.digest('hex')
}

async function outputFingerprint(outputs, options) {
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
  return files.length ? fingerprint(files, {}, options) : ''
}

export function createBuildCache(directory, log = () => {}) {
  const memo = new Map()
  return {
    async run(name, { inputs, identity = {}, outputs, build, validateOutputs = async () => true }) {
      const stampPath = path.join(directory, `${name}.json`)
      let stamp
      try { stamp = JSON.parse(await fs.readFile(stampPath, 'utf8')) }
      catch (error) { if (error.code !== 'ENOENT' && !(error instanceof SyntaxError)) throw error }
      if (stamp?.version !== 2) stamp = undefined
      const inputRecords = {}, outputRecords = {}
      const key = await fingerprint(await inputs(), identity, { memo, previous: stamp?.inputs, records: inputRecords })
      const output = await outputFingerprint(outputs, { memo, previous: stamp?.outputs, records: outputRecords })
      const save = (output, inputs, outputs) => writeIfChanged(stampPath, JSON.stringify({ version: 2, key, output, inputs, outputs }), { atomic: true })
      if (stamp?.key === key && output && stamp.output === output && await validateOutputs()) {
        // Persist refreshed metadata after a touch without invalidating content.
        await save(output, inputRecords, outputRecords)
        return false
      }
      log(`${name}: ${stamp?.key === key ? 'output missing or changed' : 'inputs changed'}`)
      await build()
      // A successful command is not sufficient; the inputs must still describe its outputs.
      const checkedInputs = {}
      if (await fingerprint(await inputs({ refresh: true }), identity, { memo, records: checkedInputs }) !== key) {
        throw Object.assign(new Error(`${name}: sources changed during build; retrying on the next reconciliation`), { code: 'DEV_INPUT_CHANGED' })
      }
      const builtOutputs = {}
      const builtOutput = await outputFingerprint(outputs, { memo, records: builtOutputs })
      if (!builtOutput) throw new Error(`${name}: build did not produce its expected output`)
      if (!(await validateOutputs())) throw new Error(`${name}: build output validation failed`)
      await save(builtOutput, checkedInputs, builtOutputs)
      return true
    },
  }
}

export const goInputTemplate = '{{.Dir}}{{range .GoFiles}}\nF\t{{.}}{{end}}{{range .CgoFiles}}\nF\t{{.}}{{end}}{{range .CFiles}}\nF\t{{.}}{{end}}{{range .CXXFiles}}\nF\t{{.}}{{end}}{{range .HFiles}}\nF\t{{.}}{{end}}{{range .SFiles}}\nF\t{{.}}{{end}}{{range .SysoFiles}}\nF\t{{.}}{{end}}{{range .EmbedFiles}}\nF\t{{.}}{{end}}{{with .Module}}{{if .GoMod}}\nM\t{{.GoMod}}{{end}}{{end}}'

export function createGoInputRegistry() {
  const graphs = new Map(), files = new Set()
  const sourceDirectories = new Set()
  const fileOwners = new Map(), directoryOwners = new Map()
  const refresh = () => {
    files.clear()
    sourceDirectories.clear()
    fileOwners.clear()
    directoryOwners.clear()
    for (const [root, inputs] of graphs) for (const file of inputs) {
      files.add(file)
      if (!fileOwners.has(file)) fileOwners.set(file, new Set())
      fileOwners.get(file).add(root)
      if (!/^go\.(?:mod|sum|work(?:\.sum)?)$/.test(path.basename(file))) {
        const directory = path.dirname(file)
        sourceDirectories.add(directory)
        if (!directoryOwners.has(directory)) directoryOwners.set(directory, new Set())
        directoryOwners.get(directory).add(root)
      }
    }
  }
  return {
    files,
    isWatchEventRelevant(file, event) {
      if (files.has(file)) return true
      // Workspace/module metadata can live beside runtime files. Its directory
      // does not become a source tree merely because go list references it.
      if (!sourceDirectories.has(path.dirname(file))) return false
      return event === 'rename' || (file.endsWith('.go') && !file.endsWith('_test.go'))
    },
    ownersForWatchEvent(file, event) {
      if (!this.isWatchEventRelevant(file, event)) return []
      return [...(fileOwners.get(file) || directoryOwners.get(path.dirname(file)) || [])]
    },
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
