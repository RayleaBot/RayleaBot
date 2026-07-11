export const authParticleConnectionDistance = 150
export const authParticleMaximumCount = 160
export const authParticleMinimumCount = 80
export const authParticleRepulsionDistance = 196

export interface AuthParticle {
  age: number
  baseOpacity: number
  lifetime: number
  radius: number
  repulsionVelocityX: number
  repulsionVelocityY: number
  velocityX: number
  velocityY: number
  x: number
  y: number
}

export interface AuthParticlePointer {
  active: boolean
  x: number
  y: number
}

export type AuthParticleRandom = () => number

const particleAreaPerItem = 15_000
const particleFadeInRatio = 0.14
const particleFadeOutRatio = 0.2
const particleMinimumSpeed = 7
const particleRepulsionEngageRate = 14
const particleRepulsionMaximumSpeed = 260
const particleRepulsionReleaseRate = 3.2

export function resolveAuthParticleCount(width: number, height: number) {
  const safeWidth = normalizeDimension(width)
  const safeHeight = normalizeDimension(height)
  if (!safeWidth || !safeHeight) {
    return authParticleMinimumCount
  }

  return clamp(
    Math.round((safeWidth * safeHeight) / particleAreaPerItem),
    authParticleMinimumCount,
    authParticleMaximumCount,
  )
}

export function createAuthParticle(
  width: number,
  height: number,
  random: AuthParticleRandom = Math.random,
  staggerLife = true,
): AuthParticle {
  const angle = random() * Math.PI * 2
  const speed = lerp(particleMinimumSpeed, 18, random())
  const lifetime = lerp(10, 22, random())

  return {
    age: staggerLife ? random() * lifetime : 0,
    baseOpacity: lerp(0.5, 0.92, random()),
    lifetime,
    radius: lerp(0.8, 1.8, random()),
    repulsionVelocityX: 0,
    repulsionVelocityY: 0,
    velocityX: Math.cos(angle) * speed,
    velocityY: Math.sin(angle) * speed,
    x: random() * normalizeDimension(width),
    y: random() * normalizeDimension(height),
  }
}

export function resolveAuthParticleOpacity(particle: AuthParticle) {
  if (particle.lifetime <= 0 || particle.age < 0 || particle.age >= particle.lifetime) {
    return 0
  }

  const progress = particle.age / particle.lifetime
  const fadeIn = clamp(progress / particleFadeInRatio, 0, 1)
  const fadeOut = clamp((1 - progress) / particleFadeOutRatio, 0, 1)
  return smoothStep(Math.min(fadeIn, fadeOut)) * particle.baseOpacity
}

export function updateAuthParticle(
  particle: AuthParticle,
  deltaSeconds: number,
  width: number,
  height: number,
  pointer: AuthParticlePointer,
  random: AuthParticleRandom = Math.random,
) {
  const safeDelta = clamp(Number.isFinite(deltaSeconds) ? deltaSeconds : 0, 0, 0.034)
  const safeWidth = normalizeDimension(width)
  const safeHeight = normalizeDimension(height)
  particle.age += safeDelta

  if (particle.age >= particle.lifetime) {
    Object.assign(particle, createAuthParticle(safeWidth, safeHeight, random, false))
    return
  }

  let targetRepulsionVelocityX = 0
  let targetRepulsionVelocityY = 0
  let pointerWithinRange = false
  if (pointer.active) {
    const offsetX = particle.x - pointer.x
    const offsetY = particle.y - pointer.y
    const distanceSquared = offsetX * offsetX + offsetY * offsetY
    const repulsionDistanceSquared = authParticleRepulsionDistance * authParticleRepulsionDistance
    if (distanceSquared < repulsionDistanceSquared) {
      pointerWithinRange = true
      const distance = Math.sqrt(distanceSquared)
      const falloff = 1 - distance / authParticleRepulsionDistance
      const targetSpeed = particleRepulsionMaximumSpeed * smoothStep(falloff)
      let directionX: number
      let directionY: number
      if (distance > 0.1) {
        directionX = offsetX / distance
        directionY = offsetY / distance
      } else {
        const driftSpeed = Math.hypot(particle.velocityX, particle.velocityY)
        directionX = driftSpeed > 0.01 ? particle.velocityX / driftSpeed : 1
        directionY = driftSpeed > 0.01 ? particle.velocityY / driftSpeed : 0
      }
      targetRepulsionVelocityX = directionX * targetSpeed
      targetRepulsionVelocityY = directionY * targetSpeed
    }
  }

  const responseRate = pointerWithinRange
    ? particleRepulsionEngageRate
    : particleRepulsionReleaseRate
  const response = 1 - Math.exp(-responseRate * safeDelta)
  particle.repulsionVelocityX += (
    targetRepulsionVelocityX - particle.repulsionVelocityX
  ) * response
  particle.repulsionVelocityY += (
    targetRepulsionVelocityY - particle.repulsionVelocityY
  ) * response

  particle.x = wrap(
    particle.x + (particle.velocityX + particle.repulsionVelocityX) * safeDelta,
    safeWidth,
  )
  particle.y = wrap(
    particle.y + (particle.velocityY + particle.repulsionVelocityY) * safeDelta,
    safeHeight,
  )
}

function normalizeDimension(value: number) {
  return Number.isFinite(value) && value > 0 ? value : 0
}

function wrap(value: number, size: number) {
  if (!size) {
    return 0
  }
  return ((value % size) + size) % size
}

function smoothStep(value: number) {
  return value * value * (3 - 2 * value)
}

function lerp(start: number, end: number, progress: number) {
  return start + (end - start) * clamp(progress, 0, 1)
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), maximum)
}
