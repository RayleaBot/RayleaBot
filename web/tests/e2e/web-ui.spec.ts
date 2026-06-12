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

async function setBackendNetworkOffline(request: import('@playwright/test').APIRequestContext) {
  await request.post(`${backendUrl}/__test/network-offline`)
}

async function setBackendNetworkOnline(request: import('@playwright/test').APIRequestContext) {
  await request.post(`${backendUrl}/__test/network-online`)
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

async function createBackupTaskRows(
  request: import('@playwright/test').APIRequestContext,
  count: number,
) {
  for (let start = 0; start < count; start += 20) {
    const end = Math.min(start + 20, count)
    await Promise.all(Array.from({ length: end - start }, (_, offset) => {
      const index = start + offset
      const sequenceText = String(index + 1).padStart(4, '0')
      return request.post(`${backendUrl}/__test/push-task`, {
        data: {
          task_id: `task_backup_create_scroll_${sequenceText}`,
          task_type: 'backup.create',
          status: 'succeeded',
          progress: 100,
          summary: `任务滚动 ${index + 1}`,
        },
      })
    }))
  }
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
  return page.locator('.tasks-data-table .ant-table-content')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-feed-card .data-viewport__scroller')
}

function logDetailWindow(page: import('@playwright/test').Page) {
  return page.getByTestId('management-log-detail-window')
}

async function expectThirdPartyAccountCardsContained(page: import('@playwright/test').Page) {
  const metrics = await page.evaluate(() => {
    const tolerance = 1
    const viewportOverflow = document.documentElement.scrollWidth - document.documentElement.clientWidth
    const cards = Array.from(document.querySelectorAll<HTMLElement>('.account-card'))
    const overflowingCards = cards.map((card) => {
      const rect = card.getBoundingClientRect()
      return {
        className: card.className,
        inlineOverflow: card.scrollWidth - card.clientWidth,
        rightOverflow: rect.right - document.documentElement.clientWidth,
      }
    }).filter((card) => card.inlineOverflow > tolerance || card.rightOverflow > tolerance)

    return {
      viewportOverflow,
      overflowingCards,
    }
  })

  expect(metrics.viewportOverflow).toBeLessThanOrEqual(1)
  expect(metrics.overflowingCards).toEqual([])
}

async function expectThirdPartyMonitoringCardsContained(page: import('@playwright/test').Page) {
  const metrics = await page.evaluate(() => {
    const tolerance = 1
    const viewportOverflow = document.documentElement.scrollWidth - document.documentElement.clientWidth
    const cards = Array.from(document.querySelectorAll<HTMLElement>('.monitor-card'))
    const overflowingCards = cards.map((card) => {
      const rect = card.getBoundingClientRect()
      return {
        className: card.className,
        inlineOverflow: card.scrollWidth - card.clientWidth,
        rightOverflow: rect.right - document.documentElement.clientWidth,
      }
    }).filter((card) => card.inlineOverflow > tolerance || card.rightOverflow > tolerance)

    return {
      viewportOverflow,
      overflowingCards,
    }
  })

  expect(metrics.viewportOverflow).toBeLessThanOrEqual(1)
  expect(metrics.overflowingCards).toEqual([])
}

async function expectThirdPartyAccountAvatarImageFillsFrame(page: import('@playwright/test').Page) {
  const metrics = await expectAvatarImageFillsFrame(page, 'bilibili-account-avatar-image', '.account-avatar')

  const frameInsetTolerance = 2
  expect(metrics).not.toBeNull()
  expect(metrics!.imageWidth).toBeGreaterThanOrEqual(metrics!.avatarWidth - frameInsetTolerance)
  expect(metrics!.imageHeight).toBeGreaterThanOrEqual(metrics!.avatarHeight - frameInsetTolerance)
  expect(metrics!.stringTransform).toBe('none')
}

async function expectThirdPartyMonitorAvatarImageFillsFrame(page: import('@playwright/test').Page) {
  const metrics = await expectAvatarImageFillsFrame(page, 'third-party-monitor-avatar-image', '.monitor-avatar')

  const frameInsetTolerance = 2
  expect(metrics).not.toBeNull()
  expect(metrics!.imageWidth).toBeGreaterThanOrEqual(metrics!.avatarWidth - frameInsetTolerance)
  expect(metrics!.imageHeight).toBeGreaterThanOrEqual(metrics!.avatarHeight - frameInsetTolerance)
  expect(metrics!.stringTransform).toBe('none')
}

async function expectAvatarImageFillsFrame(page: import('@playwright/test').Page, testId: string, avatarSelector: string) {
  return page.getByTestId(testId).first().evaluate((image, selector) => {
    const avatar = image.closest<HTMLElement>(selector)
    if (!avatar) {
      return null
    }
    const avatarRect = avatar.getBoundingClientRect()
    const imageRect = image.getBoundingClientRect()
    const string = image.closest<HTMLElement>('.ant-avatar-string')
    const stringStyle = string ? getComputedStyle(string) : null
    return {
      avatarHeight: avatarRect.height,
      avatarWidth: avatarRect.width,
      imageHeight: imageRect.height,
      imageWidth: imageRect.width,
      stringTransform: stringStyle?.transform ?? '',
    }
  }, avatarSelector)
}

async function clickConfigTocItem(page: import('@playwright/test').Page, label: string) {
  const desktopItem = page.locator('.config-toc').getByText(label, { exact: true })
  if (await desktopItem.first().isVisible().catch(() => false)) {
    await desktopItem.click()
    return
  }

  await page.locator('.config-toc-inline').getByText(label, { exact: true }).click()
}

async function scrollConfigSectionIntoView(page: import('@playwright/test').Page, sectionKey: string) {
  await page.locator(`#config-section-${sectionKey}`).scrollIntoViewIfNeeded()
}

async function visibleLogRowHeights(page: import('@playwright/test').Page) {
  return logRows(page).evaluateAll((rows) => (
    rows
      .slice(0, 6)
      .map((row) => Math.round(row.getBoundingClientRect().height))
  ))
}

async function visibleLogVirtualRowGaps(page: import('@playwright/test').Page) {
  return page.locator('.logs-feed-card .data-viewport__row').evaluateAll((rows) => {
    function translateY(row: Element) {
      const transform = window.getComputedStyle(row).transform
      if (!transform || transform === 'none') {
        return 0
      }

      const matrix3d = /^matrix3d\((.+)\)$/.exec(transform)
      if (matrix3d?.[1]) {
        const parts = matrix3d[1].split(',').map((part) => Number(part.trim()))
        return parts[13] ?? 0
      }

      const matrix = /^matrix\((.+)\)$/.exec(transform)
      if (matrix?.[1]) {
        const parts = matrix[1].split(',').map((part) => Number(part.trim()))
        return parts[5] ?? 0
      }

      const translate = /translateY\(([-\d.]+)px\)/.exec(transform)
      return translate?.[1] ? Number(translate[1]) : 0
    }

    const starts = rows.slice(0, 7).map(translateY)
    return starts.slice(1).map((start, index) => Math.round(start - starts[index]))
  })
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

async function fillRateLimit(
  page: import('@playwright/test').Page,
  label: string,
  count: string,
  windowValue: string,
  unit?: '秒' | '分钟' | '小时',
) {
  await page.getByLabel(`${label} 次数`).fill(count)
  await page.getByLabel(`${label} 时间窗口`).fill(windowValue)
  if (unit) {
    await page.getByLabel(`${label} 单位`).click()
    await page.getByTitle(unit).click()
  }
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

async function openStandardTabs(page: import('@playwright/test').Page) {
  await page.evaluate(() => window.localStorage.removeItem('rayleabot.ui-shell'))
  await page.goto('/')
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()

  await navigateThroughMenu(page, '任务', '运维')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await expect.poll(() => readTabLabels(page)).toEqual(['系统状态', '任务'])

  await navigateThroughMenu(page, '实时日志', '日志中心')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()

  await expect.poll(() => readTabLabels(page)).toEqual(['系统状态', '任务', '实时日志'])
  await expect.poll(() => readActiveTabLabel(page)).toBe('实时日志')
}

async function openTabContextMenu(page: import('@playwright/test').Page, tabName: string) {
  await page.locator('.admin-layout__tabbar .ant-tabs-tab')
    .filter({ hasText: tabName })
    .locator('.admin-layout__tab-label')
    .click({ button: 'right' })
  await expect(page.getByTestId('tab-context-menu')).toBeVisible()
}

async function clickTabContextAction(page: import('@playwright/test').Page, actionName: string) {
  await page.getByTestId('tab-context-menu').getByRole('menuitem', { name: actionName }).click()
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
    && pluginIds.includes('raylea.echo')
}

async function expectRepeatedLogFilterControls(page: import('@playwright/test').Page) {
  const levelTags = logFilterField(page, '级别').locator('.ant-select-selection-item-content')
  await expect(levelTags.filter({ hasText: '警告' })).toHaveCount(1)
  await expect(levelTags.filter({ hasText: '错误' })).toHaveCount(1)

  const pluginTags = logFilterField(page, '插件').locator('.ant-select-selection-item-content')
  await expect(pluginTags.filter({ hasText: 'weather' })).toHaveCount(1)
  await expect(pluginTags.filter({ hasText: 'raylea.echo' })).toHaveCount(1)
}

async function seedRepeatedLogFilterRows(
  request: import('@playwright/test').APIRequestContext,
  prefix: string,
) {
  const baseTimestamp = Date.now() - 20 * 60 * 1000
  const weatherRequestId = `req_${prefix}_weather`
  const echoRequestId = `req_${prefix}_echo`
  const weatherMessage = `${prefix} weather warn match`
  const echoMessage = `${prefix} echo error match`
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
      log_id: `log_${prefix}_echo_error_match`,
      timestamp: new Date(baseTimestamp + 2000).toISOString(),
      level: 'error',
      source: 'runtime',
      plugin_id: 'raylea.echo',
      request_id: echoRequestId,
      message: echoMessage,
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
    echoMessage,
    echoRequestId,
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
  await expect(logsFeed).toContainText(rows.echoMessage)
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

test('protected deep links return to the target after login', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/plugins?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '登录', level: 1 })).toBeVisible()
  await expect(page.getByTestId('auth-theme-toggle')).toBeVisible()
  await expect(page.getByTestId('auth-language')).toBeVisible()

  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()

  await expect(page.locator('#app-main').getByRole('heading', { name: '插件列表', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/plugins\?token=launcher_token_fixture_0001$/)
})

test('setup-required deep links return to the target after initialization', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/plugins?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '创建管理员账号', level: 1 })).toBeVisible()
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: '创建并进入管理界面' }).click()

  await expect(page.locator('#app-main').getByRole('heading', { name: '插件列表', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/plugins\?token=launcher_token_fixture_0001$/)
})

