import { expect, test } from '@playwright/test'

const backendUrl = 'http://127.0.0.1:4010'

interface TransitionStageSample {
  className: string
  opacity: number
}

interface TransitionSample {
  heading: string
  label: string
  stageNodes: TransitionStageSample[]
}

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
  return page.locator('.logs-data-table .logs-table-row')
}

function pluginScroller(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .ant-table-container')
}

function taskScroller(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .ant-table-container')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-data-table .data-viewport__scroller')
}

function appHeader(page: import('@playwright/test').Page) {
  return page.getByTestId('app-header')
}

function dashboardConnectionCard(page: import('@playwright/test').Page) {
  return page.getByTestId('dashboard-connection-card')
}

async function readTabLabels(page: import('@playwright/test').Page) {
  return page.locator('.admin-layout__tabbar .ant-tabs-tab-btn').evaluateAll((nodes) => (
    nodes
      .map((node) => node.textContent?.trim() ?? '')
      .filter(Boolean)
  ))
}

async function readActiveTabLabel(page: import('@playwright/test').Page) {
  return page.locator('.admin-layout__tabbar .ant-tabs-tab-active .ant-tabs-tab-btn').evaluate((node) => (
    node.textContent?.trim() ?? ''
  ))
}

async function readTabIconKeys(page: import('@playwright/test').Page) {
  return page.locator('.admin-layout__tabbar .admin-layout__tab-label').evaluateAll((nodes) => (
    nodes
      .map((node) => node.getAttribute('data-icon') ?? '')
      .filter(Boolean)
  ))
}

async function navigateThroughMenu(
  page: import('@playwright/test').Page,
  item: string,
  group?: string,
) {
  const targetItem = page.getByRole('menuitem', { name: item })

  if (group && !await targetItem.isVisible().catch(() => false)) {
    await page.locator('.ant-menu-submenu-title', { hasText: group }).click()
    await expect(targetItem).toBeVisible()
  }

  await targetItem.click()
}

async function startTransitionSampling(page: import('@playwright/test').Page) {
  await page.evaluate(() => {
    type BrowserTransitionStageSample = {
      className: string
      opacity: number
    }

    type BrowserTransitionSample = {
      heading: string
      label: string
      stageNodes: BrowserTransitionStageSample[]
    }

    const win = window as Window & { __transitionSamples?: BrowserTransitionSample[] }
    const samples: BrowserTransitionSample[] = []
    win.__transitionSamples = samples

    const sample = (label: string) => {
      const heading = document.querySelector('#app-main h1')?.textContent?.trim() ?? ''
      const stageNodes = Array.from(document.querySelectorAll<HTMLElement>('.admin-layout__route-stage')).map((node) => ({
        className: node.className,
        opacity: Number.parseFloat(window.getComputedStyle(node).opacity) || 0,
      }))

      samples.push({
        heading,
        label,
        stageNodes,
      })
    }

    sample('before')
    let count = 0
    const tick = () => {
      sample(`frame-${count}`)
      count += 1
      if (count < 50) {
        requestAnimationFrame(tick)
      }
    }

    requestAnimationFrame(tick)
  })
}

async function collectTransitionSamples(page: import('@playwright/test').Page) {
  await page.waitForTimeout(900)
  return page.evaluate(() => {
    type BrowserTransitionSample = {
      heading: string
      label: string
      stageNodes: Array<{
        className: string
        opacity: number
      }>
    }

    const win = window as Window & { __transitionSamples?: BrowserTransitionSample[] }
    return win.__transitionSamples ?? []
  }) as Promise<TransitionSample[]>
}

