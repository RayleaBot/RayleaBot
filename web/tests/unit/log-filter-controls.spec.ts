import { describe, expect, it } from 'vitest'

import { fitInlineFilterCount } from '@/components/logs/useLogFilterControls'

describe('fitInlineFilterCount', () => {
  // Level and source (220 + 12 + 240) before protocol, plugin and request ID.
  const row = { fixedWidth: 472, optionalWidths: [150, 220, 240], overflowWidth: 120, gap: 12 }

  it('keeps every filter in the row and needs no overflow trigger when they all fit', () => {
    expect(fitInlineFilterCount({ ...row, rowWidth: 1118 })).toBe(3)
  })

  it('keeps room for the trigger and gives filters to it from the end', () => {
    expect(fitInlineFilterCount({ ...row, rowWidth: 1117 })).toBe(2)
    expect(fitInlineFilterCount({ ...row, rowWidth: 766 })).toBe(1)
    expect(fitInlineFilterCount({ ...row, rowWidth: 765 })).toBe(0)
  })
})
