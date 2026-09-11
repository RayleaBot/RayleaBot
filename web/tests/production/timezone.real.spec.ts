import { expect, test } from './real-server.fixture'

test.use({ timezoneId: 'America/Los_Angeles' })

test('timezone picker covers Windows offsets, supports the keyboard, and preserves the active zone until restart', async ({ page, request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await page.clock.setFixedTime(new Date('2026-07-15T12:00:00Z'))
  await page.goto('/config')
  await page.getByRole('tab', { name: '账号与调度' }).click()
  const trigger = page.getByRole('button', { name: '时区', exact: true })
  await expect(trigger).toContainText('UTC+08:00')
  await expect(trigger).toContainText('上海')
  await trigger.focus()
  await page.keyboard.press('ArrowDown')
  const search = page.getByRole('combobox', { name: '搜索时区' })
  await expect(search).toBeFocused()
  await search.fill('Antarctica')
  await expect(page.getByRole('option')).toHaveCount(0)
  for (const [offset, id] of [['UTC-12', 'Etc/GMT+12'], ['UTC+14', 'Pacific/Kiritimati'], ['UTC+4:30', 'Asia/Kabul'], ['UTC+8:45', 'Australia/Eucla'], ['UTC+12:45', 'Pacific/Chatham']]) {
    await search.fill(offset)
    await expect(page.locator(`[role=option][title="${id}"]`)).toBeVisible()
  }
  await search.fill('UTC+5:45')
  await expect(page.getByRole('option')).toHaveCount(1)
  await expect(page.getByRole('option').first()).toContainText('UTC+05:45')
  await search.fill('Europe/Paris')
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Enter')
  await expect(trigger).toContainText('巴黎')
  await expect(trigger).toBeFocused()
  const saved = page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/config'))
  await page.getByRole('button', { name: '保存更改', exact: true }).click()
  const response = await saved
  expect(response.request().postDataJSON().scheduler.timezone).toBe('Europe/Paris')
  const result = await response.json()
  expect(result.effective_timezone).toBe('Asia/Shanghai')
  expect(result.apply_effects.restart_required_fields).toContain('scheduler.timezone')
})
