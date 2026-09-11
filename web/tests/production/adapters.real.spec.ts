import { expect, test } from './real-server.fixture'

test('protocol connection creation stays local until the completed form is saved', async ({ page, request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  await page.goto('/login')
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await page.goto('/protocols')
  const writes: unknown[] = []
  page.on('request', (entry) => {
    if (entry.method() === 'PUT' && entry.url().endsWith('/api/config')) writes.push(entry.postDataJSON())
  })
  await expect(page.getByTestId('adapter-select-qqofficial')).toHaveCount(0)
  await page.getByTestId('adapter-add').click()
  await page.getByTestId('adapter-select-qqofficial').click()
  await expect(page.getByLabel('AppID', { exact: true })).toBeFocused()
  await page.getByTestId('adapter-save').click()
  await expect(page.getByText('请输入 AppSecret。')).toBeVisible()
  expect(writes).toHaveLength(0)
  await page.getByLabel('AppID', { exact: true }).fill('100000007')
  await page.keyboard.press('Escape')
  await expect(page.getByText('放弃未保存的修改？')).toBeVisible()
  await page.getByRole('button', { name: '放弃修改' }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByTestId('adapter-add')).toBeFocused()
  expect(writes).toHaveLength(0)

  await page.getByTestId('adapter-add').click()
  await page.getByTestId('adapter-select-qqofficial').click()
  await page.getByLabel('AppID', { exact: true }).fill('100000007')
  await page.getByLabel('AppSecret', { exact: true }).fill('fixture-protocol-secret')
  await page.getByTestId('adapter-save').click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByTestId('adapter-restart-notice')).toBeVisible()
  expect(writes).toHaveLength(1)
  await page.getByTestId('adapter-qq-official').click()
  await expect(page.getByLabel('AppSecret', { exact: true })).toHaveValue('********')
  await page.getByLabel('AppID', { exact: true }).fill('100000008')
  await page.getByTestId('adapter-save').click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  expect(writes).toHaveLength(2)
  expect((writes[1] as { adapters: Array<{ id: string; qqofficial?: { app_secret: string } }> }).adapters.find((entry) => entry.id === 'qq-official')?.qqofficial?.app_secret).toBe('********')
})
