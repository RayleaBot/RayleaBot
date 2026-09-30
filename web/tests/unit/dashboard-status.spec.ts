import { describe, expect, it } from 'vitest'

import { summarizeAttention } from '@/views/dashboard/dashboard-status'

describe('status page attention summary', () => {
  it('counts a problem reported by both readiness and diagnostics once', () => {
    const issue = { code: 'platform.resource_missing', runtime_resources: ['chromium' as const], severity: 'warning' as const, summary: 'Chromium 缺失', remediation: '准备运行环境。' }
    const attention = summarizeAttention([issue], [issue], 3)

    expect(attention).toMatchObject({ count: 1, runtimeResources: ['chromium'], tone: 'warning', view: 'readiness' })
    expect(attention?.detail).toBe('准备运行环境。其余 3 项就绪检查通过。')
  })

  it('falls back to diagnostics problems and prepares every missing runtime resource', () => {
    const attention = summarizeAttention([], [
      { code: 'dependency.ffmpeg', runtime_resources: ['ffmpeg'], severity: 'warning', summary: 'FFmpeg 不可用', user_message: 'FFmpeg 媒体工具 清单不可用' },
      { code: 'filesystem.data', severity: 'error', summary: '数据目录不可写' },
      { code: 'system.ok', severity: 'ok', summary: '服务正常' },
    ], 0)

    expect(attention).toMatchObject({ count: 2, runtimeResources: ['ffmpeg'], tone: 'danger', view: 'diagnostics' })
    expect(attention?.title).toBe('2 项需要处理：数据目录不可写')
  })

  it('reports nothing when no issue needs a person', () => {
    expect(summarizeAttention([{ code: 'system.ok', severity: 'ok', summary: '服务正常' }], [], 5)).toBeNull()
  })
})
