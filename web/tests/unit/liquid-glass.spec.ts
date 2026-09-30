import { afterEach, describe, expect, it, vi } from 'vitest'

import { GlassFilterRegistry, lensGeometry, rimShift } from '@/lib/liquid-glass'

afterEach(() => {
  vi.useRealTimers()
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