function expectSingleEnterTransition(samples: TransitionSample[], heading: string) {
  const firstEnterIndex = samples.findIndex((sample) => (
    sample.heading === heading
    && sample.stageNodes.some((node) => /route-fade(?:-slide)?-enter/.test(node.className))
  ))
  expect(firstEnterIndex).toBeGreaterThanOrEqual(0)

  const firstEnter = samples[firstEnterIndex]!
  const firstEnterNode = firstEnter.stageNodes.find((node) => /route-fade(?:-slide)?-enter/.test(node.className))
  expect(firstEnter.stageNodes).toHaveLength(1)
  expect(firstEnterNode).toBeDefined()
  expect(firstEnterNode!.opacity).toBeLessThan(1)

  const showedFullyVisibleBeforeEnter = samples
    .slice(0, firstEnterIndex)
    .some((sample) => sample.heading === heading && sample.stageNodes.some((node) => (
      node.opacity >= 0.99 && !/route-fade(?:-slide)?-enter/.test(node.className)
    )))

  expect(showedFullyVisibleBeforeEnter).toBe(false)
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
  expect(pluginFirst!.y + pluginFirst!.height).toBeLessThanOrEqual(pluginSecond!.y + 1)
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
    const terminalScroller = document.querySelector<HTMLElement>('.terminal-view-scroller .data-viewport__scroller')
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

test('logs history paging stays stable until returning to latest', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await Promise.all(Array.from({ length: 51 }, (_, index) => (
    request.post(`${backendUrl}/__test/push-log`, {
      data: {
        summary: {
          log_id: `log_history_e2e_${index}`,
          timestamp: `2026-04-15T12:00:${String(index).padStart(2, '0')}Z`,
          level: 'info',
          source: 'runtime',
          request_id: 'req_logs_history_e2e',
          message: `history row ${index}`,
        },
      },
    })
  )))

  await page.goto('/logs')
  await page.getByPlaceholder('例如 req_*').fill('req_logs_history_e2e')
  await page.getByRole('button', { name: '应用筛选' }).click()

  await expect(logRows(page).first()).toContainText('history row 50')
  await page.getByRole('button', { name: '更早记录' }).click()
  await expect(logRows(page).first()).toContainText('history row 0')

  await page.getByRole('button', { name: '更新记录' }).click()
  await expect(logRows(page).first()).toContainText('history row 50')

  await page.getByRole('button', { name: '更早记录' }).click()
  await expect(logRows(page).first()).toContainText('history row 0')

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_history_e2e_latest',
        timestamp: '2026-04-15T12:01:00Z',
        level: 'info',
        source: 'runtime',
        request_id: 'req_logs_history_e2e',
        message: 'history row latest',
      },
    },
  })

  await expect(page.getByText('有 1 条新日志可查看')).toBeVisible()
  await expect(page.locator('.log-message-text', { hasText: 'history row latest' })).toHaveCount(0)
  await page.getByRole('button', { name: '回到最新' }).click()
  await expect(logRows(page).first()).toContainText('history row latest')
})

test('logs page reloads the latest page after hidden updates arrive', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '日志', '运维')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_logs_reactivate_e2e')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_reactivate_e2e')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])
  await expect(page.locator('.log-message-text', { hasText: 'reactivate latest row' })).toHaveCount(0)

  await navigateThroughMenu(page, '插件')
  await expect(page.getByRole('heading', { name: '插件', level: 1 })).toBeVisible()

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_reactivate_e2e_latest',
        timestamp: '2026-04-15T12:10:00Z',
        level: 'info',
        source: 'runtime',
        request_id: 'req_logs_reactivate_e2e',
        message: 'reactivate latest row',
      },
    },
  })

  await navigateThroughMenu(page, '日志', '运维')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await expect(page.locator('.log-message-text', { hasText: 'reactivate latest row' }).first()).toBeVisible()
  await expect(page.getByText('正在实时显示最新日志')).toBeVisible()
})

