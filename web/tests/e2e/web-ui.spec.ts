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
  channel: 'events' | 'logs' | 'plugin_console',
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

async function expectPluginCenterPage(page: import('@playwright/test').Page, name: string) {
  await expect(
    page.getByTestId('plugin-center-sidebar-navigation').first().locator('[aria-current=page]'),
  ).toHaveText(name)
  await expect(page.getByRole('heading', { name, level: 1, exact: true })).toBeAttached()
}

function pluginRows(page: import('@playwright/test').Page) {
  return page.locator('.plugin-grid-card:visible')
}

async function expectDocumentWithinViewport(page: import('@playwright/test').Page) {
  await expect.poll(() => page.evaluate(() => (
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  ))).toBeLessThanOrEqual(1)
}

async function readFocusedAuthControlStyle(
  focusTarget: import('@playwright/test').Locator,
  visualTarget = focusTarget,
) {
  await focusTarget.focus()
  return visualTarget.evaluate(async (element) => {
    await Promise.all(element.getAnimations().map(animation => animation.finished.catch(() => {})))
    const style = getComputedStyle(element)
    return {
      borderWidth: style.borderTopWidth,
      boxShadow: style.boxShadow,
      outlineStyle: style.outlineStyle,
      outlineWidth: style.outlineWidth,
    }
  })
}

async function expectConsistentAuthControlFocus(page: import('@playwright/test').Page) {
  const identifier = page.getByLabel('管理员账号')
  const secretInput = page.getByLabel('管理员密钥')

  const identifierStyle = await readFocusedAuthControlStyle(identifier)
  const secretStyle = await readFocusedAuthControlStyle(secretInput)

  // The ring replaces the outline, so it has to stay visible: 3px spread in a non-transparent color.
  const [ring] = identifierStyle.boxShadow.split(/,(?![^(]*\))/)
  expect(ring).toContain('0px 0px 0px 3px')
  expect(Number(/rgba\([^)]*,\s*([\d.]+)\)/.exec(ring)?.[1] ?? '1')).toBeGreaterThan(0.1)
  expect(identifierStyle.borderWidth).toBe('1px')
  expect(identifierStyle.outlineStyle).toBe('none')
  expect(secretStyle).toEqual(identifierStyle)
}

async function tabToLocator(
  page: import('@playwright/test').Page,
  locator: import('@playwright/test').Locator,
  maximumTabs = 20,
) {
  for (let index = 0; index < maximumTabs; index += 1) {
    await page.keyboard.press('Tab')
    if (await locator.evaluate((element) => element === document.activeElement || element.contains(document.activeElement)).catch(() => false)) {
      return
    }
  }
  throw new Error(`Keyboard focus did not reach ${await locator.first().evaluate((element) => element.outerHTML).catch(() => 'the target')}`)
}

async function expectMenuKeyboardReachable(
  page: import('@playwright/test').Page,
  menu: import('@playwright/test').Locator,
) {
  await tabToLocator(page, menu)
  await page.keyboard.press('ArrowDown')
  await expect.poll(() => menu.evaluate((element) => (
    element.contains(document.activeElement)
    && document.activeElement?.hasAttribute('data-nav-item')
  ))).toBe(true)
}

function logRows(page: import('@playwright/test').Page) {
  return page.locator('.logs-row')
}

function logScroller(page: import('@playwright/test').Page) {
  return page.locator('.logs-feed-card .data-viewport__scroller')
}

async function readLogViewportBottomState(page: import('@playwright/test').Page) {
  return logScroller(page).evaluate((scroller) => {
    const lastRow = scroller.querySelector<HTMLElement>('.data-viewport__row:last-child')
    const scrollerRect = scroller.getBoundingClientRect()
    const lastRowRect = lastRow?.getBoundingClientRect()

    return {
      scrollGap: scroller.scrollHeight - scroller.clientHeight - scroller.scrollTop,
      visualGap: lastRowRect ? lastRowRect.bottom - scrollerRect.bottom : Number.POSITIVE_INFINITY,
    }
  })
}

function logDetailWindow(page: import('@playwright/test').Page) {
  return page.getByTestId('management-log-detail-window')
}

async function scrollConfigSectionIntoView(page: import('@playwright/test').Page, sectionKey: string) {
  await page.locator(`#config-section-${sectionKey}`).scrollIntoViewIfNeeded()
}

function logFilterField(page: import('@playwright/test').Page, label: string) {
  return page.locator('.logs-filter-grid:visible .ant-form-item, .log-advanced-filters__panel:visible .ant-form-item')
    .filter({ hasText: label })
    .first()
}

async function openLogAdvancedFilters(page: import('@playwright/test').Page) {
  const panel = page.locator('.log-advanced-filters__panel:visible')
  if (!await panel.isVisible().catch(() => false)) {
    await page.locator('.log-advanced-filters__trigger:visible').click()
    await expect(panel).toBeVisible()
  }
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
    await page.getByRole('option', { name: unit, exact: true }).click()
  }
}

async function readTabLabels(page: import('@playwright/test').Page) {
  return page.locator('.workspace-tabs__trigger').evaluateAll((nodes) => (
    nodes
      .map((node) => node.textContent?.trim() ?? '')
      .filter(Boolean)
  ))
}

async function readActiveTabLabel(page: import('@playwright/test').Page) {
  return page.locator('.workspace-tabs__item[data-active=true] .workspace-tabs__trigger').evaluateAll((nodes) => (
    nodes[0]?.textContent?.trim() ?? ''
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

  await navigateThroughMenu(page, '权限策略', '治理')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect.poll(() => readTabLabels(page)).toEqual(['系统状态', '权限策略'])

  await navigateThroughMenu(page, '实时日志', '运行与诊断')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()

  await expect.poll(() => readTabLabels(page)).toEqual(['系统状态', '权限策略', '实时日志'])
  await expect.poll(() => readActiveTabLabel(page)).toBe('实时日志')
}

async function openTabContextMenu(page: import('@playwright/test').Page, tabName: string) {
  await page.locator('.workspace-tabs__item')
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
  await expect(levelTags.filter({ hasText: 'warn' })).toHaveCount(1)
  await expect(levelTags.filter({ hasText: 'error' })).toHaveCount(1)

  await openLogAdvancedFilters(page)
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
    const groupMenu = sider.locator('.sidebar-navigation__group').filter({ hasText: group }).first()
    const targetItem = groupMenu.locator('.sidebar-navigation__item').filter({ hasText: item }).first()
    if (!await targetItem.isVisible().catch(() => false)) {
      await groupMenu.locator('.sidebar-navigation__group-heading').click()
      await expect(targetItem).toBeVisible()
    }

    await targetItem.click()
    return
  }

  const targetItem = sider.locator('.sidebar-navigation__item').filter({ hasText: item }).first()
  await targetItem.click()
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
  await expect(appHeader(page)).not.toContainText('日志流')
  await expect(dashboardConnectionCard(page)).toContainText('事件流')
  await expect(dashboardConnectionCard(page)).toContainText('日志流')
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')
  await expect(page.getByTestId('connection-card-logs')).toContainText('已认证')
})

test('protected deep links return to the target after login', async ({ page, request }) => {
  await resetBackend(request, true)

  await page.goto('/plugins?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '登录', level: 1 })).toBeVisible()
  await expect(page.getByTestId('auth-theme-toggle')).toBeVisible()

  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()

  await expectPluginCenterPage(page, '插件列表')
  await expect(page).toHaveURL(/\/plugins\?token=launcher_token_fixture_0001$/)
})

test('authentication surface keeps theme controls and credentials reachable on a narrow viewport', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/login')

  const themeToggle = page.getByTestId('auth-theme-toggle')
  const authSurface = page.locator('.auth-layout__surface')
  await expect(page.getByRole('heading', { name: '登录', level: 1 })).toBeVisible()
  await expect(authSurface.getByTestId('auth-theme-toggle')).toBeVisible()
  await expect(themeToggle).toHaveAttribute('aria-label', '主题模式：跟随系统')
  await expectDocumentWithinViewport(page)

  await themeToggle.focus()
  await page.keyboard.press('ArrowDown')
  await expect(page.getByRole('menuitem', { name: '跟随系统' })).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('menuitem', { name: '跟随系统' })).toBeHidden()
  await expect(themeToggle).toBeFocused()

  await themeToggle.focus()
  await page.keyboard.press('Enter')
  await page.getByRole('menuitem', { name: '暗色' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(themeToggle).toHaveAttribute('aria-label', '主题模式：暗色')

  await themeToggle.focus()
  await page.keyboard.press('Tab')
  await expect(page.getByLabel('管理员账号')).toBeFocused()
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await expectDocumentWithinViewport(page)
})

test('authentication controls share one focus treatment in light and dark themes', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.goto('/login')

  await expectConsistentAuthControlFocus(page)

  const themeToggle = page.getByTestId('auth-theme-toggle')
  await themeToggle.click()
  await page.getByRole('menuitem', { name: '暗色' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')

  await expectConsistentAuthControlFocus(page)

  const identifier = page.getByLabel('管理员账号')
  const secret = page.getByLabel('管理员密钥')
  await identifier.fill('')
  await page.locator('.auth-form__submit').click()
  await expect(identifier).toHaveAttribute('aria-invalid', 'true')
  await expect(secret).toHaveAttribute('aria-invalid', 'true')
  await expectConsistentAuthControlFocus(page)
})

test('authentication stays stable during pointer interaction and reduced motion', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/login')

  const surface = page.locator('.auth-layout__surface')
  await page.getByLabel('管理员密钥').waitFor()
  await surface.evaluate(async element => {
    await Promise.all(element.getAnimations().map(animation => animation.finished))
  })
  const surfaceBefore = await surface.boundingBox()
  expect(surfaceBefore).not.toBeNull()

  await page.mouse.move(120, 140)

  const surfaceAfter = await surface.boundingBox()
  expect(surfaceAfter).toEqual(surfaceBefore)

  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
})

test('authentication lens adapts to recovery and narrow screens and yields to forced colors', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.goto('/login')
  const surface = page.locator('.auth-layout__surface')
  const lens = page.locator('.auth-layout__filters feImage')
  await expect(lens).toHaveCount(1)
  const loginMap = await lens.getAttribute('href')

  await page.getByRole('button', { name: '忘记密钥？' }).click()
  await expect(page.getByRole('heading', { name: '重置管理员凭据', exact: true })).toBeFocused()
  await expect.poll(() => lens.getAttribute('href')).not.toBe(loginMap)
  await page.setViewportSize({ width: 320, height: 568 })
  await expectDocumentWithinViewport(page)
  await expect.poll(async () => Number(await lens.getAttribute('width'))).toBe(await surface.evaluate(el => (el as HTMLElement).offsetWidth))
  await page.getByRole('button', { name: '返回登录' }).click()
  await expect(page.getByRole('button', { name: '忘记密钥？' })).toBeFocused()

  await page.emulateMedia({ forcedColors: 'active', reducedMotion: 'reduce' })
  await expect(surface).toHaveCSS('backdrop-filter', 'none')
  await expect(page.locator('.auth-layout__art')).toBeHidden()
  await page.getByRole('textbox', { name: '管理员密钥' }).focus()
  await expect(page.getByLabel('管理员密钥')).toHaveCSS('outline-style', 'solid')
  await tabToLocator(page, page.locator('.auth-form__submit'))
  await expect(page.locator('.auth-form__submit')).toHaveCSS('outline-style', 'solid')
})

