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
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
}

function pluginRows(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .ant-table-tbody > tr:not(.ant-table-measure-row)')
}

function taskRows(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .ant-table-tbody > tr:not(.ant-table-measure-row)')
}

function logRows(page: import('@playwright/test').Page) {
  return page.locator('.logs-data-table .ant-table-tbody > tr:not(.ant-table-measure-row)')
}

function pluginScroller(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .ant-table-container')
}

function taskScroller(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .ant-table-container')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-data-table .ant-table-container')
}

function appHeader(page: import('@playwright/test').Page) {
  return page.getByTestId('app-header')
}

function dashboardConnectionCard(page: import('@playwright/test').Page) {
  return page.getByTestId('dashboard-connection-card')
}

test('setup flow reaches protected shell and shows websocket statuses', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/')
  await expect(page.getByRole('heading', { name: '创建管理员账号', level: 1 })).toBeVisible()

  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: '创建并进入管理界面' }).click()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(appHeader(page)).not.toContainText('保持正式契约')
  await expect(appHeader(page)).not.toContainText('事件流')
  await expect(appHeader(page)).not.toContainText('任务流')
  await expect(appHeader(page)).not.toContainText('日志流')
  await expect(dashboardConnectionCard(page)).toContainText('事件流')
  await expect(dashboardConnectionCard(page)).toContainText('任务流')
  await expect(dashboardConnectionCard(page)).toContainText('日志流')
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')
  await expect(page.getByTestId('connection-card-tasks')).toContainText('已认证')
  await expect(page.getByTestId('connection-card-logs')).toContainText('已认证')
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
  await expect(page.getByTestId('auth-theme-toggle')).toBeVisible()
  await expect(page.getByTestId('auth-language')).toBeVisible()
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

  await page.goto('/plugins')
  await expect(page.locator('#app-main').getByRole('heading', { name: '插件', level: 1 })).toBeVisible()
  await expect(pluginRows(page).first()).toBeVisible()
  await expect(page.locator('.plugins-data-table')).toContainText('help')
  await expect(page.locator('.plugins-data-table')).toContainText('weather')

  await page.getByRole('button', { name: '安装插件' }).click()
  const installDialog = page.getByRole('dialog', { name: '安装插件' })
  await expect(installDialog).toBeVisible()
  await installDialog.getByRole('textbox').fill('C:/plugins/weather.zip')
  await installDialog.getByRole('button', { name: '开始安装' }).click()

  await expect(page.locator('#app-main').getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await expect(page.getByText('task_plugin_install_0001').first()).toBeVisible()

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('未验证来源')).toBeVisible()
  await expect(page.getByText('plugins/installed')).toBeVisible()
  await expect(page.locator('.ant-descriptions').getByText('命令冲突')).toBeVisible()
  await expect(page.getByText('已注册指令')).toBeVisible()
  await expect(page.getByText('查询天气')).toBeVisible()

  await page.getByRole('button', { name: '处理权限' }).click()
  await page.getByRole('checkbox', { name: /render\.image/ }).check()
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'POST'
      && response.url().includes('/api/plugins/weather/grants')
    )),
    page.getByRole('button', { name: '授权选中项' }).click(),
  ])

  const renderPermission = page.locator('.permission-item').filter({ hasText: 'render.image' })
  await expect(renderPermission).toContainText('已授权')
  await expect(renderPermission).toContainText('手动授权')

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
  const summarySurface = page.locator('.ant-drawer-content').filter({ hasText: '显示状态' }).last()
  await expect(summarySurface).toContainText('显示状态')
  await expect(summarySurface).toContainText('运行中')
  await expect(summarySurface).not.toContainText('discovered')
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

test('dashboard avoids global page overflow when the content fits', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1600, height: 1200 })
  await login(page)

  const metrics = await page.evaluate(() => {
    const doc = document.scrollingElement ?? document.documentElement
    const main = document.querySelector<HTMLElement>('#app-main')
    const overviewCards = document.querySelectorAll('.dashboard-overview-grid .stat-card').length
    const bottomCards = document.querySelectorAll('.dashboard-bottom-grid > .ant-card').length
    const tabLabels = Array.from(document.querySelectorAll('.dashboard-main-grid .ant-tabs-tab')).map((item) => item.textContent?.trim() ?? '')

    return {
      bodyClientHeight: document.body.clientHeight,
      bodyScrollHeight: document.body.scrollHeight,
      docClientHeight: doc.clientHeight,
      docScrollHeight: doc.scrollHeight,
      mainClientHeight: main?.clientHeight ?? 0,
      overviewCards,
      bottomCards,
      tabLabels,
    }
  })

  expect(metrics.docScrollHeight).toBeLessThanOrEqual(metrics.docClientHeight + 1)
  expect(metrics.bodyScrollHeight).toBeLessThanOrEqual(metrics.bodyClientHeight + 1)
  expect(metrics.mainClientHeight).toBeGreaterThan(0)
  expect(metrics.overviewCards).toBe(4)
  expect(metrics.bottomCards).toBe(3)
  expect(metrics.tabLabels).toContain('近期变化')
  expect(metrics.tabLabels).toContain('就绪检查')
})

