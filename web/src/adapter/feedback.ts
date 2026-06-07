import message from 'ant-design-vue/es/message'
import { watch, type WatchSource } from 'vue'

export type ToastLevel = 'error' | 'info' | 'success' | 'warning'

interface ToastFeedback {
  key?: string | null
  level: ToastLevel
  message?: string | null
}

export function notifySuccess(content: string) {
  void message.success(content)
}

export function notifyError(content: string) {
  void message.error(content)
}

export function notifyInfo(content: string) {
  void message.info(content)
}

export function notifyWarning(content: string) {
  void message.warning(content)
}

export function notifyToast(level: ToastLevel, content: string) {
  switch (level) {
    case 'error':
      notifyError(content)
      break
    case 'success':
      notifySuccess(content)
      break
    case 'warning':
      notifyWarning(content)
      break
    case 'info':
      notifyInfo(content)
      break
  }
}

export function useToastFeedback(source: WatchSource<ToastFeedback | null | undefined>) {
  let lastKey: string | null = null

  watch(
    source,
    (feedback) => {
      const content = feedback?.message?.trim()
      if (!content) {
        lastKey = null
        return
      }

      const nextKey = feedback?.key ?? `${feedback.level}:${content}`
      if (nextKey === lastKey) {
        return
      }

      lastKey = nextKey
      notifyToast(feedback.level, content)
    },
    { immediate: true },
  )
}
