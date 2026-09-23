import { afterEach, describe, expect, it, vi } from 'vitest'

import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { waitForTask } from '@/lib/tasks'

function taskResponse(status: string, errorCode?: string) {
  return new Response(JSON.stringify({ task_id: 'task_1', status, error_code: errorCode }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

async function displayedFailure(status: string, errorCode?: string) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(taskResponse(status, errorCode)))
  const failure = await waitForTask('task_1').catch((error: unknown) => error)
  return getDisplayErrorMessage(failure)
}

describe('waitForTask', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it.each([
    ['cancelled', 'errors.common.taskCancelled'],
    ['interrupted', 'errors.common.taskInterrupted'],
  ])('reports a %s task with its own message', async (status, key) => {
    expect(await displayedFailure(status)).toBe(t(key))
  })

  it('reports a failed task through its formal error code', async () => {
    expect(await displayedFailure('failed', 'platform.task_timeout')).toBe(t('errors.platform.task_timeout'))
  })
})
