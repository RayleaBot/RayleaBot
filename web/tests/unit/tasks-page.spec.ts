import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import TasksPage from '@/views/operations/TasksView.vue'
import { useTasksStore } from '@/stores/tasks'

describe('TasksPage', () => {
  function createTasksRouter() {
    return createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tasks', name: 'tasks', component: TasksPage },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
        { path: '/protocols', name: 'protocols', component: { template: '<div>protocols</div>' } },
        { path: '/logs/history', name: 'logs-history', component: { template: '<div>logs-history</div>' } },
        { path: '/render/templates/:templateId?', name: 'render-templates', component: { template: '<div>template</div>' } },
      ],
    })
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })) as typeof window.matchMedia
  })

  it('loads task detail from the route query', async () => {
    const router = createTasksRouter()
    await router.push('/tasks?task_id=task_plugin_install_0001')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const fetchDetailSpy = vi.spyOn(store, 'fetchDetail').mockResolvedValue({
      task_id: 'task_plugin_install_0001',
      task_type: 'plugin.install',
      status: 'running',
      progress: 55,
      summary: 'install weather',
    })

    mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(fetchDetailSpy).toHaveBeenCalledWith('task_plugin_install_0001')
  })

  it('loads the task list from the route task type filter', async () => {
    const router = createTasksRouter()
    await router.push('/tasks?task_type=recovery.recheck')
    await router.isReady()

    const store = useTasksStore()
    const fetchListSpy = vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(fetchListSpy).toHaveBeenCalledWith({ taskType: 'recovery.recheck' })
  })

  it('renders task results as structured details', async () => {
    const router = createTasksRouter()
    await router.push('/tasks?task_id=task_backup_create_0001')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(store, 'fetchDetail').mockImplementation(async () => {
      store.currentTask = {
        task_id: 'task_backup_create_0001',
        task_type: 'backup.create',
        status: 'succeeded',
        progress: 100,
        summary: '备份已完成',
        result: {
          summary: '备份报告已生成',
          details: {
            backup_id: 'backup_0001',
            artifact_path: 'backups/backup_0001.zip',
            size_bytes: 4096,
          },
        },
      }
      return store.currentTask
    })

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const bodyText = () => document.body.textContent ?? ''
    expect(bodyText()).toContain('任务类型')
    expect(bodyText()).toContain('创建备份')
    expect(bodyText()).toContain('backup.create')
    expect(bodyText()).toContain('备份报告已生成')
    expect(bodyText()).toContain('backup_id = backup_0001')
    expect(bodyText()).toContain('artifact_path = backups/backup_0001.zip')
    expect(bodyText()).toContain('size_bytes = 4096')
    wrapper.unmount()
  })

  it('renders recovery summaries in task detail without dumping the raw recovery_summary payload', async () => {
    const router = createTasksRouter()
    await router.push('/tasks?task_id=task_recovery_confirm_0001')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.spyOn(store, 'fetchDetail').mockImplementation(async () => {
      store.currentTask = {
        task_id: 'task_recovery_confirm_0001',
        task_type: 'recovery.confirm',
        status: 'succeeded',
        progress: 100,
        summary: '恢复项确认已完成',
        result: {
          summary: '所选恢复项已确认',
          details: {
            confirmed_review_ids: ['review_weather_pro'],
            operator_id: 'alice',
            note: '已确认当前跳过状态。',
            recovery_summary: {
              status: 'degraded',
              phase: 'post_startup',
              operation: 'upgrade',
              created_at: '2026-04-03T07:59:00Z',
              updated_at: '2026-04-03T08:00:03Z',
              source_core_version: '0.1.0',
              target_core_version: '0.2.0',
              skipped_plugins: [
                {
                  plugin_id: 'weather-pro',
                  version: '1.4.0',
                  reason_code: 'plugin.min_core_version',
                  summary: '插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。',
                  review_id: 'review_weather_pro',
                  review_status: 'pending',
                  manual_action: '升级程序或重新安装兼容版本插件。',
                },
                {
                  plugin_id: 'platform-mismatch-plugin',
                  version: '1.0.0',
                  reason_code: 'plugin.platform_mismatch',
                  summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
                  review_id: 'review_platform_mismatch_plugin',
                  review_status: 'confirmed',
                  reviewed_at: '2026-04-03T08:00:03Z',
                  reviewed_by: 'alice',
                  manual_action: '安装适用于当前平台的插件包。',
                },
              ],
              manual_actions: ['处理被跳过插件的兼容性问题后，再在管理面中手动重新启用。'],
              next_steps: ['查看恢复摘要中的跳过插件列表并完成兼容性处理。'],
              audit: [
                {
                  task_id: 'task_recovery_confirm_0001',
                  created_at: '2026-04-03T08:00:03Z',
                  operator_id: 'alice',
                  note: '已确认当前跳过状态。',
                  items: [
                    {
                      review_id: 'review_weather_pro',
                      plugin_id: 'weather-pro',
                      reason_code: 'plugin.min_core_version',
                      summary: '插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。',
                      version: '1.4.0',
                    },
                  ],
                },
                {
                  task_id: 'task_recovery_confirm_0002',
                  created_at: '2026-04-03T08:05:00Z',
                  operator_id: 'bob',
                  note: '已记录平台兼容性处理结论。',
                  items: [
                    {
                      review_id: 'review_platform_mismatch_plugin',
                      plugin_id: 'platform-mismatch-plugin',
                      reason_code: 'plugin.platform_mismatch',
                      summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
                      version: '1.0.0',
                    },
                  ],
                },
              ],
            },
          },
        },
      }
      return store.currentTask
    })

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const bodyText = () => document.body.textContent ?? ''
    expect(bodyText()).toContain('恢复摘要')
    expect(bodyText()).toContain('confirmed_review_ids = ["review_weather_pro"]')
    expect(bodyText()).toContain('operator_id = alice')
    expect(bodyText()).not.toContain('recovery_summary =')
    expect(document.body.querySelector('[data-testid="recovery-plugin-card-review_weather_pro"]')).not.toBeNull()
    expect(document.body.querySelector('[data-testid="recovery-plugin-card-review_platform_mismatch_plugin"]')).toBeNull()
    expect(document.body.querySelectorAll('[data-testid="recovery-audit-entry"]')).toHaveLength(2)
    expect(document.body.querySelector('[data-testid="recovery-confirm-button"]')).toBeNull()

    const filterButton = document.body.querySelector('[data-testid="recovery-filter-confirmed"]') as HTMLElement | null
    expect(filterButton).not.toBeNull()
    filterButton!.click()
    await flushPromises()

    expect(document.body.querySelector('[data-testid="recovery-plugin-card-review_weather_pro"]')).toBeNull()
    expect(document.body.querySelector('[data-testid="recovery-plugin-card-review_platform_mismatch_plugin"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('renders the task list inside a compact desktop table', async () => {
    const router = createTasksRouter()
    await router.push('/tasks')
    await router.isReady()

    const store = useTasksStore()
    store.items = [
      {
        task_id: 'task_backup_create_0001',
        task_type: 'backup.create',
        status: 'running',
        progress: 40,
        summary: '正在创建备份',
        started_at: '2026-04-01T15:20:00Z',
        finished_at: '2026-04-01T15:23:00Z',
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.tasks-data-table').exists()).toBe(true)
    expect(wrapper.find('.task-cell-identity').exists()).toBe(true)
    expect(wrapper.find('.task-cell-status').exists()).toBe(true)
    expect(wrapper.find('.task-cell-time').exists()).toBe(true)
    expect(wrapper.find('.task-summary-text').exists()).toBe(true)
    expect(wrapper.text()).toContain('结束时间')
    expect(wrapper.text()).toContain('2026')
    expect(wrapper.find('.task-summary-row').exists()).toBe(false)
    expect(wrapper.find('.desktop-table').exists()).toBe(false)
  })

  it('renders a compact empty state instead of an empty table header', async () => {
    const router = createTasksRouter()
    await router.push('/tasks')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.app-empty-state').exists()).toBe(true)
    expect(wrapper.find('.tasks-data-table').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('任务类型')
  })
})
