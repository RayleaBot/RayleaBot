import { expect, test, type Page } from '@playwright/test'

async function openPluginPage(page: Page) {
  await page.goto('/login')
  await page.getByLabel('管理员账号', { exact: true }).fill('admin')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL('http://127.0.0.1:4010/')
  await page.goto('/plugins/example-config-panel?panel=management-ui&management_page=config')
  await page.getByRole('button', { name: '确认并打开' }).click()
}

test.beforeEach(async ({ request }) => {
  await request.post('/__test/reset', { data: { initialized: true } })
})

test('production hosting serves namespaced plugin routes without turning API errors into HTML', async ({ request }) => {
  const page = await request.get('/plugins/raylea.subscription-hub')
  expect(page.status()).toBe(200)
  expect(page.headers()['content-type']).toContain('text/html')
  expect(await page.text()).toContain('<div id="app">')
  const missingAPI = await request.get('/api/missing-route')
  expect(missingAPI.status()).toBe(404)
  expect(missingAPI.headers()['content-type']).toContain('application/json')
})

test('built plugin pages use the deployed server port and complete the isolated handshake', async ({ page }) => {
  await openPluginPage(page)
  const frame = page.getByTestId('plugin-management-ui-frame')
  await expect(frame).toHaveAttribute('src', /^http:\/\/p-[a-f0-9]{16}\.plugins\.localhost:4010\/index\.html\?/)
  const plugin = page.frameLocator('[data-testid="plugin-management-ui-frame"]')
  await expect(plugin.getByTestId('settings-status')).toHaveText('配置已加载')
  await expect(plugin.getByTestId('default-city-input')).toHaveValue('上海')
  await expect(page.locator('.plugin-management-ui-host [aria-busy="true"]')).toHaveCount(0)
  await expect(plugin.getByTestId('secret-status')).toHaveText('API 密钥已配置')
})

test('a failed frame shows a contained recovery state and retry loads a new session', async ({ page }) => {
  const frameURL = /\.plugins\.localhost:4010\/index\.html/
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