test('setup-required deep links return to the target after initialization', async ({ page, request }) => {
  await resetBackend(request, false)

  await page.goto('/plugins?token=launcher_token_fixture_0001')

  await expect(page.getByRole('heading', { name: '创建管理员账号', level: 1 })).toBeVisible()
  await page.getByLabel('管理员密钥').fill('fixture-only-secret')
  await page.getByRole('button', { name: '创建并进入管理界面' }).click()

  await expectPluginCenterPage(page, '插件列表')
  await expect(page).toHaveURL(/\/plugins\?token=launcher_token_fixture_0001$/)
})

test('plugin cards load declared icons and keep a logo fallback after image failure', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await page.goto('/plugins')
  const weather = pluginRows(page).filter({ has: page.getByTestId('plugin-enable-button-weather') })
  const image = weather.locator('.plugin-icon img')
  await expect(image).toBeVisible()
  await expect.poll(() => image.evaluate(node => (node as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  const noIcon = pluginRows(page).filter({ has: page.getByTestId('plugin-enable-button-example-config-panel') })
  await expect(noIcon.locator('.plugin-icon .raylea-mark')).toBeVisible()

  await page.route('**/api/plugins/weather/icon', route => route.fulfill({ status: 404, contentType: 'application/json', body: '{}' }))
  await page.reload()
  await expect(weather.locator('.plugin-icon .raylea-mark')).toBeVisible()
  await expect(weather.locator('.plugin-icon img')).toHaveCount(0)
  await weather.getByRole('button', { name: '查看概要', exact: true }).click()
  await expect(page.locator('.ant-drawer-content')).toContainText('weather')
})

test('plugin management flow covers install, manifest detail and console recovery', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/plugins')
  await expectPluginCenterPage(page, '插件列表')
  await expect(pluginRows(page).first()).toBeVisible()
  await expect(page.locator('.plugins-grid').getByRole('button', { name: 'Example Config Panel', exact: true })).toBeVisible()
  await expect(page.locator('.plugins-grid').getByRole('button', { name: 'Weather', exact: true })).toBeVisible()

  await pluginRows(page).filter({ hasText: 'Example Config Panel' }).getByRole('button', { name: '管理', exact: true }).click()
  await expect(page).toHaveURL(/\/plugins\/example-config-panel\?panel=management-ui&management_page=config$/)
  await expect(page.getByTestId('plugin-management-ui-confirm')).toBeVisible()
  await page.goto('/plugins')
  await pluginRows(page).filter({ hasText: 'Weather' }).getByRole('button', { name: '管理', exact: true }).click()
  await expect(page).toHaveURL(/\/plugins\/weather\?panel=overview$/)
  await page.goto('/plugins')

  await page.locator('.plugins-toolbar').getByRole('button', { name: '安装插件' }).click()
  const installDialog = page.getByRole('dialog', { name: '安装插件' })
  await expect(installDialog).toBeVisible()
  await installDialog.getByRole('textbox').fill('C:/plugins/weather.zip')
  const inspectionResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().endsWith('/api/plugins/install/inspect')
  ))
  await installDialog.getByRole('button', { name: '检查插件包' }).click()
  expect((await inspectionResponsePromise).status()).toBe(200)
  await expect(installDialog.getByRole('checkbox', { name: /我已核对来源、目标平台、artifact 摘要和权限/ })).toBeVisible()
  await expect(installDialog.getByText('Weather Package（example.weather-package）')).toBeVisible()
  await expect(installDialog.getByText('a'.repeat(64))).toBeVisible()
  await expect(installDialog.getByText(/v2.*8/)).toBeVisible()
  await expect(installDialog.getByText('http.request', { exact: true })).toBeVisible()
  await expect(installDialog.getByText('message.send', { exact: true })).toBeVisible()
  await installDialog.getByRole('checkbox', { name: /我已核对来源、目标平台、artifact 摘要和权限/ }).check()
  await installDialog.getByRole('button', { name: '开始安装' }).click()

  await expectPluginCenterPage(page, '插件列表')
  await expect(page.getByText('安装任务已提交，可在实时日志查看结果').first()).toBeVisible()
  await page.goto('/logs?source=tasks')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await expect(page.getByText('task_plugin_install_0001').first()).toBeVisible()

  await page.setViewportSize({ width: 1440, height: 900 })
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
  await expect(page.getByRole('tab', { name: '插件指令' })).toBeVisible()

  await page.getByRole('tab', { name: '实时输出' }).click()
  await expect(page.locator('.console-terminal').first()).toBeVisible()
  const outputPanelBounds = await page.locator('.plugin-console-panel').evaluate((element) => {
    const bounds = element.getBoundingClientRect()
    return {
      bottom: bounds.bottom,
      height: bounds.height,
      viewportHeight: window.innerHeight,
    }
  })
  expect(outputPanelBounds.height).toBeGreaterThan(200)
  expect(outputPanelBounds.bottom).toBeLessThanOrEqual(outputPanelBounds.viewportHeight - 20)
  await page.getByRole('button', { name: '清空输出' }).click()
  await expect(page.getByText('等待插件输出')).toBeVisible()
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
    await request.delete(`${backendUrl}/api/governance/whitelist/entries/user/${encodeURIComponent(`31${String(index + 1).padStart(3, '0')}`)}`, {
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

test('permission policy page edits command policy config', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('permission-policy-summary-card')).toHaveCount(0)
  await expect(page.getByText('超级管理员可执行最高权限命令，并跳过黑白名单与冷却拦截。')).toBeVisible()
  await expect(page.getByText('未单独声明权限的命令使用此级别。')).toBeVisible()
  await expect(page.getByText('配置超级管理员、默认权限级别和聊天命令速率限制。')).toHaveCount(0)
  await expect(page.getByText('策略总览')).toHaveCount(0)
  await expect(page.getByTestId('permission-policy-unsaved-status')).toHaveCount(0)

  const superAdminsInput = page.getByTestId('permission-policy-super-admins').locator('input')
  await superAdminsInput.click()
  await superAdminsInput.fill('10002')
  await superAdminsInput.press('Enter')
  await page.getByLabel('默认权限级别').click()
  await page.getByRole('option', { name: '群管理员', exact: true }).click()
  await expect(page.getByText('用户命令速率限制')).toHaveCount(0)
  await expect(page.getByText('群命令速率限制')).toHaveCount(0)
  await expect(page.getByText('冷却提示')).toHaveCount(0)

  await expect(page.getByTestId('permission-policy-unsaved-status')).toContainText('有未保存更改')

  const policySaveResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'PUT'
    && response.url().endsWith('/api/config')
  ))
  await page.getByTestId('permission-policy-save').click()
  expect((await policySaveResponsePromise).status()).toBe(200)
  await expect(page.getByTestId('permission-policy-unsaved-status')).toHaveCount(0)
  await expect(page.getByLabel('默认权限级别')).toContainText('群管理员')

  await page.getByTestId('permission-policy-open-access-lists').click()
  await expect.poll(() => page.url()).toContain('/access-lists')
  await expect(page.getByRole('heading', { name: '黑白名单', level: 1 })).toBeVisible()

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.getByTestId('permission-policy-save-status')).toHaveCount(0)
})

