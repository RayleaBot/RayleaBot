/**
 * Liquid Glass lenses for the management shell. The material itself is CSS (`_glass.scss`); a lens
 * opts in with `data-glass="clear"` and, where backdrop-filter accepts an SVG filter (Chromium),
 * its backdrop passes through a displacement filter sized to the element. The profile follows the
 * Launcher's lenses: a squircle bevel refracting at n = 1.5 pulls the content below into a band
 * along the rim, and each colour channel bends by a slightly different amount so the rim splits
 * like thick glass. WebKit, Gecko, reduced transparency and forced colors keep the plain material.
 */

type LensSpec = { width: number; height: number; radius: number }

type LensGeometry = { bevel: number; gap: number; maxShift: number }

type Edge = { depth: number; nx: number; ny: number }

type FilterRecord<T> = {
  id: string
  resource: T
  ready: Promise<boolean>
  users: number
  releaseTimer: ReturnType<typeof setTimeout> | undefined
}

type LensState = { key: string | null; token: number }

const SVG_NS = 'http://www.w3.org/2000/svg'
const REFRACTIVE_INDEX = 1.5
const PROFILE_STEPS = 512
const FILTER_RELEASE_MS = 4000
// A lens whose size keeps changing (window drags, collapsing panes) drops its filter and waits for the
// size to settle, instead of rendering a full displacement map for every intermediate size.
const RESIZE_SETTLE_MS = 150
// Lenses inside a scrolling container skip refraction until it has been still this long (see _glass.scss).
const MOTION_SETTLE_MS = 160
// The map is smooth, so it is rendered at most this many pixels on its long edge and stretched by feImage.
const MAP_MAX_EDGE = 512
// Below this short edge the colour split is invisible, so a lens uses one displacement instead of three.
const DISPERSION_MIN_SIZE = 160
// Green and blue bend slightly less than red; the offset only shows along the rim.
const CHANNEL_SPREAD = [1, 0.94, 0.88] as const

const clamp = (value: number, low: number, high: number) => Math.min(high, Math.max(low, value))

const squircle = (x: number) => Math.pow(Math.max(0, 1 - Math.pow(1 - x, 4)), 0.25)

function squircleSlope(x: number) {
  const low = Math.max(0, x - 1e-4)
  const high = Math.min(1, x + 1e-4)
  return (squircle(high) - squircle(low)) / (high - low)
}

/** Distance inside the rounded rectangle and the outward normal of its nearest edge. */
function measureEdge(edge: Edge, px: number, py: number, halfWidth: number, halfHeight: number, radius: number) {
  const qx = Math.abs(px) - (halfWidth - radius)
  const qy = Math.abs(py) - (halfHeight - radius)
  const ox = Math.max(qx, 0)
  const oy = Math.max(qy, 0)
  const outside = Math.hypot(ox, oy)
  edge.depth = radius - outside - Math.min(Math.max(qx, qy), 0)
  edge.nx = 0
  edge.ny = 0
  if (outside > 0) {
    edge.nx = (Math.sign(px) * ox) / outside
    edge.ny = (Math.sign(py) * oy) / outside
  } else if (qx > qy) {
    edge.nx = Math.sign(px) || 1
  } else {
    edge.ny = Math.sign(py) || 1
  }
  return edge
}

/**
 * Inward sampling offset for a view ray that meets the bevel at `x = depth / bevel`. The glass floats
 * `gap` pixels above the content, so rays near the edge land far inside; that content is compressed
 * into the band just inside the rim.
 */
export function rimShift(x: number, geometry: LensGeometry) {
  if (x >= 1) return 0
  const incidence = Math.atan(squircleSlope(Math.max(x, 1e-4)))
  const deviation = incidence - Math.asin(Math.sin(incidence) / REFRACTIVE_INDEX)
  const exit = REFRACTIVE_INDEX * Math.sin(deviation)
  if (exit >= 0.9995) return Number.POSITIVE_INFINITY
  return geometry.bevel * squircle(x) * Math.tan(deviation) + geometry.gap * Math.tan(Math.asin(exit))
}

export function lensGeometry(spec: LensSpec): LensGeometry {
  const size = Math.min(spec.width, spec.height)
  const bevel = clamp(size * 0.19, 4, 36)
  return { bevel, gap: bevel * 1.1, maxShift: clamp(size * 0.36, 4, 64) }
}

