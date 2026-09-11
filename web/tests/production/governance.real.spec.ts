import { expect, test } from './real-server.fixture'

function governanceEntryCard(
  container: import('@playwright/test').Locator,
  targetId: string,
) {
  return container.locator('tr').filter({ hasText: targetId }).first()
}

test('access lists page manages blacklist and whitelist entries', async ({ page, request, baseURL, server }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  const authHeaders = { Authorization: `Bearer ${(await setup.json()).session_token}` }
  for (const list of ['blacklist', 'whitelist']) {
    for (const [entry_type, target_id, reason] of [['user', '10001', '值班账号'], ['group', '20002', '核心服务群']]) {
      const added = await request.post(`/api/governance/${list}/entries`, { headers: authHeaders, data: {
        scope: { kind: 'global', source_protocol: 'onebot11', source_adapter: '', bot_id: '' }, entry_type, target_id, reason,
      } })
      expect(added.status()).toBe(200)
    }
  }
  expect((await request.put('/api/governance/whitelist/state', { headers: authHeaders, data: { enabled: true } })).status()).toBe(200)
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  for (let index = 0; index < 10; index += 1) {
    await request.post(`${baseURL}/api/governance/whitelist/entries`, {
      headers: authHeaders,
      data: { scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
        entry_type: 'user',
        target_id: `31${String(index + 1).padStart(3, '0')}`,
        reason: `扩展白名单${index + 1}`,
      },
    })
  }
  for (let index = 0; index < 10; index += 1) {
    await request.post(`${baseURL}/api/governance/blacklist/entries`, {
      headers: authHeaders,
      data: { scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
        entry_type: 'user',
        target_id: `41${String(index + 1).padStart(3, '0')}`,
        reason: `扩展黑名单${index + 1}`,
      },
    })
  }

  await page.goto('/access-lists')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()

  const whitelistCard = page.getByTestId('access-lists-whitelist-card')
  const blacklistCard = page.getByTestId('access-lists-blacklist-card')
  await expect(whitelistCard).toContainText('10001')
  await expect(whitelistCard).toContainText('值班账号')
  await expect(whitelistCard).toContainText('31010')

  await expect(blacklistCard).toContainText('10001')
  await expect(blacklistCard).toContainText('41010')

  await page.getByTestId('access-lists-blacklist-add-btn').click()
  await page.getByTestId('blacklist-draft-target-id').fill('30003')
  await page.getByTestId('blacklist-draft-reason').fill('临时封禁')
  const blacklistAddResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().endsWith('/api/governance/blacklist/entries')
  ))
  await page.getByTestId('blacklist-draft-save').click()
  expect((await blacklistAddResponsePromise).status()).toBe(200)
  const addedBlacklistRow = governanceEntryCard(blacklistCard, '30003')
  await expect(addedBlacklistRow).toBeVisible()
  await expect(addedBlacklistRow).toContainText('临时封禁')

  await governanceEntryCard(blacklistCard, '30003').getByRole('button', { name: '移除' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: '移除', exact: true }).click()
  await expect(blacklistCard).not.toContainText('30003')

  await page.getByTestId('access-lists-whitelist-add-btn').click()
  await page.getByTestId('whitelist-draft-target-id').fill('30003')
  await page.getByTestId('whitelist-draft-reason').fill('临时放行')
  const whitelistAddResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().endsWith('/api/governance/whitelist/entries')
  ))
  await page.getByTestId('whitelist-draft-save').click()
  expect((await whitelistAddResponsePromise).status()).toBe(200)
  const addedWhitelistRow = governanceEntryCard(whitelistCard, '30003')
  await expect(addedWhitelistRow).toBeVisible()
  await expect(addedWhitelistRow).toContainText('临时放行')

  await page.getByTestId('access-lists-whitelist-enabled').dispatchEvent('click')
  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'false')

  for (const targetId of ['10001', '30003']) {
    await governanceEntryCard(whitelistCard, targetId).getByRole('button', { name: '移除' }).click()
    await page.getByRole('alertdialog').getByRole('button', { name: '移除', exact: true }).click()
    await expect(whitelistCard).not.toContainText(targetId)
  }

  await whitelistCard.locator('.access-lists-toolbar__filter').click()
  await page.getByRole('option', { name: '群', exact: true }).click()
  await expect(whitelistCard).toContainText('20002')
  await expect(whitelistCard).toContainText('核心服务群')
  await governanceEntryCard(whitelistCard, '20002').getByRole('button', { name: '移除' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: '移除', exact: true }).click()
  await expect(whitelistCard).not.toContainText('20002')

  for (let index = 0; index < 10; index += 1) {
    await request.delete(`${baseURL}/api/governance/whitelist/entries/user/${encodeURIComponent(`31${String(index + 1).padStart(3, '0')}`)}?kind=global&source_protocol=onebot11&source_adapter=&bot_id=`, {
      headers: authHeaders,
    })
  }
  await page.goto('/access-lists')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
  await expect(whitelistCard).not.toContainText('31010')

  await page.getByTestId('access-lists-whitelist-enabled').dispatchEvent('click')
  const confirmDialog = page.getByRole('alertdialog', { name: '确认启用空白名单' })
  await expect(confirmDialog).toBeVisible()

  await confirmDialog.getByRole('button', { name: '确认启用' }).dispatchEvent('click')

  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')

  await page.reload()
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')
})
