import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import DashboardPage from '@/pages/DashboardPage.vue'
import { useSystemStore } from '@/stores/system'

describe('DashboardPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('moves backup and diagnostics into the tools section', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()

    const store = useSystemStore()
    store.health = { status: 'ok' }
    store.readiness = { status: 'ready' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
    }

    vi.spyOn(store, 'refresh').mockResolvedValue(undefined)
    const createBackupSpy = vi.spyOn(store as never, 'createBackup').mockResolvedValue({ task_id: 'task_backup_create_0001' })
    const exportDiagnosticsSpy = vi.spyOn(store as never, 'exportDiagnostics').mockResolvedValue(undefined)

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    const backupButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('创建备份'))
    const diagnosticsButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('导出诊断包'))

    expect(backupButton).toBeTruthy()
    expect(diagnosticsButton).toBeTruthy()
    expect(wrapper.text()).toContain('系统工具')
    expect(wrapper.text()).not.toContain('聚合 health、ready、system status')

    await backupButton!.trigger('click')
    await diagnosticsButton!.trigger('click')

    expect(createBackupSpy).toHaveBeenCalledTimes(1)
    expect(exportDiagnosticsSpy).toHaveBeenCalledTimes(1)
  })

  it('submits render preview requests from the tools section', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()

    const store = useSystemStore()
    store.health = { status: 'ok' }
    store.readiness = { status: 'ready' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
    }

    vi.spyOn(store, 'refresh').mockResolvedValue(undefined)
    const previewSpy = vi.spyOn(store as never, 'previewRender').mockResolvedValue({ task_id: 'task_render_preview_0001' })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    const previewButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('渲染预览'))
    expect(previewButton).toBeTruthy()

    await previewButton!.trigger('click')
    await flushPromises()

    const templateInput = wrapper.find('input[placeholder="help.menu"]')
    await templateInput.setValue('help.menu')
    const submitButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('生成预览'))
    expect(submitButton).toBeTruthy()
    await submitButton!.trigger('click')

    expect(previewSpy).toHaveBeenCalledTimes(1)
  })

  it('renders readiness issues instead of legacy checks', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()

    const store = useSystemStore()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: 'OneBot authentication failed',
      reason_codes: ['adapter.auth_failed'],
      issues: [
        {
          code: 'adapter.auth_failed',
          severity: 'warning',
          summary: 'OneBot authentication failed',
          remediation: '请检查 OneBot access_token 配置后重试连接。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapter_state: 'auth_failed',
      active_plugins: 2,
      uptime_seconds: 120,
    }

    vi.spyOn(store, 'refresh').mockResolvedValue(undefined)

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('就绪检查')
    expect(wrapper.text()).toContain('adapter.auth_failed')
    expect(wrapper.text()).toContain('请检查 OneBot access_token 配置后重试连接。')
    expect(wrapper.text()).not.toContain('config = ok')
  })

  it('deduplicates readiness issue codes already represented by issue cards', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()

    const store = useSystemStore()
    store.health = { status: 'ok' }
    store.readiness = {
      status: 'degraded',
      reason: 'Render resources are incomplete',
      reason_codes: ['platform.resource_missing', 'platform.resource_missing'],
      issues: [
        {
          code: 'platform.resource_missing',
          severity: 'warning',
          summary: 'Chromium 资源尚未准备完成',
          remediation: '请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。',
        },
        {
          code: 'platform.resource_missing',
          severity: 'warning',
          summary: 'Chromium 资源尚未准备完成',
          remediation: '请先准备受控 Chromium 运行时，或在配置中显式设置 render.browser_path。',
        },
      ],
    }
    store.system = {
      status: 'running',
      adapter_state: 'idle',
      active_plugins: 0,
      uptime_seconds: 50,
    }

    vi.spyOn(store, 'refresh').mockResolvedValue(undefined)

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.findAll('.issues-list .issue-alert-card')).toHaveLength(1)
    expect(wrapper.text()).toContain('platform.resource_missing')
    expect((wrapper.text().match(/platform\.resource_missing/g) ?? []).length).toBe(1)
    expect(wrapper.text()).not.toContain('原因代码')
  })

  it('renders recovery summary as a dedicated dashboard block', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: DashboardPage }],
    })
    await router.push('/')
    await router.isReady()

    const store = useSystemStore()
    store.health = { status: 'ok' }
    store.readiness = { status: 'degraded' }
    store.system = {
      status: 'running',
      adapter_state: 'connected',
      active_plugins: 2,
      uptime_seconds: 120,
      recovery_summary: {
        status: 'degraded',
        phase: 'post_startup',
        operation: 'upgrade',
        created_at: '2026-04-02T08:00:00Z',
        updated_at: '2026-04-02T08:01:00Z',
        source_core_version: '0.1.0',
        target_core_version: '0.2.0',
        source_config_schema_version: '2',
        target_config_schema_version: '2',
        source_db_schema_version: '0014',
        target_db_schema_version: '0014',
        issues: [
          {
            code: 'recovery.plugin_min_core_version',
            severity: 'warning',
            summary: '插件 weather-pro 需要更高版本的 RayleaBot core。',
            remediation: '升级程序或安装与当前版本兼容的插件包后，再手动重新启用该插件。',
          },
        ],
        skipped_plugins: [
          {
            plugin_id: 'weather-pro',
            version: '1.4.0',
            reason_code: 'plugin.min_core_version',
            summary: '插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。',
            manual_action: '升级程序或重新安装兼容版本插件。',
          },
        ],
        manual_actions: ['处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。'],
        next_steps: ['查看恢复摘要中的跳过插件列表并完成兼容性处理。', '通过管理面、Launcher 或 diagnostics 复核 recovery_summary。'],
      },
    }

    vi.spyOn(store, 'refresh').mockResolvedValue(undefined)

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('恢复兼容性')
    expect(wrapper.text()).toContain('需要人工处理')
    expect(wrapper.text()).toContain('weather-pro')
    expect(wrapper.text()).toContain('处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。')
    expect(wrapper.text()).toContain('查看恢复摘要中的跳过插件列表并完成兼容性处理。')
    expect(wrapper.text()).toContain('通过管理面、Launcher 或 diagnostics 复核 recovery_summary。')
    expect(wrapper.findAll('[data-testid="recovery-manual-action"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="recovery-next-step"]')).toHaveLength(2)
  })
})
