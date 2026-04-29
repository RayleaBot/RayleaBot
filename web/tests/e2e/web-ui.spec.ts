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

async function pushLogsInBatches(
  request: import('@playwright/test').APIRequestContext,
  count: number,
  buildPayload: (index: number) => Record<string, unknown>,
  batchSize = 24,
) {
  for (let start = 0; start < count; start += batchSize) {
    const end = Math.min(start + batchSize, count)
    await Promise.all(Array.from({ length: end - start }, (_, offset) => (
      request.post(`${backendUrl}/__test/push-log`, {
        data: buildPayload(start + offset),
      })
    )))
  }
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
  return page.locator('.logs-row')
}

function pluginScroller(page: import('@playwright/test').Page) {
  return page.locator('.plugins-data-table .ant-table-container')
}

function taskScroller(page: import('@playwright/test').Page) {
  return page.locator('.tasks-data-table .ant-table-container')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-feed-card .data-viewport__scroller')
}

function logDetailWindow(page: import('@playwright/test').Page) {
  return page.getByTestId('management-log-detail-window')
}

function logFilterField(page: import('@playwright/test').Page, label: string) {
  return page.locator('.logs-filter-grid .ant-form-item').filter({ hasText: label }).first()
}

function appHeader(page: import('@playwright/test').Page) {
  return page.getByTestId('app-header')
}

function dashboardConnectionCard(page: import('@playwright/test').Page) {
  return page.getByTestId('dashboard-connection-card')
}

