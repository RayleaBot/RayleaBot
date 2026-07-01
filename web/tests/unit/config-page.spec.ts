import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import ConfigPage from '@/views/system/ConfigView.vue'
import { getConfigSections } from '@/lib/config-form'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createFixtureConfig(): ConfigDocument {
  return {
    schema_version: '2',
    server: { host: '127.0.0.1', port: 8080 },
    onebot: {
      reverse_ws: { enabled: false, url: '', access_token: '' },
      forward_ws: { enabled: false, url: '', access_token: '' },
      http_api: { enabled: false, url: '', access_token: '' },
      webhook: { enabled: false, url: '', access_token: '' },
    },
    database: { engine: 'sqlite', path: 'data/rayleabot.db' },
    command: { prefixes: ['/'] },
    builtin_features: {
      menu: {
        commands: ['help', '帮助'],
        prefixes: [],
      },
    },
    admin: {
      super_admins: [],
      session_ttl_days: 7,
      sliding_renewal: true,
      max_sessions: 3,
      login_fail_limit: 5,
      login_fail_window_seconds: 300,
    },
    permission: {
      default_level: 'everyone',
    },
    render: {
      worker_count: 1,
      browser_args: ['--disable-gpu'],
      browser_path: '',
      default_output: 'png',
      device_scale_percent: 100,
      timeout_seconds: 30,
      queue_wait_timeout_seconds: 15,
      queue_max_length: 32,
      footer_template: 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}',
    },
    scheduler: {
      timezone: '',
    },
    runtime: {
      plugin_init_timeout_seconds: 30,
      plugin_init_max_total_seconds: 300,
      plugin_event_timeout_seconds: 60,
      max_pending_events_per_plugin: 16,
      max_pending_control_events_per_plugin: 4,
      nodejs_max_old_space_size_mb: 256,
      dependency_install_timeout_seconds: 900,
      max_concurrent_dependency_installs: 1,
      ipc_pending_actions_max: 256,
      ipc_action_burst_limit: '100/1s',
      stderr_rate_limit_bytes_per_second: 262144,
      max_concurrent_tasks_per_plugin: 4,
      crash_backoff_initial_seconds: 2,
      crash_backoff_max_seconds: 60,
      shutdown_grace_seconds: 10,
      ipc_message_max_bytes: 8388608,
    },
    storage: { kv_value_max_bytes: 65536, kv_total_limit_mb: 16, file_max_bytes: 10485760, plugin_workdir_soft_limit_mb: 256 },
    data: {
      audit_logs_retention_days: 90,
      event_records_retention_days: 7,
      download_cache_retention_days: 15,
    },
    log: { level: 'info', retention_days: 7, rate_limit_per_plugin: '200/10s' },
    message: {
      rate_limit_per_plugin: '20/10s',
      rate_limit_per_target: '5/5s',
      circuit_breaker_seconds: 30,
    },
    user: {
      command_rate_limit: '10/60s',
      cooldown_reply: true,
    },
    group: {
      command_rate_limit: '30/60s',
    },
    adapter: {
      connect_timeout_seconds: 15,
      reconnect_initial_seconds: 2,
      reconnect_multiplier: 2,
      reconnect_max_seconds: 120,
      reconnect_jitter_ratio: 0.2,
    },
    http: { timeout_seconds: 10, max_retries: 2, allow_private_hosts: [] },
    web: { exposure_mode: 'localhost_only', setup_local_only: true },
    backup: { default_consistency: 'offline' },
  }
}

