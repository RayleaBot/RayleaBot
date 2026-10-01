import { expect, test } from './real-server.fixture'

test.beforeEach(async ({ page, request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  await page.goto('/login')
  await page.getByLabel('用户名', { exact: true }).fill('admin')
  await page.getByLabel('密码', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { level: 1, name: '系统状态' })).toBeVisible()
})

async function scrollConfigSectionIntoView(page: import('@playwright/test').Page, sectionKey: string) {
  const category = sectionKey === 'runtime' ? '插件运行' : '调度'
  await page.getByRole('tab', { name: category }).click()
  const advanced = page.locator('.config-advanced__trigger')
  if (await advanced.getAttribute('data-state') === 'closed') await advanced.click()
  await page.locator(`[data-section-key="${sectionKey}"]`).first().scrollIntoViewIfNeeded()
}

test('config page reports restart-required runtime changes from the server', async ({ page }) => {
  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()

  await scrollConfigSectionIntoView(page, 'runtime')
  await page.getByLabel('插件待处理事件上限', { exact: true }).fill('32')
  await expect(page.locator('#config-save-status')).toContainText('含重启后生效的更改')

  const [saved] = await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().endsWith('/api/config')
    )),
    page.getByRole('button', { name: '保存更改' }).click(),
  ])
  expect((await saved.json()).apply_effects.restart_required_fields).toContain('runtime.max_pending_events_per_plugin')

  await page.reload()
  await scrollConfigSectionIntoView(page, 'runtime')
  await expect(page.getByLabel('插件待处理事件上限', { exact: true })).toHaveValue('32')
})
