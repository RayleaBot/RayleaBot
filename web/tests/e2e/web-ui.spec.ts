import { expect, test } from '@playwright/test'

const backendUrl = 'http://127.0.0.1:4010'
const futureGrantExpiry = '2099-03-30T10:05:00Z'

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
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
}

function pluginRows(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .el-table__body tbody tr')
}

function taskRows(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .el-table__body tbody tr')
}

function logRows(page: import('@playwright/test').Page) {
  return page.locator('.logs-data-table .el-table__body tbody tr')
}

function pluginScroller(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .el-table__body-wrapper')
}

function taskScroller(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .el-table__body-wrapper')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-data-table .el-table__body-wrapper')
}

test('setup flow reaches protected shell and shows websocket statuses', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '创建管理员账号', level: 1 })).toBeVisible()

  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: '创建并进入管理界面' }).click()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page.getByText('管理控制台')).toBeVisible()
  await expect(page.locator('.connection-pill').filter({ hasText: '事件流' })).toContainText('已认证')
  await expect(page.locator('.connection-pill').filter({ hasText: '任务流' })).toContainText('已认证')
  await expect(page.locator('.connection-pill').filter({ hasText: '日志流' })).toContainText('已认证')
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

  await expect(page.getByRole('heading', { name: '登录', level: 1 })).toBeVisible()
  await expect(page.getByText('自动登录未完成，请手动登录。')).toBeVisible()
  await expect(page).not.toHaveURL(/token=/)
})

test('setup-required flow ignores launcher token query', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '创建管理员账号', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/token=/)
})

test('plugin management flow covers install, grants and console recovery', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.locator('#app-main').getByRole('heading', { name: '插件', level: 1 })).toBeVisible()
  await expect(pluginRows(page).first()).toBeVisible()

  await page.getByRole('button', { name: '安装插件' }).click()
  await page.getByLabel('服务器路径').fill('C:/plugins/weather.zip')
  await page.getByRole('button', { name: '开始安装' }).click()

  await expect(page.locator('#app-main').getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await expect(page.getByText('task_plugin_install_0001').first()).toBeVisible()

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('未验证来源')).toBeVisible()
  await expect(page.getByText('plugins/installed')).toBeVisible()
  await expect(page.getByText('命令冲突')).toBeVisible()

  await page.getByRole('button', { name: '新增授权' }).click()
  await page.getByLabel('能力标识').fill('storage.file')
  await page.getByLabel('过期时间（UTC RFC3339）').fill(futureGrantExpiry)
  await page.getByRole('button', { name: '保存授权' }).click()
  await expect(page.getByText('storage.file')).toBeVisible()

  await page.getByRole('button', { name: '撤销' }).last().click()
  await expect(page.getByText('storage.file')).toHaveCount(0)

  await expect(page.getByText('Traceback (most recent call last): ...').first()).toBeVisible()
  await page.getByRole('button', { name: '清空输出' }).click()
  await expect(page.getByText('等待控制台输出')).toBeVisible()
  await closeSocket(request, 'plugin_console')
  await page.getByRole('button', { name: '重新连接' }).click()
  await expect(page.getByText('Traceback (most recent call last): ...').first()).toBeVisible()
})

test('desktop list viewports fill the remaining shell height without overlapping rows', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1600, height: 1400 })

  await login(page)

  await page.goto('/plugins')
  const pluginsBody = pluginScroller(page)
  await expect(pluginsBody).toBeVisible()
  expect((await pluginsBody.getAttribute('style')) ?? '').not.toContain('560px')
  expect((await pluginsBody.getAttribute('style')) ?? '').not.toContain('620px')

  const pluginFirst = await pluginRows(page).nth(0).boundingBox()
  const pluginSecond = await pluginRows(page).nth(1).boundingBox()
  expect(pluginFirst).not.toBeNull()
  expect(pluginSecond).not.toBeNull()
  expect(pluginFirst!.y + pluginFirst!.height).toBeLessThanOrEqual(pluginSecond!.y)
  expect(pluginFirst!.height).toBeLessThan(170)
  await expect(pluginRows(page).first()).not.toContainText('discovered')

  await page.getByRole('button', { name: '查看概要' }).nth(1).click()
  await expect(page.getByRole('dialog')).toContainText('显示状态')
  await expect(page.getByRole('dialog')).toContainText('运行中')
  await expect(page.getByRole('dialog')).not.toContainText('discovered')
  await page.keyboard.press('Escape')

  await page.setViewportSize({ width: 1600, height: 900 })
  await expect(pluginsBody).toBeVisible()
  expect((await pluginsBody.getAttribute('style')) ?? '').not.toContain('560px')
  expect((await pluginsBody.getAttribute('style')) ?? '').not.toContain('620px')

  await page.goto('/tasks')
  const tasksBody = taskScroller(page)
  await expect(tasksBody).toBeVisible()
  expect((await tasksBody.getAttribute('style')) ?? '').not.toContain('560px')
  expect((await tasksBody.getAttribute('style')) ?? '').not.toContain('620px')
  await expect(taskRows(page).first()).toBeVisible()

  await page.goto('/logs')
  const logsBody = logScroller(page)
  await expect(logsBody).toBeVisible()
  expect((await logsBody.getAttribute('style')) ?? '').not.toContain('560px')
  expect((await logsBody.getAttribute('style')) ?? '').not.toContain('620px')
  await expect(logRows(page).first()).toBeVisible()
})