test('plugin management flow covers install, grants and console recovery', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/plugins')
  await expect(page.locator('#app-main').getByRole('heading', { name: '插件列表', level: 1 })).toBeVisible()
  await expect(pluginRows(page).first()).toBeVisible()
  await expect(page.locator('.plugins-data-table')).toContainText('example-config-panel')
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
  await expect(page.getByText('未验证来源').first()).toBeVisible()
  await expect(page.getByText('plugins/installed').first()).toBeVisible()
  await expect(page.getByText('运行摘要')).toBeVisible()
  await expect(page.getByText('包信息')).toBeVisible()
  await expect(page.getByText('来源信息')).toBeVisible()
  await expect(page.getByText('运行配置')).toBeVisible()
  await page.getByText('详细信息').click()
  await expect(page.getByText('Manifest 元数据')).toBeVisible()
  await expect(page.getByText('https://github.com/RayleaBot/plugins-weather')).toBeVisible()
  await expect(page.getByText('assets/overview.svg')).toBeVisible()
  await expect(page.getByText('命令冲突').first()).toBeVisible()
  await expect(page.getByRole('tab', { name: '插件指令' })).toBeVisible()
  await expect(page.getByText('查询天气')).toBeVisible()

  await page.getByRole('tab', { name: /权限与授权/ }).click()
  await expect(page.getByRole('button', { name: '处理权限' })).toBeVisible()
  await page.getByRole('button', { name: '处理权限' }).click()
  const renderPermissionChoice = page.locator('.permission-dialog-list .ant-checkbox-wrapper').filter({ hasText: '生成渲染图片' })
  await expect(renderPermissionChoice).toContainText('生成渲染图片')
  await expect(renderPermissionChoice.locator('[title="原始能力：render.image"]')).toBeVisible()
  await page.getByRole('checkbox', { name: /render\.image/ }).check()
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'POST'
      && response.url().includes('/api/plugins/weather/grants')
    )),
    page.getByRole('button', { name: '授权选中项' }).click(),
  ])

  const renderPermission = page.locator('.permission-item').filter({ hasText: '生成渲染图片' })
  await expect(renderPermission.locator('[title="原始能力：render.image"]')).toBeVisible()
  await expect(renderPermission).toContainText('已授权')
  await expect(renderPermission).toContainText('手动授权')

  await page.getByRole('tab', { name: '实时控制台' }).click()
  await expect(page.locator('.console-terminal').first()).toBeVisible()
  await page.getByRole('button', { name: '清空输出' }).click()
  await expect(page.getByText('等待控制台输出')).toBeVisible()
  await closeSocket(request, 'plugin_console')
  await page.getByRole('button', { name: '重新连接' }).click()
  await expect(page.locator('.console-terminal').first()).toBeVisible()
})

