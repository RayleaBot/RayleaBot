import { t } from '@/i18n'
import { apiPath } from '@/lib/api-path'
import { ApiError, apiRequest } from '@/lib/http'
import type { TaskStatusResponse } from '@/types/api'

export async function waitForTask(taskId: string, signal?: AbortSignal) {
  const deadline = Date.now() + 15 * 60_000
  while (true) {
    const task = await apiRequest<TaskStatusResponse>(apiPath('/api/system/tasks/{task_id}', { task_id: taskId }), { signal })
    if (task.status === 'succeeded') return task
    if (task.status === 'cancelled') throw new ApiError(t('errors.common.taskCancelled'), 0, 'client.task_cancelled')
    if (task.status === 'interrupted') throw new ApiError(t('errors.common.taskInterrupted'), 0, 'client.task_interrupted')
    if (task.status !== 'pending' && task.status !== 'running') throw new ApiError(t('errors.common.taskFailed'), 500, task.error_code || 'platform.internal_error')
    if (Date.now() >= deadline) throw new ApiError(t('errors.common.taskStillRunning'), 0, 'client.task_still_running')
    await new Promise<void>((resolve, reject) => {
      const abort = () => { clearTimeout(timer); reject(new DOMException('Aborted', 'AbortError')) }
      const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve() }, 1000)
      if (signal?.aborted) abort()
      else signal?.addEventListener('abort', abort, { once: true })
    })
  }
}
