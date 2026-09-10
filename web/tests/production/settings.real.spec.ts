import { expect, test } from './real-server.fixture'

test('a fresh Server enforces cookie CSRF and persists plugin settings through the built UI', async ({ page, request, server, baseURL }) => {
  expect((await (await request.get('/api/setup/status')).json()).initialized).toBe(false)
  expect((await request.get('/api/system/scheduler/jobs')).status()).toBe(401)
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  const headers = { Authorization: `Bearer ${(await setup.json()).session_token}` }
  const adapters = await (await request.get('/api/adapters', { headers })).json()
  expect(adapters.adapters).toEqual([])
  const jobs = await (await request.get('/api/system/scheduler/jobs', { headers })).json()
  expect(jobs.items).toEqual([])

  await page.goto('/plugins/settings')
  await expect(page).toHaveURL(/\/login\?redirect=/)
  await page.getByLabel('管理员账号', { exact: true }).fill('admin')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page).toHaveURL(/\/plugins\/settings$/)
  await expect(page.getByRole('heading', { name: '全局插件设置', level: 1 })).toBeVisible()
  const denied = await page.evaluate(async () => {
    const current = await (await fetch('/api/config')).json()
    return (await fetch('/api/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(current.config) })).status
  })
  expect(denied).toBe(401)

  const commandPrefixes = page.getByTestId('plugin-settings-command-prefixes').locator('input')
  await commandPrefixes.fill('!')
  await commandPrefixes.press('Enter')
  await page.getByLabel('插件日志速率限制 次数').fill('300')
  await page.getByLabel('插件日志速率限制 时间窗口').fill('10')
  await page.getByLabel('插件工作目录软上限（MB）').fill('512')
  const saved = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/config'))
  await page.getByTestId('plugin-settings-save').click()
  expect((await saved).status()).toBe(200)
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toHaveCount(0)
  await page.reload()
  await expect(page.getByTestId('plugin-settings-command-prefixes')).toContainText('!')
  await expect(page.getByLabel('插件工作目录软上限（MB）')).toHaveValue('512')
  await expect(page.getByLabel('插件日志速率限制 次数')).toHaveValue('300')
  await expect(page.getByLabel('插件日志速率限制 时间窗口')).toHaveValue('10')

  await page.goto('/scheduler')
  await expect(page.getByRole('heading', { level: 1, name: '定时任务' })).toBeVisible()
  await expect(page.getByRole('button', { name: '立即执行', exact: true })).toHaveCount(0)
})
