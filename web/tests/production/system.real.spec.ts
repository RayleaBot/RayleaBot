import { expect, test } from './real-server.fixture'

test('status page creates a backup task and exports a diagnostics archive', async ({ page, request, server, baseURL }) => {
  const setup = await request.post('/api/setup/admin', {
    headers: { Origin: baseURL!, 'X-Raylea-Setup-Token': server.setupToken, 'X-Raylea-Session-Transport': 'bearer' },
    data: { identifier: 'admin', secret: 'fixture-only-secret' },
  })
  expect(setup.status()).toBe(200)
  const headers = { Authorization: `Bearer ${(await setup.json()).session_token}` }

  await page.goto('/login')
  await page.getByLabel('用户名', { exact: true }).fill('admin')
  await page.getByLabel('密码', { exact: true }).fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  const accepted = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/api/system/backup'))
  await page.getByRole('button', { name: '创建备份' }).click()
  const backup = await accepted
  expect(backup.ok()).toBe(true)
  const taskId = (await backup.json()).task_id
  const taskStatus = async () => (await (await request.get(`/api/system/tasks/${taskId}`, { headers })).json()).status
  await expect.poll(taskStatus, { timeout: 30_000 }).toBe('succeeded')

  const download = page.waitForEvent('download')
  await page.getByRole('button', { name: '导出诊断包' }).click()
  expect((await download).suggestedFilename()).toMatch(/\.zip$/)
})