test('access lists page manages blacklist and whitelist entries', async ({ page, request }) => {
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

  for (let index = 0; index < 10; index += 1) {
    await request.post(`${backendUrl}/api/governance/whitelist/entries`, {
      headers: authHeaders,
      data: {
        entry_type: 'user',
        target_id: `31${String(index + 1).padStart(3, '0')}`,
        reason: `扩展白名单${index + 1}`,
      },
    })
  }
  for (let index = 0; index < 10; index += 1) {
    await request.post(`${backendUrl}/api/governance/blacklist/entries`, {
      headers: authHeaders,
      data: {
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
  await expect(whitelistCard.locator('.ant-pagination')).toHaveCount(0)

  await expect(blacklistCard).toContainText('10001')
  await expect(blacklistCard).toContainText('41010')
  await expect(blacklistCard.locator('.ant-pagination')).toHaveCount(0)

  await page.getByTestId('access-lists-blacklist-add-btn').click()
  await page.getByTestId('blacklist-draft-target-id').fill('30003')
  await page.getByTestId('blacklist-draft-reason').fill('临时封禁')
  await page.getByTestId('blacklist-draft-save').click()
  await expect(blacklistCard).toContainText('30003')
  await expect(blacklistCard).toContainText('临时封禁')

  await governanceEntryCard(blacklistCard, '30003').getByRole('button', { name: '移除' }).click()
  await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
  await expect(blacklistCard).not.toContainText('30003')

  await page.getByTestId('access-lists-whitelist-add-btn').click()
  await page.getByTestId('whitelist-draft-target-id').fill('30003')
  await page.getByTestId('whitelist-draft-reason').fill('临时放行')
  await page.getByTestId('whitelist-draft-save').click()
  await expect(whitelistCard).toContainText('30003')
  await expect(whitelistCard).toContainText('临时放行')

  await page.getByTestId('access-lists-whitelist-enabled').dispatchEvent('click')
  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'false')

  for (const targetId of ['10001', '30003']) {
    await governanceEntryCard(whitelistCard, targetId).getByRole('button', { name: '移除' }).click()
    await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
    await expect(whitelistCard).not.toContainText(targetId)
  }

  await whitelistCard.locator('.access-lists-toolbar__filter').click()
  await page.locator('.ant-select-dropdown').getByTitle('群').click()
  await expect(whitelistCard).toContainText('20002')
  await expect(whitelistCard).toContainText('核心服务群')
  await governanceEntryCard(whitelistCard, '20002').getByRole('button', { name: '移除' }).click()
  await page.locator('.ant-popconfirm-buttons button.ant-btn-primary').click()
  await expect(whitelistCard).not.toContainText('20002')

  for (let index = 0; index < 10; index += 1) {
    await request.delete(`${backendUrl}/api/governance/whitelist/entries/user/${encodeURIComponent(`31${String(index + 1).padStart(3, '0')}`)}`, {
      headers: authHeaders,
    })
  }
  await page.goto('/access-lists')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
  await expect(whitelistCard).not.toContainText('31010')

  await page.getByTestId('access-lists-whitelist-enabled').dispatchEvent('click')
  const confirmDialog = page.getByRole('dialog', { name: '确认启用空白名单' })
  await expect(confirmDialog).toContainText('当前没有任何白名单条目')
  await expect(confirmDialog).toContainText('除超级管理员外，所有命令都会被挡下')

  await confirmDialog.getByRole('button', { name: '确认启用' }).dispatchEvent('click')

  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')
  await expect(whitelistCard).toContainText('白名单已启用且当前为空')
  await expect(whitelistCard).toContainText('除超级管理员外，所有命令都会被挡下')

  await page.reload()
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
  await expect(page.getByTestId('access-lists-whitelist-enabled')).toHaveAttribute('aria-checked', 'true')
  await expect(page.getByTestId('access-lists-whitelist-card')).toContainText('白名单已启用且当前为空')
})

test('permission policy page edits command policy config', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('permission-policy-summary-card').getByText('所有成员').first()).toBeVisible()
  await expect(page.getByText('配置超级管理员、默认权限级别和聊天命令速率限制。')).toHaveCount(0)
  await expect(page.getByText('策略总览')).toHaveCount(0)
  await expect(page.getByTestId('permission-policy-unsaved-status')).toHaveCount(0)

  const superAdminsInput = page.getByTestId('permission-policy-super-admins').locator('input')
  await superAdminsInput.click()
  await superAdminsInput.fill('10002')
  await superAdminsInput.press('Enter')
  await page.getByLabel('默认权限级别').click()
  await page.getByTitle('群管理员').click()
  await expect(page.getByText('用户命令速率限制')).toHaveCount(0)
  await expect(page.getByText('群命令速率限制')).toHaveCount(0)
  await expect(page.getByText('冷却提示')).toHaveCount(0)

  await expect(page.getByTestId('permission-policy-unsaved-status')).toContainText('有未保存更改')

  await page.getByTestId('permission-policy-save').click()
  await expect(page.getByTestId('permission-policy-save-status')).toContainText('保存完成，已生效')
  await expect(page.getByTestId('permission-policy-unsaved-status')).toHaveCount(0)
  await expect(page.getByTestId('permission-policy-summary-card').getByText('群管理员').first()).toBeVisible()

  await page.getByTestId('permission-policy-open-access-lists').click()
  await expect.poll(() => page.url()).toContain('/access-lists')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('permission-policy-save-status')).toHaveCount(0)
})

test('plugin enable resumes after scope confirmation', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginEnableScopeChangedOnce: true,
  })
  await login(page)

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()
  await expect(page.getByText('未验证来源').first()).toBeVisible()
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
  await expect(dialog).toContainText('发起 HTTP 请求')
  await expect(dialog.locator('[title="原始能力：http.request"]')).toBeVisible()
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
  await expect(page.locator('.permission-item').filter({ hasText: '发起 HTTP 请求' })).toContainText('手动授权')
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

  await createBackupTaskRows(request, 80)
  await page.goto('/tasks')
  const tasksBody = taskScroller(page)
  await expect(tasksBody).toBeVisible()
  expect((await tasksBody.getAttribute('style')) ?? '').not.toContain('560px')
  expect((await tasksBody.getAttribute('style')) ?? '').not.toContain('620px')
  await expect(taskRows(page).first()).toBeVisible()
  const taskScrollMetrics = await tasksBody.evaluate((node) => {
    const style = window.getComputedStyle(node)
    return {
      clientHeight: node.clientHeight,
      overflowY: style.overflowY,
      scrollHeight: node.scrollHeight,
      scrollTop: node.scrollTop,
    }
  })
  expect(taskScrollMetrics.scrollHeight).toBeGreaterThan(taskScrollMetrics.clientHeight + 100)
  expect(['auto', 'scroll']).toContain(taskScrollMetrics.overflowY)
  await tasksBody.evaluate((node) => {
    node.scrollTop = node.scrollHeight
    node.dispatchEvent(new Event('scroll'))
  })
  await expect.poll(() => tasksBody.evaluate((node) => node.scrollTop)).toBeGreaterThan(0)
  await expect(taskRows(page).filter({ hasText: 'task_backup_create_scroll_0080' }).first()).toBeVisible()

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
  await expect.poll(async () => {
    const heights = await visibleLogRowHeights(page)
    return heights.length > 0 && heights.every((height) => height >= 72 && height <= 88)
  }).toBe(true)
  const initialRowHeights = await visibleLogRowHeights(page)
  const initialVirtualRowGaps = await visibleLogVirtualRowGaps(page)
  expect(initialVirtualRowGaps).not.toHaveLength(0)
  expect(initialVirtualRowGaps.every((gap) => gap === 80)).toBe(true)
  await page.waitForTimeout(300)
  expect(await visibleLogRowHeights(page)).toEqual(initialRowHeights)
  expect(await visibleLogVirtualRowGaps(page)).toEqual(initialVirtualRowGaps)

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