// R and G push each backdrop sample toward the center; 128 means no shift.
export function displacementPixels(spec: LensSpec, geometry: LensGeometry) {
  const profile = new Float32Array(PROFILE_STEPS + 1)
  for (let index = 0; index <= PROFILE_STEPS; index += 1) {
    profile[index] = Math.min(rimShift(index / PROFILE_STEPS, geometry), geometry.maxShift) / geometry.maxShift
  }
  const { width, height, radius } = spec
  const ratio = Math.min(1, MAP_MAX_EDGE / Math.max(width, height))
  const mapWidth = Math.max(1, Math.round(width * ratio))
  const mapHeight = Math.max(1, Math.round(height * ratio))
  const pixels = new Uint8ClampedArray(mapWidth * mapHeight * 4)
  const halfWidth = width / 2
  const halfHeight = height / 2
  const edge: Edge = { depth: 0, nx: 0, ny: 0 }
  for (let y = 0; y < mapHeight; y += 1) {
    for (let x = 0; x < mapWidth; x += 1) {
      measureEdge(edge, ((x + 0.5) / mapWidth) * width - halfWidth, ((y + 0.5) / mapHeight) * height - halfHeight, halfWidth, halfHeight, radius)
      const step = Math.min(PROFILE_STEPS, Math.round((edge.depth / geometry.bevel) * PROFILE_STEPS))
      const magnitude = edge.depth > 0 && edge.depth < geometry.bevel ? profile[step] ?? 0 : 0
      const offset = (y * mapWidth + x) * 4
      pixels[offset] = Math.round(127.5 - edge.nx * magnitude * 127.5)
      pixels[offset + 1] = Math.round(127.5 - edge.ny * magnitude * 127.5)
      pixels[offset + 2] = 128
      pixels[offset + 3] = 255
    }
  }
  return { width: mapWidth, height: mapHeight, pixels }
}

// PNG encoding runs off the main thread where OffscreenCanvas exists; blob URLs are revoked with the filter.
async function encodeMap(map: ReturnType<typeof displacementPixels>) {
  const image = new ImageData(map.pixels, map.width, map.height)
  if (typeof OffscreenCanvas !== 'undefined') {
    const canvas = new OffscreenCanvas(map.width, map.height)
    const context = canvas.getContext('2d')
    if (!context) return null
    context.putImageData(image, 0, 0)
    return URL.createObjectURL(await canvas.convertToBlob({ type: 'image/png' }))
  }
  const canvas = document.createElement('canvas')
  canvas.width = map.width
  canvas.height = map.height
  const context = canvas.getContext('2d')
  if (!context) return null
  context.putImageData(image, 0, 0)
  return canvas.toDataURL()
}

function svgElement(tag: string, attributes: Record<string, string | number>) {
  const node = document.createElementNS(SVG_NS, tag)
  for (const [name, value] of Object.entries(attributes)) node.setAttribute(name, String(value))
  return node
}

const CHANNEL_MATRICES = [
  '1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0',
  '0 0 0 0 0  0 1 0 0 0  0 0 0 0 0  0 0 0 1 0',
  '0 0 0 0 0  0 0 0 0 0  0 0 1 0 0  0 0 0 1 0',
] as const

