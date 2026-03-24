import { ApiError } from '@/lib/http'

function isNetworkError(error: unknown) {
  return error instanceof TypeError || (error instanceof Error && /fetch/i.test(error.message))
}

export function toLoginErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'permission.denied':
        return '登录未完成，请检查管理员账号和密钥。'
      case 'platform.invalid_request':
        return '登录请求未完成，请检查输入后重试。'
      default:
        return '登录未完成，请稍后重试。'
    }
  }

  if (isNetworkError(error)) {
    return '暂时无法连接管理界面，请确认服务已经启动。'
  }

  return '登录未完成，请稍后重试。'
}

export function toSetupErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'permission.denied':
        return '当前环境已经完成初始化，请直接登录。'
      case 'platform.invalid_request':
        return '创建管理员账号未完成，请检查输入后重试。'
      default:
        return '创建管理员账号未完成，请稍后重试。'
    }
  }

  if (isNetworkError(error)) {
    return '暂时无法连接管理界面，请确认服务已经启动。'
  }

  return '创建管理员账号未完成，请稍后重试。'
}

export function toLauncherAdmissionHint() {
  return '自动登录未完成，请手动登录。'
}

export function toBootstrapStatusMessage(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'permission.denied') {
      return '暂时无法确认管理界面状态，请稍后重试。'
    }

    if (error.code === 'platform.invalid_request') {
      return '管理界面暂时不可用，请稍后重试。'
    }

    return '暂时无法确认管理界面状态，请稍后重试。'
  }

  if (isNetworkError(error)) {
    return '暂时无法连接管理界面，请确认服务已经启动。'
  }

  return '暂时无法确认管理界面状态，请稍后重试。'
}
