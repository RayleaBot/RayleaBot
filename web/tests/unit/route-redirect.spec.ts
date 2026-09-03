import { describe, expect, it } from 'vitest'

import { readInternalRedirectTarget } from '@/lib/route-redirect'

describe('internal redirect targets', () => {
  it.each([
    ['/plugins?panel=settings#limits', '/plugins?panel=settings#limits'],
    [['/commands', '/logs'], '/commands'],
    [undefined, null],
    ['', null],
    ['   ', null],
    ['commands', null],
    ['https://example.com', null],
    ['//example.com/path', null],
    ['/\\example.com/path', null],
  ] as const)('normalizes %j to %j', (value, expected) => {
    expect(readInternalRedirectTarget(value)).toBe(expected)
  })
})
