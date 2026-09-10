import { expect, test } from './real-server.fixture'

test.beforeEach(async ({ page, request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  await page.goto('/login')
  await page.getByLabel('管理员账号', { exact: true }).fill('admin')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { level: 1, name: '系统状态' })).toBeVisible()
})

async function scrollConfigSectionIntoView(page: import('@playwright/test').Page, sectionKey: string) {
  const category = sectionKey === 'runtime' ? '运行与请求' : '账号与调度'
  await page.getByRole('tab', { name: category }).click()
  const advanced = page.locator('.config-advanced__trigger')
  if (await advanced.getAttribute('data-state') === 'closed') await advanced.click()
  await page.locator(`[data-section-key="${sectionKey}"]`).first().scrollIntoViewIfNeeded()
}

async function fillRateLimit(
  page: import('@playwright/test').Page,
  label: string,
  count: string,
  windowValue: string,
  unit?: '秒' | '分钟' | '小时',
) {
  await page.getByLabel(`${label} 次数`).fill(count)
  await page.getByLabel(`${label} 时间窗口`).fill(windowValue)
  if (unit) {
    await page.getByLabel(`${label} 单位`).click()
    await page.getByRole('option', { name: unit, exact: true }).click()
  }
}

test('config page edits general IPC rate limit with split inputs', async ({ page, request }) => {

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()

  await scrollConfigSectionIntoView(page, 'runtime')
  await fillRateLimit(page, 'IPC 突发限制', '180', '5')
  await expect(page.getByText('5 秒内最多 180 次')).toBeVisible()
  await expect(page.locator('#config-save-status')).toContainText('含重启后生效的更改')

  const [saved] = await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().endsWith('/api/config')
    )),
    page.getByRole('button', { name: '保存更改' }).click(),
  ])
  expect((await saved.json()).apply_effects.restart_required_fields).toContain('runtime.ipc_action_burst_limit')

  await scrollConfigSectionIntoView(page, 'runtime')
  await expect(page.getByText('5 秒内最多 180 次')).toBeVisible()
})

test('config page saves restart-required Douyin browser settings', async ({ page, request }) => {

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  await scrollConfigSectionIntoView(page, 'third-party-accounts')

  await page.getByRole('spinbutton', { name: 'CK 自动检查间隔' }).fill('720')
  await expect(page.getByRole('combobox', { name: '抖音登录浏览器模式' })).toContainText('自动选择')
  await page.getByRole('combobox', { name: '抖音登录浏览器模式' }).click()
  await page.getByRole('option', { name: '远程 CDP', exact: true }).click()
  await page.getByRole('textbox', { name: '抖音远程调试地址' }).fill('http://127.0.0.1:9222')

  const configResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'PUT'
    && response.url().endsWith('/api/config')
  ))
  await page.getByRole('button', { name: '保存更改' }).click()
  const configResponse = await configResponsePromise
  const responseBody = await configResponse.json()
  expect(responseBody.restart_required).toBe(true)
  expect(responseBody.apply_effects.restart_required_fields).toEqual(expect.arrayContaining([
    'third_party_accounts.douyin_login.browser_mode',
    'third_party_accounts.douyin_login.remote_debugging_url',
  ]))
  expect(responseBody.apply_effects.applied_now).toContain('third_party_accounts.credential_check_interval_minutes')

  await page.reload()
  await scrollConfigSectionIntoView(page, 'third-party-accounts')
  await expect(page.getByRole('spinbutton', { name: 'CK 自动检查间隔' })).toHaveValue('720')
  await expect(page.getByRole('combobox', { name: '抖音登录浏览器模式' })).toContainText('远程 CDP')
  await expect(page.getByRole('textbox', { name: '抖音远程调试地址' })).toHaveValue('http://127.0.0.1:9222')
})
