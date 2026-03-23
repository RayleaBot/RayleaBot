import { expect, test } from '@playwright/test'

const backendUrl = 'http://127.0.0.1:4010'

async function resetBackend(request: import('@playwright/test').APIRequestContext, initialized: boolean) {
  await request.post(`${backendUrl}/__test/reset`, {
    data: { initialized },
  })
}

test('setup flow reaches protected shell and shows websocket statuses', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '初始化管理账号', level: 1 })).toBeVisible()

  await page.getByLabel('Secret').fill('fixture-only-secret')
  await page.getByRole('button', { name: '初始化并登录' }).click()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page.getByText('Management Surface')).toBeVisible()
  await expect(page.locator('.connection-pill').filter({ hasText: 'events' })).toContainText('authenticated')
  await expect(page.locator('.connection-pill').filter({ hasText: 'tasks' })).toContainText('authenticated')
  await expect(page.locator('.connection-pill').filter({ hasText: 'logs' })).toContainText('authenticated')
})

test('login flow covers plugins, tasks, logs and config', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/login')
  await page.getByLabel('Secret').fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录' }).click()

  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.getByRole('heading', { name: '插件主流程' })).toBeVisible()
  await page.getByRole('button', { name: 'Enable' }).first().click()

  await page.getByText('weather').first().click()
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('Traceback (most recent call last): ...')).toBeVisible()

  await page.getByRole('link', { name: '任务' }).click()
  await expect(page.getByRole('heading', { name: '后台任务' })).toBeVisible()
  await page.getByText('task_plugin_install_0001').click()
  await page.getByRole('button', { name: '请求取消' }).click()
  await page.keyboard.press('Escape')

  await page.getByRole('link', { name: '日志' }).click()
  await expect(page.getByRole('heading', { name: '管理日志' })).toBeVisible()
  await expect(page.getByText('reverse websocket connection lost')).toBeVisible()
  await expect(page.getByText('authentication failed for reverse websocket')).toBeVisible()

  await page.getByRole('link', { name: '配置' }).click()
  await expect(page.getByRole('heading', { name: '配置表单' })).toBeVisible()
  const hostInput = page.locator('input').first()
  await hostInput.fill('0.0.0.0')
  await page.getByRole('button', { name: '保存配置' }).click()
  await expect(page.getByText('保存完成，仍需重启服务')).toBeVisible()
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/login')
  await page.getByLabel('Secret').fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录管理面' })).toBeVisible()
})
