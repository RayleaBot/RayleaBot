import { onBeforeUnmount, onMounted, shallowRef, useId, type Ref } from 'vue'

import { prefersReducedMotion } from '@/motion/runtime'

/** A rounded lens normal map. Only the bevel bends light; the center stays clear. */
export function createGlassDisplacement(width: number, height: number, radius: number) {
  const ratio = Math.min(1, 720 / Math.max(width, height))
  const mapWidth = Math.max(1, Math.round(width * ratio))
  const mapHeight = Math.max(1, Math.round(height * ratio))
  const pixels = new Uint8ClampedArray(mapWidth * mapHeight * 4)
  const bevel = 18

  for (let y = 0; y < mapHeight; y += 1) {
    for (let x = 0; x < mapWidth; x += 1) {
      const px = (x + .5) / mapWidth * width - width / 2
      const py = (y + .5) / mapHeight * height - height / 2
      const qx = Math.abs(px) - (width / 2 - radius)
      const qy = Math.abs(py) - (height / 2 - radius)
      const ox = Math.max(qx, 0)
      const oy = Math.max(qy, 0)
      const length = Math.hypot(ox, oy)
      const distance = radius - length - Math.min(Math.max(qx, qy), 0)
      const bend = distance > 0 && distance < bevel ? Math.sin(distance / bevel * Math.PI) ** .8 : 0
      const nx = length > 0 ? Math.sign(px) * ox / length : 0
      const ny = length > 0 ? Math.sign(py) * oy / length : 0
      const index = (y * mapWidth + x) * 4
      pixels[index] = Math.round(127.5 - nx * bend * 127)
      pixels[index + 1] = Math.round(127.5 - ny * bend * 127)
      pixels[index + 2] = 128
      pixels[index + 3] = 255
    }
  }
  return { width: mapWidth, height: mapHeight, pixels }
}

/**
 * The lens is only consumed where `backdrop-filter` accepts an SVG filter reference.
 * WebKit and Gecko fall back to the flat CSS material, and reduced-transparency and
 * forced-colors drop the backdrop entirely, so the raster is pure waste there.
 */
export function supportsGlassRefraction() {
  if (typeof window === 'undefined' || typeof CSS === 'undefined' || typeof CSS.supports !== 'function') return false
  if (CSS.supports('-webkit-backdrop-filter', 'blur(1px)') || CSS.supports('-moz-appearance', 'none')) return false
  if (!CSS.supports('backdrop-filter', 'blur(1px)')) return false
  return !window.matchMedia('(prefers-reduced-transparency: reduce)').matches
    && !window.matchMedia('(forced-colors: active)').matches
}

export function useLiquidGlass(surface: Ref<HTMLElement | null>) {
  const filterId = `auth-glass-${useId()}`
  const displacement = shallowRef<{ width: number, height: number, url: string } | null>(null)
  let observer: ResizeObserver | undefined
  let frame = 0
  let highlightFrame = 0
  let pointerX = 0
  let pointerY = 0
  let renderedSize = ''
  let pendingSize = ''
  let pendingImage: HTMLImageElement | undefined
  let disposed = false

  function releasePendingImage() {
    if (!pendingImage) return
    pendingImage.onload = null
    pendingImage.onerror = null
    pendingImage.src = ''
    pendingImage = undefined
  }

  function refresh() {
    frame = 0
    const element = surface.value
    if (!element || disposed) return
    const width = element.offsetWidth
    const height = element.offsetHeight
    if (!width || !height) return
    // Gate on the box before reading style: an unchanged panel must not force a recalc.
    const size = `${width}:${height}`
    if (size === renderedSize || size === pendingSize) return
    const radius = Number.parseFloat(getComputedStyle(element).borderTopLeftRadius)
    const map = createGlassDisplacement(width, height, radius)
    const canvas = document.createElement('canvas')
    canvas.width = map.width
    canvas.height = map.height
    const context = canvas.getContext('2d')
    if (!context) return
    const data = context.createImageData(map.width, map.height)
    data.data.set(map.pixels)
    context.putImageData(data, 0, 0)
    const url = canvas.toDataURL()
    // Wait for decoding before the SVG filter replaces the CSS-only material.
    releasePendingImage()
    pendingSize = size
    const image = new Image()
    pendingImage = image
    image.onload = () => {
      pendingImage = undefined
      pendingSize = ''
      if (disposed) return
      renderedSize = size
      displacement.value = { width, height, url }
    }
    // Release the key so a later refresh at this geometry can retry.
    image.onerror = () => {
      pendingImage = undefined
      pendingSize = ''
    }
    image.src = url
  }

  function scheduleRefresh() {
    if (!frame) frame = requestAnimationFrame(refresh)
  }

  onMounted(() => {
    if (typeof ResizeObserver === 'undefined' || !supportsGlassRefraction()) return
    observer = new ResizeObserver(scheduleRefresh)
    if (surface.value) observer.observe(surface.value)
    window.addEventListener('resize', scheduleRefresh)
    scheduleRefresh()
  })

  onBeforeUnmount(() => {
    disposed = true
    observer?.disconnect()
    window.removeEventListener('resize', scheduleRefresh)
    cancelAnimationFrame(frame)
    cancelAnimationFrame(highlightFrame)
    releasePendingImage()
  })

  function applyHighlight() {
    highlightFrame = 0
    const element = surface.value
    if (!element || disposed || prefersReducedMotion()) return
    const rect = element.getBoundingClientRect()
    element.style.setProperty('--glass-light-x', `${(pointerX - rect.left) / rect.width * 100}%`)
    element.style.setProperty('--glass-light-y', `${(pointerY - rect.top) / rect.height * 100}%`)
  }

  function moveHighlight(event: PointerEvent) {
    if (event.pointerType !== 'mouse') return
    pointerX = event.clientX
    pointerY = event.clientY
    if (!highlightFrame) highlightFrame = requestAnimationFrame(applyHighlight)
  }

  function resetHighlight() {
    cancelAnimationFrame(highlightFrame)
    highlightFrame = 0
    surface.value?.style.removeProperty('--glass-light-x')
    surface.value?.style.removeProperty('--glass-light-y')
  }

  return { filterId, displacement, moveHighlight, resetHighlight }
}