test('history logs stay frozen until a new anchor is loaded', async ({ page, request }) => {
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

  await expect(page.locator('.logs-row__message', { hasText: 'current scroll row 45' })).toBeVisible()
  await logScroller(page).evaluate(async (node) => {
    for (let frame = 0; frame < 4; frame += 1) {
      await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()))
    }
    node.scrollTop = 0
    node.dispatchEvent(new Event('scroll'))
  })
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

  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 145' })).toBeVisible()
  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 0' })).toHaveCount(0)
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().includes('/api/logs?')
      && response.url().includes('request_id=req_logs_history_scroll_e2e')
      && response.url().includes('direction=older')
    )),
    logScroller(page).evaluate((node) => {
      node.scrollTop = 200
      node.dispatchEvent(new Event('scroll'))
      node.scrollTop = 0
      node.dispatchEvent(new Event('scroll'))
    }),
  ])

  await expect(page.locator('.logs-row__message', { hasText: 'history scroll row 45' })).toBeVisible()
  await logScroller(page).evaluate((node) => {
    node.scrollTop = 0
    node.dispatchEvent(new Event('scroll'))
  })
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

  await navigateThroughMenu(page, '插件列表', '插件中心')
  await expect(page.getByRole('heading', { name: '插件列表', level: 1 })).toBeVisible()

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
        message: '10001: [20001]测试群名片\u2066，测试用户昵称\u202e~喵\u2069/测试用户昵称(30001): 测试消息内容',
      },
      detail: {
        log_id: 'log_bridge_unsafe_0001',
        timestamp: unsafeTimestamp,
        level: 'info',
        source: 'bridge',
        protocol: 'onebot11',
        request_id: 'req_bridge_unsafe_0001',
        message: '10001: [20001]测试群名片\u2066，测试用户昵称\u202e~喵\u2069/测试用户昵称(30001): 测试消息内容',
        details: {
          direction: 'inbound',
          self_id: '10001',
          conversation_id: '20001',
          conversation_type: 'group',
          group_name: '测试群',
          sender: {
            user_id: '30001',
            nickname: '测试用户昵称',
            card: '测试群名片\u2066，测试用户昵称\u202e~喵\u2069',
            role: 'member',
          },
          plain_text: '测试消息内容',
        },
      },
    },
  })

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.getByPlaceholder('例如 req_*').fill('req_bridge_unsafe_0001')
  await page.getByRole('button', { name: '应用筛选' }).click()

  const unsafeCurrentRow = page.locator('.logs-row').filter({ hasText: '测试群名片' }).first()
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
  const unsafeHistoryMessage = page.locator('.logs-row__message', { hasText: '测试群名片' }).first()
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
    const page = document.querySelector<HTMLElement>('.config-page')
    const toc = Array.from(document.querySelectorAll<HTMLElement>('.config-toc, .config-toc-inline'))
      .find((element) => {
        const style = window.getComputedStyle(element)
        return style.display !== 'none' && style.visibility !== 'hidden' && element.clientWidth > 0
      }) ?? null
    const sections = document.querySelectorAll('.config-section')

    return {
      bodyClientHeight: document.body.clientHeight,
      bodyScrollHeight: document.body.scrollHeight,
      docClientHeight: doc.clientHeight,
      docScrollHeight: doc.scrollHeight,
      mainClientHeight: main?.clientHeight ?? 0,
      mainScrollHeight: main?.scrollHeight ?? 0,
      pageWidth: page?.clientWidth ?? 0,
      pageScrollWidth: page?.scrollWidth ?? 0,
      tocWidth: toc?.clientWidth ?? 0,
      tocScrollWidth: toc?.scrollWidth ?? 0,
      sectionCount: sections.length,
    }
  })

  expect(metrics.docScrollHeight).toBeGreaterThanOrEqual(metrics.docClientHeight)
  expect(metrics.bodyScrollHeight).toBeGreaterThanOrEqual(metrics.bodyClientHeight)
  expect(metrics.mainScrollHeight).toBeGreaterThan(metrics.mainClientHeight)
  expect(metrics.pageWidth).toBeGreaterThan(0)
  expect(metrics.pageScrollWidth).toBeLessThanOrEqual(metrics.pageWidth + 1)
  expect(metrics.tocWidth).toBeGreaterThan(0)
  expect(metrics.tocScrollWidth).toBeLessThanOrEqual(metrics.tocWidth + 1)
  expect(metrics.sectionCount).toBeGreaterThan(4)
})

test('config page edits general IPC rate limit with split inputs', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()

  await scrollConfigSectionIntoView(page, 'runtime')
  await fillRateLimit(page, 'IPC 突发限制', '180', '5')
  await expect(page.getByText('5 秒内最多 180 次')).toBeVisible()

  await scrollConfigSectionIntoView(page, 'message')
  await expect(page.getByText('目标消息速率限制')).toHaveCount(0)

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().endsWith('/api/config')
    )),
    page.getByRole('button', { name: '保存更改' }).click(),
  ])

  await scrollConfigSectionIntoView(page, 'runtime')
  await expect(page.getByText('5 秒内最多 180 次')).toBeVisible()
  await scrollConfigSectionIntoView(page, 'message')
  await expect(page.getByText('目标消息速率限制')).toHaveCount(0)
})

