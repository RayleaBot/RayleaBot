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

export function useToastFeedback(source: WatchSource<ToastFeedback | null | undefined>) {
  let lastKey: string | null = null

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
      publishToast(feedback.level, content)
    },
    { immediate: true },
  )
}