function governanceEntryCard(
  container: import('@playwright/test').Locator,
  targetId: string,
) {
  return container.locator('tr').filter({ hasText: targetId }).first()
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

function hasRepeatedLogFilterParams(
  response: import('@playwright/test').Response,
  scope: 'current_session' | 'history',
) {
  if (response.request().method() !== 'GET') {
    return false
  }

  const url = new URL(response.url())
  const levels = url.searchParams.getAll('level')
  const pluginIds = url.searchParams.getAll('plugin_id')

  return url.pathname === '/api/logs'
    && url.searchParams.get('scope') === scope
    && levels.includes('warn')
    && levels.includes('error')
    && pluginIds.includes('weather')
    && pluginIds.includes('builtin-help')
}

async function expectRepeatedLogFilterControls(page: import('@playwright/test').Page) {
  const levelTags = logFilterField(page, '级别').locator('.ant-select-selection-item-content')
  await expect(levelTags.filter({ hasText: '警告' })).toHaveCount(1)
  await expect(levelTags.filter({ hasText: '错误' })).toHaveCount(1)

  const pluginTags = logFilterField(page, '插件').locator('.ant-select-selection-item-content')
  await expect(pluginTags.filter({ hasText: 'weather' })).toHaveCount(1)
  await expect(pluginTags.filter({ hasText: 'builtin-help' })).toHaveCount(1)
}

async function seedRepeatedLogFilterRows(
  request: import('@playwright/test').APIRequestContext,
  prefix: string,
) {
  const baseTimestamp = Date.now() - 20 * 60 * 1000
  const weatherRequestId = `req_${prefix}_weather`
  const builtinRequestId = `req_${prefix}_builtin_help`
  const weatherMessage = `${prefix} weather warn match`
  const builtinMessage = `${prefix} builtin help error match`
  const filteredLevelMessage = `${prefix} weather debug filtered out`
  const filteredPluginMessage = `${prefix} config panel error filtered out`
  const rows = [
    {
      log_id: `log_${prefix}_weather_warn_match`,
      timestamp: new Date(baseTimestamp + 1000).toISOString(),
      level: 'warn',
      source: 'runtime',
      plugin_id: 'weather',
      request_id: weatherRequestId,
      message: weatherMessage,
    },
    {
      log_id: `log_${prefix}_builtin_help_error_match`,
      timestamp: new Date(baseTimestamp + 2000).toISOString(),
      level: 'error',
      source: 'runtime',
      plugin_id: 'builtin-help',
      request_id: builtinRequestId,
      message: builtinMessage,
    },
    {
      log_id: `log_${prefix}_weather_debug_filtered`,
      timestamp: new Date(baseTimestamp + 3000).toISOString(),
      level: 'debug',
      source: 'runtime',
      plugin_id: 'weather',
      request_id: `req_${prefix}_filtered_level`,
      message: filteredLevelMessage,
    },
    {
      log_id: `log_${prefix}_config_panel_error_filtered`,
      timestamp: new Date(baseTimestamp + 4000).toISOString(),
      level: 'error',
      source: 'runtime',
      plugin_id: 'example-config-panel',
      request_id: `req_${prefix}_filtered_plugin`,
      message: filteredPluginMessage,
    },
  ]

  await pushLogsInBatches(request, rows.length, (index) => ({
    summary: rows[index],
    detail: {
      details: {
        branch: 'repeated-log-filters',
        case: prefix,
      },
    },
  }))

  return {
    builtinMessage,
    builtinRequestId,
    filteredLevelMessage,
    filteredPluginMessage,
    weatherMessage,
    weatherRequestId,
  }
}

async function expectRepeatedLogFilterRows(
  page: import('@playwright/test').Page,
  rows: Awaited<ReturnType<typeof seedRepeatedLogFilterRows>>,
) {
  const logsFeed = page.locator('.logs-feed-card')
  await expect(logsFeed).toContainText(rows.weatherMessage)
  await expect(logsFeed).toContainText(rows.builtinMessage)
  await expect(logsFeed).not.toContainText(rows.filteredLevelMessage)
  await expect(logsFeed).not.toContainText(rows.filteredPluginMessage)
}

async function navigateThroughMenu(
  page: import('@playwright/test').Page,
  item: string,
  group?: string,
) {
  const sider = page.locator('.admin-layout__sider')

  if (group) {
    const groupMenu = sider.locator('.ant-menu-submenu').filter({ hasText: group }).first()
    const targetItem = groupMenu.locator('.ant-menu-item').filter({ hasText: item }).first()
    if (!await targetItem.isVisible().catch(() => false)) {
      await groupMenu.locator('.ant-menu-submenu-title').click()
      await expect(targetItem).toBeVisible()
    }

    await targetItem.click()
    return
  }

  const targetItem = sider.locator('.ant-menu-item').filter({ hasText: item }).first()
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

test('launcher token query replaces a stale stored session and clears the URL token', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.addInitScript(() => {
    window.sessionStorage.setItem('rayleabot.session_token', 'stale-session-token')
  })

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
  await expect(page.getByText('包与运行信息')).toBeVisible()
  await expect(page.getByText('Manifest 元数据')).toBeVisible()
  await expect(page.getByText('运行配置')).toBeVisible()
  await expect(page.getByText('https://github.com/RayleaBot/plugins-weather')).toBeVisible()
  await expect(page.getByText('assets/overview.svg')).toBeVisible()
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

test('governance page manages blacklist and whitelist entries', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const sessionResponse = await request.post(`${backendUrl}/api/session/login`, {
    data: {
      identifier: 'admin',
      secret: 'fixture-only-secret',
    },
  })
  expect(sessionResponse.ok()).toBeTruthy()
  const { session_token: sessionToken } = await sessionResponse.json()
  const authHeaders = {
    Authorization: `Bearer ${sessionToken}`,
  }

  await Promise.all(Array.from({ length: 10 }, (_, index) => (
    request.post(`${backendUrl}/api/governance/whitelist/entries`, {
      headers: authHeaders,
      data: {
        entry_type: 'user',
        target_id: `31${String(index + 1).padStart(3, '0')}`,
        reason: `扩展白名单${index + 1}`,
      },
    })
  )))
  await Promise.all(Array.from({ length: 10 }, (_, index) => (
    request.post(`${backendUrl}/api/governance/blacklist/entries`, {
      headers: authHeaders,
      data: {
        entry_type: 'user',
        target_id: `41${String(index + 1).padStart(3, '0')}`,
        reason: `扩展黑名单${index + 1}`,
      },
    })
  )))

  await page.goto('/governance')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('governance-summary-card')).toContainText('治理总览')
  await expect(page.getByTestId('governance-summary-card').getByText('所有成员').first()).toBeVisible()
  await expect(page.getByText('10/60s')).toBeVisible()
  await expect(page.getByText('30/60s')).toBeVisible()

  const whitelistCard = page.getByTestId('governance-whitelist-card')
  const blacklistCard = page.getByTestId('governance-blacklist-card')
  await expect(whitelistCard).toContainText('10001')
  await expect(whitelistCard).toContainText('值班账号')
  await expect(whitelistCard).toContainText('31010')
  await expect(whitelistCard.locator('.ant-pagination')).toHaveCount(0)

  // --- Blacklist tab ---
  await page.locator('.governance-tabs .ant-tabs-tab').filter({ hasText: '黑名单' }).click()
  await expect(blacklistCard).toContainText('10001')
  await expect(blacklistCard).toContainText('41010')
  await expect(blacklistCard.locator('.ant-pagination')).toHaveCount(0)

  await page.getByTestId('governance-blacklist-add-btn').click()
  const blacklistModal = page.getByRole('dialog').filter({ hasText: '添加黑名单条目' })
  await blacklistModal.locator('.ant-input').nth(0).fill('30003')
  await blacklistModal.locator('.ant-input').nth(1).fill('临时封禁')
  await blacklistModal.locator('.ant-modal-footer button.ant-btn-primary').click()
  await expect(blacklistCard).toContainText('30003')
  await expect(blacklistCard).toContainText('临时封禁')

  await governanceEntryCard(blacklistCard, '30003').getByRole('button', { name: '移除' }).click()
  await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
  await expect(blacklistCard).not.toContainText('30003')

  // --- Whitelist tab ---
  await page.locator('.governance-tabs .ant-tabs-tab').filter({ hasText: '白名单' }).click()

  await page.getByTestId('governance-whitelist-add-btn').click()
  const whitelistModal = page.getByRole('dialog').filter({ hasText: '添加白名单条目' })
  await whitelistModal.locator('.ant-input').nth(0).fill('30003')
  await whitelistModal.locator('.ant-input').nth(1).fill('临时放行')
  await whitelistModal.locator('.ant-modal-footer button.ant-btn-primary').click()
  await expect(whitelistCard).toContainText('30003')
  await expect(whitelistCard).toContainText('临时放行')

  await page.getByTestId('governance-whitelist-enabled').dispatchEvent('click')
  await expect(page.getByTestId('governance-whitelist-enabled')).toHaveAttribute('aria-checked', 'false')

  for (const targetId of ['10001', '30003']) {
    await governanceEntryCard(whitelistCard, targetId).getByRole('button', { name: '移除' }).click()
    await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
    await expect(whitelistCard).not.toContainText(targetId)
  }

  await whitelistCard.locator('.governance-toolbar__filter').click()
  await page.locator('.ant-select-dropdown').getByTitle('群').click()
  await expect(whitelistCard).toContainText('20002')
  await expect(whitelistCard).toContainText('核心服务群')
  await governanceEntryCard(whitelistCard, '20002').getByRole('button', { name: '移除' }).click()
  await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
  await expect(whitelistCard).not.toContainText('20002')

  await Promise.all(Array.from({ length: 10 }, (_, index) => (
    request.delete(`${backendUrl}/api/governance/whitelist/entries/user/${encodeURIComponent(`31${String(index + 1).padStart(3, '0')}`)}`, {
      headers: authHeaders,
    })
  )))
  await page.getByRole('button', { name: '刷新状态' }).click()
  await expect(whitelistCard).not.toContainText('31010')

  await page.getByTestId('governance-whitelist-enabled').dispatchEvent('click')
  const confirmDialog = page.getByRole('dialog', { name: '确认启用空白名单' })
  await expect(confirmDialog).toContainText('当前没有任何白名单条目')
  await expect(confirmDialog).toContainText('除超级管理员外，所有命令都会被挡下')

  await confirmDialog.getByRole('button', { name: '确认启用' }).dispatchEvent('click')

  await expect(page.getByTestId('governance-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')
  await expect(whitelistCard).toContainText('白名单已启用且当前为空')
  await expect(whitelistCard).toContainText('除超级管理员外，所有命令都会被挡下')

  await page.reload()
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('governance-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')
  await expect(page.getByTestId('governance-whitelist-card')).toContainText('白名单已启用且当前为空')
})

test('plugin enable resumes after scope confirmation', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginEnableScopeChangedOnce: true,
  })
  await login(page)

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('未验证来源')).toBeVisible()
  const powerSwitch = page.locator('.plugin-detail-actions .plugin-holo-button')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'POST'
      && response.url().includes('/api/plugins/weather/disable')
      && response.status() === 200
    )),
    powerSwitch.click(),
  ])

  const enableSwitch = page.getByRole('switch', { name: '当前停用，点击切换为启用' })
  await expect(enableSwitch).toBeEnabled()

  await enableSwitch.click()

  const dialog = page.getByRole('dialog', { name: '重新确认插件权限' })
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('作用域发生变化')
  await expect(dialog).toContainText('http.request')
  await expect(dialog).not.toContainText('当前未声明权限')

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'POST'
      && response.url().includes('/api/plugins/weather/grants')
      && response.status() === 200
    )),
    page.waitForResponse((response) => (
      response.request().method() === 'POST'
      && response.url().includes('/api/plugins/weather/enable')
      && response.status() === 200
    )),
    dialog.getByRole('button', { name: '重新确认选中项' }).click(),
  ])

  await expect(dialog).toBeHidden()
  await expect(page.getByText('权限与授权')).toBeVisible()
  await expect(page.locator('.permission-item').filter({ hasText: 'http.request' })).toContainText('手动授权')
})