test('rate limits page edits chat and outbound limits', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '限流中心', '运维')
  await expect(page.getByRole('heading', { name: '限流中心', level: 1 })).toBeVisible()
  await expect(page.getByTestId('rate-limits-unsaved-status')).toHaveCount(0)

  await fillRateLimit(page, '用户命令速率限制', '20', '60')
  await fillRateLimit(page, '群命令速率限制', '60', '60')
  await page.getByLabel('命中后发送冷却提示').dispatchEvent('click')
  await fillRateLimit(page, '插件消息速率限制', '30', '10')
  await fillRateLimit(page, '目标消息速率限制', '12', '1', '分钟')

  await expect(page.getByTestId('rate-limits-unsaved-status')).toContainText('有未保存更改')
  await expect(page.getByText('60 秒内最多 20 次')).toBeVisible()
  await expect(page.getByText('60 秒内最多 60 次')).toBeVisible()
  await expect(page.getByText('10 秒内最多 30 次')).toBeVisible()
  await expect(page.getByText('1 分钟内最多 12 次')).toBeVisible()

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().endsWith('/api/config')
    )),
    page.getByTestId('rate-limits-save').click(),
  ])

  await expect(page.getByTestId('rate-limits-unsaved-status')).toHaveCount(0)
  await expect(page.getByTestId('rate-limits-save-status')).toContainText('保存完成，已生效')
  await expect(page.getByText('保存结果')).toHaveCount(0)

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByText('用户命令速率限制')).toHaveCount(0)
  await expect(page.getByText('群命令速率限制')).toHaveCount(0)

  await page.goto('/plugins/settings')
  await expect(page.getByRole('heading', { name: '插件设置', level: 1 })).toBeVisible()
  await expect(page.getByText('插件消息速率限制')).toHaveCount(0)
})

test('plugin settings page edits plugin global config', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '插件设置', '插件中心')
  await expect(page.getByRole('heading', { name: '插件设置', level: 1 })).toBeVisible()
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toHaveCount(0)

  const commandPrefixesInput = page.getByTestId('plugin-settings-command-prefixes').locator('input')
  await commandPrefixesInput.click()
  await commandPrefixesInput.fill('!')
  await commandPrefixesInput.press('Enter')
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toContainText('有未保存更改')

  await page.getByLabel('自动授权能力').fill('logger.write\nmessage.send')

  await fillRateLimit(page, '插件日志速率限制', '300', '10')
  await expect(
    page.locator('.plugin-settings-setting-row').filter({ hasText: '插件日志速率限制' }).locator('.plugin-settings-rate-preview'),
  ).toContainText('10 秒内最多 300 次')

  await expect(page.getByText('插件消息速率限制')).toHaveCount(0)

  await page.getByLabel('插件工作目录软上限（MB）').fill('512')

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().endsWith('/api/config')
    )),
    page.getByTestId('plugin-settings-save').click(),
  ])

  await expect(page.getByTestId('plugin-settings-unsaved-status')).toHaveCount(0)
  await expect(page.getByTestId('plugin-settings-save-status')).toContainText('保存完成，已生效')
  await expect(page.getByText('保存结果')).toHaveCount(0)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  await expect(page.locator('.config-page')).not.toContainText('命令前缀')
  await expect(page.locator('.config-page')).not.toContainText('自动授权能力')
  await expect(page.locator('.config-page')).not.toContainText('插件日志速率限制')
  await expect(page.locator('.config-page')).not.toContainText('插件消息速率限制')
  await expect(page.locator('.config-page')).not.toContainText('插件工作目录软上限')
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

test('template preview auto-updates results without editor controls', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect(page.getByText('模板不存在。')).toHaveCount(0)
  await expect(page).toHaveURL(/\/render\/templates\/help\.menu$/)
  expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)

  await expect(page.locator('.render-templates-float-panel')).toContainText('模板 ID')
  await expect(page.locator('.render-templates-float-panel')).toContainText('help.menu')
  await expect(page.locator('.render-templates-float-panel')).toContainText('渲染参数')
  await expect(page.locator('.render-templates-float-panel')).toContainText('输入结构')
  await expect(page.locator('.render-templates-card--editor')).toHaveCount(0)
  await expect(page.locator('.version-item')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '保存模板' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '执行校验' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '确认回退' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '生成预览' })).toHaveCount(0)
  await expect(page.getByText('任务 ID')).toHaveCount(0)
  await expect(page.getByText('产物 ID')).toHaveCount(0)
  await expect(page.getByText('缓存结果')).toHaveCount(0)

  const previewResult = page.getByTestId('render-template-preview-result')
  const previewFrame = page.getByTestId('render-template-preview-frame')
  await expect(previewFrame).toBeVisible()
  await expect(previewFrame).toHaveAttribute('srcdoc', /帮助菜单/)

  await page.getByLabel('输入数据 JSON').fill('{\n  "title": "帮助菜单（自动同步）"\n}')
  await expect(previewFrame).toHaveAttribute('srcdoc', /帮助菜单（自动同步）/)

  await page.locator('.template-nav-item').filter({ hasText: 'status.panel' }).first().click()
  await expect(page).toHaveURL(/\/render\/templates\/status\.panel$/)
  expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)
  await expect(page.getByTestId('render-template-preview-frame')).toHaveAttribute('data-template-id', 'status.panel')
  await expect(page.locator('.render-templates-float-panel')).toContainText('status.panel')
})

test('template preview scales wide templates without horizontal scrollbars', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1280, height: 760 })
  await login(page)

  await page.goto('/render/templates/fortune.stats')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()

  const previewResult = page.getByTestId('render-template-preview-result')
  const previewHost = page.getByTestId('render-template-preview-host')
  const previewFrame = page.getByTestId('render-template-preview-frame')

  await expect(previewFrame).toBeVisible()
  await expect(previewFrame).toHaveAttribute('data-template-id', 'fortune.stats')
  await expect(previewFrame).toHaveAttribute('data-preview-frame-width', '1124')
  await expect(previewFrame).toHaveAttribute('srcdoc', /overflow-x:hidden!important/)

  await expect.poll(async () => (
    previewFrame.evaluate(async (frame) => {
      const iframe = frame as HTMLIFrameElement
      const image = iframe.contentDocument?.querySelector<HTMLImageElement>('.external-preview-image')
      await iframe.contentDocument?.fonts?.ready
      return {
        fontReady: iframe.contentDocument?.fonts?.check('16px RayleaExternalPreview') ?? false,
        imageComplete: Boolean(image?.complete),
        imageWidth: image?.naturalWidth ?? 0,
      }
    })
  )).toEqual({
    fontReady: true,
    imageComplete: true,
    imageWidth: 1,
  })

  const frameMetrics = await previewFrame.evaluate((frame) => {
    const iframe = frame as HTMLIFrameElement
    const documentElement = iframe.contentDocument?.documentElement
    const body = iframe.contentDocument?.body
    return {
      bodyClientWidth: body?.clientWidth ?? 0,
      bodyScrollWidth: body?.scrollWidth ?? 0,
      documentClientWidth: documentElement?.clientWidth ?? 0,
      documentScrollWidth: documentElement?.scrollWidth ?? 0,
    }
  })
  expect(frameMetrics.documentScrollWidth).toBeLessThanOrEqual(frameMetrics.documentClientWidth)
  expect(frameMetrics.bodyScrollWidth).toBeLessThanOrEqual(frameMetrics.bodyClientWidth)

  const hostMetrics = await previewHost.evaluate((node) => ({
    clientWidth: node.clientWidth,
    scrollWidth: node.scrollWidth,
  }))
  expect(hostMetrics.scrollWidth).toBeLessThanOrEqual(hostMetrics.clientWidth)

  const resultMetrics = await previewResult.evaluate((node) => ({
    clientWidth: node.clientWidth,
    scrollWidth: node.scrollWidth,
  }))
  expect(resultMetrics.scrollWidth).toBeLessThanOrEqual(resultMetrics.clientWidth)
})