test('protocol logs keeps terminal and detail panes inside the viewport', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1600, height: 1200 })
  await login(page)

  await page.goto('/protocols/logs')
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()

  const lastLine = page.locator('.terminal-line').last()
  await expect(lastLine).toBeVisible()
  await lastLine.click({ force: true })

  const metrics = await page.evaluate(() => {
    const main = document.querySelector<HTMLElement>('#app-main')
    const terminalCard = document.querySelector<HTMLElement>('.terminal-card')
    const terminalBody = document.querySelector<HTMLElement>('.terminal-card .ant-card-body')
    const detailCard = document.querySelector<HTMLElement>('.detail-card')
    const detailBody = document.querySelector<HTMLElement>('.detail-card .ant-card-body')
    const terminalScroller = document.querySelector<HTMLElement>('.terminal-view-scroller')
    const detailScroller = document.querySelector<HTMLElement>('.detail-view-content')
    const lastLine = document.querySelector<HTMLElement>('.terminal-line:last-child')
    const terminalRect = terminalCard?.getBoundingClientRect()
    const terminalBodyRect = terminalBody?.getBoundingClientRect()
    const detailRect = detailCard?.getBoundingClientRect()
    const detailBodyRect = detailBody?.getBoundingClientRect()
    const scrollerRect = terminalScroller?.getBoundingClientRect()
    const lastLineRect = lastLine?.getBoundingClientRect()

    return {
      viewportHeight: window.innerHeight,
      mainClientHeight: main?.clientHeight ?? 0,
      mainScrollHeight: main?.scrollHeight ?? 0,
      terminalBottom: terminalRect?.bottom ?? 0,
      terminalBodyBottom: terminalBodyRect?.bottom ?? 0,
      detailBottom: detailRect?.bottom ?? 0,
      detailBodyBottom: detailBodyRect?.bottom ?? 0,
      terminalClientHeight: terminalScroller?.clientHeight ?? 0,
      terminalScrollHeight: terminalScroller?.scrollHeight ?? 0,
      detailClientHeight: detailScroller?.clientHeight ?? 0,
      detailScrollHeight: detailScroller?.scrollHeight ?? 0,
      lastLineBottom: lastLineRect?.bottom ?? 0,
      terminalLastLineGap: scrollerRect && lastLineRect ? scrollerRect.bottom - lastLineRect.bottom : 0,
    }
  })

  expect(metrics.mainScrollHeight).toBeLessThanOrEqual(metrics.mainClientHeight + 1)
  expect(metrics.terminalBottom).toBeLessThanOrEqual(metrics.viewportHeight + 1)
  expect(metrics.terminalBodyBottom).toBeLessThanOrEqual(metrics.terminalBottom + 1)
  expect(metrics.detailBottom).toBeLessThanOrEqual(metrics.viewportHeight + 1)
  expect(metrics.detailBodyBottom).toBeLessThanOrEqual(metrics.detailBottom + 1)
  expect(metrics.terminalScrollHeight).toBeGreaterThanOrEqual(metrics.terminalClientHeight)
  expect(metrics.terminalClientHeight).toBeGreaterThan(0)
  expect(metrics.detailClientHeight).toBeGreaterThan(0)
  expect(metrics.lastLineBottom).toBeGreaterThan(0)
  expect(metrics.terminalLastLineGap).toBeGreaterThanOrEqual(8)
})