describe('ConfigPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(notifySuccess).mockClear()
    vi.mocked(useToastFeedback).mockClear()
  })

  it('submits the edited config document', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()
    store.redactedFields = []

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: store.redactedFields,
      restart_required: true,
      apply_effects: {
        applied_now: ['log.level'],
        reloaded_now: [],
        restart_required_fields: ['server.port'],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('监听地址')
    expect(wrapper.text()).toContain('服务监听')
    const hostInput = wrapper.find('#config-field-server-host')
    expect(hostInput.exists()).toBe(true)
    await hostInput.setValue('0.0.0.0')
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.host).toBe('0.0.0.0')
  })

  it('keeps protocol fields out of the general config page', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('.app-page').exists()).toBe(true)
    expect(wrapper.find('.config-page').exists()).toBe(true)
    expect(wrapper.find('.config-stack').exists()).toBe(true)
    expect(wrapper.find('.config-toc').exists()).toBe(true)
    expect(wrapper.find('.config-toolbar').exists()).toBe(true)
    expect(wrapper.findAll('.config-section').length).toBe(getConfigSections().length)
    expect(wrapper.find('.glass-panel').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('OneBot 连接')
    expect(wrapper.text()).not.toContain('适配器')
    expect(wrapper.text()).not.toContain('超级管理员')
    expect(wrapper.text()).not.toContain('默认权限级别')
    expect(wrapper.text()).not.toContain('用户命令速率限制')
    expect(wrapper.text()).not.toContain('群命令速率限制')
  })

  it('keeps cleared numeric fields empty instead of forcing them to 0', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['command.prefixes'],
        reloaded_now: [],
        restart_required_fields: [],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const portInput = wrapper.find('.config-field__number input')
    expect(portInput.exists()).toBe(true)
    await portInput.setValue('')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0].server.port).toBeUndefined()
  })

  it('edits image generation defaults from the render section', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['render.default_output', 'render.device_scale_percent'],
        reloaded_now: [],
        restart_required_fields: [],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const viewModel = wrapper.vm as unknown as {
      writeField: (path: string, value: unknown) => void
    }

    expect(wrapper.text()).toContain('默认生成格式')
    expect(wrapper.text()).toContain('图片精度')
    const renderFields = getConfigSections().find((section) => section.key === 'render')?.fields ?? []
    const precisionField = renderFields.find((field) => field.path === 'render.device_scale_percent')
    expect(precisionField?.min).toBe(50)
    expect(precisionField?.max).toBe(500)
    expect(precisionField?.unit).toBe('%')

    viewModel.writeField('render.default_output', 'jpeg')
    viewModel.writeField('render.device_scale_percent', 200)
    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.render.default_output).toBe('jpeg')
    expect(submitted.render.device_scale_percent).toBe(200)
  })

  it('keeps apply effect details out of the page-level banner area', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()
    store.applyEffects = {
      applied_now: ['log.level'],
      reloaded_now: ['onebot.forward_ws.url'],
      restart_required_fields: ['server.port'],
    }
    store.restartRequired = true

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('保存结果')
    expect(wrapper.text()).not.toContain('已即时生效')
    expect(wrapper.text()).not.toContain('已重载')
    expect(wrapper.text()).not.toContain('需重启生效')
    expect(wrapper.text()).not.toContain('log.level')
    expect(wrapper.text()).not.toContain('onebot.forward_ws.url')
    expect(wrapper.text()).not.toContain('server.port')
    expect(vi.mocked(useToastFeedback)).toHaveBeenCalled()
  })

  it('keeps plugin-facing settings out of the general config page', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('命令前缀')
    expect(wrapper.text()).not.toContain('插件日志速率限制')
    expect(wrapper.text()).not.toContain('插件消息速率限制')
    expect(wrapper.text()).not.toContain('插件工作目录软上限')
    expect(wrapper.text()).not.toContain('默认权限级别')
    expect(wrapper.text()).not.toContain('目标消息速率限制')
  })

  it('edits general IPC rate limit with split inputs', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockResolvedValue({
      config: store.document,
      redacted_fields: [],
      restart_required: false,
      apply_effects: {
        applied_now: ['runtime.ipc_action_burst_limit'],
        reloaded_now: [],
        restart_required_fields: [],
      },
    })

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const viewModel = wrapper.vm as unknown as {
      writeField: (path: string, value: unknown) => void
    }

    viewModel.writeField('runtime.ipc_action_burst_limit', '200/10s')
    await flushPromises()
    expect(wrapper.text()).not.toContain('目标消息速率限制')

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.runtime.ipc_action_burst_limit).toBe('200/10s')
    expect(submitted.message.rate_limit_per_target).toBe('5/5s')
  })

  it('reflects dirty state in the toolbar save button', async () => {
    const store = useConfigStore()
    store.document = createFixtureConfig()
    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(ConfigPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const saveButton = wrapper.findAll('button').find((candidate) => candidate.text().includes('保存更改'))
    expect(saveButton).toBeTruthy()
    expect((saveButton!.element as HTMLButtonElement).disabled).toBe(true)

    const viewModel = wrapper.vm as unknown as { writeField: (path: string, value: unknown) => void }
    viewModel.writeField('server.host', '0.0.0.0')
    await flushPromises()

    expect((saveButton!.element as HTMLButtonElement).disabled).toBe(false)
  })
})
