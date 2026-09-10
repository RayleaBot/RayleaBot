import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { cp, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import YAML from 'yaml'

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../..')
const root = await mkdtemp(path.join(os.tmpdir(), 'raylea-real-e2e-'))
const binary = path.join(root, process.platform === 'win32' ? 'raylea-server.exe' : 'raylea-server')
const configPath = path.join(root, 'config', 'user.yaml')
const controlToken = 'B'.repeat(43)
let server
let stopping = false

async function run(command, args, cwd) {
  const child = spawn(command, args, { cwd, stdio: 'inherit', windowsHide: true })
  const [code] = await once(child, 'exit')
  if (code !== 0) throw new Error(`${command} exited with code ${code}`)
}

async function stop() {
  if (stopping) return
  stopping = true
  if (server && server.exitCode === null) {
    const exited = once(server, 'exit')
    try {
      await fetch('http://127.0.0.1:4011/api/launcher/shutdown', {
        method: 'POST', headers: { 'X-Raylea-Launcher-Control': controlToken }, signal: AbortSignal.timeout(2000),
      })
    } catch { /* the process may have exited before shutdown was requested */ }
    const killTimer = setTimeout(() => server.kill(), 5000)
    await exited
    clearTimeout(killTimer)
  }
  const relative = path.relative(os.tmpdir(), root)
  if (!relative.startsWith('raylea-real-e2e-') || relative.includes(path.sep)) throw new Error('unexpected E2E cleanup directory')
  await rm(root, { recursive: true, force: true, maxRetries: 3 })
}

process.on('SIGINT', () => { void stop() })
process.on('SIGTERM', () => { void stop() })
try {
  await run('go', ['build', '-o', binary, './cmd/raylea-server'], path.join(repoRoot, 'server'))
  await run(binary, ['-config', configPath, 'config', 'init'], root)
  const config = YAML.parse(await readFile(configPath, 'utf8'))
  config.server = { ...config.server, host: '127.0.0.1', port: 4011 }
  config.adapters = []
  await writeFile(configPath, YAML.stringify(config))
  await cp(path.join(repoRoot, 'web', 'dist'), path.join(root, 'web', 'dist'), { recursive: true })
  await cp(path.join(repoRoot, 'templates'), path.join(root, 'templates'), { recursive: true })
  server = spawn(binary, ['-config', configPath], {
    cwd: root, stdio: 'inherit', windowsHide: true,
    env: { ...process.env, RAYLEA_SETUP_TOKEN: 'A'.repeat(43), RAYLEA_LAUNCHER_CONTROL_TOKEN: controlToken },
  })
  const [code] = await once(server, 'exit')
  if (!stopping && code !== 0) throw new Error(`real Server exited with code ${code}`)
} finally {
  await stop()
}
