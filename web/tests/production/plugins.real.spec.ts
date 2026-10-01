import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { mkdtemp, readdir, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { expect, test } from './real-server.fixture'

test.use({ buildInfo: true })

const repoRoot = path.resolve(import.meta.dirname, '../../..')

function currentPlatform() {
  const system = process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'macos' : 'linux'
  return `${system}-${process.arch === 'x64' ? 'x64' : process.arch}`
}

async function buildEchoPlugin(output: string) {
  const child = spawn('go', [
    'run', './cmd/raylea-plugin', 'build-go',
    '--plugin', path.join(repoRoot, 'server/tests/testutil/testdata/echo-plugin'),
    '--backend', './cmd/echo',
    '--target', currentPlatform(),
    '--out', output,
  ], { cwd: path.join(repoRoot, 'sdk/go'), stdio: 'pipe', windowsHide: true, env: { ...process.env, GOWORK: 'off' } })
  let log = ''
  child.stdout?.on('data', chunk => { log += String(chunk) })
  child.stderr?.on('data', chunk => { log += String(chunk) })
  const [code] = await once(child, 'exit')
  if (code !== 0) throw new Error(`plugin build failed: ${log}`)
  const archive = (await readdir(output)).find(name => name.endsWith('.zip'))
  if (!archive) throw new Error(`plugin build produced no archive: ${log}`)
  return path.join(output, archive)
}

test('an installed plugin archive can be enabled and disabled through the built UI', async ({ page, request, server, baseURL }) => {
  test.setTimeout(180_000)
  const output = await mkdtemp(path.join(os.tmpdir(), 'raylea-plugin-archive-'))
  try {
    const archive = await buildEchoPlugin(output)
    const setup = await request.post('/api/setup/admin', {
      headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
      data: { identifier: 'admin', secret: 'fixture-only-secret' },
    })
    expect(setup.status()).toBe(200)
    const headers = { Authorization: `Bearer ${(await setup.json()).session_token}` }
    const pluginState = async () => {
      const response = await request.get('/api/plugins/raylea.echo', { headers })
      return response.ok() ? (await response.json()).plugin.state : `http-${response.status()}`
    }

    await page.goto('/login')
    await page.getByLabel('用户名', { exact: true }).fill('admin')
    await page.getByLabel('密码', { exact: true }).fill('fixture-only-secret')
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

    await page.goto('/plugins')
    await page.locator('.plugins-toolbar').getByRole('button', { name: '安装插件' }).click()
    const dialog = page.getByRole('dialog', { name: '安装插件' })
    await dialog.getByRole('textbox').fill(archive)
    await dialog.getByRole('checkbox', { name: /我信任此插件包/ }).check()
    await dialog.getByRole('button', { name: '开始安装' }).click()
    await expect(page.getByText('插件安装完成', { exact: true }).first()).toBeVisible({ timeout: 60_000 })
    await expect.poll(pluginState).toBe('disabled')

    const power = page.getByTestId('plugin-enable-button-raylea.echo')
    await power.click()
    await expect.poll(pluginState, { timeout: 30_000 }).toBe('running')
    await power.click()
    await expect.poll(pluginState, { timeout: 30_000 }).toBe('disabled')
  } finally {
    await rm(output, { recursive: true, force: true })
  }
})