test('plugin management ui uses an isolated bridge for settings, secrets, theme, and height', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/plugins/example-config-panel')
  await expect(page.getByRole('heading', { name: '插件：Example Config Panel', level: 1 })).toBeVisible()
  const detailNavigation = page.getByTestId('plugin-center-sidebar-navigation')
  await expect(detailNavigation).toContainText('概览')
  await expect(detailNavigation).toContainText('配置页面')
  await expect(page.locator('.plugin-detail-panel-switch')).toBeHidden()

  await detailNavigation.locator('[data-sidebar-management-page="config"]').click()
  await expect(page).toHaveURL(/panel=management-ui/)
  await expect(page.getByTestId('plugin-management-ui-confirm')).toBeVisible()
  await page.locator('.admin-layout__nav-trigger.desktop-only').click()
  await expect(page.locator('.plugin-detail-panel-switch')).toBeVisible()
  await page.locator('.admin-layout__nav-trigger.desktop-only').click()
  await expect(page.locator('.plugin-detail-panel-switch')).toBeHidden()
  const secretStatusResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'GET'
    && response.url().includes('/api/plugins/example-config-panel/secrets')
  ))
  await page.getByRole('button', { name: '确认并打开' }).click()

  const pluginFrame = page.frameLocator('[data-testid="plugin-management-ui-frame"]')
  await expect(pluginFrame.getByRole('heading', { name: '配置面板示例', level: 1 })).toBeVisible()
  await expect(pluginFrame.getByTestId('settings-status')).toHaveText('配置已加载')
  await expect(pluginFrame.getByTestId('default-city-input')).toHaveValue('上海')
  await expect(pluginFrame.getByTestId('unit-select')).toContainText('华氏度')
  await expect(pluginFrame.getByTestId('settings-preview')).toContainText('上海')
  await expect(pluginFrame.getByTestId('settings-preview')).toContainText('fahrenheit')
  await expect(pluginFrame.locator('html')).toHaveAttribute('data-theme', /^(light|dark)$/)

  const initialPluginTheme = await pluginFrame.locator('html').getAttribute('data-theme')
  const nextPluginTheme = initialPluginTheme === 'dark' ? 'light' : 'dark'
  await page.getByTestId('theme-toggle').click()
  await page.getByRole('menuitem').filter({ hasText: nextPluginTheme === 'dark' ? '暗色' : '亮色' }).click()
  await expect(pluginFrame.locator('html')).toHaveAttribute('data-theme', nextPluginTheme)

  const frameSource = new URL((await page.getByTestId('plugin-management-ui-frame').getAttribute('src'))!)
  expect(frameSource.hostname).toMatch(/^p-[a-f0-9]{16}\.plugins\.localhost$/)
  expect(frameSource.origin).not.toBe(new URL(page.url()).origin)
  // Chromium resolves *.localhost to loopback; Node may use the host DNS resolver.
  const isolatedAPIResponse = await request.get(`${backendUrl}/api/config`, {
    headers: { Host: frameSource.host },
  })
  expect(isolatedAPIResponse.status()).toBe(404)
  expect(isolatedAPIResponse.headers()['access-control-allow-origin']).toBeUndefined()
  expect(isolatedAPIResponse.headers()['set-cookie']).toBeUndefined()

  const initialSecretResponse = await secretStatusResponsePromise
  const initialSecretBody = await initialSecretResponse.text()
  expect(initialSecretBody).toContain('"configured"')
  expect(initialSecretBody).not.toContain('stored-secret-must-not-leak')
  await expect(pluginFrame.getByTestId('secret-status')).toHaveText('API 密钥已配置')

  await pluginFrame.getByTestId('default-city-input').fill('广州')
  const unitSelect = pluginFrame.getByTestId('unit-select').getByRole('combobox')
  await pluginFrame.getByTestId('unit-select').click()
  await unitSelect.press('ArrowUp')
  await unitSelect.press('Enter')
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'PUT'
      && response.url().includes('/api/plugins/example-config-panel/settings')
      && response.status() === 200
    )),
    pluginFrame.getByTestId('save-settings').click(),
  ])

  await expect(pluginFrame.getByTestId('settings-status')).toHaveText('配置已保存')
  await expect(pluginFrame.getByTestId('settings-preview')).toContainText('广州')
  await expect(pluginFrame.getByTestId('settings-preview')).toContainText('celsius')

  await pluginFrame.getByTestId('secret-input').fill('replacement-secret-for-e2e')
  const setSecretResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'PUT'
    && response.url().includes('/api/plugins/example-config-panel/secrets')
  ))
  await pluginFrame.getByTestId('save-secret').click()
  const setSecretBody = await (await setSecretResponsePromise).text()
  expect(setSecretBody).toContain('"configured"')
  expect(setSecretBody).not.toContain('replacement-secret-for-e2e')
  expect(setSecretBody).not.toContain('stored-secret-must-not-leak')
  await expect(pluginFrame.getByTestId('secret-input')).toHaveValue('')
  await expect(pluginFrame.getByTestId('secret-status')).toHaveText('API 密钥已覆盖')

  const deleteSecretResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'DELETE'
    && response.url().includes('/api/plugins/example-config-panel/secrets')
  ))
  await pluginFrame.getByTestId('delete-secret').click()
  const deleteSecretBody = await (await deleteSecretResponsePromise).text()
  expect(deleteSecretBody).not.toContain('replacement-secret-for-e2e')
  await expect(pluginFrame.getByTestId('secret-status')).toHaveText('API 密钥已删除')

  const frameHeight = await page.getByTestId('plugin-management-ui-frame').evaluate((frame) => Number.parseFloat((frame as HTMLIFrameElement).style.height))
  expect(frameHeight).toBeGreaterThanOrEqual(320)
  expect(frameHeight).toBeLessThanOrEqual(1600)

  await page.getByTestId('plugin-management-ui-frame').scrollIntoViewIfNeeded()
  await pluginFrame.locator('body').evaluate((body) => {
    const longContent = document.createElement('div')
    longContent.dataset.testid = 'plugin-management-ui-long-content'
    longContent.style.height = '2400px'
    body.append(longContent)
  })
  await expect.poll(async () => page.getByTestId('plugin-management-ui-frame').evaluate((frame) => {
    const bounds = frame.getBoundingClientRect()
    return bounds.height < 1600 && bounds.bottom <= window.innerHeight
  })).toBe(true)
  const longFrameGeometry = await page.getByTestId('plugin-management-ui-frame').evaluate((frame) => {
    const bounds = frame.getBoundingClientRect()
    return { bottom: bounds.bottom, height: bounds.height, viewportHeight: window.innerHeight }
  })
  expect(longFrameGeometry.height).toBeLessThan(1600)
  expect(longFrameGeometry.bottom).toBeLessThanOrEqual(longFrameGeometry.viewportHeight)
  const reachedPluginPageBottom = await pluginFrame.locator('html').evaluate(() => {
    const scrollingElement = document.scrollingElement
    if (!scrollingElement) return false
    scrollingElement.scrollTop = scrollingElement.scrollHeight
    return scrollingElement.scrollTop + scrollingElement.clientHeight >= scrollingElement.scrollHeight - 1
  })
  expect(reachedPluginPageBottom).toBe(true)

  await page.reload()
  await expect(page.getByRole('heading', { name: '插件：Example Config Panel', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/panel=management-ui/)
  await expect(pluginFrame.getByTestId('settings-status')).toHaveText('配置已加载')
  await expect(pluginFrame.getByTestId('default-city-input')).toHaveValue('广州')
  await expect(pluginFrame.getByTestId('unit-select')).toContainText('摄氏度')
  await expect(pluginFrame.getByTestId('secret-status')).toHaveText('API 密钥未配置')

  const tabLabels = await readTabLabels(page)
  expect(tabLabels.filter((label) => label === '插件：Example Config Panel')).toHaveLength(1)
})

test('history logs stay frozen until a new anchor is loaded', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 2048, height: 1148 })
  await login(page)

  const baseTimestamp = Date.now() - 60 * 60 * 1000

  await pushLogsInBatches(request, 100, (index) => ({
    summary: {
      log_id: `log_history_e2e_${index}`,
      timestamp: new Date(baseTimestamp + index * 1000).toISOString(),
      level: 'info',
      source: 'runtime',
      request_id: 'req_logs_history_e2e',
      message: index % 9 === 0
        ? `history row ${index}\n${'variable-height detail '.repeat(32)}`
        : `history row ${index}`,
    },
  }))

  await page.goto('/logs/history')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await openLogAdvancedFilters(page)
  await expect(page.locator('.logs-toolbar .log-advanced-filters__panel')).toHaveCount(0)
  await page.getByPlaceholder('例如 req_*').fill('req_logs_history_e2e')
  await page.getByRole('button', { name: '应用筛选' }).click()

  await expect(page.locator('.logs-row__message', { hasText: 'history row 99' })).toBeVisible()
  await expect.poll(async () => (await readLogViewportBottomState(page)).scrollGap).toBeLessThanOrEqual(1)
  await expect.poll(async () => (await readLogViewportBottomState(page)).visualGap).toBeLessThanOrEqual(1)
  const initialMetrics = await logScroller(page).evaluate((node) => ({
    clientHeight: node.clientHeight,
    scrollHeight: node.scrollHeight,
    scrollTop: node.scrollTop,
  }))
  expect(initialMetrics.scrollHeight).toBeGreaterThan(initialMetrics.clientHeight)
  expect(initialMetrics.scrollHeight - initialMetrics.clientHeight - initialMetrics.scrollTop).toBeLessThanOrEqual(1)

  await logScroller(page).locator('.data-viewport__row').last().evaluate((row) => {
    const element = row as HTMLElement
    element.style.minHeight = `${element.offsetHeight + 96}px`
  })
  await expect.poll(async () => (await readLogViewportBottomState(page)).scrollGap).toBeLessThanOrEqual(1)
  await expect.poll(async () => (await readLogViewportBottomState(page)).visualGap).toBeLessThanOrEqual(1)

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
        timestamp: new Date(baseTimestamp + 120_000).toISOString(),
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
  await openLogAdvancedFilters(page)
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
  await openLogAdvancedFilters(page)
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
  await openLogAdvancedFilters(page)
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
  await openLogAdvancedFilters(page)
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
  await openLogAdvancedFilters(page)
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
  )).toBeLessThanOrEqual(1)

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
  await logScroller(page).evaluate((node) => {
    node.scrollTop = 0
    node.dispatchEvent(new Event('scroll'))
  })
})

test('logs page reloads the latest page after hidden updates arrive', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '实时日志', '运行与诊断')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await openLogAdvancedFilters(page)
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

  await navigateThroughMenu(page, '插件中心')
  await expectPluginCenterPage(page, '插件列表')

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

  await page.locator('.admin-layout__tabbar [data-tab-path="/logs"]').click()
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
  await openLogAdvancedFilters(page)
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
  await openLogAdvancedFilters(page)
  await page.getByPlaceholder('例如 req_*').fill('req_bridge_unsafe_0001')
  await page.getByRole('button', { name: '应用筛选' }).click()
  const unsafeHistoryMessage = page.locator('.logs-row__message', { hasText: '测试群名片' }).first()
  await expect(unsafeHistoryMessage).toContainText('\\u2066')

  const historyText = await unsafeHistoryMessage.evaluate((node) => node.textContent ?? '')
  expect(historyText.includes('\u2066')).toBe(false)
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

test('config page saves restart-required Douyin browser settings', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  await scrollConfigSectionIntoView(page, 'third-party-accounts')

  await page.getByRole('spinbutton', { name: 'CK 自动检查间隔' }).fill('720')
  await expect(page.locator('#config-section-third-party-accounts').getByText('自动选择', { exact: true })).toBeVisible()
  await page.locator('#config-section-third-party-accounts').getByRole('combobox').click()
  await page.getByRole('option', { name: '远程 CDP', exact: true }).click()
  await page.getByRole('textbox', { name: '抖音远程调试地址' }).fill('http://127.0.0.1:9222')

  const configResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'PUT'
    && response.url().endsWith('/api/config')
  ))
  await page.getByRole('button', { name: '保存更改' }).click()
  const configResponse = await configResponsePromise
  const responseBody = await configResponse.json()
  expect(responseBody.restart_required).toBe(true)
  expect(responseBody.apply_effects.restart_required_fields).toEqual(expect.arrayContaining([
    'third_party_accounts.douyin_login.browser_mode',
    'third_party_accounts.douyin_login.remote_debugging_url',
  ]))
  expect(responseBody.apply_effects.applied_now).toContain('third_party_accounts.credential_check_interval_minutes')

  await page.reload()
  await scrollConfigSectionIntoView(page, 'third-party-accounts')
  await expect(page.getByRole('spinbutton', { name: 'CK 自动检查间隔' })).toHaveValue('720')
  await expect(page.locator('#config-section-third-party-accounts').getByText('远程 CDP', { exact: true })).toBeVisible()
  await expect(page.getByRole('textbox', { name: '抖音远程调试地址' })).toHaveValue('http://127.0.0.1:9222')
})

test('rate limits page edits chat and outbound limits', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '限流中心', '治理')
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
  await expectPluginCenterPage(page, '全局插件设置')
  await expect(page.getByText('插件消息速率限制')).toHaveCount(0)
})

test('plugin settings page edits plugin global config', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await navigateThroughMenu(page, '插件中心')
  await page.getByTestId('plugin-center-sidebar-navigation').getByText('全局插件设置', { exact: true }).click()
  await expectPluginCenterPage(page, '全局插件设置')
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toHaveCount(0)

  const commandPrefixesInput = page.getByTestId('plugin-settings-command-prefixes').locator('input')
  await commandPrefixesInput.click()
  await commandPrefixesInput.fill('!')
  await commandPrefixesInput.press('Enter')
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toContainText('有未保存更改')

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
  await expect(page.locator('.config-page')).not.toContainText('插件日志速率限制')
  await expect(page.locator('.config-page')).not.toContainText('插件消息速率限制')
  await expect(page.locator('.config-page')).not.toContainText('插件工作目录软上限')
})