test('status page can start backup tasks and export diagnostics', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '创建备份' }).click()
  await expect(page.locator('#app-main').getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await expect(page.getByText('task_backup_create_0001').first()).toBeVisible()

  const downloadPromise = page.waitForEvent('download')
  await page.goto('/')
  await page.getByRole('button', { name: '导出诊断包' }).click()
  const download = await downloadPromise
  expect(await download.suggestedFilename()).toContain('rayleabot-diagnostics')
})

test('status page can submit render previews and show the artifact', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '图片预览' }).click()
  await page.getByPlaceholder('help.menu').fill('help.menu')
  await page.getByRole('button', { name: '生成预览' }).click()

  await expect(page.locator('#app-main').getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await expect(page.getByText('task_render_preview_0001').first()).toBeVisible()
  await expect(page.getByRole('img', { name: '图片预览结果' })).toBeVisible()
})

test('protocol center owns OneBot settings and keeps protocol logs scoped to OneBot11', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/config')
  await expect(page.getByText('协议连接设置')).toBeVisible()
  await expect(page.getByText('反向 WebSocket 地址')).toHaveCount(0)
  await page.getByRole('button', { name: '打开协议中心' }).click()

  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  await expect(page.getByText('当前正式支持协议：OneBot11')).toBeVisible()
  await expect(page.getByText('OneBot authentication failed')).toBeVisible()

  await page.getByLabel('反向 WebSocket 地址').fill('ws://127.0.0.1:8090/onebot')
  await page.getByLabel('连接超时（秒）').fill('18')
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存，重启后生效')).toBeVisible()

  const protocolLogsTable = page.locator('.logs-data-table')
  await expect(protocolLogsTable.locator('.protocol-name-pill')).toHaveCount(2)
  await expect(protocolLogsTable.getByText('authentication failed for reverse websocket').first()).toBeVisible()
  await expect(protocolLogsTable.getByText('plugin runtime stderr truncated')).toHaveCount(0)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await expect(page.getByText('plugin runtime stderr truncated').first()).toBeVisible()
  await expect(page.locator('.logs-filter-toolbar').getByText('协议')).toHaveCount(0)
})

test('login keeps the protected shell after reload', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await expect(page).not.toHaveURL(/\/login$/)
  await expect(page.locator('.connection-pill').filter({ hasText: '事件流' })).toContainText('已认证')

  await page.reload()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/\/login$/)
})

test('error recovery covers retry, invalid grant expiry and uninstall failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginsListOnce: true,
    failUninstallOnce: true,
  })
  await login(page)

  await page.getByRole('link', { name: '插件' }).click()
  await expect(page.getByText('读取未完成，请稍后重试。').first()).toBeVisible()
  await page.locator('.retry-panel').getByRole('button', { name: '重试' }).click({ force: true })
  await expect(page.getByText('weather').first()).toBeVisible()

  await page.getByRole('button', { name: '查看详情' }).first().click()
  await page.getByRole('button', { name: '新增授权' }).click()
  await page.getByLabel('能力标识').fill('http.request')
  await page.getByLabel('过期时间（UTC RFC3339）').fill('not-a-timestamp')
  await page.getByRole('button', { name: '保存授权' }).click()
  await expect(page.getByText('请求参数不正确，请检查后重试。')).toBeVisible()
  await page.getByRole('button', { name: '取消' }).click()

  await page.getByRole('button', { name: '卸载' }).click()
  await page.getByRole('button', { name: '确认卸载' }).click()
  await expect(page.getByText('缺少必要资源')).toBeVisible()
})

test('shutdown flow shows the draining banner', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '关闭服务' }).click({ force: true })
  await page.getByRole('button', { name: '确认关闭' }).click()

  await expect(page.getByText('服务正在停止', { exact: true })).toBeVisible()
  await expect(page.getByText('停机请求已发送')).toBeVisible()
})

test('mobile navigation and card layouts remain usable', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 390, height: 844 })

  await login(page)

  await page.getByRole('link', { name: '插件' }).click()
  await expect(pluginRows(page).first()).toBeVisible()

  await page.goto('/logs')
  await expect(logRows(page).first()).toBeVisible()
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
})