function createLensFilter(spec: LensSpec, id: string, host: Element) {
  const geometry = lensGeometry(spec)
  const filter = svgElement('filter', {
    id,
    x: 0,
    y: 0,
    width: spec.width,
    height: spec.height,
    filterUnits: 'userSpaceOnUse',
    primitiveUnits: 'userSpaceOnUse',
    'color-interpolation-filters': 'sRGB',
  })
  host.append(filter)
  const add = (tag: string, attributes: Record<string, string | number>) => {
    const node = svgElement(tag, attributes)
    filter.append(node)
    return node
  }
  const mapImage = add('feImage', { x: 0, y: 0, width: spec.width, height: spec.height, preserveAspectRatio: 'none', result: 'map' })
  if (Math.min(spec.width, spec.height) >= DISPERSION_MIN_SIZE) {
    CHANNEL_SPREAD.forEach((factor, channel) => {
      add('feDisplacementMap', {
        in: 'SourceGraphic',
        in2: 'map',
        scale: (2 * geometry.maxShift * factor).toFixed(2),
        xChannelSelector: 'R',
        yChannelSelector: 'G',
        result: `shift${channel}`,
      })
      add('feColorMatrix', { in: `shift${channel}`, type: 'matrix', values: CHANNEL_MATRICES[channel], result: `channel${channel}` })
    })
    add('feComposite', { in: 'channel0', in2: 'channel1', operator: 'arithmetic', k1: 0, k2: 1, k3: 1, k4: 0, result: 'redGreen' })
    add('feComposite', { in: 'redGreen', in2: 'channel2', operator: 'arithmetic', k1: 0, k2: 1, k3: 1, k4: 0, result: 'lens' })
  } else {
    add('feDisplacementMap', {
      in: 'SourceGraphic',
      in2: 'map',
      scale: (2 * geometry.maxShift).toFixed(2),
      xChannelSelector: 'R',
      yChannelSelector: 'G',
      result: 'lens',
    })
  }
  // The displacement samples whole pixels, which steps where the rim compresses the backdrop.
  add('feGaussianBlur', { in: 'lens', stdDeviation: 0.4, edgeMode: 'duplicate' })

  const ready = encodeMap(displacementPixels(spec, geometry)).then(async (url) => {
    if (!url) return false
    mapImage.setAttribute('href', url)
    filter.setAttribute('data-map-url', url)
    const probe = new Image()
    probe.src = url
    await probe.decode()
    return true
  }).catch(() => false)
  return { resource: filter, ready }
}

/** Shares one lens filter per geometry and removes filters a while after the last lens lets go. */
export class GlassFilterRegistry<T> {
  private readonly records = new Map<string, FilterRecord<T>>()
  private sequence = 0

  constructor(
    private readonly create: (id: string, key: string) => { resource: T; ready: Promise<boolean> },
    private readonly destroy: (resource: T) => void,
    private readonly releaseDelay = FILTER_RELEASE_MS,
  ) {}

  get size() {
    return this.records.size
  }

  acquire(key: string): FilterRecord<T> {
    let record = this.records.get(key)
    if (!record) {
      const id = `app-glass-${(this.sequence += 1)}`
      record = { id, users: 0, releaseTimer: undefined, ...this.create(id, key) }
      this.records.set(key, record)
    }
    clearTimeout(record.releaseTimer)
    record.releaseTimer = undefined
    record.users += 1
    return record
  }

  release(key: string) {
    const record = this.records.get(key)
    if (!record) return
    record.users = Math.max(0, record.users - 1)
    if (record.users > 0 || record.releaseTimer !== undefined) return
    record.releaseTimer = setTimeout(() => {
      record.releaseTimer = undefined
      if (record.users > 0 || this.records.get(key) !== record) return
      this.records.delete(key)
      this.destroy(record.resource)
    }, this.releaseDelay)
  }

  clear() {
    for (const record of this.records.values()) {
      clearTimeout(record.releaseTimer)
      this.destroy(record.resource)
    }
    this.records.clear()
  }
}

const SOFTWARE_RENDERERS = /swiftshader|llvmpipe|softpipe|software|basic render/i

/**
 * True when the page renders without GPU acceleration: blocklisted drivers, sessions without a GPU or hardware
 * acceleration turned off. Backdrop filters are then rendered on the CPU, so the glass goes lite (opaque).
 * Chromium may still hand out a SwiftShader context despite failIfMajorPerformanceCaveat, so the renderer
 * name is checked as well.
 */
export function rendersInSoftware(): boolean {
  if (typeof document === 'undefined') return false
  try {
    const canvas = document.createElement('canvas')
    const options: WebGLContextAttributes = { failIfMajorPerformanceCaveat: true }
    const context = canvas.getContext('webgl2', options) ?? canvas.getContext('webgl', options)
    if (!context) return true
    const info = context.getExtension('WEBGL_debug_renderer_info')
    const renderer = String(context.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : context.RENDERER) ?? '')
    context.getExtension('WEBGL_lose_context')?.loseContext()
    return SOFTWARE_RENDERERS.test(renderer)
  } catch {
    return false
  }
}