test('protocol logs reloads the latest page after hidden updates arrive', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '协议日志', '协议')
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_protocol_reactivate_e2e')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_protocol_reactivate_e2e')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])
  await expect(page.locator('.line-text', { hasText: 'reactivate protocol latest row' })).toHaveCount(0)

  await navigateThroughMenu(page, '插件')
  await expect(page.getByRole('heading', { name: '插件', level: 1 })).toBeVisible()

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_protocol_reactivate_e2e_latest',
        timestamp: '2026-04-15T12:12:00Z',
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_protocol_reactivate_e2e',
        message: 'reactivate protocol latest row',
      },
      detail: {
        log_id: 'log_protocol_reactivate_e2e_latest',
        timestamp: '2026-04-15T12:12:00Z',
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_protocol_reactivate_e2e',
        message: 'reactivate protocol latest row',
        details: {
          direction: 'inbound',
          plain_text: 'reactivate protocol latest row',
        },
      },
    },
  })

  await navigateThroughMenu(page, '协议日志', '协议')
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()
  const latestProtocolLine = page.locator('.terminal-line').filter({ hasText: 'reactivate protocol latest row' }).first()
  await expect(latestProtocolLine).toBeVisible()
  await expect(page.locator('.follow-status-pill')).toContainText('最新页')
})

test('unsafe OneBot text stays escaped in protocol logs and logs list', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: '2026-04-14T02:49:45Z',
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_bridge_unsafe_0001',
        message: '721011692: [760384342]群星怒\u2066，大明云玩家\u202e~喵\u2069/没错，是魔法！(2896109796): 除了战猎这种抓不到加费就完全没法打的角色',
      },
      detail: {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: '2026-04-14T02:49:45Z',
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_bridge_unsafe_0001',
        message: '721011692: [760384342]群星怒\u2066，大明云玩家\u202e~喵\u2069/没错，是魔法！(2896109796): 除了战猎这种抓不到加费就完全没法打的角色',
        details: {
          direction: 'inbound',
          self_id: '721011692',
          conversation_id: '760384342',
          conversation_type: 'group',
          group_name: '测试群',
          sender: {
            user_id: '2896109796',
            nickname: '没错，是魔法！',
            card: '群星怒\u2066，大明云玩家\u202e~喵\u2069',
            role: 'member',
          },
          plain_text: '除了战猎这种抓不到加费就完全没法打的角色',
        },
      },
    },
  })

  await page.goto('/protocols/logs')
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()
  const unsafeTerminalLine = page.locator('.terminal-line', { hasText: '\\u2066' }).last()
  await expect(unsafeTerminalLine.locator('.line-text')).toContainText('\\u2066')
  await expect(unsafeTerminalLine.locator('.line-source')).toHaveText('bridge · onebot11')
  await unsafeTerminalLine.click()
  await expect(page.locator('.detail-hero-message')).toContainText('\\u2066')
  await expect(page.locator('.field-value').filter({ hasText: '\\u2066' }).first()).toBeVisible()
  await expect(page.locator('.json-content')).toContainText('\\u2066')

  const protocolTexts = await page.evaluate(() => ({
    line: document.querySelector('.terminal-line:last-child .line-text')?.textContent ?? '',
    hero: document.querySelector('.detail-hero-message')?.textContent ?? '',
    json: document.querySelector('.json-content')?.textContent ?? '',
  }))
  expect(protocolTexts.line.includes('\u2066')).toBe(false)
  expect(protocolTexts.hero.includes('\u2066')).toBe(false)
  expect(protocolTexts.json.includes('\u2066')).toBe(false)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  const unsafeLogMessage = page.locator('.log-message-text', { hasText: '群星怒' }).first()
  await expect(unsafeLogMessage).toContainText('\\u2066')

  const logsText = await unsafeLogMessage.evaluate((node) => node.textContent ?? '')
  expect(logsText.includes('\u2066')).toBe(false)
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

  const reverseSettingsCard = page.locator('.protocol-settings-layout .protocol-config-card').filter({ hasText: '回连 WebSocket' })
  const reverseStatusCard = page.locator('.transport-cards-grid .transport-card').filter({ hasText: '回连 WebSocket' })
  await page.getByLabel('回连地址').fill('wss://bot.example.com/reverse/onebot')
  await page.getByLabel('连接超时（秒）').fill('18')
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存并已生效')).toBeVisible()
  await expect(reverseStatusCard).toContainText('未启用')

  await page.reload()
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  await expect(page.locator('.transport-cards-grid .transport-card').filter({ hasText: '回连 WebSocket' })).toContainText('未启用')

  await reverseSettingsCard.getByRole('switch', { name: '启用' }).click()
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存并已生效')).toBeVisible()
  await expect(page.locator('.transport-cards-grid .transport-card').filter({ hasText: '回连 WebSocket' })).toContainText('等待 OneBot 回连')

  await page.getByRole('button', { name: '查看协议日志' }).click()
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()

  const terminal = page.locator('.terminal-view-scroller')
  await expect(terminal).toBeVisible()
  await expect(terminal.getByText('ignored OneBot API response with unsupported echo')).toBeVisible()
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
      detail: {
        details: {
          direction: 'inbound',
          frame_type: 'api.response.ignored',
          sender: {
            user_id: '3001',
            nickname: 'Alice',
            role: 'admin',
          },
          reason: 'api response echo must be a non-empty string',
          echo_value_type: 'number',
          plain_text: 'hello bridge',
          payload_preview: {
            echo: 123,
            status: 'ok',
          },
        },
      },
    },
  })

  const liveLine = terminal.locator('.terminal-line').filter({ hasText: 'ignored OneBot API response with unsupported echo' }).first()
  await expect(liveLine).toBeVisible()
  await liveLine.click({ force: true })
  await expect(page.locator('.detail-fields-grid').getByText('api.response.ignored', { exact: true })).toBeVisible()
  await expect(page.getByText('发送者昵称')).toBeVisible()
  await expect(page.locator('.detail-fields-grid').getByText('Alice', { exact: true })).toBeVisible()
  await expect(page.locator('.json-content')).toContainText('"echo": 123')
  await expect(page.locator('.json-content')).toContainText('"sender"')
  await expect(page.locator('.json-content')).not.toContainText('sender_id')
  await expect(page.locator('.json-content')).not.toContainText('sender_nickname')

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await expect(page.getByText('plugin runtime stderr truncated').first()).toBeVisible()
  await expect(page.locator('.logs-filter-toolbar').getByText('协议')).toHaveCount(0)
})