test('status page can start backup tasks and export diagnostics', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '创建备份' }).click()
  await expect(page.locator('#app-main').getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page.getByText('备份任务已提交，可在实时日志查看结果').first()).toBeVisible()
  await page.goto('/logs?source=tasks')
  await expect(page.locator('#app-main').getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
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

  const initialPreviewResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().includes('/api/system/render/templates/help.menu/preview-html')
  ))
  await page.goto('/render/templates/help.menu')
  expect((await initialPreviewResponsePromise).status()).toBe(200)
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

  const previewFrame = page.getByTestId('render-template-preview-frame')
  await expect(previewFrame).toBeVisible()
  await expect(previewFrame).toHaveAttribute('srcdoc', /帮助菜单/)

  const updatedPreviewResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().includes('/api/system/render/templates/help.menu/preview-html')
  ))
  await page.getByLabel('输入数据 JSON').fill('{\n  "title": "帮助菜单（自动同步）"\n}')
  expect((await updatedPreviewResponsePromise).status()).toBe(200)
  await expect(previewFrame).toHaveAttribute('srcdoc', /帮助菜单（自动同步）/)

  const statusPreviewResponsePromise = page.waitForResponse((response) => (
    response.request().method() === 'POST'
    && response.url().includes('/api/system/render/templates/status.panel/preview-html')
  ))
  await page.locator('.template-nav-item').filter({ hasText: 'status.panel' }).first().click()
  expect((await statusPreviewResponsePromise).status()).toBe(200)
  await expect(page).toHaveURL(/\/render\/templates\/status\.panel$/)
  expect((await readTabLabels(page)).filter((label) => label === '模板预览')).toHaveLength(1)
  await expect(page.getByTestId('render-template-preview-frame')).toHaveAttribute('data-template-id', 'status.panel')
  await expect(page.locator('.render-templates-float-panel')).toContainText('status.panel')
})

test('menu center preview loads the bundled chat menu font', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/menu-center')
  const rootPreviewFrame = page.getByTestId('menu-center-root-preview').getByTestId('native-template-preview-frame')
  await expect(rootPreviewFrame).toBeVisible()
  await expect(rootPreviewFrame).toHaveAttribute('srcdoc', /data-help-menu-fonts/)

  await expect.poll(() => rootPreviewFrame.evaluate(async (frame) => {
    const document = (frame as HTMLIFrameElement).contentDocument
    if (!document) {
      return false
    }
    await document.fonts.ready
    const loadedFontFaces = Array.from(document.fonts).filter((font) => (
      font.family.replace(/["']/g, '') === 'Noto Sans SC'
      && font.status === 'loaded'
    )).length
    const view = document.defaultView
    const targetSelectors = ['h1', '.description', '.meta']
    const targetsUseNotoSans = targetSelectors.every((selector) => {
      const element = document.querySelector(selector)
      return element && view?.getComputedStyle(element).fontFamily.includes('Noto Sans SC')
    })
    return loadedFontFaces > 0 && targetsUseNotoSans
  })).toBe(true)

  await page.locator('.menu-center-tabs').getByRole('tab').nth(1).click()
  const pluginPreviewFrame = page.getByTestId('menu-center-plugin-preview').getByTestId('native-template-preview-frame')
  await expect(pluginPreviewFrame).toBeVisible()

  await expect.poll(() => pluginPreviewFrame.evaluate(async (frame) => {
    const document = (frame as HTMLIFrameElement).contentDocument
    if (!document) {
      return false
    }
    await document.fonts.ready
    const loadedFontFaces = Array.from(document.fonts).filter((font) => (
      font.family.replace(/["']/g, '') === 'Noto Sans SC'
      && font.status === 'loaded'
    )).length
    const view = document.defaultView
    const targetSelectors = ['h1', '.subtitle', '.help-group h2', '.meta', '.description', '.command-usage__name']
    const targetsUseNotoSans = targetSelectors.every((selector) => {
      const element = document.querySelector(selector)
      return element && view?.getComputedStyle(element).fontFamily.includes('Noto Sans SC')
    })
    const permission = document.querySelector('.command-permission')
    const command = document.querySelector('.command-usage code')
    const permissionFamily = permission && view?.getComputedStyle(permission).fontFamily
    const commandFamily = command && view?.getComputedStyle(command).fontFamily
    const permissionKeepsSystemFont = permissionFamily && !/^["']?Noto Sans SC/.test(permissionFamily)
    const commandKeepsMonoFont = commandFamily && /^["']?JetBrains Mono/.test(commandFamily)
    return loadedFontFaces > 0 && targetsUseNotoSans && permissionKeepsSystemFont && commandKeepsMonoFont
  })).toBe(true)

  await page.setViewportSize({ width: 393, height: 852 })
  await page.locator('.menu-center-tabs').getByRole('tab').first().click()
  await expect.poll(() => rootPreviewFrame.evaluate((frame) => {
    const host = frame.closest('.native-template-preview')?.getBoundingClientRect()
    const preview = frame.getBoundingClientRect()
    return Boolean(host && preview.width > 0 && preview.left >= host.left && preview.right <= host.right + 1)
  })).toBe(true)
})

test('protocol dialogs stay centered throughout their opening animation', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.setViewportSize({ width: 1440, height: 960 })
  await page.goto('/protocols')

  async function expectCenteredOpening(selector: string, trigger: import('@playwright/test').Locator) {
    // Observe the live transition, including the size change after config loads.
    // A screenshot taken after the animation ends cannot detect a drifting center.
    const motion = page.evaluate((target) => new Promise<Array<{ dx: number; dy: number; scaled: boolean }>>((resolve) => {
      const frames: Array<{ dx: number; dy: number; scaled: boolean }> = []
      const started = performance.now()
      let appeared: number | undefined
      const sample = (now: number) => {
        const dialog = document.querySelector<HTMLElement>(target)
        if (dialog) {
          appeared ??= now
          const rect = dialog.getBoundingClientRect()
          frames.push({
            dx: Math.abs(rect.x + rect.width / 2 - window.innerWidth / 2),
            dy: Math.abs(rect.y + rect.height / 2 - window.innerHeight / 2),
            scaled: rect.width < dialog.offsetWidth - 1,
          })
        }
        if ((appeared !== undefined && now - appeared >= 600) || now - started > 3000) resolve(frames)
        else requestAnimationFrame(sample)
      }
      requestAnimationFrame(sample)
    }), selector)
    await trigger.click()
    const frames = await motion
    expect(frames.some((frame) => frame.scaled)).toBe(true)
    expect(Math.max(...frames.map((frame) => frame.dx))).toBeLessThanOrEqual(1)
    expect(Math.max(...frames.map((frame) => frame.dy))).toBeLessThanOrEqual(1)
  }

  await expectCenteredOpening('[data-slot=app-dialog][role=dialog]', page.getByTestId('adapter-add'))
  await page.getByTestId('adapter-select-qqofficial').click()
  await page.getByLabel('AppID', { exact: true }).fill('100000007')
  await expectCenteredOpening('[data-slot=app-dialog][role=alertdialog]', page.locator('[aria-label="关闭弹窗"]'))
  await page.getByRole('button', { name: '放弃修改' }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expectCenteredOpening('[data-slot=app-dialog][role=dialog]', page.getByTestId('adapter-onebot11'))
  await page.locator('[aria-label="关闭弹窗"]').click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expectCenteredOpening('[data-slot=app-dialog][role=dialog]', page.getByRole('button', { name: '兼容矩阵', exact: true }))
})

test('protocol connection creation stays local until the completed form is saved', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
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

test('protocol legacy links open dialogs within one workspace and fit a narrow viewport', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/protocols/onebot11/onebot11')
  await expect(page).toHaveURL(/\/protocols\?adapter=onebot11$/)
  await expect(page.getByRole('dialog', { name: '配置 OneBot11' })).toBeVisible()
  await expectDocumentWithinViewport(page)
  await expect(page.getByTestId('adapter-save')).toBeInViewport()
  await page.locator('[aria-label="关闭弹窗"]').click()
  await page.goto('/protocols/compatibility')
  await expect(page).toHaveURL(/\/protocols\?view=compatibility$/)
  await expect(page.getByRole('dialog', { name: '协议兼容矩阵' })).toBeVisible()
  await page.getByLabel('筛选兼容能力').fill('group.sign')
  await expect(page.locator('.protocol-compatibility-table')).toContainText('provider.napcat.group.sign.set')
  await expectDocumentWithinViewport(page)
  await page.getByLabel('筛选兼容能力').fill('no-matching-capability')
  await expect(page.getByRole('status')).toContainText('没有匹配的兼容能力')
})

test('protocol center owns OneBot settings and logs center keeps protocol filtering', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/config')
  await expect(page.getByText('协议连接设置')).toHaveCount(0)
  await expect(page.getByText('反向 WebSocket 地址')).toHaveCount(0)
  await page.goto('/protocols')

  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  await expect(page.locator('.connections-grid')).toContainText('OneBot11')
  await expect(page.getByTestId('adapter-select-onebot11')).toHaveCount(0)
  await page.getByTestId('adapter-onebot11').click()
  await expect(page.getByRole('dialog', { name: '配置 OneBot11' })).toBeVisible()
  await page.getByText('高级设置', { exact: false }).click()
  await page.getByLabel('连接超时（秒）').fill('18')
  const saveResponse = page.waitForResponse((response) => response.request().method() === 'PUT' && response.url().endsWith('/api/config'))
  await page.getByTestId('adapter-save').click()
  expect((await saveResponse).status()).toBe(200)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.getByTestId('adapter-onebot11').click()
  await page.locator('summary').filter({ hasText: '高级设置' }).click()
  await expect(page.getByLabel('连接超时（秒）')).toHaveValue('18')
  await page.locator('[aria-label="关闭弹窗"]').click()

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await expect(page.getByRole('button', { name: '更多筛选' })).toBeVisible()

  await openLogAdvancedFilters(page)
  const protocolField = logFilterField(page, '协议')
  await expect(protocolField).toBeVisible()
  await protocolField.locator('.ant-select').click()
  await page.getByTitle('OneBot11').click()
  await page.getByRole('button', { name: '应用筛选' }).click()

  await expect(page.getByText('req_adapter_ignored_0001')).toBeVisible()
  await expect(page.getByText('req_plugin_0001')).toHaveCount(0)

  const protocolRow = page.locator('.logs-row').filter({ hasText: 'req_adapter_ignored_0001' }).first()
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
  await expect.poll(() => page.url()).toContain('/protocols?view=compatibility')
  await expect(page.getByRole('dialog', { name: '协议兼容矩阵' })).toBeVisible()

  await page.goto('/protocols')
  await page.getByTestId('adapter-onebot11').click()
  await page.locator('summary').filter({ hasText: '运行状态与诊断' }).click()
  await page.getByRole('button', { name: '查看此协议的实时日志' }).click()
  await expect.poll(() => page.url()).toContain('/logs')
  await expect(page.url()).toContain('protocol=onebot11')

  await page.goto('/logs?plugin_id=weather')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.locator('.logs-row').filter({ hasText: 'req_plugin_0001' }).first().click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '查看插件' }).click()
  await expect.poll(() => page.url()).toContain('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()

  await page.getByRole('button', { name: '当前插件指令' }).click()
  await expect.poll(() => page.url()).toContain('/commands')
  await expect(page.url()).toContain('plugin_id=weather')
  await expectPluginCenterPage(page, '指令中心')
  await expect(page.locator('.commands-data-table')).toContainText('weather')
  await expect((await readTabLabels(page)).filter((label) => label === '插件中心')).toHaveLength(1)

  await request.post(`${backendUrl}/__test/push-task`, {
    data: {
      task_id: 'task_plugin_install_succeeded_0001',
      task_type: 'plugin.install',
      status: 'succeeded',
      summary: 'install weather',
      plugin_id: 'weather',
    },
  })
  await page.goto('/logs?source=tasks&request_id=task_plugin_install_succeeded_0001')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  await page.locator('.logs-row').filter({ hasText: 'task_plugin_install_succeeded_0001' }).first().click()
  await expect(logDetailWindow(page)).toBeVisible()
  await logDetailWindow(page).getByRole('button', { name: '查看插件' }).click()
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

  await openLogAdvancedFilters(page)
  await Promise.all([
    page.waitForResponse((response) => (
      response.request().method() === 'GET'
      && response.url().endsWith('/api/plugins')
    )),
    logFilterField(page, '插件').locator('.ant-select').click(),
  ])
  expect(pluginRequests).toHaveLength(1)
  await page.keyboard.press('Escape')

  await navigateThroughMenu(page, '历史日志', '运行与诊断')
  await expect(page.getByRole('heading', { name: '历史日志', level: 1 })).toBeVisible()
  await page.waitForTimeout(200)
  expect(pluginRequests).toHaveLength(1)

  await openLogAdvancedFilters(page)
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
  await expect(logsTable).toContainText('req_plugin_0001')
  await expect(logsTable).not.toContainText('adapter.onebot11')

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
    const tableBody = document.querySelector<HTMLElement>('.logs-feed-card .data-viewport__scroller')
    if (!tableBody) {
      return {
        hasTableBody: false,
        scrollHeight: 0,
        clientHeight: 0,
        scrollTop: 0,
      }
    }

    tableBody.scrollTop = tableBody.scrollHeight

    return {
      hasTableBody: true,
      scrollHeight: tableBody.scrollHeight,
      clientHeight: tableBody.clientHeight,
      scrollTop: tableBody.scrollTop,
    }
  })

  expect(metrics.hasTableBody).toBe(true)
  expect(metrics.scrollHeight).toBeGreaterThan(metrics.clientHeight)
  expect(metrics.scrollTop).toBeGreaterThan(0)
})

test('command center shows all declared commands and filters by plugin selection', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/commands')
  await expectPluginCenterPage(page, '指令中心')
  const commandsTable = page.locator('.commands-data-table')

  await expect(page.getByTestId('commands-open-permission-policy')).toBeVisible()
  await expect(page.getByText('策略总览', { exact: true })).toHaveCount(0)
  await expect(page.getByText('白名单', { exact: true })).toHaveCount(0)
  await expect(page.getByText('黑名单', { exact: true })).toHaveCount(0)
  await expect(commandsTable).toContainText('hello')
  await expect(commandsTable).toContainText('weather')

  const pluginSelector = page.locator('.commands-filter-toolbar').getByRole('combobox')
  await expect(pluginSelector).toBeVisible()
  await pluginSelector.click()
  await page.getByRole('option', { name: /Weather/ }).click()
  await page.keyboard.press('Escape')

  await expect(commandsTable).toContainText('weather')
  await expect(commandsTable).not.toContainText('hello')
  await expect(commandsTable).toContainText('查询天气')
  await expect(commandsTable).not.toContainText('查看帮助')

  await page.getByTestId('commands-open-permission-policy').click()
  await expect.poll(() => page.url()).toContain('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
})

test('plugin center switches original routes while retaining drafts and independent details', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await navigateThroughMenu(page, '插件中心')
  const navigation = page.getByTestId('plugin-center-sidebar-navigation')
  const destinations = [
    ['菜单中心', '/menu-center', 'menu-center'], ['插件商店', '/plugins/store', 'plugin-store'],
    ['插件列表', '/plugins', 'plugins'], ['全局插件设置', '/plugins/settings', 'plugin-settings'],
    ['指令中心', '/commands', 'commands'],
  ] as const
  await navigation.locator('[data-sidebar-page="plugin-settings"]').click()
  const limit = page.getByLabel('插件工作目录软上限（MB）')
  await limit.fill('257')
  for (const [name, path, routeName] of destinations) {
    await navigation.locator(`[data-sidebar-page="${routeName}"]`).click()
    await expect(page).toHaveURL(new RegExp(`${path}$`))
    await expectPluginCenterPage(page, name)
    await expect(navigation.locator('[aria-current=page]')).toHaveText(name)
    await expect.poll(() => readTabLabels(page)).toEqual(['系统状态', '插件中心'])
  }
  await page.goBack()
  await expect(page).toHaveURL(/\/plugins\/settings$/)
  await expect(limit).toHaveValue('257')
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toBeVisible()

  await page.goto('/commands?plugin_id=weather')
  await expect(navigation.locator('[aria-current=page]')).toHaveText('指令中心')
  await navigation.locator('[data-sidebar-scope-back="plugin-center"]').click()
  await expect(page).toHaveURL(/\/commands\?plugin_id=weather$/)
  await navigateThroughMenu(page, '权限策略', '治理')
  await page.locator('.admin-layout__tabbar [data-tab-path="/plugins"]').click()
  await expect(page).toHaveURL(/\/commands\?plugin_id=weather$/)
  await page.goto('/plugins/weather?panel=overview')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()
  await expect(navigation).toHaveCount(1)
  await expect(navigation.locator('[data-sidebar-plugin-overview]')).toHaveAttribute('aria-current', 'page')
  await expect.poll(() => readTabLabels(page)).toEqual(expect.arrayContaining(['系统状态', '插件：Weather']))
  await page.locator('.admin-layout__nav-trigger.desktop-only').click()
  await expect(page.locator('.app-menu-popup:visible')).toHaveCount(0)
  await page.locator('.admin-layout__sider [data-sidebar-entry="plugin-center"]').click()
  const pluginCenterPopup = page.locator('.app-menu-popup:visible')
  await expect(pluginCenterPopup.getByRole('menuitem')).toHaveCount(6)
  await expect(pluginCenterPopup.locator('[data-sidebar-open-plugin-id="weather"]')).toBeVisible()
  await pluginCenterPopup.getByText('全局插件设置', { exact: true }).click()
  await expect(page).toHaveURL(/\/plugins\/settings$/)
  await expect(page.locator('.app-menu-popup:visible')).toHaveCount(0)
})