test('plugin management ui reads and saves plugin settings inside the detail page', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/plugins/example-config-panel')
  await expect(page.getByRole('heading', { name: 'example-config-panel', level: 1 })).toBeVisible()
  await expect(page.locator('.plugin-detail-panel-switch')).toContainText('概览')
  await expect(page.locator('.plugin-detail-panel-switch')).toContainText('配置页面')

  await page.locator('.plugin-detail-panel-switch').getByText('配置页面').click()
  await expect(page).toHaveURL(/panel=management-ui/)
  await expect(page.getByTestId('plugin-management-ui-confirm')).toBeVisible()
  await page.getByRole('button', { name: '确认并打开' }).click()

  const pluginFrame = page.frameLocator('[data-testid="plugin-management-ui-frame"]')
  await expect(pluginFrame.locator('#page-title')).toHaveText('配置页面')
  await expect(pluginFrame.locator('#plugin-id')).toHaveText('example-config-panel')
  await expect(pluginFrame.locator('#status-text')).toHaveText('已载入设置')
  await expect(pluginFrame.locator('#default-city-input')).toHaveValue('上海')
  await expect(pluginFrame.locator('#unit-select')).toHaveValue('fahrenheit')
  await expect(pluginFrame.locator('#settings-preview')).toContainText('"default_city": "上海"')
  await expect(pluginFrame.locator('#settings-preview')).toContainText('"unit": "fahrenheit"')

  await pluginFrame.locator('#default-city-input').fill('广州')
  await pluginFrame.locator('#unit-select').selectOption('celsius')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().includes('/api/plugins/example-config-panel/settings')
      && response.status() === 200
    )),
    pluginFrame.locator('#save-button').click(),
  ])

  await expect(pluginFrame.locator('#status-text')).toHaveText('设置已更新')
  await expect(pluginFrame.locator('#settings-preview')).toContainText('"default_city": "广州"')
  await expect(pluginFrame.locator('#settings-preview')).toContainText('"unit": "celsius"')

  await page.reload()
  await expect(page.getByRole('heading', { name: 'example-config-panel', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/panel=management-ui/)
  await expect(pluginFrame.locator('#status-text')).toHaveText('已载入设置')
  await expect(pluginFrame.locator('#default-city-input')).toHaveValue('广州')
  await expect(pluginFrame.locator('#unit-select')).toHaveValue('celsius')

  const tabLabels = await readTabLabels(page)
  expect(tabLabels.filter((label) => label === 'example-config-panel')).toHaveLength(1)
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

  const weatherRow = pluginRows(page).filter({ hasText: 'weather' }).first()
  await weatherRow.getByRole('button', { name: '查看概要' }).click()
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

test('logs page keeps the feed and floating detail window inside the viewport', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1600, height: 1200 })
  await login(page)

  const baseTimestamp = Date.now() - 2 * 60 * 60 * 1000
  await pushLogsInBatches(request, 96, (index) => ({
    summary: {
      log_id: `log_detail_window_seed_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_log_detail_window_seed',
      message: `detail window seed ${index}`,
    },
    detail: {
      log_id: `log_detail_window_seed_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_log_detail_window_seed',
      message: `detail window seed ${index}`,
      details: {
        branch: 'detail-window-seed',
        index,
      },
    },
  }))

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()

  const targetRow = logRows(page).last()
  await expect(targetRow).toBeVisible()
  await targetRow.click({ force: true })
  await expect(logDetailWindow(page)).toBeVisible()

  const initialMessage = (await logDetailWindow(page).locator('.log-detail-card__content--message').textContent())?.trim() ?? ''
  const header = logDetailWindow(page).locator('.log-detail-window__header')
  const headerBox = await header.boundingBox()
  expect(headerBox).not.toBeNull()

  await page.mouse.move(headerBox!.x + 80, headerBox!.y + 18)
  await page.mouse.down()
  await page.mouse.move(headerBox!.x - 520, headerBox!.y + 180, { steps: 8 })
  await page.mouse.up()

  const draggedBox = await logDetailWindow(page).boundingBox()
  expect(draggedBox).not.toBeNull()

  const alternateRow = page.locator('.logs-row').filter({ hasNotText: initialMessage }).first()
  await expect(alternateRow).toBeVisible()
  await alternateRow.click({
    position: { x: 48, y: 28 },
    force: true,
  })
  await expect(logDetailWindow(page)).toBeVisible()
  const selectedAlternateRow = page.locator('.logs-row.is-selected').first()
  await expect(selectedAlternateRow).toBeVisible()
  const selectedAlternateMessage = (await selectedAlternateRow.locator('.logs-row__message').textContent())?.trim() ?? ''
  await expect(logDetailWindow(page).locator('.log-detail-card__content--message')).toContainText(selectedAlternateMessage)

  const switchedMessage = (await logDetailWindow(page).locator('.log-detail-card__content--message').textContent())?.trim() ?? ''
  const switchedBox = await logDetailWindow(page).boundingBox()
  expect(switchedBox).not.toBeNull()
  expect(switchedMessage).not.toBe(initialMessage)
  expect(Math.abs((switchedBox?.x ?? 0) - (draggedBox?.x ?? 0))).toBeLessThanOrEqual(1)
  expect(Math.abs((switchedBox?.y ?? 0) - (draggedBox?.y ?? 0))).toBeLessThanOrEqual(1)

  await logScroller(page).evaluate((node) => {
    node.scrollTop = Math.max(0, node.scrollTop - 260)
    node.dispatchEvent(new Event('scroll'))
  })
  await expect(page.getByRole('button', { name: '滚动到最新' })).toBeVisible()

  const metrics = await page.evaluate(() => {
    const main = document.querySelector<HTMLElement>('#app-main')
    const layout = document.querySelector<HTMLElement>('.logs-layout')
    const feedCard = document.querySelector<HTMLElement>('.logs-feed-card')
    const feedBody = document.querySelector<HTMLElement>('.logs-feed-card .ant-card-body')
    const floating = document.querySelector<HTMLElement>('[data-testid="management-log-detail-window"]')
    const feedScroller = document.querySelector<HTMLElement>('.logs-feed-card .data-viewport__scroller')
    const detailScroller = document.querySelector<HTMLElement>('.log-detail-card__content--json')
    const layoutRect = layout?.getBoundingClientRect()
    const feedRect = feedCard?.getBoundingClientRect()
    const feedBodyRect = feedBody?.getBoundingClientRect()
    const floatingRect = floating?.getBoundingClientRect()

    return {
      viewportHeight: window.innerHeight,
      mainClientHeight: main?.clientHeight ?? 0,
      mainScrollHeight: main?.scrollHeight ?? 0,
      layoutBottom: layoutRect?.bottom ?? 0,
      feedLeft: feedRect?.left ?? 0,
      feedWidth: feedRect?.width ?? 0,
      feedBottom: feedRect?.bottom ?? 0,
      feedBodyBottom: feedBodyRect?.bottom ?? 0,
      floatingBottom: floatingRect?.bottom ?? 0,
      floatingLeft: floatingRect?.left ?? 0,
      floatingRight: floatingRect?.right ?? 0,
      floatingTop: floatingRect?.top ?? 0,
      feedClientHeight: feedScroller?.clientHeight ?? 0,
      feedScrollHeight: feedScroller?.scrollHeight ?? 0,
      detailClientHeight: detailScroller?.clientHeight ?? 0,
      detailScrollHeight: detailScroller?.scrollHeight ?? 0,
    }
  })

  expect(metrics.mainScrollHeight).toBeLessThanOrEqual(metrics.mainClientHeight + 1)
  expect(metrics.feedBottom).toBeLessThanOrEqual(metrics.viewportHeight + 1)
  expect(metrics.feedBodyBottom).toBeLessThanOrEqual(metrics.feedBottom + 1)
  expect(metrics.floatingTop).toBeGreaterThanOrEqual(0)
  expect(metrics.floatingBottom).toBeLessThanOrEqual(metrics.layoutBottom + 1)
  expect(metrics.floatingLeft).toBeGreaterThanOrEqual(metrics.feedLeft + metrics.feedWidth * 0.4)
  expect(metrics.floatingRight).toBeLessThanOrEqual(1600)
  expect(metrics.feedScrollHeight).toBeGreaterThanOrEqual(metrics.feedClientHeight)
  expect(metrics.feedClientHeight).toBeGreaterThan(0)
  expect(metrics.detailClientHeight).toBeGreaterThan(0)
})

