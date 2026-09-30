import { publishToast, type ToastLevel } from '@/components/toast-state'
import { watch, type WatchSource } from 'vue'

interface ToastFeedback {
  key?: string | null
  level: ToastLevel
  message?: string | null
}

export function notifySuccess(content: string) {
  publishToast('success', content)
}

export function notifyError(content: string) {
  publishToast('error', content)
}

export function notifyInfo(content: string) {
  publishToast('info', content)
}

export function notifyWarning(content: string) {
  publishToast('warning', content)
}

// skipFirst: the page already shows this state persistently, so only changes after the first value are announced.
export function useToastFeedback(source: WatchSource<ToastFeedback | null | undefined>, options: { skipFirst?: boolean } = {}) {
  let lastKey: string | null = null
  let skipNext = options.skipFirst === true

  watch(
    source,
    (feedback) => {
      const content = feedback?.message?.trim()
      if (!feedback || !content) {
        lastKey = null
        return
      }

      const nextKey = feedback.key ?? `${feedback.level}:${content}`
      if (nextKey === lastKey) {
        return
      }

      lastKey = nextKey
      if (skipNext) {
        skipNext = false
        return
      }
      publishToast(feedback.level, content)
    },
    { immediate: true },
  )
}
