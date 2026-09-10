import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'

import BasicLayout from '@/layouts/BasicLayout.vue'
import RouteView from '@/layouts/RouteView.vue'
import AppSidebarNavigation from '@/components/shell/AppSidebarNavigation.vue'
import { usePluginsStore } from '@/stores/plugins'
import { apiRequest } from '@/lib/http'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore } from '@/stores/system'
import { useUiShellStore } from '@/stores/ui-shell'
import type { PluginDetail } from '@/types/api'

vi.mock('@/lib/http', async importOriginal => ({ ...await importOriginal<typeof import('@/lib/http')>(), apiRequest: vi.fn() }))

describe('BasicLayout', () => {
  function createShellRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: BasicLayout,
          children: [
            {
              path: '',
              name: 'status',
              component: { template: '<div>系统状态页</div>' },
              meta: { affixTab: true, icon: 'dashboard', title: '系统状态' },
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'menu-center' },
              meta: { hideInTab: true, icon: 'features', order: 2, titleKey: 'routes.features' },
              children: [
                {
                  path: '/menu-center',
                  name: 'menu-center',
                  component: { template: '<div>菜单中心页</div>' },
                  meta: { icon: 'menu-center', keepAlive: true, order: 1, title: '菜单中心', viewKey: 'menu-center' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'plugins' },
              meta: { hideInTab: true, icon: 'plugin-store', order: 3, title: '插件中心' },
              children: [
                {
                  path: '/plugins',
                  name: 'plugins',
                  component: { template: '<div>插件列表页</div>' },
                  meta: { icon: 'plugins', keepAlive: true, order: 1, title: '插件列表' },
                },
                {
                  path: '/plugins/store',
                  name: 'plugin-store',
                  component: { template: '<div>插件商店页</div>' },
                  meta: { icon: 'plugin-store', keepAlive: true, title: '插件商店', viewKey: 'plugin-store' },
                },
                {
                  path: '/plugins/settings',
                  name: 'plugin-settings',
                  component: { data: () => ({ draft: '' }), template: '<div>插件设置页<input data-testid="settings-draft" v-model="draft" /></div>' },
                  meta: { icon: 'plugin-settings', keepAlive: true, order: 2, title: '全局插件设置', viewKey: 'plugin-settings' },
                },
                {
                  path: '/plugins/:id',
                  name: 'plugin-detail',
                  component: { template: '<div data-testid="plugin-detail-page">插件详情页</div>' },
                  meta: { activePath: '/plugins', hideInMenu: true, title: '插件详情' },
                },
                {
                  path: '/commands',
                  name: 'commands',
                  component: { template: '<div>指令中心页</div>' },
                  meta: { icon: 'commands', keepAlive: true, order: 3, title: '指令中心', viewKey: 'commands' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'permission-policy' },
              meta: { hideInTab: true, order: 4, title: '运维' },
              children: [
                {
                  path: '/permission-policy',
                  name: 'permission-policy',
                  component: { template: '<div>权限策略页</div>' },
                  meta: { icon: 'permission-policy', keepAlive: true, title: '权限策略', viewKey: 'permission-policy' },
                },
                {
                  path: '/access-lists',
                  name: 'access-lists',
                  component: { template: '<div>黑白名单页</div>' },
                  meta: { icon: 'access-lists', keepAlive: true, title: '黑白名单', viewKey: 'access-lists' },
                },
                {
                  path: '/rate-limits',
                  name: 'rate-limits',
                  component: { template: '<div>限流中心页</div>' },
                  meta: { icon: 'rate-limits', keepAlive: true, title: '限流中心', viewKey: 'rate-limits' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'logs' },
              meta: { hideInTab: true, order: 5, title: '日志中心' },
              children: [
                {
                  path: '/logs',
                  name: 'logs',
                  component: { template: '<div>实时日志页</div>' },
                  meta: { icon: 'logs', keepAlive: true, title: '实时日志', viewKey: 'logs' },
                },
                {
                  path: '/logs/history',
                  name: 'logs-history',
                  component: { template: '<div>历史日志页</div>' },
                  meta: { icon: 'history-logs', keepAlive: true, title: '历史日志', viewKey: 'logs-history' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'protocols' },
              meta: { hideInTab: true, order: 6, title: '协议' },
              children: [
                {
                  path: '/protocols',
                  name: 'protocols',
                  component: { template: '<div>协议中心页</div>' },
                  meta: { icon: 'protocols', keepAlive: true, title: '协议中心' },
                },
                {
                  path: '/protocols/compatibility',
                  name: 'protocols-compatibility',
                  component: { template: '<div>兼容矩阵页</div>' },
                  meta: { icon: 'protocol-compatibility', keepAlive: true, title: '兼容矩阵' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'config' },
              meta: { hideInTab: true, order: 7, title: '系统' },
              children: [
                {
                  path: '/config',
                  name: 'config',
                  component: { template: '<div>配置页</div>' },
                  meta: { icon: 'config', keepAlive: true, title: '配置' },
                },
                {
                  path: '/render/templates/:templateId?',
                  name: 'render-templates',
                  component: { template: '<div>模板预览页</div>' },
                  meta: {
                    activePath: '/render/templates',
                    entryPath: '/render/templates',
                    icon: 'render-templates',
                    keepAlive: true,
                    title: '模板预览',
                    viewKey: 'render-templates',
                  },
                },
              ],
            },
          ],
        },
      ],
    })
  }

  function seedShellStores() {
    const systemStore = useSystemStore()
    systemStore.system = {
      status: 'running',
      adapters: [{ id: 'onebot11', protocol: 'onebot11', enabled: true, state: 'connected' }],
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'ready',
    }

    const socketStore = useSocketStore()
    socketStore.snapshots.events.status = 'authenticated'
    socketStore.snapshots.logs.status = 'authenticated'
    socketStore.snapshots.pluginConsole.status = 'disconnected'

    const uiShellStore = useUiShellStore()
    uiShellStore.setThemeMode('light')
    usePluginsStore().listLoaded = true

    return {
      uiShellStore,
    }
  }

  async function mountShell(initialPath = '/') {
    const router = createShellRouter()
    await router.push(initialPath)
    await router.isReady()
    const stores = seedShellStores()

    const wrapper = mount(BasicLayout, {
      attachTo: document.body,
      global: {
        plugins: [getActivePinia()!, router],
      },
    })

    await flushPromises()

    return {
      router,
      wrapper,
      ...stores,
    }
  }

  function getTabLabels() {
    const labels = Array.from(document.body.querySelectorAll('.admin-layout__tabbar [role=tab]'))
      .map((node) => node.textContent?.trim() ?? '')
      .filter(Boolean)

    return Array.from(new Set(labels))
  }

  function getActiveTabLabel() {
    return document.body.querySelector('.admin-layout__tabbar [role=tab][data-state=active]')
      ?.textContent
      ?.trim() ?? ''
  }

  function getTabIconKeys() {
    return Array.from(document.body.querySelectorAll<HTMLElement>('.admin-layout__tabbar .admin-layout__tab-label'))
      .map((node) => node.dataset.icon ?? '')
      .filter(Boolean)
  }

  async function openStandardTabs(router: Router) {
    for (const path of ['/permission-policy', '/commands', '/logs']) {
      await router.push(path)
      await flushPromises()
    }
  }

  async function openTabContextMenu(tabTitle: string) {
    const target = Array.from(document.body.querySelectorAll<HTMLElement>('.admin-layout__tabbar .admin-layout__tab-label'))
      .find((node) => node.textContent?.includes(tabTitle))
    if (!target) {
      throw new Error(`tab not found: ${tabTitle}`)
    }

    target.dispatchEvent(new MouseEvent('contextmenu', {
      bubbles: true,
      button: 2,
      cancelable: true,
    }))
    await flushPromises()
  }

  function getContextMenuItem(label: string) {
    const item = Array.from(document.body.querySelectorAll<HTMLElement>('.app-menu-item'))
      .filter((node) => node.textContent?.trim() === label)
      .at(-1)
    if (!item) {
      throw new Error(`context menu item not found: ${label}`)
    }

    return item
  }

  async function clickContextMenuItem(label: string) {
    getContextMenuItem(label).dispatchEvent(new MouseEvent('click', {
      bubbles: true,
      cancelable: true,
    }))
    await flushPromises()
  }

  function isMenuItemDisabled(item: HTMLElement) {
    return item.getAttribute('aria-disabled') === 'true'
  }

  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      const url = new URL(path, 'http://fixture')
      const plugins = usePluginsStore()
      if (url.pathname.startsWith('/api/plugins/')) {
        const id = url.pathname.slice('/api/plugins/'.length)
        const detail = plugins.detailsByPluginId[id] ?? plugins.knownItems.find(item => item.id === id)
        return { plugin: { ...detail, permissions: {}, webhooks: [] } } as never
      }
      const all = plugins.knownItems
      const query = url.searchParams.get('query')?.toLowerCase() ?? ''
      const items = all.filter(plugin => `${plugin.id} ${plugin.name}`.toLowerCase().includes(query))
      return { items, total: items.length } as never
    })
  })

  it('combines the five plugin pages without merging other workspace tabs', async () => {
    const { router, uiShellStore } = await mountShell('/')
    await router.push('/permission-policy')
    await flushPromises()
    for (const path of ['/commands', '/menu-center', '/plugins/store', '/plugins/settings', '/plugins']) {
      await router.push(path)
      await flushPromises()
      expect(uiShellStore.tabs.map(item => item.path)).toEqual(['/', '/permission-policy', '/plugins'])
      expect(uiShellStore.tabs.at(-1)).toMatchObject({ name: 'plugin-center', fullPath: path, icon: 'plugins' })
      expect(getActiveTabLabel()).toBe('插件中心')
    }
    for (const path of ['/logs', '/logs/history', '/render/templates/help.menu', '/render/templates/status.panel']) {
      await router.push(path)
      await flushPromises()
    }
    expect(uiShellStore.tabs.map(item => item.path)).toEqual(['/', '/permission-policy', '/plugins', '/logs', '/logs/history', '/render/templates'])
    expect(uiShellStore.tabs.at(-1)?.fullPath).toBe('/render/templates/status.panel')
    expect(getActiveTabLabel()).toBe('模板预览')
  })

  it('renders full breadcrumbs with a clickable parent group', async () => {
    const { wrapper } = await mountShell('/permission-policy')

    const breadcrumb = wrapper.get('[data-testid="header-breadcrumb"]')
    const parentItem = breadcrumb.get('.admin-layout__breadcrumb-item')
    const parentLink = parentItem.get('.admin-layout__breadcrumb-link')
    const currentItem = breadcrumb.get('.admin-layout__breadcrumb-item--current')
    const current = breadcrumb.get('.admin-layout__breadcrumb-current')

    expect(parentLink.text()).toBe('运维')
    expect(parentLink.attributes('href')).toBe('/permission-policy')
    expect(current.text()).toBe('权限策略')
  })

  it('drills into the plugin center and switches all five workspace routes', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/plugins')
    uiShellStore.patchPreferences({ pageTransition: 'none' })
    const sidebar = wrapper.get('.admin-layout__sider')
    const destinations = [
      ['plugins', '/plugins'],
      ['plugin-store', '/plugins/store'],
      ['plugin-settings', '/plugins/settings'],
      ['menu-center', '/menu-center'],
      ['commands', '/commands'],
    ] as const
    expect(sidebar.get('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(true)
    expect(sidebar.findAll('[data-sidebar-page]').map(item => item.attributes('data-sidebar-page'))).toEqual(destinations.map(([name]) => name))
    for (const [name, path] of destinations) {
      await sidebar.get(`[data-sidebar-page="${name}"]`).trigger('click')
      await flushPromises()
      expect(router.currentRoute.value.path).toBe(path)
      expect(sidebar.get(`[data-sidebar-page="${name}"]`).attributes('aria-current')).toBe('page')
      expect(uiShellStore.tabs.map(tab => tab.path)).toEqual(['/', '/plugins'])
    }

    await sidebar.get('[data-sidebar-scope-back="plugin-center"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/commands')
    expect(sidebar.get('[data-sidebar-entry="plugin-center"]').exists()).toBe(true)
    await sidebar.get('[data-sidebar-entry="plugin-center"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/commands')
    expect(sidebar.get('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(true)
  })

  it('keeps plugin pages inline, restores open workspaces, and returns to root in one step', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/plugins')
    uiShellStore.patchPreferences({ pageTransition: 'none' })
    const pluginsStore = usePluginsStore()
    const detail = {
      id: 'example-config-panel',
      name: 'Example Config Panel',
      role: 'community',
      state: 'running',
      management_ui: {
        entry: 'ui/index.html',
        pages: [
          { id: 'config', label: '配置页面' },
          { id: 'secrets', label: '密钥设置' },
        ],
      },
      commands: [],
      command_groups: [],
      help: {},
      command_conflicts: [],
      permissions: {},
      webhooks: [],
    } as PluginDetail
    const weatherDetail = {
      id: 'weather',
      name: 'Weather',
      role: 'community',
      state: 'disabled',
      management_ui: {
        entry: 'ui/index.html',
        pages: [{ id: 'settings', label: '天气设置' }],
      },
      commands: [],
      command_groups: [],
      help: {},
      command_conflicts: [],
      permissions: {},
      webhooks: [],
    } as PluginDetail
    vi.mocked(apiRequest).mockImplementation(async path => {
      const url = new URL(path, 'http://fixture')
      if (url.pathname === '/api/plugins') return { items: [detail, weatherDetail], total: 2 } as never
      return { plugin: url.pathname.endsWith('/weather') ? weatherDetail : detail } as never
    })
    pluginsStore.current = detail
    pluginsStore.detailsByPluginId = {
      'example-config-panel': detail,
      weather: weatherDetail,
    }
    pluginsStore.upsert(detail)
    pluginsStore.upsert(weatherDetail)
    await flushPromises()

    const sidebar = wrapper.get('.admin-layout__sider')
    expect(sidebar.findAll('[data-sidebar-plugin-id]').map(item => item.attributes('data-sidebar-plugin-id'))).toEqual([
      'example-config-panel',
      'weather',
    ])
    await sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel')
    expect(sidebar.get('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(true)
    expect(sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').classes()).toContain('sidebar-navigation__plugin-resource--active')
    expect(sidebar.get('[data-sidebar-plugin-overview="example-config-panel"]').attributes('aria-current')).toBe('page')

    await sidebar.get('[data-sidebar-management-page="secrets"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel?panel=management-ui&management_page=secrets')
    expect(sidebar.get('[data-sidebar-management-page="secrets"]').attributes('aria-current')).toBe('page')

    await pluginsStore.fetchList({})
    await flushPromises()
    await sidebar.get('[data-sidebar-plugin-disclosure="weather"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel?panel=management-ui&management_page=secrets')
    expect(sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').attributes('aria-expanded')).toBe('true')
    expect(sidebar.get('[data-sidebar-plugin-id="weather"]').attributes('aria-expanded')).toBe('true')
    expect(sidebar.get('[data-sidebar-plugin-overview="weather"]').exists()).toBe(true)

    await sidebar.get('[data-sidebar-scope-back="plugin-center"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel?panel=management-ui&management_page=secrets')
    expect(sidebar.get('[data-sidebar-entry="plugin-center"]').exists()).toBe(true)

    await router.replace('/plugins/example-config-panel?panel=management-ui&management_page=config')
    await flushPromises()
    expect(sidebar.find('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(false)
    await sidebar.get('[data-sidebar-entry="plugin-center"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel?panel=management-ui&management_page=config')
    expect(sidebar.get('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(true)

    await sidebar.get('[data-sidebar-plugin-id="weather"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/weather')
    expect(sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').attributes('aria-expanded')).toBe('true')
    expect(sidebar.get('[data-sidebar-plugin-id="weather"]').attributes('aria-expanded')).toBe('true')
    await sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel?panel=management-ui&management_page=config')

    uiShellStore.removeTab('/plugins/example-config-panel')
    await sidebar.get('[data-sidebar-plugin-id="weather"]').trigger('click')
    await flushPromises()
    await sidebar.get('[data-sidebar-plugin-id="example-config-panel"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/example-config-panel')
  })

  it('keeps the plugin-center flyout entry in the root menu when the desktop sidebar is collapsed', async () => {
    const { wrapper, uiShellStore } = await mountShell('/plugins/weather?panel=overview')

    uiShellStore.toggleSider()
    await flushPromises()

    const sidebar = wrapper.get('.admin-layout__sider')
    expect(sidebar.find('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(false)
    const pluginCenterTrigger = sidebar.get('[data-sidebar-entry="plugin-center"]')
    expect(pluginCenterTrigger.attributes('aria-label')).toBe('插件中心')
    expect(wrapper.findAllComponents(AppSidebarNavigation)[0]?.props('openPluginTargets')).toEqual([
      { fullPath: '/plugins/weather?panel=overview', pluginId: 'weather' },
    ])
  })

  it('keeps the mobile drawer open when navigating back through sidebar levels', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/plugins')
    uiShellStore.setMobileMenuOpen(true)
    await flushPromises()

    const mobileCenter = document.body.querySelector<HTMLElement>('[data-mobile="true"][data-scope="plugin-center"]')
    expect(mobileCenter).not.toBeNull()
    mobileCenter?.querySelector<HTMLElement>('[data-sidebar-scope-back="plugin-center"]')?.click()
    await flushPromises()

    expect(uiShellStore.mobileMenuOpen).toBe(true)
    const mobileRoot = document.body.querySelector<HTMLElement>('[data-mobile="true"][data-scope="root"]')
    expect(mobileRoot).not.toBeNull()
    mobileRoot?.querySelector<HTMLElement>('[data-sidebar-entry="plugin-center"]')?.click()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/plugins')
    expect(uiShellStore.mobileMenuOpen).toBe(true)
    const reopenedCenter = document.body.querySelector<HTMLElement>('[data-mobile="true"][data-scope="plugin-center"]')
    reopenedCenter?.querySelector<HTMLElement>('[data-sidebar-page="plugins"]')?.click()
    await flushPromises()
    expect(uiShellStore.mobileMenuOpen).toBe(false)
  })

  it('searches installed plugins remotely and leaves retry under explicit user control', async () => {
    const pluginsStore = usePluginsStore()
    for (let index = 0; index < 8; index += 1) {
      pluginsStore.upsert({ id: `plugin-${index}`, name: index === 7 ? 'Weather Tools' : `Plugin ${index}`, state: 'disabled' })
    }
    const { wrapper } = await mountShell('/plugins')
    const sidebar = wrapper.get('.admin-layout__sider')
    const filter = sidebar.get('input[aria-label="筛选已安装插件"]')
    vi.mocked(apiRequest).mockRejectedValueOnce(new Error('network unavailable'))
    await filter.setValue('weather')
    await vi.waitFor(() => expect(sidebar.text()).toContain('插件列表加载失败'))
    const previousCalls = vi.mocked(apiRequest).mock.calls.length
    await sidebar.get('.sidebar-navigation__feedback button').trigger('click')
    await flushPromises()
    expect(apiRequest).toHaveBeenCalledTimes(previousCalls + 1)
    expect(sidebar.findAll('[data-sidebar-plugin-id]').map(item => item.attributes('data-sidebar-plugin-id'))).toEqual(['plugin-7'])
    expect(vi.mocked(apiRequest).mock.calls.at(-1)?.[0]).toContain('query=weather')
  })

  it('waits for exact plugin pages instead of rendering guessed skeleton rows', async () => {
    const { wrapper } = await mountShell('/plugins/missing-plugin')
    const pluginsStore = usePluginsStore()
    const sidebar = wrapper.get('.admin-layout__sider')

    pluginsStore.detailErrorsByPluginId = { 'missing-plugin': null }
    pluginsStore.detailLoadingByPluginId = { 'missing-plugin': true }
    await flushPromises()
    expect(sidebar.get('[data-sidebar-plugin-disclosure="missing-plugin"]').attributes('aria-busy')).toBe('true')
    expect(sidebar.get('[data-sidebar-plugin-id="missing-plugin"]').attributes('aria-expanded')).toBe('false')
    expect(sidebar.findAll('[data-sidebar-plugin-page-owner="missing-plugin"]')).toHaveLength(0)

    pluginsStore.detailsByPluginId = {
      'missing-plugin': {
        id: 'missing-plugin',
        name: 'Missing Plugin',
        role: 'community',
        state: 'disabled',
        commands: [],
        help: { groups: [] },
        management_ui: {
          entry: 'ui/index.html',
          pages: [
            { id: 'config', label: '配置页面' },
            { id: 'secrets', label: '密钥设置' },
          ],
        },
      },
    }
    pluginsStore.detailLoadingByPluginId = { 'missing-plugin': false }
    await flushPromises()
    expect(sidebar.get('[data-sidebar-plugin-id="missing-plugin"]').attributes('aria-expanded')).toBe('true')
    expect(sidebar.findAll('[data-sidebar-plugin-page-owner="missing-plugin"]')).toHaveLength(3)
    expect(sidebar.get('[data-sidebar-plugin-overview="missing-plugin"]').text()).toBe('概览')
    expect(sidebar.findAll('[data-sidebar-management-page]').map(item => item.text())).toEqual(['配置页面', '密钥设置'])
  })

  it('keeps overview and retry available after plugin page loading fails', async () => {
    const { wrapper } = await mountShell('/plugins/missing-plugin')
    const pluginsStore = usePluginsStore()
    const ensureDetailSpy = vi.spyOn(pluginsStore, 'ensureDetail').mockResolvedValue(undefined)
    const sidebar = wrapper.get('.admin-layout__sider')

    pluginsStore.detailErrorsByPluginId = { 'missing-plugin': 'network unavailable' }
    pluginsStore.detailLoadingByPluginId = { 'missing-plugin': false }
    await flushPromises()
    expect(sidebar.text()).toContain('插件页面暂不可用')
    expect(sidebar.get('[data-sidebar-plugin-overview="missing-plugin"]').text()).toBe('概览')
    await sidebar.get('[data-sidebar-plugin-retry="missing-plugin"]').trigger('click')
    expect(ensureDetailSpy).toHaveBeenCalledWith('missing-plugin', { refresh: true })
  })

  it('preserves a settings draft and returns to the last full URL through the merged tab', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/plugins/settings')
    uiShellStore.patchPreferences({ pageTransition: 'none' })
    await wrapper.get('[data-testid="settings-draft"]').setValue('fixture draft')
    await wrapper.get('[data-sidebar-page="commands"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-sidebar-page="plugin-settings"]').trigger('click')
    await flushPromises()
    expect((wrapper.get('[data-testid="settings-draft"]').element as HTMLInputElement).value).toBe('fixture draft')
    await router.push('/plugins/settings?section=runtime#limits')
    await flushPromises()
    await router.push('/logs')
    await flushPromises()
    expect(wrapper.find('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(false)
    await wrapper.get('[data-tab-path="/plugins"]').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(router.currentRoute.value.fullPath).toBe('/plugins/settings?section=runtime#limits')
    expect((wrapper.get('[data-testid="settings-draft"]').element as HTMLInputElement).value).toBe('fixture draft')
  })

  it('closing the center keeps an active plugin detail separate', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/plugins')
    await router.push('/plugins/weather?panel=overview')
    await flushPromises()
    expect(wrapper.get('[data-testid="plugin-center-sidebar-navigation"]').exists()).toBe(true)
    expect(wrapper.get('[data-sidebar-plugin-overview]').attributes('aria-current')).toBe('page')
    await openTabContextMenu('插件中心')
    await clickContextMenuItem('关闭当前标签')
    expect(router.currentRoute.value.fullPath).toBe('/plugins/weather?panel=overview')
    expect(uiShellStore.tabs.map(tab => tab.path)).toEqual(['/', '/plugins/weather'])
    expect(uiShellStore.effectiveCachedViewNames).not.toContain('plugin-settings')
  })

  it('renders plugin settings under the plugin center group', async () => {
    const { wrapper } = await mountShell('/plugins/settings')

    const breadcrumb = wrapper.get('[data-testid="header-breadcrumb"]')
    const parentLink = breadcrumb.get('.admin-layout__breadcrumb-link')

    expect(parentLink.text()).toBe('插件中心')
    expect(parentLink.attributes('href')).toBe('/plugins')
    expect(breadcrumb.get('.admin-layout__breadcrumb-current').text()).toBe('全局插件设置')
    expect(getTabLabels()).toEqual(['系统状态', '插件中心'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'plugins'])
    expect(getActiveTabLabel()).toBe('插件中心')
  })

  it('uses current route metadata instead of a stale persisted tab icon', async () => {
    const { uiShellStore } = await mountShell('/plugins/settings')
    const settingsTab = uiShellStore.tabs.find((item) => item.path === '/plugins')

    expect(settingsTab).toBeDefined()
    uiShellStore.upsertTab({
      ...settingsTab!,
      icon: 'setting',
    })
    await flushPromises()

    expect(uiShellStore.tabs.find((item) => item.path === '/plugins')?.icon).toBe('setting')
    expect(getTabIconKeys()).toEqual(['dashboard', 'plugins'])
  })

  it('renders menu center inside the shared plugin center', async () => {
    const { wrapper } = await mountShell('/menu-center')

    const breadcrumb = wrapper.get('[data-testid="header-breadcrumb"]')
    const parentLink = breadcrumb.get('.admin-layout__breadcrumb-link')

    expect(parentLink.text()).toBe('插件中心')
    expect(parentLink.attributes('href')).toBe('/plugins')
    expect(breadcrumb.get('.admin-layout__breadcrumb-current').text()).toBe('菜单中心')
    expect(getTabLabels()).toEqual(['系统状态', '插件中心'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'plugins'])
    expect(getActiveTabLabel()).toBe('插件中心')
  })

  it('keeps a single workspace tab when only query state changes', async () => {
    const { router, uiShellStore } = await mountShell('/permission-policy')

    await router.push('/permission-policy')
    await flushPromises()
    await router.push('/permission-policy')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'permission-policy')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('权限策略')

    await router.push('/commands?plugin_id=weather')
    await flushPromises()
    await router.push('/commands?plugin_id=raylea.echo')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'plugin-center')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('插件中心')

    await router.push('/logs?protocol=onebot11')
    await flushPromises()
    await router.push('/logs?protocol=onebot11&request_id=req_1&log_id=log_1')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'logs')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('实时日志')

    await router.push('/logs/history?source=tasks')
    await flushPromises()
    await router.push('/logs/history?source=tasks&request_id=req_1')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'logs-history')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('历史日志')
  })

  it('closes the right-clicked tab without changing the active page when it remains open', async () => {
    const { router } = await mountShell('/')
    await openStandardTabs(router)

    await openTabContextMenu('插件中心')
    await clickContextMenuItem('关闭当前标签')

    expect(router.currentRoute.value.path).toBe('/logs')
    expect(getTabLabels()).toEqual(['系统状态', '权限策略', '实时日志'])
    expect(getActiveTabLabel()).toBe('实时日志')
  })

  it('closes other tabs from the right-clicked tab and activates that tab when needed', async () => {
    const { router } = await mountShell('/')
    await openStandardTabs(router)

    await openTabContextMenu('插件中心')
    await clickContextMenuItem('关闭其他标签')

    expect(router.currentRoute.value.path).toBe('/commands')
    expect(getTabLabels()).toEqual(['系统状态', '插件中心'])
    expect(getActiveTabLabel()).toBe('插件中心')
  })

  it('closes tabs to the left of the right-clicked tab', async () => {
    const { router } = await mountShell('/')
    await openStandardTabs(router)

    await openTabContextMenu('实时日志')
    await clickContextMenuItem('关闭左侧标签')

    expect(router.currentRoute.value.path).toBe('/logs')
    expect(getTabLabels()).toEqual(['系统状态', '实时日志'])
    expect(getActiveTabLabel()).toBe('实时日志')
  })

  it('closes tabs to the right of the right-clicked tab and falls back to it', async () => {
    const { router } = await mountShell('/')
    await openStandardTabs(router)

    await openTabContextMenu('插件中心')
    await clickContextMenuItem('关闭右侧标签')

    expect(router.currentRoute.value.path).toBe('/commands')
    expect(getTabLabels()).toEqual(['系统状态', '权限策略', '插件中心'])
    expect(getActiveTabLabel()).toBe('插件中心')
  })

  it('closes all non-affix tabs from the right-click menu', async () => {
    const { router, wrapper } = await mountShell('/')
    await openStandardTabs(router)

    await openTabContextMenu('插件中心')
    await clickContextMenuItem('关闭所有标签')

    expect(router.currentRoute.value.path).toBe('/')
    expect(getTabLabels()).toEqual(['系统状态'])
    expect(wrapper.find('.admin-layout__tabbar').exists()).toBe(true)
  })

  it('keeps the affix tab protected in the right-click menu', async () => {
    const { router } = await mountShell('/')
    await router.push('/commands')
    await flushPromises()

    await openTabContextMenu('系统状态')

    expect(isMenuItemDisabled(getContextMenuItem('关闭当前标签'))).toBe(true)
  })

  it('creates a closable detail tab for plugin pages', async () => {
    const { uiShellStore, wrapper } = await mountShell('/plugins/weather')

    expect(uiShellStore.tabs).toEqual(expect.arrayContaining([
      expect.objectContaining({
        affix: false,
        icon: 'plugins',
        path: '/plugins/weather',
        title: '插件：weather',
      }),
    ]))
    expect(getTabLabels()).toEqual(['系统状态', '插件：weather'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'plugin:weather'])
    expect(getActiveTabLabel()).toBe('插件：weather')

    const tabIcon = wrapper.get('.admin-layout__tab-label[data-tab-path="/plugins/weather"] .plugin-icon')
    expect(tabIcon.find('img').exists()).toBe(false)
    expect(tabIcon.find('.raylea-mark').exists()).toBe(true)

    const pluginsStore = usePluginsStore()
    pluginsStore.upsert({
      icon: 'assets/weather.svg',
      id: 'weather',
      name: 'Weather',
      state: 'running',
      version: '1.4.2',
    })
    await flushPromises()

    expect(uiShellStore.tabs.find((item) => item.path === '/plugins/weather')?.title).toBe('插件：Weather')
    expect(getActiveTabLabel()).toBe('插件：Weather')
    expect(tabIcon.get('img').attributes('src')).toBe('/api/plugins/weather/icon')

    await tabIcon.get('img').trigger('error')
    expect(tabIcon.find('img').exists()).toBe(false)
    expect(tabIcon.find('.raylea-mark').exists()).toBe(true)
  })

  it('keeps the same plugin detail page instance when only the panel query changes', async () => {
    const { router, uiShellStore, wrapper } = await mountShell('/plugins/weather?panel=overview')

    const initialNode = wrapper.get('[data-testid="plugin-detail-page"]').element

    await router.push('/plugins/weather?panel=management-ui')
    await flushPromises()

    expect(uiShellStore.tabs.filter((item) => item.name === 'plugin-detail')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('插件：weather')
    expect(wrapper.get('[data-testid="plugin-detail-page"]').element).toBe(initialNode)
  })

  it('opens the preference drawer and applies shell settings', async () => {
    const { wrapper, uiShellStore } = await mountShell('/')

    await wrapper.get('[data-testid="header-more"]').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    const settingsItem = Array.from(document.body.querySelectorAll<HTMLElement>('.app-menu-item')).find(
      (node) => node.textContent?.includes('设置'),
    )
    settingsItem?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    const darkOption = Array.from(document.body.querySelectorAll('[role=radio]')).find(
      (node) => node.textContent?.includes('暗色'),
    )
    darkOption?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    expect(uiShellStore.preferences.themeMode).toBe('dark')
  })

  it('opens the route search panel and navigates to the matched page', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/')

    await wrapper.get('[data-testid="header-search"]').trigger('click')
    await flushPromises()

    const input = document.body.querySelector<HTMLInputElement>('.route-search-panel input')
    expect(input).not.toBeNull()
    input!.value = '插件'
    input!.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const pluginItem = Array.from(document.body.querySelectorAll<HTMLButtonElement>('.route-search-panel__result')).find(
      (node) => node.textContent?.includes('/plugins'),
    )
    const pluginSettingsItem = Array.from(document.body.querySelectorAll<HTMLButtonElement>('.route-search-panel__result')).find(
      (node) => node.textContent?.includes('/plugins/settings'),
    )
    expect(pluginSettingsItem).toBeTruthy()
    pluginItem?.click()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/plugins')
    expect(uiShellStore.searchOpen).toBe(false)
  })

  it('uses the stable template preview entry path for menu and route search', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/')

    const systemGroup = wrapper.findAll('.admin-layout__sider .sidebar-navigation__group').find((item) => item.text().includes('系统'))
    expect(systemGroup).toBeDefined()

    const templateMenuItem = systemGroup!.findAll('button.sidebar-navigation__item').find((item) => item.text().includes('模板预览'))
    expect(templateMenuItem).toBeDefined()
    await templateMenuItem!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/render/templates')

    await wrapper.get('[data-testid="header-search"]').trigger('click')
    await flushPromises()

    const input = document.body.querySelector<HTMLInputElement>('.route-search-panel input')
    expect(input).not.toBeNull()
    input!.value = '模板预览'
    input!.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()

    const templateItem = Array.from(document.body.querySelectorAll<HTMLButtonElement>('.route-search-panel__result')).find(
      (node) => node.textContent?.includes('/render/templates'),
    )
    expect(templateItem?.textContent).not.toContain('/render/templates/:templateId?')
    templateItem?.click()
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/render/templates')
    expect(uiShellStore.searchOpen).toBe(false)
  })
})