test('history logs stay frozen until the user refreshes the anchor', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 60 * 60 * 1000

  await pushLogsInBatches(request, 51, (index) => ({
    summary: {
      log_id: `log_history_e2e_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_logs_history_e2e',
      message: `history row ${index}`,
    },
  }))

  await page.goto('/logs/history')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_logs_history_e2e')
  await page.getByRole('button', { name: '应用筛选' }).click()

  await expect(page.locator('.logs-row__message', { hasText: 'history row 50' })).toBeVisible()
  await expect.poll(async () => (
    logScroller(page).evaluate((node) => (
      node.scrollHeight - node.clientHeight - node.scrollTop
    ))
  )).toBeLessThanOrEqual(4)
  const initialMetrics = await logScroller(page).evaluate((node) => ({
    clientHeight: node.clientHeight,
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
  }))
  expect(initialMetrics.scrollHeight).toBeGreaterThan(initialMetrics.clientHeight)
  expect(initialMetrics.scrollHeight - initialMetrics.clientHeight - initialMetrics.scrollTop).toBeLessThanOrEqual(4)
  await expect(page.getByRole('button', { name: '更早记录' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '更新记录' })).toHaveCount(0)

  await logScroller(page).evaluate((node) => {
    node.scrollTop = 0
    node.dispatchEvent(new Event('scroll'))
  })
  await expect(page.locator('.logs-row__message', { hasText: 'history row 0' })).toBeVisible()

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_history_e2e_latest',
        timestamp: new Date(baseTimestamp + 60_000).toISOString(),
        level: 'info',
        source: 'runtime',
        request_id: 'req_logs_history_e2e',
        message: 'history row latest',
      },
    },
  })

  await expect(page.locator('.logs-row__message', { hasText: 'history row latest' })).toHaveCount(0)
  await page.getByRole('button', { name: '刷新到最新时间' }).click()
  await expect(page.locator('.logs-row__message', { hasText: 'history row latest' })).toBeVisible()

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.goto('/logs/history?request_id=req_logs_history_e2e')
  await expect(page.locator('.logs-row__message', { hasText: 'history row latest' })).toBeVisible()
})

test('current logs reveal older rows after scrolling to the top edge', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 2 * 60 * 60 * 1000

  await pushLogsInBatches(request, 151, (index) => ({
    summary: {
      log_id: `log_current_scroll_e2e_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_logs_current_scroll_e2e',
      message: `current scroll row ${index}`,
    },
  }))

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_logs_current_scroll_e2e')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_current_scroll_e2e')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])

  await expect(page.locator('.logs-row__message', { hasText: 'current scroll row 150' })).toBeVisible()
  await expect(page.locator('.logs-row__message', { hasText: 'current scroll row 0' })).toHaveCount(0)

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_current_scroll_e2e')
      && response.url().includes('direction=older')
    )),
    logScroller(page).evaluate((node) => {
      node.scrollTop = 0
      node.dispatchEvent(new Event('scroll'))
    }),
  ])

  await expect(page.locator('.logs-row__message', { hasText: 'current scroll row 0' })).toBeVisible()
})

