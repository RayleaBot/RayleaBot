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

function renderPreviewRunningTask(): TaskSummary {
  return {
    task_id: 'task_render_preview_0001',
    task_type: 'render.preview',
    status: 'running',
    progress: 90,
    summary: '生成渲染产物',
    started_at: '2026-04-01T15:21:43Z',
  }
}

function renderPreviewSucceededTask(): TaskSummary {
  return {
    task_id: 'task_render_preview_0001',
    task_type: 'render.preview',
    status: 'succeeded',
    progress: 100,
    summary: '渲染预览已生成',
    started_at: '2026-04-01T15:21:43Z',
    finished_at: '2026-04-01T15:21:44Z',
    result: {
      summary: '渲染预览已生成',
      details: {
        artifact_id: 'render_preview_0001.png',
        image_url: '/api/system/render/artifacts/render_preview_0001.png',
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

    store.upsert(renderPreviewSucceededTask())
    resolveFetch?.(jsonResponse({ items: [renderPreviewRunningTask()] }))
    await pending

    expect(store.items).toHaveLength(1)
    expect(store.items[0]?.status).toBe('succeeded')
    expect(store.items[0]?.progress).toBe(100)
    expect(store.items[0]?.result?.details?.image_url).toBe('/api/system/render/artifacts/render_preview_0001.png')
  })

  it('keeps a newer terminal snapshot when a stale task detail response resolves later', async () => {
    let resolveFetch: ((value: Response) => void) | undefined
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise((resolve) => {
      resolveFetch = resolve
    })))

    const store = useTasksStore()
    const pending = store.fetchDetail('task_render_preview_0001')

    store.upsert(renderPreviewSucceededTask())
    resolveFetch?.(jsonResponse({ task: renderPreviewRunningTask() }))
    const task = await pending

    expect(task.status).toBe('succeeded')
    expect(store.currentTask?.status).toBe('succeeded')
    expect(store.currentTask?.result?.summary).toBe('渲染预览已生成')
    expect(store.currentTask?.finished_at).toBe('2026-04-01T15:21:44Z')
  })
})
