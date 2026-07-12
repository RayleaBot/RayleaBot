import { describe, expect, it } from 'vitest'

import { resolveStatusTone } from '@/lib/status-tone'

describe('status tone', () => {
  it.each([
    ['stopped', 'neutral'],
    ['starting', 'info'],
    ['stopping', 'info'],
    ['running', 'success'],
    ['degraded', 'warning'],
    ['setup_required', 'attention'],
    ['failed', 'danger'],
    ['unknown-state', 'neutral'],
  ] as const)('maps %s to %s', (status, tone) => {
    expect(resolveStatusTone(status)).toBe(tone)
  })
})
