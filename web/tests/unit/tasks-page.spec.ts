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
      summary: '渲染预览已完成',
      result: {
        summary: '渲染预览已生成',
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
    expect(wrapper.text()).toContain('渲染预览')
    expect(wrapper.text()).toContain('render.preview')
    expect(wrapper.text()).toContain('渲染预览已生成')
    expect(apiDownload).toHaveBeenCalledWith('/api/system/render/artifacts/render_preview_0001.png')
    const image = wrapper.find('img[alt="渲染预览结果"]')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toBe('blob:task-preview')
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
})
