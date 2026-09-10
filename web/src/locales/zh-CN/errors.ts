import { errorMessages } from '@/types/error-codes.generated'

export const errors = {
  ...errorMessages,
  common: {
    actionFailed: '操作未完成，请稍后重试。',
    loadFailed: '读取未完成，请稍后重试。',
    saveFailed: '保存未完成，请稍后重试。',
    requestCancelled: '请求已取消。',
    requestTimeout: '请求超时，请稍后重试。',
  },
} as const
