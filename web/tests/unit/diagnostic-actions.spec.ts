import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { ApiError } from '@/lib/http'
import { resolveExceptionStatus } from '@/lib/exception-status'
import { getPluginTrustLabel } from '@/lib/display'
import { useDashboardRecoveryState } from '@/views/dashboard/useDashboardRecoveryState'

describe('machine-readable management decisions', () => {
  it('selects runtime resources independently of message language and misleading keywords', () => {
    const issues = ref<any[]>([])
    const state = useDashboardRecoveryState({ readinessIssues: issues, readiness: ref(null), system: ref(null), selectedRecoveryReviewIds: ref([]) })
    for (const summary of ['Chromium 不可用', 'Browser ready', 'FFmpeg 初始化失败']) {
      issues.value = [{ code: 'platform.resource_missing', summary, runtime_resources: ['ffmpeg'] }]
      expect(state.recoveryBootstrapResources.value).toEqual(['ffmpeg'])
      issues.value = [{ code: 'platform.resource_missing', summary }]
      expect(state.recoveryBootstrapResources.value).toEqual([])
    }
    issues.value = [{ runtime_resources: ['chromium', 'ffmpeg'] }, { runtime_resources: ['ffmpeg', 'unknown'] }]
    expect(state.recoveryBootstrapResources.value).toEqual(['chromium', 'ffmpeg'])
  })

  it('resolves failure illustrations from codes and HTTP status only', () => {
    for (const message of ['无权访问', 'Not found', 'Internal server error']) {
      expect(resolveExceptionStatus(new ApiError(message, 404, 'platform.resource_not_found'))).toBe('404')
      expect(resolveExceptionStatus(new ApiError(message, 403, 'permission.denied'))).toBe('403')
      expect(resolveExceptionStatus(new Error(message))).toBe('500')
    }
  })

  it('covers every trust level and degrades unknown display values without granting official status', () => {
    expect(['official', 'development', 'third_party', 'unverified', 'future'].map(getPluginTrustLabel))
      .toEqual(['官方', '开发中', '第三方', '未验证来源', '来源未知'])
  })
})