test('plugin store manages sources and confirms first installs', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await page.goto('/plugins/store')
  await expectPluginCenterPage(page, '插件商店')

  await expect(page.getByText('Echo', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '管理插件源' }).click()
  const sourceDialog = page.getByRole('dialog', { name: '管理插件源' })
  await expect(sourceDialog.getByText('RayleaBot 官方插件', { exact: true })).toBeVisible()
  await expect(sourceDialog.getByText('社区插件', { exact: true })).toBeVisible()
  await sourceDialog.getByRole('button', { name: '关闭弹窗' }).click()
  await expect(sourceDialog).toBeHidden()

  await page.getByTestId('plugin-store-install-raylea.echo').click()
  const confirmDialog = page.getByRole('dialog', { name: '确认安装插件' })
  await expect(confirmDialog.getByText('message.send', { exact: true })).toBeVisible()
  const installResponsePromise = page.waitForResponse(response => (
    response.request().method() === 'POST'
    && response.url().endsWith('/api/plugin-store/plugins/raylea.echo/install')
  ))
  await confirmDialog.getByRole('button', { name: '确认并安装' }).click()
  expect((await installResponsePromise).status()).toBe(202)
  await expect(page.getByText(/安装任务已提交/).first()).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByText('Echo', { exact: true })).toBeVisible()
  await expect(page.getByTestId('plugin-store-refresh')).toBeVisible()
})

