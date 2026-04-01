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
})
