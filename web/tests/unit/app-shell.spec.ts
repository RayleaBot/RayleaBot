import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import BasicLayout from '@/layouts/BasicLayout.vue'
import RouteView from '@/layouts/RouteView.vue'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore } from '@/stores/system'
import { useUiShellStore } from '@/stores/ui-shell'

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
              path: 'plugins',
              name: 'plugins',
              component: { template: '<div>插件页</div>' },
              meta: { icon: 'appstore', keepAlive: true, title: '插件' },
            },
            {
              path: 'plugins/:id',
              name: 'plugin-detail',
              component: { template: '<div>插件详情页</div>' },
              meta: { activePath: '/plugins', hideInMenu: true, title: '插件详情' },
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'commands' },
              meta: { hideInTab: true, title: '运维' },
              children: [
                {
                  path: '/commands',
                  name: 'commands',
                  component: { template: '<div>指令中心页</div>' },
                  meta: { icon: 'commands', keepAlive: true, title: '指令中心', viewKey: 'commands' },
                },
                {
                  path: '/tasks',
                  name: 'tasks',
                  component: { template: '<div>任务页</div>' },
                  meta: { icon: 'tasks', keepAlive: true, title: '任务', viewKey: 'tasks' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'logs' },
              meta: { hideInTab: true, title: '日志中心' },
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
              meta: { hideInTab: true, title: '协议' },
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
                  meta: { icon: 'protocols', keepAlive: true, title: '兼容矩阵' },
                },
              ],
            },
            {
              path: '',
              component: RouteView,
              redirect: { name: 'config' },
              meta: { hideInTab: true, title: '系统' },
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
                  component: { template: '<div>模板编辑页</div>' },
                  meta: {
                    activePath: '/render/templates',
                    entryPath: '/render/templates',
                    icon: 'render-templates',
                    keepAlive: true,
                    title: '模板编辑',
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
      adapter_state: 'connected',
      active_plugins: 1,
      uptime_seconds: 12,
    }
    systemStore.readiness = {
      status: 'ready',
    }

    const socketStore = useSocketStore()
    socketStore.snapshots.events.status = 'authenticated'
    socketStore.snapshots.tasks.status = 'authenticated'
    socketStore.snapshots.logs.status = 'authenticated'
    socketStore.snapshots.pluginConsole.status = 'disconnected'

    const uiShellStore = useUiShellStore()
    uiShellStore.setThemeMode('light')

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
        plugins: [Antd, router],
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
    const labels = Array.from(document.body.querySelectorAll('.admin-layout__tabbar .ant-tabs-tab-btn'))
      .map((node) => node.textContent?.trim() ?? '')
      .filter(Boolean)

    return Array.from(new Set(labels))
  }

  function getActiveTabLabel() {
    return document.body.querySelector('.admin-layout__tabbar .ant-tabs-tab-active .ant-tabs-tab-btn')
      ?.textContent
      ?.trim() ?? ''
  }

  function getTabIconKeys() {
    return Array.from(document.body.querySelectorAll<HTMLElement>('.admin-layout__tabbar .admin-layout__tab-label'))
      .map((node) => node.dataset.icon ?? '')
      .filter(Boolean)
  }

  beforeEach(() => {
    window.localStorage.clear()
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('renders a compact shell header with theme-aware sider styling', async () => {
    const { wrapper } = await mountShell('/')
    const headerLeft = wrapper.get('.admin-layout__header-left')
    const breadcrumb = wrapper.get('[data-testid="header-breadcrumb"]')
    const firstHeaderButton = headerLeft.find('button')
    const currentBreadcrumb = breadcrumb.get('.admin-layout__breadcrumb-current')

    expect(wrapper.get('[data-testid="app-sider"]').classes()).toContain('ant-layout-sider-light')
    expect(wrapper.get('[data-testid="theme-toggle"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="header-search"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="app-header"]').text()).not.toContain('事件流')
    expect(wrapper.get('[data-testid="app-header"]').text()).not.toContain('保持正式契约')
    expect(wrapper.find('.admin-layout__breadcrumb-row').exists()).toBe(false)
    expect(headerLeft.element.contains(breadcrumb.element)).toBe(true)
    expect(
      firstHeaderButton.element.compareDocumentPosition(breadcrumb.element) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).not.toBe(0)
    expect(firstHeaderButton.classes()).toContain('admin-layout__nav-trigger')
    expect(breadcrumb.classes()).toContain('admin-layout__header-breadcrumb--single')
    expect(breadcrumb.find('.admin-layout__breadcrumb-item--ancestor').exists()).toBe(false)
    expect(breadcrumb.find('.admin-layout__breadcrumb-link').exists()).toBe(false)
    expect(currentBreadcrumb.text()).toBe('系统状态')
    expect(wrapper.text()).toContain('系统状态')
    expect(wrapper.text()).toContain('协议中心')
    expect(wrapper.text()).toContain('指令中心')
    expect(wrapper.text()).toContain('运维')
    expect(wrapper.text()).toContain('日志中心')
    expect(wrapper.text()).toContain('协议')
    expect(wrapper.text()).toContain('系统')
    expect(wrapper.text()).toContain('模板编辑')
  })

  it('creates leaf tabs for grouped pages and keeps the active tab in sync', async () => {
    const { router, uiShellStore } = await mountShell('/')

    expect(getTabLabels()).toEqual(['系统状态'])
    expect(getTabIconKeys()).toEqual(['dashboard'])

    await router.push('/commands')
    await flushPromises()
    expect(uiShellStore.tabs.map((item) => ({ title: item.title, icon: item.icon }))).toEqual([
      { title: '系统状态', icon: 'dashboard' },
      { title: '指令中心', icon: 'commands' },
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'commands'])
    expect(getActiveTabLabel()).toBe('指令中心')

    await router.push('/tasks')
    await flushPromises()
    expect(uiShellStore.tabs.map((item) => ({ title: item.title, icon: item.icon }))).toEqual([
      { title: '系统状态', icon: 'dashboard' },
      { title: '指令中心', icon: 'commands' },
      { title: '任务', icon: 'tasks' },
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心', '任务'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'commands', 'tasks'])
    expect(getActiveTabLabel()).toBe('任务')

    await router.push('/logs')
    await flushPromises()
    expect(uiShellStore.tabs.map((item) => ({ title: item.title, icon: item.icon }))).toEqual([
      { title: '系统状态', icon: 'dashboard' },
      { title: '指令中心', icon: 'commands' },
      { title: '任务', icon: 'tasks' },
      { title: '实时日志', icon: 'logs' },
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心', '任务', '实时日志'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'commands', 'tasks', 'logs'])
    expect(getActiveTabLabel()).toBe('实时日志')
    expect(uiShellStore.tabs.map((item) => item.title)).not.toContain('运维')

    await router.push('/logs/history')
    await flushPromises()
    expect(uiShellStore.tabs.map((item) => ({ title: item.title, icon: item.icon }))).toEqual([
      { title: '系统状态', icon: 'dashboard' },
      { title: '指令中心', icon: 'commands' },
      { title: '任务', icon: 'tasks' },
      { title: '实时日志', icon: 'logs' },
      { title: '历史日志', icon: 'history-logs' },
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心', '任务', '实时日志', '历史日志'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'commands', 'tasks', 'logs', 'history-logs'])
    expect(getActiveTabLabel()).toBe('历史日志')

    await router.push('/render/templates/help.menu')
    await flushPromises()
    expect(uiShellStore.tabs.map((item) => ({ title: item.title, icon: item.icon, path: item.path, fullPath: item.fullPath }))).toEqual([
      { title: '系统状态', icon: 'dashboard', path: '/', fullPath: '/' },
      { title: '指令中心', icon: 'commands', path: '/commands', fullPath: '/commands' },
      { title: '任务', icon: 'tasks', path: '/tasks', fullPath: '/tasks' },
      { title: '实时日志', icon: 'logs', path: '/logs', fullPath: '/logs' },
      { title: '历史日志', icon: 'history-logs', path: '/logs/history', fullPath: '/logs/history' },
      { title: '模板编辑', icon: 'render-templates', path: '/render/templates', fullPath: '/render/templates/help.menu' },
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心', '任务', '实时日志', '历史日志', '模板编辑'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'commands', 'tasks', 'logs', 'history-logs', 'render-templates'])
    expect(getActiveTabLabel()).toBe('模板编辑')

    await router.push('/render/templates/status.panel')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'render-templates')).toEqual([
      expect.objectContaining({
        fullPath: '/render/templates/status.panel',
        path: '/render/templates',
        title: '模板编辑',
      }),
    ])
    expect(getTabLabels()).toEqual(['系统状态', '指令中心', '任务', '实时日志', '历史日志', '模板编辑'])
    expect(getActiveTabLabel()).toBe('模板编辑')
  })

  it('renders full breadcrumbs with a clickable parent group', async () => {
    const { wrapper } = await mountShell('/tasks')

    const breadcrumb = wrapper.get('[data-testid="header-breadcrumb"]')
    const parentItem = breadcrumb.get('.admin-layout__breadcrumb-item')
    const parentOuter = parentItem.get('.ant-breadcrumb-link')
    const parentLink = parentItem.get('.admin-layout__breadcrumb-link')
    const parentText = parentItem.get('.admin-layout__breadcrumb-link-text')
    const currentItem = breadcrumb.get('.admin-layout__breadcrumb-item--current')
    const currentOuter = currentItem.get('.ant-breadcrumb-link')
    const current = breadcrumb.get('.admin-layout__breadcrumb-current')
    const currentText = currentItem.get('.admin-layout__breadcrumb-current-text')

    expect(parentItem.classes()).toContain('admin-layout__breadcrumb-item')
    expect(parentItem.classes()).toContain('admin-layout__breadcrumb-item--ancestor')
    expect(parentOuter.exists()).toBe(true)
    expect(parentLink.text()).toBe('运维')
    expect(parentLink.attributes('href')).toBe('/commands')
    expect(parentLink.classes()).toContain('admin-layout__breadcrumb-link')
    expect(parentText.text()).toBe('运维')
    expect(breadcrumb.classes()).toContain('admin-layout__header-breadcrumb--multi')
    expect(currentItem.classes()).toContain('admin-layout__breadcrumb-item--current')
    expect(currentOuter.exists()).toBe(true)
    expect(current.text()).toBe('任务')
    expect(current.classes()).toContain('admin-layout__breadcrumb-current')
    expect(currentText.text()).toBe('任务')
    expect(wrapper.find('.admin-layout__breadcrumb-row').exists()).toBe(false)
  })

  it('keeps a single workspace tab when only query state changes', async () => {
    const { router, uiShellStore } = await mountShell('/commands')

    await router.push('/commands?plugin_id=weather')
    await flushPromises()
    await router.push('/commands?plugin_id=help')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'commands')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('指令中心')

    await router.push('/logs?protocol=onebot11')
    await flushPromises()
    await router.push('/logs?protocol=onebot11&request_id=req_1&log_id=log_1')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'logs')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('实时日志')

    await router.push('/tasks?task_id=task_render_preview_0001')
    await flushPromises()
    await router.push('/tasks?task_id=task_render_preview_0002')
    await flushPromises()
    expect(uiShellStore.tabs.filter((item) => item.name === 'tasks')).toHaveLength(1)
    expect(getActiveTabLabel()).toBe('任务')
  })

  it('creates a closable detail tab for plugin pages', async () => {
    const { uiShellStore } = await mountShell('/plugins/weather')

    expect(uiShellStore.tabs).toEqual(expect.arrayContaining([
      expect.objectContaining({
        affix: false,
        icon: 'appstore',
        path: '/plugins/weather',
        title: 'weather',
      }),
    ]))
    expect(getTabLabels()).toEqual(['系统状态', 'weather'])
    expect(getTabIconKeys()).toEqual(['dashboard', 'appstore'])
    expect(getActiveTabLabel()).toBe('weather')
  })

  it('renders child menu icons for grouped pages', async () => {
    const { wrapper } = await mountShell('/commands')

    const menuGroups = wrapper.findAll('.admin-layout__sider .ant-menu-submenu')
    const operationsGroup = menuGroups.find((item) => item.text().includes('运维'))
    const logsGroup = menuGroups.find((item) => item.text().includes('日志中心'))
    const protocolGroup = menuGroups.find((item) => item.text().includes('协议'))

    expect(operationsGroup?.findAll('.admin-layout__menu-icon')).toHaveLength(3)
    expect(logsGroup?.findAll('.admin-layout__menu-icon')).toHaveLength(3)
    expect(protocolGroup?.findAll('.admin-layout__menu-icon')).toHaveLength(3)
    expect(wrapper.find('.admin-layout__sider .ant-menu-item-selected .admin-layout__menu-icon').exists()).toBe(true)
  })

  it('opens the preference drawer and applies shell settings', async () => {
    const { wrapper, uiShellStore } = await mountShell('/')

    await wrapper.get('[data-testid="header-settings"]').trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('偏好设置')
    expect(document.body.textContent).toContain('外观')
    expect(document.body.textContent).toContain('布局')
    expect(document.body.textContent).toContain('快捷键')
    expect(document.body.textContent).toContain('通用')

    const darkOption = Array.from(document.body.querySelectorAll('.ant-segmented-item')).find(
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
    pluginItem?.click()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/plugins')
    expect(uiShellStore.searchOpen).toBe(false)
  })

  it('uses the stable template editor entry path for menu and route search', async () => {
    const { wrapper, router, uiShellStore } = await mountShell('/')

    const systemGroup = wrapper.findAll('.admin-layout__sider .ant-menu-submenu').find((item) => item.text().includes('系统'))
    expect(systemGroup).toBeDefined()

    const templateMenuItem = systemGroup!.findAll('.ant-menu-item').find((item) => item.text().includes('模板编辑'))
    expect(templateMenuItem).toBeDefined()
    await templateMenuItem!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/render/templates')

    await wrapper.get('[data-testid="header-search"]').trigger('click')
    await flushPromises()

    const input = document.body.querySelector<HTMLInputElement>('.route-search-panel input')
    expect(input).not.toBeNull()
    input!.value = '模板编辑'
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