test('plugin sidebar keeps resources visible, resumes workspaces, and returns to root once', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.route('**/api/plugins', async (route) => {
    const outbound = route.request()
    if (outbound.method() !== 'GET' || new URL(outbound.url()).pathname !== '/api/plugins') {
      await route.continue()
      return
    }

    const response = await route.fetch()
    const payload = await response.json() as { items: Array<Record<string, unknown>> }
    const template = payload.items[0]
    const extraItems = Array.from({ length: 8 }, (_, index) => ({
      ...template,
      id: `fixture-addon-${index}`,
      name: index === 7
        ? 'Needle Plugin With An Intentionally Long Display Name For Sidebar Truncation'
        : `Fixture Addon ${index}`,
      state: index % 2 === 0 ? 'running' : 'disabled',
    }))
    await route.fulfill({ response, json: { ...payload, items: [...payload.items, ...extraItems] } })
  })
  await login(page)
  await navigateThroughMenu(page, '插件中心')

  let navigation = page.getByTestId('plugin-center-sidebar-navigation')
  await expect(navigation.locator('[data-sidebar-page]')).toHaveText([
    '插件列表',
    '插件商店',
    '全局插件设置',
    '菜单中心',
    '指令中心',
  ])
  const pluginIds = await navigation.locator('[data-sidebar-plugin-id]').evaluateAll(nodes => (
    nodes.map(node => node.getAttribute('data-sidebar-plugin-id') ?? '')
  ))
  expect(pluginIds).toEqual([...pluginIds].sort((left, right) => left.localeCompare(right)))
  const filter = navigation.getByLabel('筛选已安装插件')
  await expect(filter).toBeVisible()
  await filter.fill('needle')
  await expect(navigation.locator('[data-sidebar-plugin-id]')).toHaveCount(1)
  await expect(navigation.locator('[data-sidebar-plugin-id="fixture-addon-7"] .sidebar-navigation__plugin-copy')).toHaveAttribute('title', /Needle Plugin/)
  await expectDocumentWithinViewport(page)
  await filter.clear()

  await navigation.locator('[data-sidebar-plugin-id="example-config-panel"]').click()
  await expect(page).toHaveURL(/\/plugins\/example-config-panel$/)
  await expect(navigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toHaveClass(/sidebar-navigation__plugin-resource--active/)
  await expect(page.getByTestId('plugin-detail-icon').locator('img')).toHaveCount(0)
  await expect(page.getByTestId('plugin-detail-icon').locator('.raylea-mark')).toBeVisible()
  const configPanelTabIcon = page.locator('.admin-layout__tab-label[data-tab-path="/plugins/example-config-panel"] .plugin-icon')
  await expect(configPanelTabIcon.locator('img')).toHaveCount(0)
  await expect(configPanelTabIcon.locator('.raylea-mark')).toBeVisible()
  await expect(navigation.locator('[data-sidebar-plugin-overview="example-config-panel"]')).toHaveAttribute('aria-current', 'page')
  await expect(navigation.locator('[data-sidebar-management-page="config"]')).toBeVisible()

  await navigation.locator('[data-sidebar-management-page="config"]').click()
  await expect(page).toHaveURL(/panel=management-ui&management_page=config/)
  const managementUrl = page.url()
  let releaseWeatherDetail: () => void = () => undefined
  const weatherDetailGate = new Promise<void>((resolve) => {
    releaseWeatherDetail = resolve
  })
  await page.route('**/api/plugins/weather', async (route) => {
    const request = route.request()
    if (request.method() === 'GET' && new URL(request.url()).pathname === '/api/plugins/weather') {
      await weatherDetailGate
    }
    await route.continue()
  })
  const weatherDisclosure = navigation.locator('[data-sidebar-plugin-disclosure="weather"]')
  await weatherDisclosure.click()
  await expect(page).toHaveURL(managementUrl)
  try {
    await expect(weatherDisclosure).toHaveAttribute('aria-busy', 'true')
    await expect(weatherDisclosure).toHaveAttribute('aria-expanded', 'false')
    await expect(navigation.locator('[data-sidebar-plugin-page-owner="weather"]')).toHaveCount(0)
  } finally {
    releaseWeatherDetail()
  }
  await expect(navigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(navigation.locator('[data-sidebar-plugin-id="weather"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(navigation.locator('[data-sidebar-plugin-overview="weather"]')).toBeVisible()
  await navigation.locator('[data-sidebar-plugin-id="weather"]').click()
  await expect(page).toHaveURL(/\/plugins\/weather$/)
  await expect(page.getByTestId('plugin-detail-icon').locator('img')).toHaveAttribute('src', '/api/plugins/weather/icon')
  await expect(page.locator('.admin-layout__tab-label[data-tab-path="/plugins/weather"] .plugin-icon img')).toHaveAttribute('src', '/api/plugins/weather/icon')
  await expect(navigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(navigation.locator('[data-sidebar-plugin-id="weather"]')).toHaveAttribute('aria-expanded', 'true')
  await navigation.locator('[data-sidebar-plugin-id="example-config-panel"]').click()
  await expect(page).toHaveURL(managementUrl)
  await expect(navigation.locator('[data-sidebar-management-page="config"]')).toHaveAttribute('aria-current', 'page')

  await filter.fill('needle')
  await expect(navigation.locator('[data-sidebar-plugin-id]')).toHaveCount(2)
  await expect(navigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toBeVisible()
  await filter.clear()

  await navigation.locator('[data-sidebar-scope-back="plugin-center"]').click()
  await expect(page).toHaveURL(managementUrl)
  const rootEntry = page.locator('.admin-layout__sider [data-sidebar-entry="plugin-center"]')
  await expect(rootEntry).toBeFocused()
  await rootEntry.click()
  await expect(page).toHaveURL(managementUrl)
  navigation = page.getByTestId('plugin-center-sidebar-navigation')
  await expect(navigation.locator('[data-sidebar-scope-back="plugin-center"]')).toBeFocused()
})

test('legacy plugin tabs merge on reload and the current deep link wins', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)
  await page.evaluate(() => {
    const tab = (name: string, path: string) => ({ name, path, fullPath: path, title: name, keepAlive: true })
    window.localStorage.setItem('rayleabot.ui-shell', JSON.stringify({
      version: 3, preferences: { rememberTabs: true },
      tabs: [tab('logs', '/logs'), tab('commands', '/commands'), tab('plugin-detail', '/plugins/weather'), tab('plugin-settings', '/plugins/settings')],
    }))
  })
  await page.goto('/plugins/settings?section=runtime')
  await expectPluginCenterPage(page, '全局插件设置')
  await expect.poll(() => page.locator('.admin-layout__tabbar [data-tab-path]').evaluateAll(nodes => nodes.map(node => node.getAttribute('data-tab-path')))).toEqual(['/', '/logs', '/plugins', '/plugins/weather'])
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__item[aria-current=page]')).toHaveText('全局插件设置')
  await page.reload()
  await expect(page.getByTestId('plugin-center-sidebar-navigation').locator('[aria-current=page]')).toHaveText('全局插件设置')
  expect(await readActiveTabLabel(page)).toBe('插件中心')
})

test('breadcrumb and tabbar track leaf pages instead of hidden route groups', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await expect(page.locator('.admin-layout__breadcrumb-current')).toHaveText('系统状态')

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page.locator('.admin-layout__header-breadcrumb').getByRole('link', { name: '治理' })).toHaveAttribute('href', '/permission-policy')
  await expect(page.locator('.admin-layout__breadcrumb-current')).toHaveText('权限策略')
  await expect(page.getByRole('tab', { name: '权限策略' })).toBeVisible()

  let tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(['系统状态', '权限策略'])
  expect(await readTabIconKeys(page)).toEqual(['dashboard', 'permission-policy'])
  expect(await readActiveTabLabel(page)).toBe('权限策略')
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__group').filter({ hasText: '治理' }).locator('.sidebar-navigation__item .admin-layout__menu-icon')).toHaveCount(3)

  await page.goto('/commands')
  await expectPluginCenterPage(page, '指令中心')
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(expect.arrayContaining(['系统状态', '权限策略', '插件中心']))
  expect(await readTabIconKeys(page)).toEqual(expect.arrayContaining(['dashboard', 'permission-policy', 'plugins']))
  expect(await readActiveTabLabel(page)).toBe('插件中心')
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__item[aria-current=page]')).toHaveText('指令中心')
  await expect(page.getByTestId('plugin-center-sidebar-navigation').locator('[data-sidebar-page]')).toHaveCount(5)

  await page.goto('/logs')
  await expect(page.getByRole('heading', { name: '实时日志', level: 1 })).toBeVisible()
  tabLabels = await readTabLabels(page)
  expect(tabLabels).toEqual(expect.arrayContaining(['系统状态', '权限策略', '插件中心', '实时日志']))
  expect(await readTabIconKeys(page)).toEqual(expect.arrayContaining(['dashboard', 'permission-policy', 'plugins', 'logs']))
  expect(await readActiveTabLabel(page)).toBe('实时日志')

  await page.goto('/protocols')
  await expect(page.getByRole('heading', { name: '协议中心', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('协议中心')
  expect(await readTabLabels(page)).toContain('协议中心')
  expect(await readTabIconKeys(page)).toContain('protocols')
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__group').filter({ hasText: '账号与连接' }).locator('.sidebar-navigation__item .admin-layout__menu-icon')).toHaveCount(2)

  await page.goto('/config')
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('配置')
  expect(await readTabLabels(page)).toContain('配置')
  expect(await readTabIconKeys(page)).toContain('config')
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__group').filter({ hasText: '系统' }).locator('.sidebar-navigation__item .admin-layout__menu-icon')).toHaveCount(2)
  await expect(page.locator('.admin-layout__sider .sidebar-navigation__item[aria-current=page] .admin-layout__menu-icon')).toHaveCount(1)

  await page.goto('/permission-policy')
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  await expect(page).toHaveURL(/\/permission-policy$/)

  await page.reload()
  await expect(page.getByRole('heading', { name: '权限策略', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('权限策略')
  expect(await readTabLabels(page)).toContain('权限策略')

  await page.goto('/plugins/weather')
  await expect(page.getByRole('heading', { name: 'weather', level: 1 })).toBeVisible()
  expect(await readActiveTabLabel(page)).toBe('插件：Weather')
  expect(await readTabLabels(page)).toContain('插件：Weather')
  expect(await readTabIconKeys(page)).toContain('plugins')
})

test('tab context menu closes tabs relative to the clicked tab', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await openStandardTabs(page)
  await openTabContextMenu(page, '权限策略')
  await clickTabContextAction(page, '关闭当前标签')
  await expect(page).toHaveURL(/\/logs$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '实时日志'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')

  await openStandardTabs(page)
  await openTabContextMenu(page, '权限策略')
  await clickTabContextAction(page, '关闭其他标签')
  await expect(page).toHaveURL(/\/permission-policy$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '权限策略'])
  expect(await readActiveTabLabel(page)).toBe('权限策略')

  await openStandardTabs(page)
  await openTabContextMenu(page, '实时日志')
  await clickTabContextAction(page, '关闭左侧标签')
  await expect(page).toHaveURL(/\/logs$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '实时日志'])
  expect(await readActiveTabLabel(page)).toBe('实时日志')

  await openStandardTabs(page)
  await openTabContextMenu(page, '权限策略')
  await clickTabContextAction(page, '关闭右侧标签')
  await expect(page).toHaveURL(/\/permission-policy$/)
  expect(await readTabLabels(page)).toEqual(['系统状态', '权限策略'])
  expect(await readActiveTabLabel(page)).toBe('权限策略')

  await openStandardTabs(page)
  await openTabContextMenu(page, '权限策略')
  await clickTabContextAction(page, '关闭所有标签')
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  expect(await readTabLabels(page)).toEqual(['系统状态'])
  expect(await readActiveTabLabel(page)).toBe('系统状态')
})

test('login keeps the protected shell after reload', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await expect(page).not.toHaveURL(/\/login$/)
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')

  await page.reload()

  await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
  await expect(page).not.toHaveURL(/\/login$/)
  await expect(page.getByTestId('connection-card-events')).toContainText('已认证')
})

test('third-party accounts show Bilibili CK cards and QR login updates account cards', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/third-party-accounts')
  await expect(page.getByRole('heading', { name: '三方账号', level: 1 })).toBeVisible()
  await expect(page.locator('.source-summary-strip')).toHaveCount(0)
  const credentialAuthorityHelp = page.getByRole('button', { name: '查看账号状态说明' })
  await expect(credentialAuthorityHelp).toBeVisible()
  await credentialAuthorityHelp.click()
  await expect(page.getByText('单次 HTTP 403 不会直接判定 CK 失效')).toBeVisible()

  const accountCard = page.locator('.account-card').filter({ hasText: '测试账号昵称' }).first()
  await expect(accountCard).toBeVisible()
  await expect(accountCard).toContainText('UID 123456')
  await expect(accountCard).toContainText('服务器确认有效')
  const avatarImage = accountCard.getByTestId('bilibili-account-avatar-image')
  await expect(avatarImage).toBeVisible()
  await expect(avatarImage).toHaveAttribute('src', '/api/third-party/accounts/bilibili/primary/avatar')
  await page.clock.install()
  await avatarImage.evaluate((element) => element.dispatchEvent(new Event('error')))
  await expect(accountCard.getByTestId('bilibili-account-avatar-fallback')).toBeVisible()
  await expect(accountCard.getByTestId('bilibili-account-avatar-image')).toHaveCount(0)
  await page.clock.fastForward(30_001)
  await expect(accountCard.getByTestId('bilibili-account-avatar-image')).toBeVisible()
  await expect.poll(() => accountCard.getByTestId('bilibili-account-avatar-image')
    .evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
  await accountCard.getByTestId('bilibili-account-avatar-image')
    .evaluate((element) => element.dispatchEvent(new Event('error')))
  await expect(accountCard.getByTestId('bilibili-account-avatar-fallback')).toBeVisible()
  await accountCard.getByRole('button', { name: '检查 CK' }).click()
  await expect(accountCard.getByTestId('bilibili-account-avatar-image')).toBeVisible()
  await page.clock.resume()

  const weiboAccountCard = page.locator('.account-card').filter({ hasText: '微博扫码账号' }).first()
  const weiboAvatarImage = weiboAccountCard.getByTestId('bilibili-account-avatar-image')
  await expect(weiboAvatarImage).toBeVisible()
  await expect(weiboAvatarImage).toHaveAttribute('src', '/api/third-party/accounts/weibo/primary/avatar')
  await expect.poll(() => weiboAvatarImage.evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)

  await accountCard.getByRole('button', { name: '编辑' }).click()
  const editingExistingCard = page.locator('.account-card--editing').filter({ hasText: '留空时保留当前 CK。' }).first()
  await expect(editingExistingCard.locator('textarea')).toBeVisible()
  await expect(editingExistingCard).toContainText('留空时保留当前 CK。')
  await editingExistingCard.getByRole('button', { name: /取\s*消/ }).click()

  await page.getByRole('button', { name: '添加 Bilibili CK' }).first().click()
  await page.getByRole('button', { name: '添加 Bilibili CK' }).first().click()
  const draftCards = page.locator('.account-card--editing').filter({ hasText: 'Bilibili Cookie' })
  await expect(draftCards).toHaveCount(2)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.setViewportSize({ width: 1280, height: 720 })
  const draftCard = page.locator('.account-card--editing').nth(0)
  await expect(draftCard).toBeVisible()
  await expect(draftCard.getByRole('button', { name: '检查 CK' })).toHaveCount(0)
  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()
  await expect(draftCard.locator('.qr-panel')).toBeVisible()
  const scannedAccountCard = page.locator('.account-card--editing').filter({ has: page.locator('.qr-panel') }).first()
  const scannedInputs = scannedAccountCard.locator('input')
  await expect(scannedInputs.nth(0)).toHaveValue('123456', { timeout: 5000 })
  await expect(scannedInputs.nth(1)).toHaveValue('测试账号昵称')
  const savedQRCodeAccountCards = page.locator('.account-card').filter({ hasText: '账号 ID123456' })
  await expect(savedQRCodeAccountCards).toHaveCount(1)
  await expect(scannedAccountCard).toContainText('服务器确认有效')
  const saveRequestOrder: string[] = []
  const recordSaveRequest = (request: import('@playwright/test').Request) => {
    const url = new URL(request.url())
    if (url.pathname.includes('/login/qrcode/') && request.method() === 'DELETE') {
      saveRequestOrder.push('cancel')
    } else if (url.pathname === '/api/third-party/accounts/bilibili/123456' && request.method() === 'PUT') {
      saveRequestOrder.push('save')
    }
  }
  page.on('request', recordSaveRequest)
  await scannedAccountCard.getByRole('button', { name: '保存' }).click()
  await expect.poll(() => saveRequestOrder).toEqual(['cancel', 'save'])
  page.off('request', recordSaveRequest)
  await expect(savedQRCodeAccountCards).toHaveCount(1)
  await expect(savedQRCodeAccountCards.first()).not.toHaveClass(/account-card--editing/)

  const additionalQRLogins = [
    {
      platform: 'weibo',
      addLabel: '添加微博 Cookie',
      draftTitle: '微博 Cookie',
      savedAccountId: '123456',
      savedCardText: '微博扫码账号',
    },
    {
      platform: 'douyin',
      addLabel: '添加抖音 Cookie',
      draftTitle: '抖音 Cookie',
      savedAccountId: 'primary',
    },
    {
      platform: 'netease_music',
      addLabel: '添加网易云音乐 Cookie',
      draftTitle: '网易云音乐 Cookie',
      savedAccountId: '123456789',
      savedCardText: '网易云音乐账号',
    },
  ]
  for (const item of additionalQRLogins) {
    const addButton = page.getByRole('button', { name: item.addLabel }).first()
    const platformSection = addButton.locator('xpath=ancestor::section[contains(@class,"platform-section")]')
    const editingCount = await platformSection.locator('.account-card--editing').count()
    await addButton.click()
    const platformDraftCard = platformSection.locator('.account-card--editing').nth(editingCount)
    await expect(platformDraftCard).toContainText(item.draftTitle)
    await platformDraftCard.getByRole('button', { name: '扫码获取 CK' }).click()
    await expect(platformDraftCard.locator('.qr-panel')).toBeVisible()
    const scannedPlatformCard = platformSection.locator('.account-card--editing').filter({ has: page.locator('.qr-panel') }).first()
    const platformInputs = scannedPlatformCard.locator('input')
    await expect(platformInputs.nth(0)).toHaveValue(item.savedAccountId, { timeout: 5000 })
    if (item.savedCardText) {
      await expect(platformInputs.nth(1)).toHaveValue(item.savedCardText)
    }
    const savedAccountCards = platformSection.locator('.account-card').filter({ hasText: `账号 ID${item.savedAccountId}` })
    await expect(savedAccountCards).toHaveCount(1)
    await expect(scannedPlatformCard).toContainText(`账号 ID${item.savedAccountId}`)
    await expect(scannedPlatformCard).toContainText('已配置')
    await scannedPlatformCard.getByRole('button', { name: '保存' }).click()
    await expect(savedAccountCards).toHaveCount(1)
    await expect(savedAccountCards.first()).not.toHaveClass(/account-card--editing/)
    await expect(savedAccountCards.first().getByTestId('bilibili-account-avatar-image')).toHaveAttribute(
      'src',
      `/api/third-party/accounts/${item.platform}/${item.savedAccountId}/avatar`,
    )
  }

  const weiboSection = page.getByRole('heading', { name: '微博', level: 3 }).locator('xpath=ancestor::section[contains(@class,"platform-section")]')
  const savedWeiboCard = weiboSection.locator('.account-card').filter({ hasText: '账号 ID123456' }).first()
  await expect(savedWeiboCard.getByTestId('bilibili-account-avatar-image')).toHaveAttribute(
    'src',
    '/api/third-party/accounts/weibo/123456/avatar',
  )
  await expect.poll(() => savedWeiboCard.getByTestId('bilibili-account-avatar-image')
    .evaluate((element) => (element as HTMLImageElement).naturalWidth)).toBeGreaterThan(0)
})

test('third-party account manual check updates an expired Weibo CK', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/third-party-accounts')
  const accountCard = page.locator('.account-card').filter({ hasText: '微博扫码账号' }).first()
  await expect(accountCard).toContainText('等待服务器确认')
  const validateButton = accountCard.getByRole('button', { name: '检查 CK' })
  await validateButton.click()
  await expect(validateButton).toHaveAttribute('aria-busy', 'true')
  await expect(accountCard).toContainText('服务器确认失效')
  await expect(accountCard).toContainText('微博账号 CK 已失效，请重新扫码')
  await expect(page.getByText('服务器已确认 CK 失效，请重新登录', { exact: true })).toBeVisible()

  await page.reload()
  const persistedCard = page.locator('.account-card').filter({ hasText: '微博扫码账号' }).first()
  await expect(persistedCard).toContainText('服务器确认失效')
})

test('third-party QR login survives a transient polling failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failThirdPartyQRCodePollOnce: true,
  })
  await login(page)

  await page.goto('/third-party-accounts')
  await page.getByRole('button', { name: '添加 Bilibili CK' }).first().click()
  const draftCard = page.locator('.account-card--editing').filter({ hasText: 'Bilibili Cookie' }).first()
  const failedPoll = page.waitForResponse((response) => (
    response.url().includes('/api/third-party/accounts/bilibili/login/qrcode/')
    && response.status() === 502
  ))
  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()

  const scannedCard = page.locator('.account-card--editing').filter({ has: page.locator('.qr-panel') }).first()
  const qrPanel = scannedCard.locator('.qr-panel')
  await expect(qrPanel).toBeVisible()
  await failedPoll
  await expect(qrPanel).toBeVisible()
  await expect(scannedCard.locator('input').nth(0)).toHaveValue('123456', { timeout: 7000 })
})

