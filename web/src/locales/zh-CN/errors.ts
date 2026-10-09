import { errorMessages } from '@/types/error-codes.generated'

export const errors = {
  ...errorMessages,
  common: {
    actionFailed: '操作未完成，请稍后重试。',
    loadFailed: '读取未完成，请稍后重试。',
    saveFailed: '保存未完成，请稍后重试。',
    requestCancelled: '请求已取消。',
    requestTimeout: '请求超时，请稍后重试。',
    taskCancelled: '任务已取消。',
    taskInterrupted: '任务已中断，请检查服务状态。',
    taskFailed: '任务执行失败。',
    taskStillRunning: '任务仍在执行，请稍后刷新查看结果。',
  },
  coreVersion: {
    unknown: '无法确认当前 RayleaBot 版本，不能安装要求最低版本的插件。',
    tooOld: '插件需要 RayleaBot {version} 或更高版本。',
  },
  dependencyMissing: '请先安装前置插件：{plugins}',
} as const
