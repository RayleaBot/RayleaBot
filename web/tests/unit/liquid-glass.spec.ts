import { afterEach, describe, expect, it, vi } from 'vitest'

import { createGlassDisplacement } from '@/components/auth/liquid-glass'
import { GlassFilterRegistry, lensGeometry, rimShift } from '@/lib/liquid-glass'

afterEach(() => {
  vi.useRealTimers()
})

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

describe('shell glass lenses', () => {
  it('bends light only inside the bevel and most strongly at the rim', () => {
    const geometry = lensGeometry({ width: 244, height: 876, radius: 28 })
    expect(rimShift(1, geometry)).toBe(0)
    expect(rimShift(0.9, geometry)).toBeGreaterThan(0)
    expect(rimShift(0.2, geometry)).toBeGreaterThan(rimShift(0.6, geometry))
    // Small controls keep a usable bevel; large panes stop growing it.
    expect(lensGeometry({ width: 12, height: 12, radius: 6 }).bevel).toBe(4)
    expect(lensGeometry({ width: 1200, height: 900, radius: 24 }).bevel).toBe(36)
  })

  it('removes filters for sizes no longer on screen after the release delay', () => {
    vi.useFakeTimers()
    const destroyed: string[] = []
    const registry = new GlassFilterRegistry<string>(
      (id) => ({ resource: id, ready: Promise.resolve(true) }),
      (resource) => destroyed.push(resource),
      1000,
    )

    const sidebar = registry.acquire('sidebar-720')
    registry.acquire('sidebar-720')
    registry.release('sidebar-720')
    vi.advanceTimersByTime(1000)
    expect(destroyed).toEqual([])

    registry.release('sidebar-720')
    const resized = registry.acquire('sidebar-640')
    registry.release('sidebar-640')
    vi.advanceTimersByTime(999)
    expect(registry.size).toBe(2)

    // A lens that returns to a size before the delay reuses the same filter.
    expect(registry.acquire('sidebar-720').id).toBe(sidebar.id)
    registry.release('sidebar-720')
    vi.advanceTimersByTime(1000)

    expect(destroyed).toEqual([resized.id, sidebar.id])
    expect(registry.size).toBe(0)
  })
})
