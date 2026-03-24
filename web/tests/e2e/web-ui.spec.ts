import { expect, test } from '@playwright/test'

const backendUrl = 'http://127.0.0.1:4010'

async function resetBackend(
  request: import('@playwright/test').APIRequestContext,
  initialized: boolean,
  failures: Record<string, boolean> = {},
) {
  await request.post(`${backendUrl}/__test/reset`, {
    data: { initialized, failures },
  })
}

async function closeSocket(
  request: import('@playwright/test').APIRequestContext,
  channel: 'events' | 'tasks' | 'logs' | 'plugin_console',
) {
  await request.post(`${backendUrl}/__test/socket-close`, {
    data: { channel },
  })
}

async function login(page: import('@playwright/test').Page) {
  await page.goto('/login')
  await page.getByLabel('Secret').fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
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

test('launcher token query admits a session and clears the URL token', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/token=/)
})

test('invalid launcher token falls back to login and clears the URL token', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/?token=invalid_launcher_token')

  await expect(page.getByRole('heading', { name: '登录管理面', level: 1 })).toBeVisible()
  await expect(page.getByText('Launcher 登录令牌无效或已过期')).toBeVisible()
  await expect(page).not.toHaveURL(/token=/)
})

test('setup-required flow ignores launcher token query', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '初始化管理账号', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/token=/)
})

test('plugin management flow covers install, grants and console recovery', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.getByRole('heading', { name: '插件主流程' })).toBeVisible()

  await page.getByRole('button', { name: '安装插件' }).click()
  await page.getByLabel('Server Path').fill('C:/plugins/weather.zip')
  await page.getByRole('button', { name: '开始安装' }).click()

  await expect(page.getByRole('heading', { name: '后台任务' })).toBeVisible()
  await expect(page.getByText('task_plugin_install_0001').first()).toBeVisible()

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('未验证来源')).toBeVisible()
  await expect(page.getByText('plugins/installed')).toBeVisible()
  await expect(page.getByText('命令冲突')).toBeVisible()

  await page.getByRole('button', { name: '新增授权' }).click()
  await page.getByLabel('Capability').fill('storage.file')
  await page.getByLabel('Expires At (UTC RFC3339)').fill('2026-03-30T10:05:00Z')
  await page.getByRole('button', { name: '保存授权' }).click()
  await expect(page.getByText('storage.file')).toBeVisible()

  await page.getByRole('button', { name: '撤销' }).last().click()
  await expect(page.getByText('storage.file')).toHaveCount(0)

  await expect(page.getByText('Traceback (most recent call last): ...').first()).toBeVisible()
  await page.getByRole('button', { name: '清空输出' }).click()
  await expect(page.getByText('等待 console 输出')).toBeVisible()
  await closeSocket(request, 'plugin_console')
  await page.getByRole('button', { name: '重连' }).click()
  await expect(page.getByText('Traceback (most recent call last): ...').first()).toBeVisible()
})

test('status page can start backup tasks and export diagnostics', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '创建在线备份' }).click()
  await expect(page.getByRole('heading', { name: '后台任务' })).toBeVisible()
  await expect(page.getByText('task_backup_create_0001').first()).toBeVisible()

  const downloadPromise = page.waitForEvent('download')
  await page.goto('/')
  await page.getByRole('button', { name: '导出诊断包' }).click()
  const download = await downloadPromise
  expect(await download.suggestedFilename()).toContain('rayleabot-diagnostics')
})

test('error recovery covers retry, invalid grant expiry and uninstall failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginsListOnce: true,
    failUninstallOnce: true,
  })
  await login(page)

  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.getByText('插件列表读取失败')).toBeVisible()
  await page.getByRole('button', { name: '重试' }).click()
  await expect(page.getByText('weather').first()).toBeVisible()

  await page.getByText('weather').first().click()
  await page.getByRole('button', { name: '新增授权' }).click()
  await page.getByLabel('Capability').fill('http.request')
  await page.getByLabel('Expires At (UTC RFC3339)').fill('not-a-timestamp')
  await page.getByRole('button', { name: '保存授权' }).click()
  await expect(page.getByText('expires_at must be a future UTC RFC3339 timestamp')).toBeVisible()
  await page.getByRole('button', { name: '取消' }).click()

  await page.getByRole('button', { name: 'Uninstall' }).click()
  await page.getByRole('button', { name: '确认卸载' }).click()
  await expect(page.getByText('必要运行时资源缺失')).toBeVisible()
})

test('shutdown flow shows the draining banner', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '关闭服务' }).click({ force: true })
  await page.getByRole('button', { name: '确认关闭' }).click()

  await expect(page.getByText('服务正在停止')).toBeVisible()
  await expect(page.getByText('平台已接受 shutdown 请求')).toBeVisible()
})

test('mobile navigation and card layouts remain usable', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 390, height: 844 })

  await login(page)

  await page.getByRole('button', { name: '导航' }).click({ force: true })
  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.locator('.mobile-data-card').first()).toBeVisible()

  await page.goto('/logs')
  await expect(page.locator('.mobile-data-card').first()).toBeVisible()
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录管理面' })).toBeVisible()
})
