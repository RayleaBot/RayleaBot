import { describe, expect, it } from 'vitest'

import {
  authParticleMaximumCount,
  authParticleMinimumCount,
  resolveAuthParticleCount,
  resolveAuthParticleOpacity,
  updateAuthParticle,
  type AuthParticle,
} from '@/components/auth/auth-particles'

function createParticle(overrides: Partial<AuthParticle> = {}): AuthParticle {
  return {
    age: 5,
    baseOpacity: 0.8,
    lifetime: 10,
    radius: 1,
    repulsionVelocityX: 0,
    repulsionVelocityY: 0,
    velocityX: 0,
    velocityY: 0,
    x: 100,
    y: 100,
    ...overrides,
  }
}

describe('authentication particle field math', () => {
  it('adapts particle density within the 80 to 160 item budget', () => {
    expect(resolveAuthParticleCount(390, 844)).toBe(authParticleMinimumCount)
    expect(resolveAuthParticleCount(1440, 900)).toBe(86)
    expect(resolveAuthParticleCount(4096, 2160)).toBe(authParticleMaximumCount)
    expect(resolveAuthParticleCount(0, Number.NaN)).toBe(authParticleMinimumCount)
  })

  it('fades particles at both ends of their lifetime', () => {
    expect(resolveAuthParticleOpacity(createParticle({ age: 0 }))).toBe(0)
    expect(resolveAuthParticleOpacity(createParticle({ age: 5 }))).toBeCloseTo(0.8)
    expect(resolveAuthParticleOpacity(createParticle({ age: 9.8 }))).toBeGreaterThan(0)
    expect(resolveAuthParticleOpacity(createParticle({ age: 10 }))).toBe(0)
  })

  it('creates a visible bounded displacement while the pointer remains nearby', () => {
    const particle = createParticle()

    for (let frame = 0; frame < 12; frame += 1) {
      updateAuthParticle(
        particle,
        0.016,
        800,
        600,
        { active: true, x: 90, y: 100 },
        () => 0.5,
      )
    }

    expect(particle.repulsionVelocityX).toBeGreaterThan(220)
    expect(Math.hypot(particle.repulsionVelocityX, particle.repulsionVelocityY)).toBeLessThanOrEqual(260)
    expect(particle.x).toBeGreaterThan(130)
  })

  it('repels an exactly overlapping particle and eases the impulse after pointer leave', () => {
    const particle = createParticle({ velocityX: 8 })

    updateAuthParticle(
      particle,
      0.016,
      800,
      600,
      { active: true, x: 100, y: 100 },
      () => 0.5,
    )
    expect(particle.repulsionVelocityX).toBeGreaterThan(0)
    const repulsionBeforeLeave = particle.repulsionVelocityX

    updateAuthParticle(
      particle,
      0.016,
      800,
      600,
      { active: false, x: 100, y: 100 },
      () => 0.5,
    )
    expect(particle.repulsionVelocityX).toBeLessThan(repulsionBeforeLeave)
  })

  it('keeps the repulsion trajectory stable across 60Hz and 120Hz updates', () => {
    const at60Hz = createParticle({ velocityX: 8 })
    const at120Hz = createParticle({ velocityX: 8 })

    for (let frame = 0; frame < 60; frame += 1) {
      updateAuthParticle(at60Hz, 1 / 60, 800, 600, { active: true, x: 40, y: 100 })
    }
    for (let frame = 0; frame < 120; frame += 1) {
      updateAuthParticle(at120Hz, 1 / 120, 800, 600, { active: true, x: 40, y: 100 })
    }

    expect(at120Hz.x).toBeCloseTo(at60Hz.x, 0)
    expect(at120Hz.repulsionVelocityX).toBeCloseTo(at60Hz.repulsionVelocityX, 0)
  })
})