test('logs page filters both history and live log appends', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  await page.locator('.logs-filter-toolbar .ant-form-item').filter({ hasText: '来源' }).locator('input').fill('runtime')
  await page.getByRole('button', { name: '应用筛选' }).click()

  const logsTable = page.locator('.logs-data-table')
  await expect(logsTable).toContainText('plugin runtime stderr truncated')
  await expect(logsTable).not.toContainText('reverse websocket connection lost')

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_runtime_filtered_out_0001',
        timestamp: '2026-04-08T10:28:00Z',
        level: 'warn',
        source: 'adapter.onebot11',
        protocol: 'onebot11',
        message: 'live adapter log filtered out',
        request_id: 'req_adapter_filtered_out_0001',
      },
      detail: {
        details: {
          direction: 'inbound',
          frame_type: 'socket.close',
          reason: 'live adapter log filtered out',
        },
      },
    },
  })

  await page.waitForTimeout(200)
  await expect(logsTable).not.toContainText('live adapter log filtered out')

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_runtime_kept_0001',
        timestamp: '2026-04-08T10:29:00Z',
        level: 'error',
        source: 'runtime',
        message: 'live runtime log kept',
        plugin_id: 'weather',
        request_id: 'req_runtime_kept_0001',
      },
      detail: {
        details: {
          direction: 'internal',
          reason: 'live runtime log kept',
        },
      },
    },
  })

  await expect(logsTable).toContainText('live runtime log kept')
})