/** WebKit and Gecko do not take an SVG filter in backdrop-filter. */
export function supportsGlassRefraction(): boolean {
  if (typeof window === 'undefined' || typeof CSS === 'undefined' || typeof CSS.supports !== 'function') return false
  if (CSS.supports('-webkit-backdrop-filter', 'blur(1px)') || CSS.supports('-moz-appearance', 'none')) return false
  if (!CSS.supports('backdrop-filter', 'blur(1px)')) return false
  return !window.matchMedia?.('(prefers-reduced-transparency: reduce)').matches
    && !window.matchMedia?.('(forced-colors: active)').matches
}

function radiusOf(style: CSSStyleDeclaration, width: number, height: number) {
  const first = style.borderTopLeftRadius.split(' ')[0] ?? '0'
  const value = Number.parseFloat(first) || 0
  const radius = first.endsWith('%') ? (value / 100) * Math.min(width, height) : value
  return Math.round(Math.min(radius, width / 2, height / 2) * 4) / 4
}

const LENS_SELECTOR = '[data-glass="clear"]'

const isLens = (node: Node): node is HTMLElement => node instanceof HTMLElement && node.dataset.glass === 'clear'

/** Tracks clear lenses under `root` and keeps each one on the refraction filter for its size. */
export function startGlassSurfaces(root: HTMLElement): () => void {
  if (typeof ResizeObserver === 'undefined' || typeof MutationObserver === 'undefined') {
    return () => undefined
  }

  let host: Element | null = null
  const filterHost = () => {
    if (!host) {
      const svg = svgElement('svg', { width: 0, height: 0, 'aria-hidden': 'true', focusable: 'false', class: 'app-glass-filters' })
      host = svgElement('defs', {})
      svg.append(host)
      document.body.append(svg)
    }
    return host
  }
  const registry = new GlassFilterRegistry<Element>(
    (id, key) => createLensFilter(JSON.parse(key) as LensSpec, id, filterHost()),
    (filter) => {
      const url = filter.getAttribute('data-map-url')
      if (url?.startsWith('blob:')) URL.revokeObjectURL(url)
      filter.remove()
    },
  )
  const lenses = new Map<HTMLElement, LensState>()
  let lite = false
  let refracts = supportsGlassRefraction()
  document.documentElement.toggleAttribute('data-glass-refraction', refracts)

  const clearLens = (element: HTMLElement, state: LensState) => {
    state.token += 1
    if (state.key) registry.release(state.key)
    state.key = null
    element.style.removeProperty('--glass-filter')
    delete element.dataset.glassReady
  }

  const refresh = (element: HTMLElement) => {
    const state = lenses.get(element)
    if (!state) return
    const width = Math.round(element.offsetWidth)
    const height = Math.round(element.offsetHeight)
    if (!refracts || !element.isConnected || width < 2 || height < 2) {
      clearLens(element, state)
      return
    }
    const key = JSON.stringify({ width, height, radius: radiusOf(getComputedStyle(element), width, height) } satisfies LensSpec)
    if (key === state.key) return
    const token = (state.token += 1)
    const record = registry.acquire(key)
    void record.ready.then((ok) => {
      if (state.token !== token || lenses.get(element) !== state || !ok) {
        registry.release(key)
        return
      }
      const previous = state.key
      state.key = key
      element.style.setProperty('--glass-filter', `url(#${record.id})`)
      element.dataset.glassReady = ''
      if (previous) registry.release(previous)
    })
  }

  // Lenses whose size changed wait in one batch until sizes settle, and until any running view transition
  // has finished, so map generation and style reads never land inside a page or theme animation.
  const pendingLenses = new Set<HTMLElement>()
  let settleTimer: ReturnType<typeof setTimeout> | undefined
  const flushPending = () => {
    settleTimer = undefined
    if (document.querySelector('[data-view-transition-kind]')) {
      settleTimer = setTimeout(flushPending, RESIZE_SETTLE_MS)
      return
    }
    const batch = [...pendingLenses]
    pendingLenses.clear()
    batch.forEach(refresh)
  }
  const scheduleRefresh = (element: HTMLElement) => {
    const state = lenses.get(element)
    if (state?.key) clearLens(element, state)
    pendingLenses.add(element)
    clearTimeout(settleTimer)
    settleTimer = setTimeout(flushPending, RESIZE_SETTLE_MS)
  }

  const resizeObserver = new ResizeObserver((entries) => {
    for (const entry of entries) scheduleRefresh(entry.target as HTMLElement)
  })

  const attach = (element: HTMLElement) => {
    if (lenses.has(element)) return
    lenses.set(element, { key: null, token: 0 })
    resizeObserver.observe(element)
  }

  const forget = (element: HTMLElement) => {
    const state = lenses.get(element)
    if (!state) return
    clearLens(element, state)
    pendingLenses.delete(element)
    lenses.delete(element)
    resizeObserver.unobserve(element)
  }

  const visitLenses = (node: Node, visit: (element: HTMLElement) => void) => {
    if (!(node instanceof HTMLElement)) return
    if (isLens(node)) visit(node)
    node.querySelectorAll<HTMLElement>(LENS_SELECTOR).forEach(visit)
  }

  const mutationObserver = new MutationObserver((records) => {
    for (const record of records) {
      if (record.type === 'attributes') {
        if (isLens(record.target)) attach(record.target)
        else if (record.target instanceof HTMLElement) forget(record.target)
        continue
      }
      record.removedNodes.forEach((node) => visitLenses(node, (element) => {
        if (!element.isConnected) forget(element)
      }))
      record.addedNodes.forEach((node) => visitLenses(node, attach))
    }
  })
  mutationObserver.observe(root, { subtree: true, childList: true, attributes: true, attributeFilter: ['data-glass'] })

  // A scrolling container moves its lenses over the field; while it moves, the lenses inside skip the
  // displacement filter (see _glass.scss) instead of re-rendering it for every frame.
  const motionTimers = new Map<Element, ReturnType<typeof setTimeout>>()
  const onScroll = (event: Event) => {
    if (!refracts) return
    const scroller = event.target instanceof Element ? event.target : document.documentElement
    if (!scroller.hasAttribute('data-glass-motion')) scroller.setAttribute('data-glass-motion', '')
    clearTimeout(motionTimers.get(scroller))
    motionTimers.set(scroller, setTimeout(() => {
      motionTimers.delete(scroller)
      scroller.removeAttribute('data-glass-motion')
    }, MOTION_SETTLE_MS))
  }
  document.addEventListener('scroll', onScroll, { capture: true, passive: true })

  const preferenceQueries = ['(prefers-reduced-transparency: reduce)', '(forced-colors: active)']
    .map((query) => window.matchMedia?.(query))
    .filter((query): query is MediaQueryList => Boolean(query))
  const onPreferenceChange = () => {
    refracts = supportsGlassRefraction() && !lite
    document.documentElement.toggleAttribute('data-glass-refraction', refracts)
    for (const element of lenses.keys()) refresh(element)
  }
  preferenceQueries.forEach((query) => query.addEventListener('change', onPreferenceChange))

  // Probing WebGL costs a context creation, so it waits for an idle moment after start.
  const idle = typeof window.requestIdleCallback === 'function'
    ? (callback: () => void) => window.requestIdleCallback(callback, { timeout: 3000 })
    : (callback: () => void) => window.setTimeout(callback, 1000)
  const cancelIdle = typeof window.cancelIdleCallback === 'function'
    ? (handle: number) => window.cancelIdleCallback(handle)
    : (handle: number) => window.clearTimeout(handle)
  const liteProbe = idle(() => {
    if (!rendersInSoftware()) return
    lite = true
    document.documentElement.setAttribute('data-glass-lite', '')
    onPreferenceChange()
  })

  root.querySelectorAll<HTMLElement>(LENS_SELECTOR).forEach(attach)

  return () => {
    mutationObserver.disconnect()
    resizeObserver.disconnect()
    clearTimeout(settleTimer)
    pendingLenses.clear()
    document.removeEventListener('scroll', onScroll, { capture: true })
    motionTimers.forEach((timer, scroller) => {
      clearTimeout(timer)
      scroller.removeAttribute('data-glass-motion')
    })
    motionTimers.clear()
    preferenceQueries.forEach((query) => query.removeEventListener('change', onPreferenceChange))
    cancelIdle(liteProbe)
    document.documentElement.removeAttribute('data-glass-lite')
    for (const [element, state] of lenses) clearLens(element, state)
    lenses.clear()
    registry.clear()
    document.documentElement.removeAttribute('data-glass-refraction')
    host?.parentElement?.remove()
  }
}