test('Douyin QR login keeps polling while browser verification is required', async ({ page, request }) => {
  await resetBackend(request, true, {
    douyinQRCodeVerificationRequired: true,
  })
  await login(page)

  await page.goto('/third-party-accounts')
  const addButton = page.getByRole('button', { name: '添加抖音 Cookie' }).first()
  const platformSection = addButton.locator('xpath=ancestor::section[contains(@class,"platform-section")]')
  await addButton.click()
  const draftCard = platformSection.locator('.account-card--editing').filter({ hasText: '抖音 Cookie' }).first()
  await expect(draftCard.locator('textarea')).toHaveAttribute('placeholder', 'sessionid=...; sid_guard=...')

  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()
  const scannedCard = platformSection.locator('.account-card--editing').filter({ has: page.locator('.qr-panel') }).first()
  const qrPanel = scannedCard.locator('.qr-panel')
  await expect(qrPanel).toContainText('等待完成安全验证', { timeout: 7000 })
  await expect(qrPanel).toContainText('登录浏览器窗口')
  await expect(qrPanel).toContainText('远程 CDP')
  await expect(qrPanel).toContainText('已获取 CK', { timeout: 5000 })
  await expect(scannedCard).toContainText('已配置')
})

test('Douyin QR login exposes failure recovery and cancels abandoned sessions', async ({ page, request }) => {
  await resetBackend(request, true, {
    douyinQRCodeFailed: true,
  })
  await login(page)

  await page.goto('/third-party-accounts')
  const addButton = page.getByRole('button', { name: '添加抖音 Cookie' }).first()
  const platformSection = addButton.locator('xpath=ancestor::section[contains(@class,"platform-section")]')
  await addButton.click()
  let draftCard = platformSection.locator('.account-card--editing').filter({ hasText: '抖音 Cookie' }).first()
  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()

  const qrPanel = draftCard.locator('.qr-panel')
  await expect(qrPanel).toContainText('扫码登录失败', { timeout: 7000 })
  await expect(qrPanel).toContainText('手动填写 CK')
  const retryButton = draftCard.getByRole('button', { name: '重新扫码' })
  await expect(retryButton).toBeVisible()

  const retryLifecycle = [] as string[]
  const recordRetryLifecycle = (outbound: import('@playwright/test').Request) => {
    if (outbound.url().includes('/api/third-party/accounts/douyin/login/qrcode')) {
      const method = outbound.method()
      if (method === 'DELETE' || method === 'POST') {
        retryLifecycle.push(method)
      }
    }
  }
  page.on('request', recordRetryLifecycle)
  await retryButton.click()
  await expect.poll(() => retryLifecycle).toEqual(['DELETE', 'POST'])
  page.off('request', recordRetryLifecycle)

  const cancelRequest = page.waitForRequest((outbound) => (
    outbound.method() === 'DELETE'
    && outbound.url().includes('/api/third-party/accounts/douyin/login/qrcode/')
  ))
  await draftCard.getByRole('button', { name: /取\s*消/ }).click()
  await cancelRequest
  await expect(draftCard).toHaveCount(0)

  await addButton.click()
  draftCard = platformSection.locator('.account-card--editing').filter({ hasText: '抖音 Cookie' }).first()
  await draftCard.getByRole('button', { name: '扫码获取 CK' }).click()
  await expect(draftCard.locator('.qr-panel')).toBeVisible()
  const pageLeaveCancel = page.waitForRequest((outbound) => (
    outbound.method() === 'DELETE'
    && outbound.url().includes('/api/third-party/accounts/douyin/login/qrcode/')
  ))
  await navigateThroughMenu(page, '配置', '系统')
  await pageLeaveCancel
  await expect(page.getByRole('heading', { name: '配置', level: 1 })).toBeVisible()
})