test('template preview page stays scrollable on shorter viewports', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1280, height: 640 })
  await login(page)

  await page.goto('/render/templates/help.menu')
  await expect(page.getByRole('heading', { name: '模板预览', level: 1 })).toBeVisible()
  await expect(page.getByTestId('render-template-preview-frame')).toHaveAttribute('srcdoc', /帮助菜单/)

  const scrollPanel = page.locator('.render-templates-float-panel__body')
  const initialMetrics = await scrollPanel.evaluate((node) => ({
    clientHeight: node.clientHeight,
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
  }))

  expect(initialMetrics.scrollHeight).toBeGreaterThan(initialMetrics.clientHeight)

  const scrollPanelBox = await scrollPanel.boundingBox()
  if (!scrollPanelBox) {
    throw new Error('.render-templates-float-panel__body is not visible')
  }
  await page.mouse.move(
    scrollPanelBox.x + scrollPanelBox.width - 24,
    scrollPanelBox.y + Math.min(scrollPanelBox.height - 24, 320),
  )
  await page.mouse.wheel(0, 1200)

  await expect.poll(async () => (
    scrollPanel.evaluate((node) => node.scrollTop)
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
  await expect(page.locator('.integrated-protocol-table')).toContainText('主动连接 WebSocket')

  const reverseTransportRow = page.locator('.integrated-protocol-table tr').filter({ hasText: '回连 WebSocket' }).first()
  await page.getByLabel('回连地址').fill('wss://bot.example.com/reverse/onebot')
  await page.getByText('展开更多配置项').click()
  await page.getByLabel('连接超时（秒）').fill('18')
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存并已生效')).toBeVisible()
  await expect(reverseTransportRow).toContainText('未启用')

  await page.reload()
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  await expect(page.locator('.integrated-protocol-table tr').filter({ hasText: '回连 WebSocket' }).first()).toContainText('未启用')

  await page.locator('.integrated-protocol-table tr').filter({ hasText: '回连 WebSocket' }).first().getByRole('switch', { name: '回连 WebSocket' }).click()
  await page.getByRole('button', { name: '保存协议设置' }).click()
  await expect(page.getByText('配置已保存并已生效')).toBeVisible()
  await expect(page.locator('.integrated-protocol-table tr').filter({ hasText: '回连 WebSocket' }).first()).toContainText('等待 OneBot 回连')
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
  await expect(logDetailWindow(page)).toBeHidden()
  await expect(page.locator('.ant-drawer-mask')).toHaveCount(0)

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
  await expect(page.locator('.commands-data-table')).toContainText('weather')
  await expect((await readTabLabels(page)).filter((label) => label === '指令中心')).toHaveLength(1)

  await page.goto('/tasks?task_id=task_plugin_install_succeeded_0001')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  await page.getByRole('button', { name: '查看插件' }).click()
  await expect.poll(() => page.url()).toContain('/plugins/weather')
})

test('repeated log filters restore current logs and preserve workspace jumps', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const rows = await seedRepeatedLogFilterRows(request, 'repeated_current')

  await page.goto('/logs?level=warn&level=error&plugin_id=weather&plugin_id=raylea.echo')
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

  await page.goto('/logs/history?level=warn&level=error&plugin_id=weather&plugin_id=raylea.echo')
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
  expect(historyUrl.searchParams.getAll('plugin_id')).toEqual(expect.arrayContaining(['weather', 'raylea.echo']))
  expect(historyUrl.searchParams.get('start_at')).toBeTruthy()
  expect(historyUrl.searchParams.get('end_at')).toBeTruthy()

  const echoRow = logRows(page).filter({ hasText: rows.echoMessage }).first()
  await expect(echoRow).toBeVisible()
  await echoRow.click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '相关历史日志' }).click()

  await expect.poll(() => new URL(page.url()).pathname).toBe('/logs/history')
  await expect.poll(() => new URL(page.url()).searchParams.get('request_id')).toBe(rows.echoRequestId)
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('start_at'))).toBe(true)
  await expect.poll(() => Boolean(new URL(page.url()).searchParams.get('end_at'))).toBe(true)
  const relatedHistoryUrl = new URL(page.url())
  expect(relatedHistoryUrl.searchParams.getAll('level')).toHaveLength(0)
  expect(relatedHistoryUrl.searchParams.getAll('plugin_id')).toHaveLength(0)
  await expect(page.locator('.logs-feed-card')).toContainText(rows.echoMessage)
  expect((await readTabLabels(page)).filter((label) => label === '历史日志')).toHaveLength(1)
})

test('logs pages load plugin options only when the plugin filter is opened', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const pluginRequests: string[] = []
  page.on('request', (requestEvent) => {
    if (requestEvent.method() === 'GET' && requestEvent.url().includes('/api/plugins')) {
      pluginRequests.push(requestEvent.url())
    }
  })

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.waitForTimeout(200)
  expect(pluginRequests).toHaveLength(0)

  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().endsWith('/api/plugins')
    )),
    logFilterField(page, '插件').locator('.ant-select').click(),
  ])
  expect(pluginRequests).toHaveLength(1)

  await navigateThroughMenu(page, '历史日志', '日志中心')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await page.waitForTimeout(200)
  expect(pluginRequests).toHaveLength(1)

  await logFilterField(page, '插件').locator('.ant-select').click()
  await page.waitForTimeout(200)
  expect(pluginRequests).toHaveLength(1)
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
  const commandsTable = page.locator('.commands-data-table')

  await expect(page.getByTestId('commands-open-permission-policy')).toBeVisible()
  await expect(page.getByText('策略总览', { exact: true })).toHaveCount(0)
  await expect(page.getByText('白名单', { exact: true })).toHaveCount(0)
  await expect(page.getByText('黑名单', { exact: true })).toHaveCount(0)
  await expect(commandsTable).toContainText('hello')
  await expect(commandsTable).toContainText('weather')

  const pluginSelector = page.locator('.commands-filter-toolbar .ant-select').first()
  await expect(pluginSelector).toBeVisible()
  await pluginSelector.click()
  await page.keyboard.type('Weather')
  await page.keyboard.press('Enter')

  await expect(commandsTable).toContainText('weather')
  await expect(commandsTable).not.toContainText('hello')
  await expect(commandsTable).toContainText('查询天气')
  await expect(commandsTable).not.toContainText('查看帮助')

  await page.getByTestId('commands-open-permission-policy').click()
  await expect.poll(() => page.url()).toContain('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
})

