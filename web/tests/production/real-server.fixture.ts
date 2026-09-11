import { test as base, expect } from '@playwright/test'
import { spawn, type ChildProcess } from 'node:child_process'
import { once } from 'node:events'
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { createServer } from 'node:net'
import os from 'node:os'
import path from 'node:path'
import { randomBytes } from 'node:crypto'
import YAML from 'yaml'

const repoRoot = path.resolve(import.meta.dirname, '../../..')
const tempPrefix = 'raylea-real-e2e-'
type Server = { url: string; setupToken: string }

async function removeFixtureRoot(root: string) {
  const relative = path.relative(os.tmpdir(), root)
  if (!path.isAbsolute(root) || !relative.startsWith(tempPrefix) || relative.includes(path.sep)) throw new Error('unexpected E2E cleanup directory')
  await rm(root, { recursive: true, force: true, maxRetries: 4 })
}

async function run(command: string, args: string[], cwd: string) {
  const child = spawn(command, args, { cwd, stdio: 'pipe', windowsHide: true })
  let output = ''
  child.stdout?.on('data', chunk => { output += String(chunk) })
  child.stderr?.on('data', chunk => { output += String(chunk) })
  const timer = setTimeout(() => child.kill(), 90_000)
  let code: number | null
  try { [code] = await once(child, 'exit') } finally { clearTimeout(timer) }
  if (code !== 0) throw new Error(`${command} exited with ${code}: ${output}`)
}

async function freePort() {
  const server = createServer()
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('missing fixture port')
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()))
  return address.port
}

async function stopServer(server: ChildProcess, url: string, controlToken: string) {
  if (server.exitCode !== null || server.signalCode !== null) return
  const exited = once(server, 'exit')
  try {
    await fetch(`${url}/api/launcher/shutdown`, { method: 'POST', headers: { 'X-Raylea-Launcher-Control': controlToken }, signal: AbortSignal.timeout(2000) })
  } catch { /* startup may have failed before HTTP became available */ }
  const timer = setTimeout(() => server.kill(), 5000)
  try { await exited } finally { clearTimeout(timer) }
}

export const test = base.extend<{ server: Server; configPanel: boolean }, { serverBinary: string }>({
  configPanel: [false, { option: true }],
  serverBinary: [async ({}, use) => {
    const root = await mkdtemp(path.join(os.tmpdir(), tempPrefix))
    try {
      const binary = path.join(root, process.platform === 'win32' ? 'raylea-server.exe' : 'raylea-server')
      await run('go', ['build', '-o', binary, './cmd/raylea-server'], path.join(repoRoot, 'server'))
      await use(binary)
    } finally { await removeFixtureRoot(root) }
  }, { scope: 'worker', timeout: 120_000 }],
  server: [async ({ serverBinary, configPanel }, use, testInfo) => {
    const root = await mkdtemp(path.join(os.tmpdir(), tempPrefix))
    const configPath = path.join(root, 'config', 'user.yaml')
    const setupToken = randomBytes(32).toString('base64url')
    const controlToken = randomBytes(32).toString('base64url')
    const port = await freePort()
    const url = `http://127.0.0.1:${port}`
    let server: ChildProcess | undefined
    let output = ''
    try {
      await run(serverBinary, ['-config', configPath, 'config', 'init'], root)
      const config = YAML.parse(await readFile(configPath, 'utf8'))
      config.server = { ...config.server, host: '127.0.0.1', port }
      config.adapters = []
      await writeFile(configPath, YAML.stringify(config))
      await cp(path.join(repoRoot, 'web', 'dist'), path.join(root, 'web', 'dist'), { recursive: true })
      await cp(path.join(repoRoot, 'templates'), path.join(root, 'templates'), { recursive: true })
      if (configPanel) {
        const source = path.join(repoRoot, 'examples/plugins/example-config-panel')
        const destination = path.join(root, 'plugins/installed/example-config-panel')
        const entry = process.platform === 'win32' ? 'bin/example-config-panel.exe' : 'bin/example-config-panel'
        const targetPlatform = `${process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'macos' : 'linux'}-${process.arch === 'x64' ? 'x64' : process.arch}`
        await mkdir(path.join(destination, 'bin'), { recursive: true })
        await run('go', ['build', '-o', path.join(destination, entry), './cmd/example-config-panel'], source)
        await cp(path.join(source, 'info.json'), path.join(destination, 'info.json'))
        await cp(path.join(source, 'ui/dist'), path.join(destination, 'ui'), { recursive: true })
        await writeFile(path.join(destination, 'artifact.json'), JSON.stringify({ artifact_version: '2', target_platform: targetPlatform, entry }))
      }
      server = spawn(serverBinary, ['-config', configPath], {
        cwd: root, stdio: 'pipe', windowsHide: true,
        env: { ...process.env, RAYLEA_SETUP_TOKEN: setupToken, RAYLEA_LAUNCHER_CONTROL_TOKEN: controlToken },
      })
      const append = (chunk: Buffer) => { output = (output + String(chunk)).slice(-250_000) }
      server.stdout?.on('data', append)
      server.stderr?.on('data', append)
      await expect.poll(async () => {
        if (server?.exitCode !== null) throw new Error(`Server stopped before readiness: ${output}`)
        try { return (await fetch(`${url}/healthz`, { signal: AbortSignal.timeout(1000) })).ok } catch { return false }
      }, { timeout: 20_000 }).toBe(true)
      await use({ url, setupToken })
    } finally {
      if (server) await stopServer(server, url, controlToken)
      if (testInfo.status !== testInfo.expectedStatus) await testInfo.attach('real-server.log', { body: output, contentType: 'text/plain' })
      await removeFixtureRoot(root)
    }
  }, { timeout: 45_000 }],
  baseURL: async ({ server }, use) => { await use(server.url) },
})

export { expect }
