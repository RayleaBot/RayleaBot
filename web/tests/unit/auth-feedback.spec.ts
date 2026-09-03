import { describe, expect, it } from 'vitest'

import {
  toBootstrapStatusMessage,
  toLoginErrorMessage,
  toSetupErrorMessage,
} from '@/lib/auth-feedback'
import { ApiError } from '@/lib/http'

function apiError(code: string) {
  return new ApiError('raw backend diagnostic', 400, code)
}

describe('authentication feedback', () => {
  it.each([
    ['denied credentials', apiError('permission.denied'), '管理员账号和密钥'],
    ['invalid request', apiError('platform.invalid_request'), '检查输入'],
    ['unknown API error', apiError('platform.unknown'), '稍后重试'],
    ['network error', new TypeError('Failed to fetch'), '服务已经启动'],
  ] as const)('classifies login %s', (_label, error, expectedGuidance) => {
    const message = toLoginErrorMessage(error)

    expect(message).toContain(expectedGuidance)
    expect(message).not.toContain('raw backend diagnostic')
  })

  it.each([
    ['already initialized', apiError('permission.denied'), '已经完成初始化'],
    ['invalid request', apiError('platform.invalid_request'), '检查输入'],
    ['unknown API error', apiError('platform.unknown'), '稍后重试'],
    ['network error', new TypeError('Failed to fetch'), '服务已经启动'],
  ] as const)('classifies setup %s', (_label, error, expectedGuidance) => {
    const message = toSetupErrorMessage(error)

    expect(message).toContain(expectedGuidance)
    expect(message).not.toContain('raw backend diagnostic')
  })

  it.each([
    ['denied status', apiError('permission.denied'), '管理界面状态'],
    ['invalid request', apiError('platform.invalid_request'), '暂时不可用'],
    ['unknown API error', apiError('platform.unknown'), '管理界面状态'],
    ['network error', new TypeError('Failed to fetch'), '服务已经启动'],
  ] as const)('classifies bootstrap %s', (_label, error, expectedGuidance) => {
    const message = toBootstrapStatusMessage(error)

    expect(message).toContain(expectedGuidance)
    expect(message).not.toContain('raw backend diagnostic')
  })
})
