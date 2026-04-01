import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import TasksPage from '@/pages/TasksPage.vue'
import { useTasksStore } from '@/stores/tasks'

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

    expect(wrapper.text()).toContain('render.preview')
    expect(wrapper.text()).toContain('渲染预览已生成')
    const image = wrapper.find('img[alt="render preview"]')
    expect(image.exists()).toBe(true)
    expect(image.attributes('src')).toContain('/api/system/render/artifacts/render_preview_0001.png')
  })
})