test('logs page keeps older history reachable inside the table scroller', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()

  await Promise.all(Array.from({ length: 36 }, (_, index) => (
    request.post(`${backendUrl}/__test/push-log`, {
      data: {
        summary: {
          log_id: `log_runtime_scroll_${index}`,
          timestamp: `2026-04-15T10:${String(Math.floor(index / 60)).padStart(2, '0')}:${String(index % 60).padStart(2, '0')}Z`,
          level: 'info',
          source: 'runtime',
          message: `scroll history row ${index}`,
          request_id: `req_runtime_scroll_${index}`,
        },
        detail: {
          details: {
            direction: 'internal',
            reason: `scroll history row ${index}`,
          },
        },
      },
    })
  )))

  await expect(page.locator('.logs-data-table')).toContainText('scroll history row 35')

  const metrics = await page.evaluate(() => {
    const doc = document.scrollingElement ?? document.documentElement
    const tableBody = document.querySelector<HTMLElement>('.logs-data-table .data-viewport__scroller')
    if (!tableBody) {
      return {
        hasTableBody: false,
        pageOverflow: doc.scrollHeight - doc.clientHeight,
        scrollHeight: 0,
        clientHeight: 0,
        scrollTop: 0,
      }
    }

    tableBody.scrollTop = tableBody.scrollHeight

    return {
      hasTableBody: true,
      pageOverflow: doc.scrollHeight - doc.clientHeight,
      scrollHeight: tableBody.scrollHeight,
      clientHeight: tableBody.clientHeight,
      scrollTop: tableBody.scrollTop,
    }
  })

  expect(metrics.hasTableBody).toBe(true)
  expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight)
  expect(metrics.scrollTop).toBeGreaterThan(0)
  expect(metrics.pageOverflow).toBeLessThanOrEqual(2)
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