test('config keeps the section list scroll inside the card without page overflow', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1600, height: 1200 })
  await login(page)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()

  const metrics = await page.evaluate(() => {
    const doc = document.scrollingElement ?? document.documentElement
    const main = document.querySelector<HTMLElement>('#app-main')
    const navBody = document.querySelector<HTMLElement>('.config-nav-card .ant-card-body')
    const firstNavItem = document.querySelector<HTMLElement>('.config-nav-item')

    return {
      bodyClientHeight: document.body.clientHeight,
      bodyScrollHeight: document.body.scrollHeight,
      docClientHeight: doc.clientHeight,
      docScrollHeight: doc.scrollHeight,
      mainClientHeight: main?.clientHeight ?? 0,
      mainScrollHeight: main?.scrollHeight ?? 0,
      navClientHeight: navBody?.clientHeight ?? 0,
      navScrollHeight: navBody?.scrollHeight ?? 0,
      navClientWidth: navBody?.clientWidth ?? 0,
      firstNavItemWidth: firstNavItem?.clientWidth ?? 0,
    }
  })

  expect(metrics.docScrollHeight).toBeLessThanOrEqual(metrics.docClientHeight + 1)
  expect(metrics.bodyScrollHeight).toBeLessThanOrEqual(metrics.bodyClientHeight + 1)
  expect(metrics.mainScrollHeight).toBeLessThanOrEqual(metrics.mainClientHeight + 1)
  expect(metrics.navScrollHeight).toBeGreaterThan(metrics.navClientHeight)
  expect(metrics.navClientHeight).toBeGreaterThan(0)
  expect(metrics.firstNavItemWidth).toBeGreaterThanOrEqual(metrics.navClientWidth - 40)
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
  await expect(page.getByText('协议连接设置')).toHaveCount(0)
  await expect(page.getByText('反向 WebSocket 地址')).toHaveCount(0)
  await page.goto('/protocols')

  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  await expect(page.getByText('当前正式支持协议：OneBot11')).toBeVisible()
  await expect(page.getByText('OneBot11 主动连接已就绪')).toBeVisible()
  await expect(page.locator('.transport-cards-grid')).toContainText('主动连接 WebSocket')

  await page.getByLabel('回连地址').fill('wss://bot.example.com/reverse/onebot')
  await page.getByLabel('连接超时（秒）').fill('18')
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存，重启后生效')).toBeVisible()

  await page.getByRole('button', { name: '查看协议日志' }).click()
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()

  const terminal = page.locator('.terminal-view-scroller')
  await expect(terminal).toBeVisible()
  await expect(terminal.getByText('reverse websocket connection lost')).toBeVisible()
  await expect(terminal.getByText('plugin runtime stderr truncated')).toHaveCount(0)

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_adapter_live_0002',
        timestamp: '2026-04-08T10:18:00Z',
        level: 'warn',
        source: 'adapter.onebot11',
        protocol: 'onebot11',
        message: 'ignored OneBot API response with unsupported echo',
        request_id: 'req_adapter_ignored_0002',
      },
      details: {
        details: {
          direction: 'inbound',
          frame_type: 'api.response.ignored',
          reason: 'api response echo must be a non-empty string',
          echo_value_type: 'number',
          payload_preview: {
            echo: 123,
            status: 'ok',
          },
        },
      },
    },
  })

  const liveLine = terminal.locator('.terminal-line').filter({ hasText: 'ignored OneBot API response with unsupported echo' }).last()
  await expect(liveLine).toBeVisible()
  await liveLine.click({ force: true })
  await expect(page.getByText('api.response.ignored')).toBeVisible()
  await expect(page.locator('.json-content')).toContainText('"echo": 123')

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await expect(page.getByText('plugin runtime stderr truncated').first()).toBeVisible()
  await expect(page.locator('.logs-filter-toolbar').getByText('协议')).toHaveCount(0)
})

test('command center shows all declared commands and filters by plugin selection', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/commands')
  await expect(page.locator('#app-main').getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  await expect(page.locator('.commands-data-table')).toContainText('help')
  await expect(page.locator('.commands-data-table')).toContainText('weather')

  const pluginSelector = page.locator('.commands-filter-toolbar .ant-select').first()
  await expect(pluginSelector).toBeVisible()
  await pluginSelector.click()
  await page.keyboard.type('Weather')
  await page.keyboard.press('Enter')

  await expect(page.locator('.commands-data-table')).toContainText('查询天气')
  await expect(page.locator('.commands-data-table')).not.toContainText('查看帮助菜单')
})

test('light theme uses a light sider and keeps the header clean', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const sider = page.getByTestId('app-sider')
  await expect(sider).toHaveClass(/ant-layout-sider-light/)
  await expect(appHeader(page)).not.toContainText('事件流')
  await expect(appHeader(page)).not.toContainText('保持正式契约')

  const shellMetrics = await page.evaluate(() => {
    const header = document.querySelector<HTMLElement>('[data-testid="app-header"]')
    const sider = document.querySelector<HTMLElement>('[data-testid="app-sider"]')
    const headerRect = header?.getBoundingClientRect()
    const siderRect = sider?.getBoundingClientRect()

    return {
      headerHeight: headerRect?.height ?? 0,
      siderWidth: siderRect?.width ?? 0,
    }
  })

  expect(shellMetrics.headerHeight).toBeLessThanOrEqual(90)
  expect(shellMetrics.siderWidth).toBeGreaterThanOrEqual(220)
  expect(shellMetrics.siderWidth).toBeLessThanOrEqual(228)

  await page.getByTestId('theme-toggle').click()
  await expect(sider).toHaveClass(/ant-layout-sider-dark/)
})

test('login keeps the protected shell after reload', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await expect(page).not.toHaveURL(/\/login$/)
  await expect(appHeader(page)).not.toContainText('事件流')
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')

  await page.reload()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/\/login$/)
  await expect(appHeader(page)).not.toContainText('事件流')
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')
})

test('error recovery covers retry and uninstall failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginsListOnce: true,
    failPluginDetailOnce: true,
    failUninstallOnce: true,
  })
  await login(page)

  await page.goto('/plugins')
  await expect(page.getByText('读取未完成，请稍后重试。').first()).toBeVisible()
  await page.getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByText('weather').first()).toBeVisible()

  const weatherRow = pluginRows(page).filter({ hasText: 'Weather' })
  await weatherRow.getByRole('button', { name: '查看详情' }).click()
  await expect(page.getByText('读取未完成，请稍后重试。').first()).toBeVisible()
  await page.getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()

  await page.getByRole('button', { name: /卸\s*载/ }).click()
  await page.getByRole('button', { name: /确认卸载/ }).click()
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

  await page.locator('.admin-layout__icon-button.mobile-only').first().click()
  await page.locator('.ant-drawer-content').getByRole('menuitem', { name: '插件' }).click()
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
