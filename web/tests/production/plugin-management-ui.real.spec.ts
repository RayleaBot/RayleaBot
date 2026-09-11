import { type Page } from '@playwright/test'
import { expect, test } from './real-server.fixture'

test.use({ configPanel: true })

async function openPluginPage(page: Page) {
  await page.goto('/login')
  await page.getByLabel('管理员账号', { exact: true }).fill('admin')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/$/)
  await page.goto('/plugins/example-config-panel?panel=management-ui&management_page=config')
  await expect(page.getByRole('heading', { name: '插件：Example Config Panel', level: 1 })).toBeVisible()
}

test.beforeEach(async ({ request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  const headers = { Authorization: `Bearer ${(await setup.json()).session_token}` }
  const settings = await request.put('/api/plugins/example-config-panel/settings', { headers, data: { values: { default_city: '上海', unit: 'fahrenheit' } } })
  expect(settings.status()).toBe(200)
  const secrets = await request.put('/api/plugins/example-config-panel/secrets', { headers, data: { values: { api_key: 'fixture-only-plugin-secret' } } })
  expect(secrets.status()).toBe(200)
})

test('built plugin pages use the real Server port and complete the isolated handshake', async ({ page, request, baseURL }) => {
  await openPluginPage(page)
  const frame = page.getByTestId('plugin-management-ui-frame')
  await expect(frame).toHaveAttribute('src', /^http:\/\/p-[a-f0-9]{16}\.plugins\.localhost:\d+\/index\.html\?/)
  const frameURL = new URL((await frame.getAttribute('src'))!)
  expect(frameURL.port).toBe(new URL(baseURL!).port)
  const isolated = await request.get('/api/config', { headers: { Host: frameURL.host } })
  expect(isolated.status()).toBe(404)
  expect(isolated.headers()['set-cookie']).toBeUndefined()
  expect(isolated.headers()['access-control-allow-origin']).toBeUndefined()
  const plugin = page.frameLocator('[data-testid="plugin-management-ui-frame"]')
  await expect(plugin.getByTestId('settings-status')).toHaveText('配置已加载')
  await expect(plugin.getByTestId('default-city-input')).toHaveValue('上海')
  await expect(page.locator('.plugin-management-ui-host [aria-busy="true"]')).toHaveCount(0)
  await expect(plugin.getByTestId('secret-status')).toHaveText('API 密钥已配置')
  const configured = await page.request.get('/api/plugins/example-config-panel/secrets')
  expect(await configured.json()).toEqual({ plugin_id: 'example-config-panel', configured: { api_key: true } })
  await plugin.getByTestId('default-city-input').fill('广州')
  const save = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/plugins/example-config-panel/settings'))
  await plugin.getByTestId('save-settings').click()
  expect((await save).status()).toBe(200)
  await page.reload()
  await expect(plugin.getByTestId('default-city-input')).toHaveValue('广州')
})

test('a failed frame shows a contained recovery state and retry loads a new session', async ({ page }) => {
  const frameURL = /\.plugins\.localhost:\d+\/index\.html/
  await page.route(frameURL, (route) => route.abort())
  await openPluginPage(page)
  const recovery = page.getByRole('alert').filter({ hasText: '插件页面未打开' })
  await expect(recovery).toBeVisible({ timeout: 12_000 })
  await expect(page.getByTestId('plugin-management-ui-frame')).toHaveCount(0)
  await page.unroute(frameURL)
  await recovery.getByRole('button', { name: '重试', exact: true }).click()
  await expect(page.frameLocator('[data-testid="plugin-management-ui-frame"]').getByTestId('settings-status')).toHaveText('配置已加载')
  await expect(recovery).toHaveCount(0)
})