test('breadcrumb and tabbar track leaf pages instead of hidden route groups', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--single/)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-link')).toHaveCount(0)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-current')).toHaveText('系统状态')

  await page.goto('/commands')
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--multi/)
  await expect(page.locator('.admin-layout__header-breadcrumb').getByRole('link', { name: '运维' })).toHaveAttribute('href', '/commands')
  await expect(page.locator('.admin-layout__breadcrumb-current')).toHaveText('指令中心')
  await expect(page.getByRole('tab', { name: '指令中心' })).toBeVisible()

  let tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '指令中心'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands'])
  expect(await readActiveTabLabel(page)).toBe('指令中心')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '运维' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(3)

  await page.goto('/tasks')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '指令中心', '任务'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands', 'tasks'])
  expect(await readActiveTabLabel(page)).toBe('任务')

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '日志', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '指令中心', '任务', '日志'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands', 'tasks', 'logs'])
  expect(await readActiveTabLabel(page)).toBe('日志')
  await expect(page.getByRole('tab', { name: '指令中心' })).toBeVisible()
  await page.locator('.admin-layout__breadcrumb-item--ancestor .ant-breadcrumb-link').hover()

  const breadcrumbMetrics = await page.evaluate(() => {
    const header = document.querySelector<HTMLElement>('[data-testid="app-header"]')
    const toggle = document.querySelector<HTMLElement>('.admin-layout__header-left .admin-layout__nav-trigger')
    const toggleIcon = toggle?.querySelector<HTMLElement>('.anticon')
    const breadcrumb = document.querySelector<HTMLElement>('.admin-layout__header-breadcrumb')
    const account = document.querySelector<HTMLElement>('.admin-layout__account-button')
    const ancestorOuter = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--ancestor > .ant-breadcrumb-link')
    const link = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--ancestor .admin-layout__breadcrumb-link')
    const linkText = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--ancestor .admin-layout__breadcrumb-link-text')
    const currentOuter = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--current > .ant-breadcrumb-link')
    const current = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--current .admin-layout__breadcrumb-current')
    const currentText = document.querySelector<HTMLElement>('.admin-layout__breadcrumb-item--current .admin-layout__breadcrumb-current-text')
    const separator = document.querySelector<HTMLElement>('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-separator')
    const headerRect = header?.getBoundingClientRect()
    const toggleRect = toggle?.getBoundingClientRect()
    const breadcrumbRect = breadcrumb?.getBoundingClientRect()
    const accountRect = account?.getBoundingClientRect()
    const ancestorOuterRect = ancestorOuter?.getBoundingClientRect()
    const linkRect = link?.getBoundingClientRect()
    const linkTextRect = linkText?.getBoundingClientRect()
    const currentOuterRect = currentOuter?.getBoundingClientRect()
    const currentRect = current?.getBoundingClientRect()
    const currentTextRect = currentText?.getBoundingClientRect()
    const separatorRect = separator?.getBoundingClientRect()
    const ancestorOuterStyles = ancestorOuter ? window.getComputedStyle(ancestorOuter) : null
    const linkStyles = link ? window.getComputedStyle(link) : null
    const currentOuterStyles = currentOuter ? window.getComputedStyle(currentOuter) : null
    const currentStyles = current ? window.getComputedStyle(current) : null

    return {
      accountRightGap: headerRect && accountRect ? headerRect.right - accountRect.right : 0,
      ancestorInnerFitsOuter: Boolean(
        ancestorOuterRect
        && linkRect
        && linkRect.left >= ancestorOuterRect.left - 0.5
        && linkRect.right <= ancestorOuterRect.right + 0.5
        && linkRect.top >= ancestorOuterRect.top - 0.5
        && linkRect.bottom <= ancestorOuterRect.bottom + 0.5,
      ),
      ancestorTextFitsOuter: Boolean(
        ancestorOuterRect
        && linkTextRect
        && linkTextRect.left >= ancestorOuterRect.left - 0.5
        && linkTextRect.right <= ancestorOuterRect.right + 0.5
        && linkTextRect.top >= ancestorOuterRect.top - 0.5
        && linkTextRect.bottom <= ancestorOuterRect.bottom + 0.5,
      ),
      ancestorOuterHeight: ancestorOuterRect?.height ?? 0,
      ancestorOuterHoverHasVisibleBackground: ancestorOuterStyles
        ? !['rgba(0, 0, 0, 0)', 'transparent'].includes(ancestorOuterStyles.backgroundColor)
        : false,
      ancestorOuterHoverHasVisibleBorder: ancestorOuterStyles
        ? Number.parseFloat(ancestorOuterStyles.borderTopWidth) > 0
          && !['rgba(0, 0, 0, 0)', 'transparent'].includes(ancestorOuterStyles.borderTopColor)
        : false,
      ancestorTextNotClipped: Boolean(
        linkText
        && linkText.scrollWidth <= linkText.clientWidth + 1,
      ),
      breadcrumbHeight: breadcrumbRect?.height ?? 0,
      breadcrumbIsMulti: breadcrumb?.classList.contains('admin-layout__header-breadcrumb--multi') ?? false,
      breadcrumbLeft: breadcrumbRect?.left ?? 0,
      breadcrumbMid: breadcrumbRect ? breadcrumbRect.top + breadcrumbRect.height / 2 : 0,
      currentOuterHeight: currentOuterRect?.height ?? 0,
      currentMid: currentRect ? currentRect.top + currentRect.height / 2 : 0,
      fontSize: link ? Number.parseFloat(window.getComputedStyle(link).fontSize) : 0,
      headerLeftGap: headerRect && toggleRect ? toggleRect.left - headerRect.left : 0,
      linkDisplay: linkStyles?.display ?? '',
      linkPaddingLeft: linkStyles ? Number.parseFloat(linkStyles.paddingLeft) || 0 : 0,
      linkPaddingRight: linkStyles ? Number.parseFloat(linkStyles.paddingRight) || 0 : 0,
      linkMid: linkRect ? linkRect.top + linkRect.height / 2 : 0,
      linkFontWeight: linkStyles ? Number.parseFloat(linkStyles.fontWeight) || 0 : 0,
      currentDisplay: currentStyles?.display ?? '',
      currentHasVisibleBackground: currentOuterStyles
        ? !['rgba(0, 0, 0, 0)', 'transparent'].includes(currentOuterStyles.backgroundColor)
        : false,
      currentHasVisibleBorder: currentOuterStyles
        ? Number.parseFloat(currentOuterStyles.borderTopWidth) > 0
          && !['rgba(0, 0, 0, 0)', 'transparent'].includes(currentOuterStyles.borderTopColor)
        : false,
      currentFontWeight: currentStyles ? Number.parseFloat(currentStyles.fontWeight) || 0 : 0,
      separatorMid: separatorRect ? separatorRect.top + separatorRect.height / 2 : 0,
      standaloneRowExists: Boolean(document.querySelector('.admin-layout__breadcrumb-row')),
      toggleIconCenterDeltaX: toggleRect && toggleIcon
        ? Math.abs((toggleRect.left + toggleRect.width / 2) - (
          toggleIcon.getBoundingClientRect().left + toggleIcon.getBoundingClientRect().width / 2
        ))
        : Number.POSITIVE_INFINITY,
      toggleIconCenterDeltaY: toggleRect && toggleIcon
        ? Math.abs((toggleRect.top + toggleRect.height / 2) - (
          toggleIcon.getBoundingClientRect().top + toggleIcon.getBoundingClientRect().height / 2
        ))
        : Number.POSITIVE_INFINITY,
      toggleMid: toggleRect ? toggleRect.top + toggleRect.height / 2 : 0,
      toggleRight: toggleRect?.right ?? 0,
    }
  })

  expect(breadcrumbMetrics.fontSize).toBeGreaterThanOrEqual(13)
  expect(breadcrumbMetrics.breadcrumbHeight).toBeGreaterThanOrEqual(28)
  expect(breadcrumbMetrics.breadcrumbIsMulti).toBe(true)
  expect(breadcrumbMetrics.standaloneRowExists).toBe(false)
  expect(breadcrumbMetrics.headerLeftGap).toBeGreaterThanOrEqual(8)
  expect(breadcrumbMetrics.headerLeftGap).toBeLessThanOrEqual(12)
  expect(breadcrumbMetrics.accountRightGap).toBeLessThanOrEqual(4)
  expect(breadcrumbMetrics.breadcrumbLeft).toBeGreaterThan(breadcrumbMetrics.toggleRight)
  expect(breadcrumbMetrics.breadcrumbLeft - breadcrumbMetrics.toggleRight).toBeLessThanOrEqual(12)
  expect(breadcrumbMetrics.ancestorInnerFitsOuter).toBe(true)
  expect(breadcrumbMetrics.ancestorTextFitsOuter).toBe(true)
  expect(breadcrumbMetrics.ancestorTextNotClipped).toBe(true)
  expect(breadcrumbMetrics.ancestorOuterHoverHasVisibleBackground).toBe(true)
  expect(breadcrumbMetrics.ancestorOuterHoverHasVisibleBorder).toBe(true)
  expect(breadcrumbMetrics.ancestorOuterHeight).toBeGreaterThan(0)
  expect(Math.abs(breadcrumbMetrics.ancestorOuterHeight - breadcrumbMetrics.currentOuterHeight)).toBeLessThanOrEqual(1)
  expect(breadcrumbMetrics.toggleIconCenterDeltaX).toBeLessThanOrEqual(1)
  expect(breadcrumbMetrics.toggleIconCenterDeltaY).toBeLessThanOrEqual(1)
  expect(Math.abs(breadcrumbMetrics.toggleMid - breadcrumbMetrics.breadcrumbMid)).toBeLessThanOrEqual(8)
  expect(Math.abs(breadcrumbMetrics.linkMid - breadcrumbMetrics.separatorMid)).toBeLessThanOrEqual(4)
  expect(Math.abs(breadcrumbMetrics.currentMid - breadcrumbMetrics.separatorMid)).toBeLessThanOrEqual(4)
  expect(['flex', 'inline-flex']).toContain(breadcrumbMetrics.linkDisplay)
  expect(breadcrumbMetrics.linkPaddingLeft).toBeGreaterThanOrEqual(6)
  expect(breadcrumbMetrics.linkPaddingLeft).toBeLessThanOrEqual(8.5)
  expect(breadcrumbMetrics.linkPaddingRight).toBeGreaterThanOrEqual(6)
  expect(breadcrumbMetrics.linkPaddingRight).toBeLessThanOrEqual(8.5)
  expect(['flex', 'inline-flex']).toContain(breadcrumbMetrics.currentDisplay)
  expect(breadcrumbMetrics.currentHasVisibleBackground).toBe(true)
  expect(breadcrumbMetrics.currentHasVisibleBorder).toBe(true)
  expect(breadcrumbMetrics.currentFontWeight).toBeGreaterThanOrEqual(breadcrumbMetrics.linkFontWeight)

  await page.goto('/protocols')
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态', '指令中心', '任务', '日志', '协议中心'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands', 'tasks', 'logs', 'protocols'])
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '协议' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(2)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态', '指令中心', '任务', '日志', '协议中心', '配置'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands', 'tasks', 'logs', 'protocols', 'config'])
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '系统' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(1)
  await expect(page.locator('.admin-layout__sider .ant-menu-item-selected .admin-layout__menu-icon')).toHaveCount(1)

  await page.getByRole('tab', { name: '指令中心' }).click()
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/commands$/)

  await page.reload()
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态', '指令中心', '任务', '日志', '协议中心', '配置'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands', 'tasks', 'logs', 'protocols', 'config'])

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('weather')
  expect(await readTabLabels(page)).toContain('weather')
  expect(await readTabIconKeys(page)).toContain('appstore')
})

