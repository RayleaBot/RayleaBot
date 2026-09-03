import { describe, expect, it } from 'vitest'

import { resolveStatusTone } from '@/lib/status-tone'

describe('status tone', () => {
  it('falls back to neutral for unknown states', () => {
    expect(resolveStatusTone('unknown-state')).toBe('neutral')
  })
})
