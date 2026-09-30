import { nextTick, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

const publishToast = vi.fn()
vi.mock('@/components/toast-state', () => ({ publishToast }))

const { useToastFeedback } = await import('@/adapter/feedback')

describe('useToastFeedback', () => {
  it('announces only changes after the first value when the page already shows that state', async () => {
    publishToast.mockClear()
    const source = ref<{ key: string; level: 'warning'; message: string } | null>({ key: 'a', level: 'warning', message: '连接已断开' })

    useToastFeedback(source, { skipFirst: true })
    expect(publishToast).not.toHaveBeenCalled()

    source.value = { key: 'b', level: 'warning', message: '连接仍在重试' }
    await nextTick()
    expect(publishToast).toHaveBeenCalledWith('warning', '连接仍在重试')

    source.value = null
    await nextTick()
    source.value = { key: 'a', level: 'warning', message: '连接已断开' }
    await nextTick()
    expect(publishToast).toHaveBeenCalledTimes(2)
  })
})