test('nested admin pages animate only once when entering grouped routes', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '协议中心', '协议')
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '协议中心')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '插件')
  await expect(page.getByRole('heading', { name: '插件', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '插件')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '协议日志', '协议')
  await expect(page.getByRole('heading', { name: '协议日志', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '协议日志')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '指令中心', '运维')
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '指令中心')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '系统状态')
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '系统状态')
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

  expect(shellMetrics.headerHeight).toBeLessThanOrEqual(120)
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

  const headerMetrics = await page.evaluate(() => {
    const breadcrumb = document.querySelector<HTMLElement>('.admin-layout__header-breadcrumb')
    const headerLeft = document.querySelector<HTMLElement>('.admin-layout__header-left')
    const headerRight = document.querySelector<HTMLElement>('.admin-layout__header-right')
    const tabbarMain = document.querySelector<HTMLElement>('.admin-layout__tabbar-main')
    const headerLeftRect = headerLeft?.getBoundingClientRect()
    const headerRightRect = headerRight?.getBoundingClientRect()
    const tabbarRect = tabbarMain?.getBoundingClientRect()

    return {
      breadcrumbWidth: breadcrumb?.getBoundingClientRect().width ?? 0,
      leftTop: headerLeftRect?.top ?? 0,
      rightTop: headerRightRect?.top ?? 0,
      tabbarHeight: tabbarRect?.height ?? 0,
    }
  })

  expect(headerMetrics.breadcrumbWidth).toBeGreaterThan(0)
  expect(Math.abs(headerMetrics.leftTop - headerMetrics.rightTop)).toBeLessThanOrEqual(8)
  expect(headerMetrics.tabbarHeight).toBeLessThanOrEqual(44)

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