test('error recovery covers retry and uninstall failure', async ({ page, request }) => {
  await resetBackend(request, true, {
    failPluginsListOnce: true,
    failPluginDetailOnce: true,
    failUninstallOnce: true,
  })
  await login(page)

  await page.goto('/plugins')
  await expect(page.locator('.retry-panel__inline')).toBeVisible()
  await page.locator('.retry-panel__inline').getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByText('weather').first()).toBeVisible()

  const weatherRow = pluginRows(page).filter({ hasText: 'Weather' })
  await weatherRow.getByRole('button', { name: 'Weather', exact: true }).click()
  await expect(page.locator('.retry-panel__inline')).toBeVisible()
  await page.locator('.retry-panel__inline').getByRole('button', { name: /重\s*试/ }).click({ force: true })
  await expect(page.getByRole('heading', { name: 'weather' })).toBeVisible()

  await page.getByRole('button', { name: /卸\s*载/ }).click()
  await page.getByRole('button', { name: /确认卸载/ }).click()
  await expect(page.getByText('缺少必要资源')).toBeVisible()
})

test('missing routes keep their fallback while network recovery stays in place', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.goto('/missing-admin-page')
  await expect(page.getByRole('heading', { name: '哎呀！未找到页面' })).toBeVisible()

  await page.goto('/commands')
  await expectPluginCenterPage(page, '指令中心')

  await setBackendNetworkOffline(request)
  await page.goto('/access-lists')
  await expect(page.locator('[data-testid="connection-reconnect-notice"]')).toBeVisible({ timeout: 7000 })
  await expect(page).toHaveURL(/\/access-lists$/)
  await expect(page.getByRole('heading', { name: '哎呀！网络错误' })).toHaveCount(0)

  await setBackendNetworkOnline(request)
  await expect(page.locator('[data-testid="connection-reconnect-notice"]')).toBeHidden({ timeout: 7000 })
  await expect(page).toHaveURL(/\/access-lists$/)
})

test('shutdown flow shows the draining toast', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  await page.getByRole('button', { name: '更多操作' }).click()
  await page.getByRole('menuitem', { name: '关闭服务' }).click()
  await page.getByRole('button', { name: '确认关闭' }).click()

  await expect(page.locator('.app-toast-viewport')).toContainText('停机请求已发送')
  await expect(page.locator('.app-toast-viewport')).toContainText('服务正在停止')
})

test('mobile navigation and card layouts remain usable', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 390, height: 844 })

  await login(page)

  await page.getByRole('button', { name: '打开菜单' }).click()
  await page.locator('[data-slot=app-dialog][data-placement=left]:visible [data-sidebar-entry="plugin-center"]').click()
  await expect(pluginRows(page).first()).toBeVisible()

  await page.getByRole('button', { name: '打开菜单' }).click()
  const mobileNavigation = page.locator('[data-slot=app-dialog][data-placement=left]:visible [data-mobile="true"]')
  await expect(mobileNavigation).toHaveAttribute('data-scope', 'plugin-center')
  await mobileNavigation.locator('[data-sidebar-scope-back="plugin-center"]').click()
  await expect(page.locator('[data-slot=app-dialog][data-placement=left]:visible')).toHaveCount(1)
  await expect(mobileNavigation).toHaveAttribute('data-scope', 'root')
  await mobileNavigation.locator('[data-sidebar-entry="plugin-center"]').click()
  await expect(page.locator('[data-slot=app-dialog][data-placement=left]:visible')).toHaveCount(1)
  await expect(mobileNavigation).toHaveAttribute('data-scope', 'plugin-center')
  await mobileNavigation.locator('[data-sidebar-page="plugins"]').click()
  await expect(page.locator('[data-slot=app-dialog][data-placement=left]:visible')).toHaveCount(0)

  await page.getByRole('button', { name: '打开菜单' }).click()
  await page.locator('[data-slot=app-dialog][data-placement=left]:visible [data-sidebar-page="plugin-settings"]').click()
  await expectPluginCenterPage(page, '全局插件设置')

  await page.goto('/plugins/example-config-panel')
  await expect(page.getByRole('heading', { name: '插件：Example Config Panel', level: 1 })).toBeVisible()
  await expect(page.locator('.plugin-detail-panel-switch')).toBeVisible()
  await expect(page.locator('.plugin-detail-panel-switch')).toContainText('配置页面')
  await page.getByRole('button', { name: '打开菜单' }).click()
  const mobileDetailNavigation = page.locator('[data-slot=app-dialog][data-placement=left]:visible [data-mobile="true"][data-scope="plugin-center"]')
  await expect(mobileDetailNavigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(mobileDetailNavigation.locator('[data-sidebar-management-page="config"]')).toBeVisible()
  const mobileDetailUrl = page.url()
  await mobileDetailNavigation.locator('[data-sidebar-plugin-disclosure="weather"]').click()
  await expect(page).toHaveURL(mobileDetailUrl)
  await expect(mobileDetailNavigation.locator('[data-sidebar-plugin-id="example-config-panel"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(mobileDetailNavigation.locator('[data-sidebar-plugin-id="weather"]')).toHaveAttribute('aria-expanded', 'true')
  await expect(mobileDetailNavigation.locator('[data-sidebar-plugin-overview="weather"]')).toBeVisible()
  const mobileDisclosureBounds = await mobileDetailNavigation.locator('[data-sidebar-plugin-disclosure="weather"]').boundingBox()
  expect(mobileDisclosureBounds?.width ?? 0).toBeGreaterThanOrEqual(44)
  expect(mobileDisclosureBounds?.height ?? 0).toBeGreaterThanOrEqual(44)
  const mobileBackBounds = await mobileDetailNavigation.locator('[data-sidebar-scope-back="plugin-center"]').boundingBox()
  expect(mobileBackBounds?.height ?? 0).toBeGreaterThanOrEqual(44)
  await expectDocumentWithinViewport(page)

  await page.goto('/logs?log_id=log_adapter_live_0001')
  await expect(logRows(page).filter({ hasText: 'ignored OneBot API response with unsupported echo' }).first()).toBeVisible()
  await expect(page.locator('.log-detail-drawer:visible')).toContainText('api response echo must be a non-empty string')
  await expect(logDetailWindow(page)).toHaveCount(0)
})

test('mobile settings keeps save and draft feedback reachable at the end of the form', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 393, height: 852 })
  await login(page)
  await page.goto('/plugins/settings')

  const limit = page.getByLabel('插件工作目录软上限（MB）')
  await limit.fill('257')
  await page.locator('.admin-layout__content').evaluate(element => { element.scrollTop = element.scrollHeight })
  const save = page.getByTestId('plugin-settings-save')
  await expect(save).toBeInViewport({ ratio: 1 })
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toBeInViewport()

  await Promise.all([
    page.waitForResponse(response => response.request().method() === 'PUT' && response.url().endsWith('/api/config')),
    save.click(),
  ])
  await expect(page.getByTestId('plugin-settings-unsaved-status')).toHaveCount(0)
  await expect(page.getByTestId('plugin-settings-save-status')).toBeInViewport()
  await page.reload()
  await expect(limit).toHaveValue('257')
})

test('mobile history filters preserve the query and return focus to the reading workspace', async ({ page, request }) => {
  await resetBackend(request, true)
  await page.setViewportSize({ width: 360, height: 800 })
  await login(page)
  await page.goto('/logs/history')

  const toggle = page.getByRole('button', { name: '筛选条件', exact: true })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await toggle.click()
  await page.getByRole('button', { name: '最近半年', exact: true }).click()
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await expect(toggle).toBeFocused()
  await expect(logRows(page).first()).toBeVisible()
  const appliedUrl = page.url()
  const readingArea = page.locator('.logs-feed-card .data-viewport')
  await expect.poll(async () => (await readingArea.boundingBox())?.height ?? 0).toBeGreaterThan(400)

  await toggle.press('Enter')
  await expect(toggle).toHaveAttribute('aria-expanded', 'true')
  await toggle.press('Enter')
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  expect(page.url()).toBe(appliedUrl)
})

test('critical workspaces fit supported viewports and keep shell navigation keyboard reachable', async ({ page, request }) => {
  await resetBackend(request, true)
  await login(page)

  const viewports = [
    { width: 360, height: 800 },
    { width: 768, height: 1024 },
    { width: 1280, height: 800 },
  ]
  const workspaces = [
    { path: '/', heading: '系统状态' },
    { path: '/plugins', heading: '插件列表' },
    { path: '/protocols', heading: '协议中心' },
    { path: '/render/templates/help.menu', heading: '模板预览' },
  ]

  for (const viewport of viewports) {
    await page.setViewportSize(viewport)
    for (const workspace of workspaces) {
      await page.goto(workspace.path)
      await expect(page.getByRole('heading', { name: workspace.heading, level: 1 })).toBeVisible()
      if (workspace.path === '/plugins') await expect(pluginRows(page).first()).toBeVisible()
      if (workspace.path === '/protocols') await expect(page.locator('.connections-surface')).toBeVisible()
      if (workspace.path.startsWith('/render/')) await expect(page.getByTestId('render-template-preview-result').locator('iframe')).toBeVisible()
      await expectDocumentWithinViewport(page)
    }

    await page.goto('/')
    await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
    const skipLink = page.getByRole('link', { name: '跳到主内容' })
    await page.keyboard.press('Tab')
    await expect(skipLink).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(page.locator('#app-main')).toBeFocused()

    await page.goto('/')
    await expect(page.getByRole('heading', { name: '系统状态', level: 1 })).toBeVisible()
    await page.keyboard.press('Tab')
    await expect(skipLink).toBeFocused()

    if (viewport.width < 992) {
      const openMenuButton = page.getByRole('button', { name: '打开菜单' })
      await tabToLocator(page, openMenuButton)
      await page.keyboard.press('Enter')
      await expect(page.locator('.admin-layout__mobile-drawer')).toBeVisible()
      await expectMenuKeyboardReachable(page, page.locator('.admin-layout__mobile-drawer:visible .admin-layout__primary-navigation'))
    } else {
      await expectMenuKeyboardReachable(page, page.locator('.admin-layout__sider:visible .admin-layout__primary-navigation'))
    }
  }
})

test('session expiration redirects back to login', async ({ page, request }) => {
  await resetBackend(request, true)

  await login(page)
  await request.post(`${backendUrl}/__test/session-expire`)

  await expect(page.getByRole('heading', { name: '登录' })).toBeVisible()
})
