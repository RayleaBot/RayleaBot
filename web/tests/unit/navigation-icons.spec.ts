import { describe, expect, it } from 'vitest'

import { resolveMenuIcon } from '@/access/icons'
import { adminRoutes } from '@/router/routes/modules/admin'

describe('primary navigation icons', () => {
  it('assigns a distinct registered icon to every visible navigation entry', () => {
    const entries = (adminRoutes[0]?.children ?? [])
      .filter((route) => !route.meta?.hideInMenu)
      .flatMap((group) => [
        group,
        ...(group.children ?? []).filter((route) => !route.meta?.hideInMenu),
      ])
    const iconNames = entries.map((route) => String(route.meta?.icon || ''))
    const iconComponents = iconNames.map((icon) => resolveMenuIcon(icon))

    expect(iconNames.every(Boolean)).toBe(true)
    expect(new Set(iconNames).size).toBe(iconNames.length)
    expect(iconComponents.every(Boolean)).toBe(true)
    expect(new Set(iconComponents).size).toBe(iconComponents.length)
  })
})
