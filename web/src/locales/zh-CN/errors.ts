export const errors = {
  common: {
    actionFailed: '操作未完成，请稍后重试。',
    loadFailed: '读取未完成，请稍后重试。',
  },
  permission: {
    denied: '当前会话无权执行该操作。',
  },
  platform: {
    invalidRequest: '请求参数不正确，请检查后重试。',
    notFound: '请求的资源不存在或已被移除。',
    resourceMissing: '缺少必要资源，请检查当前环境。',
  },
} as const
