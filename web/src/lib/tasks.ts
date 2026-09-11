import { apiPath } from '@/lib/api-path'
import { ApiError, apiRequest } from '@/lib/http'
import type { TaskStatusResponse } from '@/types/api'

export async function waitForTask(taskId: string, signal?: AbortSignal) {
  const deadline = Date.now() + 15 * 60_000
  while (true) {
    const task = await apiRequest<TaskStatusResponse>(apiPath('/api/system/tasks/{task_id}', { task_id: taskId }), { signal })
    if (task.status === 'succeeded') return task
    if (task.status !== 'pending' && task.status !== 'running') {
      const message = task.status === 'cancelled' ? '任务已取消' : task.status === 'interrupted' ? '任务已中断，请检查服务状态' : '任务执行失败'
      if (task.status === 'cancelled' || task.status === 'interrupted') throw new Error(message)
      throw new ApiError(message, 500, task.error_code || 'platform.internal_error')
    }
    if (Date.now() >= deadline) throw new Error('任务仍在执行，请稍后刷新查看结果')
    await new Promise<void>((resolve, reject) => {
      const abort = () => { clearTimeout(timer); reject(new DOMException('Aborted', 'AbortError')) }
      const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve() }, 1000)
      if (signal?.aborted) abort()
      else signal?.addEventListener('abort', abort, { once: true })
    })
  }
}
