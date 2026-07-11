<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import {
  authParticleConnectionDistance,
  authParticleMaximumCount,
  createAuthParticle,
  resolveAuthParticleCount,
  resolveAuthParticleOpacity,
  updateAuthParticle,
  type AuthParticle,
  type AuthParticlePointer,
} from '@/components/auth/auth-particles'
import type { AuthParticlePalette } from '@/preferences/auth'

const props = defineProps<{
  palette: AuthParticlePalette
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const particles: AuthParticle[] = []
const maximumConnectionCount = authParticleMaximumCount * (authParticleMaximumCount - 1) / 2
const connectionBuckets = new Uint8Array(maximumConnectionCount)
const connectionSegments = new Float32Array(maximumConnectionCount * 4)
const connectionOpacityLevels = new Float32Array([0.06, 0.14, 0.24, 0.36, 0.5, 0.64, 0.8, 0.94])
const particleOpacities = new Float32Array(authParticleMaximumCount)
const pointer: AuthParticlePointer = { active: false, x: 0, y: 0 }

let context: CanvasRenderingContext2D | null = null
let finePointerMediaQuery: MediaQueryList | null = null
let reducedMotionMediaQuery: MediaQueryList | null = null
let frameId: number | null = null
let lastDrawAt = 0
let viewportHeight = 0
let viewportWidth = 0
let pointerListenersAttached = false

function handlePointerMove(event: PointerEvent) {
  if (event.pointerType === 'touch') {
    return
  }
  const becameActive = !pointer.active
  pointer.active = true
  pointer.x = event.clientX
  pointer.y = event.clientY
  if (becameActive && canvasRef.value) {
    canvasRef.value.dataset.authPointerActive = 'true'
  }
}

function handlePointerLeave() {
  pointer.active = false
  if (canvasRef.value) {
    canvasRef.value.dataset.authPointerActive = 'false'
  }
}

function attachPointerListeners() {
  if (pointerListenersAttached) {
    return
  }
  window.addEventListener('pointermove', handlePointerMove, { passive: true })
  document.addEventListener('pointerleave', handlePointerLeave)
  pointerListenersAttached = true
}

function detachPointerListeners() {
  if (!pointerListenersAttached) {
    return
  }
  window.removeEventListener('pointermove', handlePointerMove)
  document.removeEventListener('pointerleave', handlePointerLeave)
  pointerListenersAttached = false
  handlePointerLeave()
}

function reconcileParticleCount() {
  const targetCount = resolveAuthParticleCount(viewportWidth, viewportHeight)
  if (particles.length > targetCount) {
    particles.length = targetCount
  }
  while (particles.length < targetCount) {
    particles.push(createAuthParticle(viewportWidth, viewportHeight))
  }
}

function resizeCanvas() {
  const canvas = canvasRef.value
  if (!canvas || !context) {
    return
  }

  viewportWidth = Math.max(1, window.innerWidth)
  viewportHeight = Math.max(1, window.innerHeight)
  const scale = Math.min(Math.max(window.devicePixelRatio || 1, 1), 1.5)
  canvas.width = Math.round(viewportWidth * scale)
  canvas.height = Math.round(viewportHeight * scale)
  context.setTransform(scale, 0, 0, scale, 0, 0)
  reconcileParticleCount()
  canvas.dataset.authParticleCount = String(particles.length)
  drawParticleField()
}

function drawParticleField() {
  if (!context) {
    return
  }

  context.clearRect(0, 0, viewportWidth, viewportHeight)
  context.lineWidth = 0.75
  context.strokeStyle = props.palette.line

  for (let index = 0; index < particles.length; index += 1) {
    particleOpacities[index] = resolveAuthParticleOpacity(particles[index])
  }

  const connectionDistanceSquared = authParticleConnectionDistance * authParticleConnectionDistance
  let connectionCount = 0
  for (let particleIndex = 0; particleIndex < particles.length; particleIndex += 1) {
    const particle = particles[particleIndex]
    const particleOpacity = particleOpacities[particleIndex]
    if (particleOpacity < 0.025) {
      continue
    }

    for (let otherIndex = particleIndex + 1; otherIndex < particles.length; otherIndex += 1) {
      const otherParticle = particles[otherIndex]
      const otherOpacity = particleOpacities[otherIndex]
      if (otherOpacity < 0.025) {
        continue
      }

      const offsetX = particle.x - otherParticle.x
      const offsetY = particle.y - otherParticle.y
      const distanceSquared = offsetX * offsetX + offsetY * offsetY
      if (distanceSquared >= connectionDistanceSquared) {
        continue
      }

      const distanceRatio = Math.sqrt(distanceSquared) / authParticleConnectionDistance
      const opacity = (1 - distanceRatio) * Math.min(particleOpacity, otherOpacity)
      if (opacity < 0.025) {
        continue
      }
      const segmentOffset = connectionCount * 4
      connectionSegments[segmentOffset] = particle.x
      connectionSegments[segmentOffset + 1] = particle.y
      connectionSegments[segmentOffset + 2] = otherParticle.x
      connectionSegments[segmentOffset + 3] = otherParticle.y
      connectionBuckets[connectionCount] = Math.min(
        connectionOpacityLevels.length - 1,
        Math.floor(opacity * connectionOpacityLevels.length),
      )
      connectionCount += 1
    }
  }

  for (let bucket = 0; bucket < connectionOpacityLevels.length; bucket += 1) {
    let hasSegments = false
    context.beginPath()
    for (let connectionIndex = 0; connectionIndex < connectionCount; connectionIndex += 1) {
      if (connectionBuckets[connectionIndex] !== bucket) {
        continue
      }
      const segmentOffset = connectionIndex * 4
      context.moveTo(connectionSegments[segmentOffset], connectionSegments[segmentOffset + 1])
      context.lineTo(connectionSegments[segmentOffset + 2], connectionSegments[segmentOffset + 3])
      hasSegments = true
    }
    if (hasSegments) {
      context.globalAlpha = connectionOpacityLevels[bucket]
      context.stroke()
    }
  }

  context.fillStyle = props.palette.particle
  for (let index = 0; index < particles.length; index += 1) {
    const particle = particles[index]
    const opacity = particleOpacities[index]
    if (opacity < 0.025) {
      continue
    }
    context.globalAlpha = opacity
    context.beginPath()
    context.arc(particle.x, particle.y, particle.radius, 0, Math.PI * 2)
    context.fill()
  }
  context.globalAlpha = 1
}

function animate(timestamp: number) {
  frameId = window.requestAnimationFrame(animate)
  const targetFrameInterval = finePointerMediaQuery?.matches ? 0 : 1000 / 30
  if (targetFrameInterval && timestamp - lastDrawAt + 0.5 < targetFrameInterval) {
    return
  }

  const deltaSeconds = lastDrawAt ? (timestamp - lastDrawAt) / 1000 : 1 / 60
  lastDrawAt = timestamp
  for (const particle of particles) {
    updateAuthParticle(particle, deltaSeconds, viewportWidth, viewportHeight, pointer)
  }
  drawParticleField()
}

function stopAnimation(state: 'paused' | 'static') {
  if (frameId !== null) {
    window.cancelAnimationFrame(frameId)
    frameId = null
  }
  lastDrawAt = 0
  if (canvasRef.value) {
    canvasRef.value.dataset.authParticleState = state
  }
}

function startAnimation() {
  if (frameId !== null || document.hidden || reducedMotionMediaQuery?.matches) {
    return
  }
  if (canvasRef.value) {
    canvasRef.value.dataset.authParticleState = 'running'
  }
  frameId = window.requestAnimationFrame(animate)
}

function syncMotionAvailability() {
  if (finePointerMediaQuery?.matches && !reducedMotionMediaQuery?.matches) {
    attachPointerListeners()
  } else {
    detachPointerListeners()
  }

  if (document.hidden) {
    stopAnimation('paused')
    return
  }
  if (reducedMotionMediaQuery?.matches) {
    stopAnimation('static')
    drawParticleField()
    return
  }
  startAnimation()
}

watch(() => props.palette, () => {
  if (reducedMotionMediaQuery?.matches || document.hidden) {
    drawParticleField()
  }
})

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) {
    return
  }
  context = canvas.getContext('2d', { alpha: true })
  if (!context) {
    canvas.dataset.authParticleState = 'unavailable'
    return
  }

  finePointerMediaQuery = window.matchMedia('(any-hover: hover) and (any-pointer: fine)')
  reducedMotionMediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  finePointerMediaQuery.addEventListener('change', syncMotionAvailability)
  reducedMotionMediaQuery.addEventListener('change', syncMotionAvailability)
  document.addEventListener('visibilitychange', syncMotionAvailability)
  window.addEventListener('resize', resizeCanvas, { passive: true })
  resizeCanvas()
  syncMotionAvailability()
})

onBeforeUnmount(() => {
  stopAnimation('paused')
  detachPointerListeners()
  finePointerMediaQuery?.removeEventListener('change', syncMotionAvailability)
  reducedMotionMediaQuery?.removeEventListener('change', syncMotionAvailability)
  document.removeEventListener('visibilitychange', syncMotionAvailability)
  window.removeEventListener('resize', resizeCanvas)
  finePointerMediaQuery = null
  reducedMotionMediaQuery = null
  context = null
  particles.length = 0
})
</script>

<template>
  <canvas
    ref="canvasRef"
    aria-hidden="true"
    class="auth-particle-field"
    data-auth-particle-state="paused"
    data-auth-pointer-active="false"
    data-testid="auth-particle-field"
  />
</template>

<style scoped>
.auth-particle-field {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
</style>
