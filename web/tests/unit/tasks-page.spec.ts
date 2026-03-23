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
})