test('breadcrumb and tabbar track leaf pages instead of hidden route groups', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--single/)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-link')).toHaveCount(0)
  await expect(page.locator('.admin-layout__header-breadcrumb .admin-layout__breadcrumb-current')).toHaveText('系统状态')

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.locator('.admin-layout__header-breadcrumb')).toHaveClass(/admin-layout__header-breadcrumb--multi/)
  await expect(page.locator('.admin-layout__header-breadcrumb').getByRole('link', { name: '运维' })).toHaveAttribute('href', '/permission-policy')
  await expect(page.locator('.admin-layout__breadcrumb-current')).toHaveText('权限策略')
  await expect(page.getByRole('tab', { name: '权限策略' })).toBeVisible()

  let tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'permission-policy'])
  expect(await readActiveTabLabel(page)).toBe('权限策略')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '运维' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(5)

  await page.goto('/commands')
  await expect(page.getByRole('heading', { name: '指令中心', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '指令中心'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'commands'])
  expect(await readActiveTabLabel(page)).toBe('指令中心')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '插件中心' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(3)

  await page.goto('/tasks')
  await expect(page.getByRole('heading', { name: '任务', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '任务'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'tasks'])
  expect(await readActiveTabLabel(page)).toBe('任务')

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '实时日志'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'logs'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')
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
  expect(await readActiveTabLabel(page)).toBe('协议中心')
  expect(await readTabLabels(page)).toContain('协议中心')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '协议' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(2)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('配置')
  expect(await readTabLabels(page)).toContain('配置')
  await expect(page.locator('.admin-layout__sider .ant-menu-submenu-open').filter({ hasText: '系统' }).locator('.ant-menu-item .admin-layout__menu-icon')).toHaveCount(2)
  await expect(page.locator('.admin-layout__sider .ant-menu-item-selected .admin-layout__menu-icon')).toHaveCount(1)

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/permission-policy$/)

  await page.reload()
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('权限策略')
  expect(await readTabLabels(page)).toContain('权限策略')

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('weather')
  expect(await readTabLabels(page)).toContain('weather')
  expect(await readTabIconKeys(page)).toContain('appstore')
})

test('tab context menu closes tabs relative to the clicked tab', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await openStandardTabs(page)
  await openTabContextMenu(page, '任务')
  await clickTabContextAction(page, '关闭当前标签')
  await expect(page).toHaveURL(/\/logs$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '实时日志'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')

  await openStandardTabs(page)
  await openTabContextMenu(page, '任务')
  await clickTabContextAction(page, '关闭其他标签')
  await expect(page).toHaveURL(/\/tasks$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '任务'])
  expect(await readActiveTabLabel(page)).toBe('任务')

  await openStandardTabs(page)
  await openTabContextMenu(page, '实时日志')
  await clickTabContextAction(page, '关闭左侧标签')
  await expect(page).toHaveURL(/\/logs$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '实时日志'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')

  await openStandardTabs(page)
  await openTabContextMenu(page, '任务')
  await clickTabContextAction(page, '关闭右侧标签')
  await expect(page).toHaveURL(/\/tasks$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '任务'])
  expect(await readActiveTabLabel(page)).toBe('任务')

  await openStandardTabs(page)
  await openTabContextMenu(page, '任务')
  await clickTabContextAction(page, '关闭所有标签')
  await expect(page).toHaveURL(/\/$/)
  expect(await readTabLabels(page)).toEqual(['系统状态'])
  expect(await readActiveTabLabel(page)).toBe('系统状态')
})

test('nested admin pages animate only once when entering grouped routes', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '协议中心', '协议')
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '协议中心')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '插件列表', '插件中心')
  await expect(page.getByRole('heading', { name: '插件列表', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '插件列表')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '历史日志', '日志中心')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '历史日志')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '权限策略', '运维')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '权限策略')

  await startTransitionSampling(page)
  await navigateThroughMenu(page, '黑白名单', '运维')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
  expectSingleEnterTransition(await collectTransitionSamples(page), '黑白名单')

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

test('third-party accounts show Bilibili CK cards and QR login fills the editor', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/third-party-accounts')
  await expect(page.getByRole('heading', { name: '三方账号', level: 1 })).toBeVisible()
  await expect(page.locator('.source-summary-strip')).toHaveCount(0)

  const accountCard = page.locator('.account-card').filter({ hasText: '测试账号昵称' }).first()
  await expect(accountCard).toBeVisible()
  await expect(accountCard).toContainText('UID 123456')
  await expect(accountCard).toContainText('CK 有效')
  await expect(accountCard).toContainText('轮询')
  await expect(accountCard).toContainText('上次使用')
  const avatarImage = accountCard.getByTestId('bilibili-account-avatar-image')
  await expect(avatarImage).toBeVisible()
  await expect(avatarImage).toHaveAttribute('src', /external-preview\/avatar\.png/)
  await expectThirdPartyAccountAvatarImageFillsFrame(page)
  await avatarImage.evaluate((element) => element.dispatchEvent(new Event('error')))
  await expect(accountCard.getByTestId('bilibili-account-avatar-fallback')).toBeVisible()
  await expect(accountCard.getByTestId('bilibili-account-avatar-image')).toHaveCount(0)
  await expectThirdPartyAccountCardsContained(page)

  await accountCard.getByRole('button', { name: '编辑' }).click()
  const editingExistingCard = page.locator('.account-card--editing').filter({ hasText: '留空时保留当前 CK。' }).first()
  await expect(editingExistingCard.locator('textarea')).toBeVisible()
  await expect(editingExistingCard).toContainText('留空时保留当前 CK。')
  await expectThirdPartyAccountCardsContained(page)
  await editingExistingCard.getByRole('button', { name: /取\s*消/ }).click()

  await page.getByRole('button', { name: '添加 Bilibili CK' }).first().click()
  await page.getByRole('button', { name: '添加 Bilibili CK' }).first().click()
  const draftCards = page.locator('.account-card--editing').filter({ hasText: 'Bilibili CK' })
  await expect(draftCards).toHaveCount(2)
  await expectThirdPartyAccountCardsContained(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await expectThirdPartyAccountCardsContained(page)
  await page.setViewportSize({ width: 1280, height: 720 })
  await expectThirdPartyAccountCardsContained(page)
  const draftCard = page.locator('.account-card--editing').nth(0)
  await expect(draftCard).toBeVisible()
  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()
  await expect(draftCard.locator('.qr-panel')).toContainText('等待确认')
  await expect(draftCard.locator('.qr-panel')).toContainText('有效期至')
  await expect(draftCard.locator('.qr-panel .ant-qrcode')).toHaveCount(1)
  await expectThirdPartyAccountCardsContained(page)
  const scannedDraftCard = page.locator('.account-card--editing').filter({ has: page.locator('.qr-panel') }).first()
  const scannedInputs = scannedDraftCard.locator('input')
  await expect(scannedInputs.nth(0)).toHaveValue('123456', { timeout: 5000 })
  await expect(scannedInputs.nth(1)).toHaveValue('测试账号昵称')
  await expect(scannedDraftCard.locator('textarea')).toHaveValue(/SESSDATA=fixture/)
  await scannedDraftCard.getByRole('button', { name: /保\s*存/ }).click()
  const savedQRCodeAccountCard = page.locator('.account-card').filter({ hasText: '账号 ID123456' }).first()
  await expect(savedQRCodeAccountCard).toBeVisible()
  await expect(savedQRCodeAccountCard).toContainText('CK 有效')
  await expect(savedQRCodeAccountCard.getByTestId('bilibili-account-avatar-image')).toBeVisible()
  await expectThirdPartyAccountCardsContained(page)
})

