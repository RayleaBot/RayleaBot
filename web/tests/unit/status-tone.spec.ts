import { describe, expect, it } from 'vitest'

import { resolveStatusTone } from '@/lib/status-tone'
import { getStatusType } from '@/lib/display'

describe('status tone', () => {
  it('projects the same severity into full and compact indicators', () => {
    for (const status of ['running', 'connected', 'authenticated', 'failed', 'auth_failed', 'degraded']) {
      expect(getStatusType(status)).toBe(resolveStatusTone(status))
    }
    expect(resolveStatusTone('setup_required')).toBe('attention')
    expect(getStatusType('setup_required')).toBe('warning')
    expect(resolveStatusTone('connecting')).toBe('info')
    expect(getStatusType('connecting')).toBe('warning')
  })
  it('falls back to neutral for unknown states', () => {
    expect(resolveStatusTone('unknown-state')).toBe('neutral')
  })
})
