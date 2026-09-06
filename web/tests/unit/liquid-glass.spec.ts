import { describe, expect, it } from 'vitest'

import { createGlassDisplacement } from '@/components/auth/liquid-glass'

describe('authentication glass lens', () => {
  it('leaves the center clear and bends opposite edges symmetrically', () => {
    const map = createGlassDisplacement(448, 600, 36)
    const channel = (x: number, y: number, offset: number) => map.pixels[(y * map.width + x) * 4 + offset]
    expect(channel(224, 300, 0)).toBe(128)
    expect(channel(224, 300, 1)).toBe(128)
    expect(channel(8, 300, 0)).toBeGreaterThan(200)
    expect(channel(439, 300, 0)).toBeLessThan(55)
    expect(channel(8, 300, 0) + channel(439, 300, 0)).toBe(255)
    expect(channel(224, 8, 1) + channel(224, 591, 1)).toBe(255)
    expect(channel(224, 300, 3)).toBe(255)
  })

  it('bounds the raster size for long recovery content without changing its aspect ratio', () => {
    const map = createGlassDisplacement(448, 1800, 28)
    expect(Math.max(map.width, map.height)).toBeLessThanOrEqual(720)
    expect(map.width / map.height).toBeCloseTo(448 / 1800, 2)
    expect(map.pixels.length).toBe(map.width * map.height * 4)
  })
})
