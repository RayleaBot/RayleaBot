import { expect, test } from '@playwright/test'

test('real Server serves the built UI, authenticates and persists a configuration edit', async ({ page, request, baseURL }) => {
  const shell = await request.get('/plugins/example-config-panel')
  expect(shell.status()).toBe(200)
  expect(shell.headers()['content-type']).toContain('text/html')
  const missingAPI = await request.get('/api/missing-route')
  expect(missingAPI.status()).toBe(404)
  expect(missingAPI.headers()['content-type']).not.toContain('text/html')
  expect((await request.get('/api/config')).status()).toBe(401)

  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': 'A'.repeat(43), 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  const headers = { Authorization: `Bearer ${(await setup.json()).session_token}` }
  const firstScope = { kind: 'instance', source_protocol: 'qqofficial', source_adapter: 'qq', bot_id: 'bot-a' }
  const secondScope = { ...firstScope, bot_id: 'bot-b' }
  for (const scope of [firstScope, secondScope]) {
    const added = await request.post('/api/governance/blacklist/entries', { headers, data: { scope, entry_type: 'user', target_id: 'shared-id', reason: 'scope fixture' } })
    expect(added.status()).toBe(200)
  }
  const removed = await request.delete(`/api/governance/blacklist/entries/user/shared-id?${new URLSearchParams(firstScope)}`, { headers })
  expect(removed.status()).toBe(204)
  const rules = await (await request.get('/api/governance/blacklist', { headers })).json()
  expect(rules.user_entries).toHaveLength(1)
  expect(rules.user_entries[0].scope).toEqual(secondScope)

  await page.goto('/login')
  await page.getByLabel('管理员账号', { exact: true }).fill('admin')
  await page.getByLabel('管理员密钥', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  await page.goto('/access-lists')
  const blacklist = page.getByTestId('access-lists-blacklist-card')
  await page.getByTestId('access-lists-blacklist-add-btn').click()
  await blacklist.getByRole('combobox', { name: '适用范围' }).click()
  await page.getByRole('option', { name: 'QQ 官方 · 指定机器人' }).click()
  await page.getByTestId('blacklist-draft-target-id').fill('shared-id')
  await page.getByTestId('blacklist-draft-reason').fill('created through UI')
  await page.getByTestId('blacklist-draft-save').click()
  await expect(blacklist.getByRole('alert')).toContainText('请输入适配器与机器人 ID')
  await blacklist.getByRole('textbox', { name: '适配器 ID' }).fill('qq')
  await blacklist.getByRole('textbox', { name: '机器人 ID' }).fill('bot-a')
  const scopedSave = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/api/governance/blacklist/entries'))
  await page.getByTestId('blacklist-draft-save').click()
  const scopedResponse = await scopedSave
  expect(scopedResponse.status()).toBe(200)
  expect((await scopedResponse.json()).scope).toEqual(firstScope)
  await page.reload()
  await expect(blacklist).toContainText('created through UI')
  await expect(blacklist).toContainText('scope fixture')

  await page.goto('/permission-policy')
  const admins = page.getByTestId('permission-policy-super-admins').locator('input')
  await admins.fill('10002')
  await admins.press('Enter')
  const saved = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/config'))
  await page.getByTestId('permission-policy-save').click()
  expect((await saved).status()).toBe(200)
  await expect(page.getByTestId('permission-policy-unsaved-status')).toHaveCount(0)
  await page.reload()
  await expect(page.getByTestId('permission-policy-super-admins')).toContainText('10002')

  await page.goto('/logs')
  await expect(page.locator('.logs-row').first()).toBeVisible()
  await page.locator('.logs-row').first().click()
  await expect(page.getByRole('dialog')).toBeVisible()
})

// Shut the fixture down through its real lifecycle before Playwright terminates
// the webServer wrapper, allowing Windows to remove the temporary SQLite files.
test.afterAll(async ({ baseURL }) => {
  const response = await fetch(`${baseURL}/api/launcher/shutdown`, {
    method: 'POST', headers: { 'X-Raylea-Launcher-Control': 'B'.repeat(43) }, signal: AbortSignal.timeout(5000),
  })
  expect(response.status).toBe(202)
  await expect.poll(async () => {
    try { await fetch(`${baseURL}/healthz`, { signal: AbortSignal.timeout(1000) }); return true } catch { return false }
  }).toBe(false)
})
