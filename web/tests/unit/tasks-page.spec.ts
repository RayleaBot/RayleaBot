import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import TasksPage from '@/pages/TasksPage.vue'
import { apiDownload } from '@/lib/http'
import { useTasksStore } from '@/stores/tasks'

vi.mock('@/lib/http', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/http')>()),
  apiDownload: vi.fn(),
}))

describe('TasksPage', () => {
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
    window.URL.createObjectURL = vi.fn(() => 'blob:task-preview')
    window.URL.revokeObjectURL = vi.fn()
    vi.mocked(apiDownload).mockReset()
  })

  it('loads task detail from the route query', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/tasks', component: TasksPage }],
    })
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
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(fetchDetailSpy).toHaveBeenCalledWith('task_plugin_install_0001')
  })

  it('renders render preview task results including the preview image', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/tasks', component: TasksPage }],
    })
    await router.push('/tasks?task_id=task_render_preview_0001')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    vi.mocked(apiDownload).mockResolvedValue({
      blob: new Blob(['preview'], { type: 'image/png' }),
      filename: null,
    })
    vi.spyOn(store, 'fetchDetail').mockImplementation(async () => {
      store.currentTask = {
      task_id: 'task_render_preview_0001',
      task_type: 'render.preview',
      status: 'succeeded',
      progress: 100,
      summary: '图片预览已完成',
      result: {
        summary: '图片预览已生成',
        details: {
          image_url: '/api/system/render/artifacts/render_preview_0001.png',
          mime: 'image/png',
          template: 'help.menu',
        },
      },
      }
      return store.currentTask
    })

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('任务类型')
    expect(wrapper.text()).toContain('图片预览')
    expect(wrapper.text()).toContain('render.preview')
    expect(wrapper.text()).toContain('图片预览已生成')
    expect(apiDownload).toHaveBeenCalledWith('/api/system/render/artifacts/render_preview_0001.png')
    const image = wrapper.find('img[alt="图片预览结果"]')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe('blob:task-preview')
  })

  it('renders recovery summaries in task detail without dumping the raw recovery_summary payload', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/tasks', component: TasksPage },
        { path: '/plugins/:id', name: 'plugin-detail', component: { template: '<div>plugin</div>' } },
      ],
    })
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
                  plugin_id: 'legacy-plugin',
                  version: '1.0.0',
                  reason_code: 'plugin.platform_mismatch',
                  summary: '插件平台兼容性不满足，已保留安装目录并跳过自动启用。',
                  review_id: 'review_legacy_plugin',
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
                      review_id: 'review_legacy_plugin',
                      plugin_id: 'legacy-plugin',
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
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('恢复摘要')
    expect(wrapper.text()).toContain('confirmed_review_ids = ["review_weather_pro"]')
    expect(wrapper.text()).toContain('operator_id = alice')
    expect(wrapper.text()).not.toContain('recovery_summary =')
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_weather_pro"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_legacy_plugin"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="recovery-audit-entry"]')).toHaveLength(2)
    expect(wrapper.find('[data-testid="recovery-confirm-button"]').exists()).toBe(false)

    await wrapper.find('[data-testid="recovery-filter-confirmed"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="recovery-plugin-card-review_weather_pro"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="recovery-plugin-card-review_legacy_plugin"]').exists()).toBe(true)
  })

  it('renders the task list inside a compact desktop table', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/tasks', component: TasksPage }],
    })
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
      },
    ]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.tasks-data-table').exists()).toBe(true)
    expect(wrapper.find('.task-cell-identity').exists()).toBe(true)
    expect(wrapper.find('.task-cell-status').exists()).toBe(true)
    expect(wrapper.find('.task-cell-time').exists()).toBe(true)
    expect(wrapper.find('.task-summary-text').exists()).toBe(true)
    expect(wrapper.find('.task-summary-row').exists()).toBe(false)
    expect(wrapper.find('.desktop-table').exists()).toBe(false)
  })

  it('renders a compact empty state instead of an empty table header', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/tasks', component: TasksPage }],
    })
    await router.push('/tasks')
    await router.isReady()

    const store = useTasksStore()
    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)

    const wrapper = mount(TasksPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.tasks-empty-card').exists()).toBe(true)
    expect(wrapper.find('.tasks-data-table').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('任务类型')
  })
})
