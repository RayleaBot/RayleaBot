import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useTasksStore } from '@/stores/tasks'
import type { TaskSummary } from '@/types/api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function runtimeBootstrapRunningTask(): TaskSummary {
  return {
    task_id: 'task_runtime_bootstrap_0001',
    task_type: 'runtime.bootstrap',
    status: 'running',
    progress: 90,
    summary: '准备运行环境',
    started_at: '2026-04-01T15:21:43Z',
  }
}

function runtimeBootstrapSucceededTask(): TaskSummary {
  return {
    task_id: 'task_runtime_bootstrap_0001',
    task_type: 'runtime.bootstrap',
    status: 'succeeded',
    progress: 100,
    summary: '运行环境已准备',
    started_at: '2026-04-01T15:21:43Z',
    finished_at: '2026-04-01T15:21:44Z',
    result: {
      summary: '运行环境已准备',
      details: {
        resource_count: 2,
      },
    },
  }
}

describe('tasks store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps a newer terminal snapshot when a stale task list response resolves later', async () => {
    let resolveFetch: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise((resolve) => {
      resolveFetch = resolve
    })))

    const store = useTasksStore()
    const pending = store.fetchList()

    store.upsert(runtimeBootstrapSucceededTask())
    resolveFetch?.(jsonResponse({ items: [runtimeBootstrapRunningTask()] }))
    await pending

    expect(store.items).toHaveLength(1)
    expect(store.items[0]?.status).toBe('succeeded')
    expect(store.items[0]?.progress).toBe(100)
    expect(store.items[0]?.result?.details?.resource_count).toBe(2)
  })

  it('keeps a newer terminal snapshot when a stale task detail response resolves later', async () => {
    let resolveFetch: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise((resolve) => {
      resolveFetch = resolve
    })))

    const store = useTasksStore()
    const pending = store.fetchDetail('task_runtime_bootstrap_0001')

    store.upsert(runtimeBootstrapSucceededTask())
    resolveFetch?.(jsonResponse({ task: runtimeBootstrapRunningTask() }))
    const task = await pending

    expect(task.status).toBe('succeeded')
    expect(store.currentTask?.status).toBe('succeeded')
    expect(store.currentTask?.result?.summary).toBe('运行环境已准备')
    expect(store.currentTask?.finished_at).toBe('2026-04-01T15:21:44Z')
  })

  it('sorts tasks by latest task time first', () => {
    const store = useTasksStore()

    store.upsert({
      task_id: 'task_backup_0001',
      task_type: 'backup.create',
      status: 'succeeded',
      summary: '备份已完成',
      finished_at: '2026-04-01T15:25:00Z',
    })
    store.upsert({
      task_id: 'task_runtime_bootstrap_0001',
      task_type: 'runtime.bootstrap',
      status: 'running',
      summary: '准备运行环境',
      started_at: '2026-04-01T15:24:00Z',
    })
    store.upsert({
      task_id: 'task_plugin_install_0001',
      task_type: 'plugin.install',
      status: 'running',
      summary: '安装插件',
      started_at: '2026-04-01T15:20:00Z',
    })

    expect(store.sortedItems.map((task) => task.task_id)).toEqual([
      'task_backup_0001',
      'task_runtime_bootstrap_0001',
      'task_plugin_install_0001',
    ])
  })
})
