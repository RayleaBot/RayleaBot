import { describe, expect, it } from 'vitest'
import { getPluginTrustLabel } from '@/lib/display'

describe('machine-readable management decisions', () => {
  it('covers every trust level and degrades unknown display values without granting official status', () => {
    expect(['official', 'development', 'third_party', 'unverified', 'future'].map(getPluginTrustLabel))
      .toEqual(['官方', '开发中', '第三方', '未验证来源', '来源未知'])
  })
})