test('current logs keep following new rows while follow mode stays active', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 2 * 60 * 60 * 1000

  await pushLogsInBatches(request, 121, (index) => ({
    summary: {
      log_id: `log_live_follow_keep_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_live_follow_keep',
      message: `live follow keep seed ${index}`,
    },
  }))

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_live_follow_keep')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_live_follow_keep')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])

  await expect(page.getByText('跟随最新')).toBeVisible()

  const before = await logScroller(page).evaluate((node) => ({
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
    clientHeight: node.clientHeight,
  }))

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_live_follow_keep_latest',
        timestamp: new Date(Date.now() + 90_000).toISOString(),
        level: 'info',
        source: 'runtime',
        request_id: 'req_live_follow_keep',
        message: `live follow keep latest ${'x'.repeat(480)}`,
      },
    },
  })

  await expect(page.locator('.logs-row__message', { hasText: 'live follow keep latest' })).toBeVisible()
  await expect(page.getByText('跟随最新')).toBeVisible()
  await expect(page.getByText('已暂停跟随')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '滚动到最新' })).toHaveCount(0)

  const after = await logScroller(page).evaluate((node) => ({
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
    clientHeight: node.clientHeight,
    distanceToBottom: node.scrollHeight - node.clientHeight - node.scrollTop,
  }))

  expect(after.scrollHeight).toBeGreaterThan(before.scrollHeight)
  expect(after.scrollTop).toBeGreaterThanOrEqual(before.scrollTop)
  expect(after.distanceToBottom).toBeLessThanOrEqual(2)
})

test('current logs stop following immediately when the user scrolls upward during live appends', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 2 * 60 * 60 * 1000

  await pushLogsInBatches(request, 121, (index) => ({
    summary: {
      log_id: `log_live_follow_pause_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_live_follow_pause',
      message: `live follow seed ${index}`,
    },
  }))

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_live_follow_pause')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_live_follow_pause')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])

  await expect(page.getByText('跟随最新')).toBeVisible()

  const bottomBefore = await logScroller(page).evaluate((node) => node.scrollTop)

  await page.evaluate(() => {
    const state = { index: 121, timer: 0 }
    state.timer = window.setInterval(() => {
        state.index += 1
        const current = state.index
        void fetch('http://127.0.0.1:4010/__test/push-log', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            summary: {
              log_id: `log_live_follow_pause_dyn_${current}`,
              timestamp: new Date(Date.now() + current * 1000).toISOString(),
              level: 'info',
              source: 'runtime',
              request_id: 'req_live_follow_pause',
              message: `live follow dyn ${current}`,
            },
          }),
        })
      }, 120)

    ;(window as Window & { __liveFollowPauseState?: typeof state }).__liveFollowPauseState = state
  })

  try {
    await logScroller(page).hover()
    for (let step = 0; step < 6; step += 1) {
      await page.mouse.wheel(0, -220)
      await page.waitForTimeout(80)
    }

    await expect(page.getByText('已暂停跟随')).toBeVisible()
    await expect(page.getByRole('button', { name: '滚动到最新' })).toBeVisible()

    await page.waitForTimeout(700)

    const metrics = await logScroller(page).evaluate((node) => ({
      scrollTop: node.scrollTop,
      scrollHeight: node.scrollHeight,
      clientHeight: node.clientHeight,
    }))

    expect(metrics.scrollTop).toBeLessThan(bottomBefore - 120)
    await expect(page.locator('.logs-row__message', { hasText: 'live follow dyn' })).toHaveCount(0)
  } finally {
    await page.evaluate(() => {
      const state = (window as Window & {
        __liveFollowPauseState?: { timer: number }
      }).__liveFollowPauseState
      if (state) {
        window.clearInterval(state.timer)
      }
    })
  }
})

test('current logs do not snap back to the bottom when scrolling upward without live updates', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 2 * 60 * 60 * 1000

  for (let index = 0; index < 161; index += 1) {
    await request.post(`${backendUrl}/__test/push-log`, {
      data: {
        summary: {
          log_id: `log_scroll_hold_${index}`,
          timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
          level: 'info',
          source: 'runtime',
          request_id: 'req_scroll_hold',
          message: `scroll hold row ${index} ${'x'.repeat((index % 5 + 1) * 32)}`,
        },
      },
    })
  }

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_scroll_hold')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_scroll_hold')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])

  const before = await logScroller(page).evaluate((node) => ({
    scrollTop: node.scrollTop,
    firstVisibleText: document.querySelector('.logs-row__message')?.textContent?.trim() ?? '',
  }))

  const afterImmediate = await logScroller(page).evaluate((node) => {
    node.scrollTop = Math.max(0, node.scrollTop - 260)
    node.dispatchEvent(new Event('scroll'))
    return {
      scrollTop: node.scrollTop,
      firstVisibleText: document.querySelector('.logs-row__message')?.textContent?.trim() ?? '',
    }
  })

  await page.waitForTimeout(250)

  const afterSettled = await logScroller(page).evaluate((node) => ({
    scrollTop: node.scrollTop,
    firstVisibleText: document.querySelector('.logs-row__message')?.textContent?.trim() ?? '',
  }))

  expect(afterImmediate.scrollTop).toBeLessThan(before.scrollTop)
  expect(afterSettled.scrollTop).toBeLessThan(before.scrollTop - 180)
  expect(afterSettled.firstVisibleText).not.toBe(before.firstVisibleText)
  await expect(page.getByText('已暂停跟随')).toBeVisible()
})