test('third-party monitoring shows Bilibili targets with realtime updates', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/third-party-monitoring')
  await expect(page.getByRole('heading', { name: '三方监控', level: 1 })).toBeVisible()
  const diagnosisBar = page.locator('.monitoring-strip')
  await expect(diagnosisBar.getByTestId('third-party-monitoring-live-indicator')).toContainText('实时更新中', { timeout: 10000 })
  await expect(diagnosisBar).toContainText('直播备用检查中')
  await expect(diagnosisBar).toContainText('运行受限')
  await expect(diagnosisBar).toContainText('原因')
  await expect(diagnosisBar).toContainText('影响')
  await expect(diagnosisBar).toContainText('处理')
  await expect(diagnosisBar).toContainText('动态接收不受影响')
  await expect(diagnosisBar).toContainText('CK 有效')
  await expect(diagnosisBar).not.toContainText('降级检查')
  await expect(page.locator('.uid-strip')).toContainText('UID 123456')

  const monitorCard = page.locator('.monitor-card').filter({ hasText: '测试 UP' }).first()
  await expect(monitorCard).toBeVisible()
  await expect(monitorCard).toContainText('UID 123456')
  await expect(monitorCard).toContainText('新视频标题')
  await expect(monitorCard).toContainText('监控更新时间')
  await expect(monitorCard).toContainText('开播中')
  await expect(monitorCard).toContainText('直播间标题')
  await expect(monitorCard).toContainText('10001')
  await expect(monitorCard).toContainText('已连接')
  await expect(monitorCard.getByRole('link', { name: '测试 UP' })).toHaveAttribute('href', 'https://space.bilibili.com/123456/')
  await expect(monitorCard.getByRole('link', { name: '新视频标题' })).toHaveAttribute('href', 'https://www.bilibili.com/video/BV1RayleaBot')
  const monitorAvatar = monitorCard.getByTestId('third-party-monitor-avatar-image')
  await expect(monitorAvatar).toBeVisible()
  await expect(monitorAvatar).toHaveAttribute('src', /^blob:/)
  await expectThirdPartyMonitorAvatarImageFillsFrame(page)
  await expectThirdPartyMonitoringCardsContained(page)

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(diagnosisBar).toContainText('直播备用检查中')
  await expect(diagnosisBar).toContainText('处理')
  await expectThirdPartyMonitoringCardsContained(page)
  await expect(monitorCard).toContainText('新视频标题')
  await page.setViewportSize({ width: 1280, height: 720 })
})

test('error recovery covers retry and uninstall failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginsListOnce: true,
    failPluginDetailOnce: true,
    failUninstallOnce: true,
  })
  await login(page)

  await page.goto('/plugins')
  await expect(page.getByRole('heading', { name: '哎呀！出错了' })).toBeVisible()
  await page.getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByText('weather').first()).toBeVisible()

  const weatherRow = pluginRows(page).filter({ hasText: 'Weather' })
  await weatherRow.getByRole('button', { name: '查看详情' }).click()
  await expect(page.getByRole('heading', { name: '哎呀！出错了' })).toBeVisible()
  await page.getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()

  await page.getByRole('button', { name: /卸\s*载/ }).click()
  await page.getByRole('button', { name: /确认卸载/ }).click()
  await expect(page.getByText('缺少必要资源')).toBeVisible()
})

test('fallback pages cover missing routes and server offline recovery', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/missing-admin-page')
  await expect(page.getByRole('heading', { name: '哎呀！未找到页面' })).toBeVisible()

  await page.goto('/commands')
  await expect(page.getByRole('heading', { name: '指令中心' })).toBeVisible()

  await setBackendNetworkOffline(request)
  await page.goto('/access-lists')
  await expect(page.getByRole('heading', { name: '哎呀！网络错误' })).toBeVisible({ timeout: 7000 })

  await setBackendNetworkOnline(request)
  await page.getByRole('button', { name: '重新检测' }).click()
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()
})

test('shutdown flow shows the draining toast', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '关闭服务' }).click({ force: true })
  await page.getByRole('button', { name: '确认关闭' }).click()

  await expect(page.locator('.ant-message')).toContainText('停机请求已发送')
  await expect(page.locator('.ant-message')).toContainText('服务正在停止')
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

  await page.getByRole('button', { name: '打开菜单' }).click()
  const mobilePluginGroup = page.locator('.ant-drawer-content .ant-menu-submenu').filter({ hasText: '插件中心' }).first()
  const mobilePluginListItem = mobilePluginGroup.locator('.ant-menu-item').filter({ hasText: '插件列表' }).first()
  if (!await mobilePluginListItem.isVisible().catch(() => false)) {
    await mobilePluginGroup.locator('.ant-menu-submenu-title').click()
    await expect(mobilePluginListItem).toBeVisible()
  }
  await mobilePluginListItem.click()
  await expect(pluginRows(page).first()).toBeVisible()

  await page.getByRole('button', { name: '打开菜单' }).click()
  const mobilePluginSettingsGroup = page.locator('.ant-drawer-content .ant-menu-submenu').filter({ hasText: '插件中心' }).first()
  const mobilePluginSettingsItem = mobilePluginSettingsGroup.locator('.ant-menu-item').filter({ hasText: '插件设置' }).first()
  if (!await mobilePluginSettingsItem.isVisible().catch(() => false)) {
    await mobilePluginSettingsGroup.locator('.ant-menu-submenu-title').click()
    await expect(mobilePluginSettingsItem).toBeVisible()
  }
  await mobilePluginSettingsItem.click()
  await expect(page.getByRole('heading', { name: '插件设置', level: 1 })).toBeVisible()

  await page.goto('/logs?log_id=log_adapter_live_0001')
  await expect(logRows(page).filter({ hasText: 'ignored OneBot API response with unsupported echo' }).first()).toBeVisible()
  await expect(page.locator('.log-detail-drawer, .log-detail-window')).toContainText('api response echo must be a non-empty string')
  await expect(logDetailWindow(page)).toHaveCount(0)
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
})
