export const errors = {
  common: {
    actionFailed: '操作未完成，请稍后重试。',
    loadFailed: '读取未完成，请稍后重试。',
  },
  permission: {
    denied: '当前会话无权执行该操作。',
    blacklisted: '当前用户或群处于黑名单中。',
    notWhitelisted: '当前用户或群不在白名单中。',
  },
  platform: {
    invalidRequest: '请求参数不正确，请检查后重试。',
    notFound: '请求的资源不存在或已被移除。',
    resourceMissing: '缺少必要资源，请检查当前环境。',
    templateNotFound: '模板不存在。',
    templateSourceInvalid: '模板源码不合法，请检查 JSON、HTML 和输入结构。',
    templateRevisionConflict: '模板版本已变化，请先重新加载最新版本。',
    templateRevisionNotFound: '目标模板版本不存在。',
    templateRollbackTargetInvalid: '回退目标不合法，请重新选择版本。',
  },
} as const