test('history logs reveal older rows after scrolling to the top edge', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const baseTimestamp = Date.now() - 3 * 60 * 60 * 1000

  await pushLogsInBatches(request, 251, (index) => ({
    summary: {
      log_id: `log_history_scroll_e2e_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_logs_history_scroll_e2e',
      message: `history scroll row ${index}`,
    },
  }))

  await page.goto('/logs/history')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_logs_history_scroll_e2e')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_history_scroll_e2e')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])

  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 250' })).toBeVisible()
  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 150' })).toHaveCount(0)
  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 0' })).toHaveCount(0)
  await expect.poll(async () => (
    logScroller(page).evaluate((node) => (
      node.scrollHeight - node.clientHeight - node.scrollTop
    ))
  )).toBeLessThanOrEqual(4)

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_history_scroll_e2e')
      && response.url().includes('direction=older')
    )),
    logScroller(page).evaluate((node) => {
      node.scrollTop = 0
      node.dispatchEvent(new Event('scroll'))
    }),
  ])

  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 0' })).toHaveCount(0)
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_history_scroll_e2e')
      && response.url().includes('direction=older')
    )),
    logScroller(page).evaluate((node) => {
      node.scrollTop = 0
      node.dispatchEvent(new Event('scroll'))
    }),
  ])

  await logScroller(page).evaluate((node) => {
    node.scrollTop = 0
    node.dispatchEvent(new Event('scroll'))
  })
  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 0' })).toBeVisible()
})

test('logs page reloads the latest page after hidden updates arrive', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '实时日志', '日志中心')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_logs_reactivate_e2e')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_reactivate_e2e')
    )),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])
  await expect(page.locator('.logs-row__message', { hasText: 'reactivate latest row' })).toHaveCount(0)

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

  await navigateThroughMenu(page, '实时日志', '日志中心')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await expect(page.locator('.logs-row__message', { hasText: 'reactivate latest row' }).first()).toBeVisible()
  await expect(page.getByText('跟随最新')).toBeVisible()
})

test('unsafe OneBot text stays escaped in current logs and history logs', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  const unsafeTimestamp = new Date(Date.now() - 5 * 60 * 1000).toISOString()

  await request.post(`${backendUrl}/__test/push-log`, {
    data: {
      summary: {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: unsafeTimestamp,
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_bridge_unsafe_0001',
        message: '721011692: [760384342]群星怒\u2066，大明云玩家\u202e~喵\u2069/没错，是魔法！(2896109796): 除了战猎这种抓不到加费就完全没法打的角色',
      },
      detail: {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: unsafeTimestamp,
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

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_bridge_unsafe_0001')
  await page.getByRole('button', { name: '应用筛选' }).click()

  const unsafeCurrentRow = page.locator('.logs-row').filter({ hasText: '群星怒' }).first()
  const unsafeCurrentMessage = unsafeCurrentRow.locator('.logs-row__message')
  await expect(unsafeCurrentMessage).toContainText('\\u2066')
  await unsafeCurrentRow.click()
  await expect(page.locator('.log-detail-card__content--message')).toContainText('\\u2066')
  await expect(page.locator('.log-detail-card__content--json')).toContainText('\\u2066')

  const currentTexts = await page.evaluate(() => ({
    row: document.querySelector('.logs-row .logs-row__message')?.textContent ?? '',
    detail: document.querySelector('.log-detail-card__content--message')?.textContent ?? '',
    json: document.querySelector('.log-detail-card__content--json')?.textContent ?? '',
  }))
  expect(currentTexts.row.includes('\u2066')).toBe(false)
  expect(currentTexts.detail.includes('\u2066')).toBe(false)
  expect(currentTexts.json.includes('\u2066')).toBe(false)

  await page.goto('/logs/history')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_bridge_unsafe_0001')
  await page.getByRole('button', { name: '应用筛选' }).click()
  const unsafeHistoryMessage = page.locator('.logs-row__message', { hasText: '群星怒' }).first()
  await expect(unsafeHistoryMessage).toContainText('\\u2066')

  const historyText = await unsafeHistoryMessage.evaluate((node) => node.textContent ?? '')
  expect(historyText.includes('\u2066')).toBe(false)
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

test('template preview auto-refreshes results without editor controls', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '模板预览', '系统')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect(page.getByText('模板不存在。')).toHaveCount(0)
  await expect(page).toHaveURL(/\/render\/templates\/help\.menu$/)
  expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)

  await expect(page.locator('.app-card__title-text').filter({ hasText: '模板信息' }).first()).toBeVisible()
  await expect(page.locator('.app-card__title-text').filter({ hasText: '输入结构' }).first()).toBeVisible()
  await expect(page.locator('.render-templates-card--editor')).toHaveCount(0)
  await expect(page.locator('.version-item')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '保存模板' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '执行校验' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '确认回退' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '生成预览' })).toHaveCount(0)

  const previewResult = page.getByTestId('render-template-preview-result')
  await expect(previewResult).toContainText('task_render_preview_0001')
  await expect(previewResult).toContainText('render_preview_0001.png')
  await expect(page.getByRole('img', { name: '模板预览结果' })).toBeVisible()

  await page.getByLabel('输入数据 JSON').fill('{\n  "title": "帮助菜单（自动刷新）"\n}')
  await expect(previewResult).toContainText('task_render_preview_0002')
  await expect(previewResult).toContainText('render_preview_0002.png')

  await page.locator('.template-nav-item').filter({ hasText: 'status.panel' }).first().click()
  await expect(page).toHaveURL(/\/render\/templates\/status\.panel$/)
  expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)
  await expect(previewResult).toContainText('task_render_preview_0003')
  await expect(previewResult).toContainText('render_preview_0003.png')
  await expect(page.locator('.summary-grid')).toContainText('status.panel')
})

test('template preview page stays scrollable on shorter viewports', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1280, height: 640 })
  await login(page)

  await page.goto('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect(page.getByTestId('render-template-preview-result')).toContainText('task_render_preview_0001')

  const appMain = page.locator('#app-main')
  const initialMetrics = await appMain.evaluate((node) => ({
    clientHeight: node.clientHeight,
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
  }))

  expect(initialMetrics.scrollHeight).toBeGreaterThan(initialMetrics.clientHeight)

  await appMain.hover()
  await page.mouse.wheel(0, 1200)

  await expect.poll(async () => (
    appMain.evaluate((node) => node.scrollTop)
  )).toBeGreaterThan(initialMetrics.scrollTop)
})

test('protocol center owns OneBot settings and logs center keeps protocol filtering', async ({ page, request }) => {
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
  await expect(page.getByRole('button', { name: '查看实时日志' })).toBeVisible()

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await expect(page.locator('.logs-toolbar').getByText('协议')).toHaveCount(1)

  const protocolField = page.locator('.logs-filter-grid .ant-form-item').filter({ hasText: '协议' })
  await protocolField.locator('.ant-select').click()
  await page.getByTitle('OneBot11').click()
  await page.getByRole('button', { name: '应用筛选' }).click()

  await expect(page.getByText('ignored OneBot API response with unsupported echo')).toBeVisible()
  await expect(page.getByText('plugin runtime stderr truncated')).toHaveCount(0)

  const protocolRow = page.locator('.logs-row').filter({ hasText: 'ignored OneBot API response with unsupported echo' }).first()
  await protocolRow.click()
  await expect(page.locator('.log-detail-card__content--json')).toContainText('api response echo must be a non-empty string')
})

test('template preview task detail links return to the preview workspace', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/tasks?task_id=task_render_preview_0001')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await page.getByRole('button', { name: '打开模板预览' }).click()
  await expect.poll(() => page.url()).toContain('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)
})

test('management links connect protocol, logs, plugin, and commands workspaces', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/logs?protocol=onebot11')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()

  const protocolLog = page.locator('.logs-row').filter({ hasText: 'ignored OneBot API response with unsupported echo' }).first()
  await protocolLog.click()
  await expect(logDetailWindow(page)).toBeVisible()

  await logDetailWindow(page).getByRole('button', { name: '查看协议' }).click()
  await expect.poll(() => page.url()).toContain('/protocols')
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()

  await page.getByRole('button', { name: '兼容矩阵' }).click()
  await expect.poll(() => page.url()).toContain('/protocols/compatibility')
  await expect(page.getByRole('heading', { name: '协议兼容矩阵', level: 1 })).toBeVisible()

  await page.goto('/protocols')
  await page.getByRole('button', { name: '查看实时日志' }).click()
  await expect.poll(() => page.url()).toContain('/logs')
  await expect(page.url()).toContain('protocol=onebot11')

  await page.goto('/logs?plugin_id=weather')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.locator('.logs-row').filter({ hasText: 'plugin runtime stderr truncated' }).first().click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '查看插件' }).click()
  await expect.poll(() => page.url()).toContain('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()

  await page.getByRole('button', { name: '当前插件指令' }).click()
  await expect.poll(() => page.url()).toContain('/commands')
  await expect(page.url()).toContain('plugin_id=weather')
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  await expect(page.locator('.commands-section-card').filter({ hasText: '全部声明命令' })).toContainText('weather')
  await expect((await readTabLabels(page)).filter((label) => label === '指令中心')).toHaveLength(1)

  await page.goto('/tasks?task_id=task_render_preview_0001')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await page.getByRole('button', { name: '打开模板预览' }).click()
  await expect.poll(() => page.url()).toContain('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)
})

test('repeated log filters restore current logs and preserve workspace jumps', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const rows = await seedRepeatedLogFilterRows(request, 'repeated_current')

  await page.goto('/logs?level=warn&level=error&plugin_id=weather&plugin_id=builtin-help')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await expectRepeatedLogFilterControls(page)

  await Promise.all([
    page.waitForResponse((response) => hasRepeatedLogFilterParams(response, 'current_session')),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])
  await expectRepeatedLogFilterRows(page, rows)

  const weatherRow = logRows(page).filter({ hasText: rows.weatherMessage }).first()
  await expect(weatherRow).toBeVisible()
  await weatherRow.click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '相关实时日志' }).click()

  await expect.poll(() => new URL(page.url()).pathname).toBe('/logs')
  await expect.poll(() => new URL(page.url()).searchParams.get('request_id')).toBe(rows.weatherRequestId)
  const currentUrl = new URL(page.url())
  expect(currentUrl.searchParams.getAll('level')).toHaveLength(0)
  expect(currentUrl.searchParams.getAll('plugin_id')).toHaveLength(0)
  await expect(page.locator('.logs-feed-card')).toContainText(rows.weatherMessage)
  expect((await readTabLabels(page)).filter((label) => label === '实时日志')).toHaveLength(1)
})

test('repeated log filters restore history logs and preserve workspace jumps', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const rows = await seedRepeatedLogFilterRows(request, 'repeated_history')

  await page.goto('/logs/history?level=warn&level=error&plugin_id=weather&plugin_id=builtin-help')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('start_at'))).toBe(true)
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('end_at'))).toBe(true)
  await expectRepeatedLogFilterControls(page)

  await Promise.all([
    page.waitForResponse((response) => hasRepeatedLogFilterParams(response, 'history')),
    page.getByRole('button', { name: '应用筛选' }).click(),
  ])
  await expectRepeatedLogFilterRows(page, rows)

  const historyUrl = new URL(page.url())
  expect(historyUrl.searchParams.getAll('level')).toEqual(expect.arrayContaining(['warn', 'error']))
  expect(historyUrl.searchParams.getAll('plugin_id')).toEqual(expect.arrayContaining(['weather', 'builtin-help']))
  expect(historyUrl.searchParams.get('start_at')).toBeTruthy()
  expect(historyUrl.searchParams.get('end_at')).toBeTruthy()

  const builtinRow = logRows(page).filter({ hasText: rows.builtinMessage }).first()
  await expect(builtinRow).toBeVisible()
  await builtinRow.click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '相关历史日志' }).click()

  await expect.poll(() => new URL(page.url()).pathname).toBe('/logs/history')
  await expect.poll(() => new URL(page.url()).searchParams.get('request_id')).toBe(rows.builtinRequestId)
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('start_at'))).toBe(true)
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('end_at'))).toBe(true)
  const relatedHistoryUrl = new URL(page.url())
  expect(relatedHistoryUrl.searchParams.getAll('level')).toHaveLength(0)
  expect(relatedHistoryUrl.searchParams.getAll('plugin_id')).toHaveLength(0)
  await expect(page.locator('.logs-feed-card')).toContainText(rows.builtinMessage)
  expect((await readTabLabels(page)).filter((label) => label === '历史日志')).toHaveLength(1)
})

test('logs page filters both history and live log appends', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.locator('.logs-filter-grid .ant-form-item').filter({ hasText: '来源' }).locator('input').fill('runtime')
  await page.getByRole('button', { name: '应用筛选' }).click()

  const logsTable = page.locator('.logs-feed-card')
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

test('logs page keeps older current-session rows reachable inside the table scroller', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()

  await pushLogsInBatches(request, 36, (index) => ({
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
  }))

  const jumpToBottomButton = page.getByRole('button', { name: '回到底部' })
  if (await jumpToBottomButton.isVisible().catch(() => false)) {
    await jumpToBottomButton.click()
  }
  await expect(page.locator('.logs-row__message', { hasText: 'scroll history row 35' })).toBeVisible()

  const metrics = await page.evaluate(() => {
    const doc = document.scrollingElement ?? document.documentElement
    const tableBody = document.querySelector<HTMLElement>('.logs-feed-card .data-viewport__scroller')
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
  const effectivePoliciesTable = page.locator('.commands-section-card').filter({ hasText: '生效命令策略' }).locator('.commands-data-table')
  const declaredCommandsTable = page.locator('.commands-section-card').filter({ hasText: '全部声明命令' }).locator('.commands-data-table')

  await expect(page.getByTestId('commands-open-governance')).toBeVisible()
  await expect(page.getByText('治理总览', { exact: true })).toHaveCount(0)
  await expect(page.getByText('白名单', { exact: true })).toHaveCount(0)
  await expect(page.getByText('黑名单', { exact: true })).toHaveCount(0)
  await expect(effectivePoliciesTable).toContainText('hello')
  await expect(declaredCommandsTable).toContainText('weather')

  const pluginSelector = page.locator('.commands-filter-toolbar .ant-select').first()
  await expect(pluginSelector).toBeVisible()
  await pluginSelector.click()
  await page.keyboard.type('Weather')
  await page.keyboard.press('Enter')

  await expect(effectivePoliciesTable).toContainText('weather')
  await expect(effectivePoliciesTable).not.toContainText('hello')
  await expect(declaredCommandsTable).toContainText('查询天气')
  await expect(declaredCommandsTable).not.toContainText('查看帮助')

  await page.getByTestId('commands-open-governance').click()
  await expect.poll(() => page.url()).toContain('/governance')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
})

test('breadcrumb and tabbar track leaf pages instead of hidden route groups', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--single/)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-link')).toHaveCount(0)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-current')).toHaveText('系统状态')

  await page.goto('/governance')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--multi/)
  await expect(page.locator('.admin-layout__header-breadcrumb').getByRole('link', { name: '运维' })).toHaveAttribute('href', '/governance')
  await expect(page.locator('.admin-layout__breadcrumb-current')).toHaveText('权限策略')
  await expect(page.getByRole('tab', { name: '权限策略' })).toBeVisible()

  let tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance'])
  expect(await readActiveTabLabel(page)).toBe('权限策略')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '运维' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(3)

  await page.goto('/commands')
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略', '指令中心'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands'])
  expect(await readActiveTabLabel(page)).toBe('指令中心')

  await page.goto('/tasks')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略', '指令中心', '任务'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands', 'tasks'])
  expect(await readActiveTabLabel(page)).toBe('任务')

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略', '指令中心', '任务', '实时日志'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands', 'tasks', 'logs'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')
  await expect(page.getByRole('tab', { name: '权限策略' })).toBeVisible()
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
  expect(await readTabLabels(page)).toEqual(['系统状态', '权限策略', '指令中心', '任务', '实时日志', '协议中心'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands', 'tasks', 'logs', 'protocols'])
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '协议' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(2)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态', '权限策略', '指令中心', '任务', '实时日志', '协议中心', '配置'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands', 'tasks', 'logs', 'protocols', 'config'])
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '系统' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(2)
  await expect(page.locator('.admin-layout__sider .ant-menu-item-selected .admin-layout__menu-icon')).toHaveCount(1)

  await page.getByRole('tab', { name: '权限策略' }).click()
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/governance$/)

  await page.reload()
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态', '权限策略', '指令中心', '任务', '实时日志', '协议中心', '配置'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'governance', 'commands', 'tasks', 'logs', 'protocols', 'config'])

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
  await navigateThroughMenu(page, '历史日志', '日志中心')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '历史日志')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '权限策略', '运维')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '权限策略')

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
  await logRows(page).first().click({ force: true })
  await expect(page.locator('.ant-drawer')).toBeVisible()
  await expect(logDetailWindow(page)).toHaveCount(0)
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
})
