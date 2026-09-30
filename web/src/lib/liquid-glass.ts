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
function displacementMap(spec: LensSpec, geometry: LensGeometry) {
  const profile = new Float32Array(PROFILE_STEPS + 1)
  for (let index = 0; index <= PROFILE_STEPS; index += 1) {
    profile[index] = Math.min(rimShift(index / PROFILE_STEPS, geometry), geometry.maxShift) / geometry.maxShift
  }
  const { width, height, radius } = spec
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) return null
  const image = context.createImageData(width, height)
  const halfWidth = width / 2
  const halfHeight = height / 2
  const edge: Edge = { depth: 0, nx: 0, ny: 0 }
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      measureEdge(edge, x + 0.5 - halfWidth, y + 0.5 - halfHeight, halfWidth, halfHeight, radius)
      const step = Math.min(PROFILE_STEPS, Math.round((edge.depth / geometry.bevel) * PROFILE_STEPS))
      const magnitude = edge.depth > 0 && edge.depth < geometry.bevel ? profile[step] ?? 0 : 0
      const offset = (y * width + x) * 4
      image.data[offset] = Math.round(127.5 - edge.nx * magnitude * 127.5)
      image.data[offset + 1] = Math.round(127.5 - edge.ny * magnitude * 127.5)
      image.data[offset + 2] = 128
      image.data[offset + 3] = 255
    }
  }
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
  const map = displacementMap(spec, geometry)
  if (!map) return { resource: filter, ready: Promise.resolve(false) }

  const add = (tag: string, attributes: Record<string, string | number>) => filter.append(svgElement(tag, attributes))
  add('feImage', { href: map, x: 0, y: 0, width: spec.width, height: spec.height, preserveAspectRatio: 'none', result: 'map' })
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
  // The displacement samples whole pixels, which steps where the rim compresses the backdrop.
  add('feGaussianBlur', { in: 'lens', stdDeviation: 0.4, edgeMode: 'duplicate' })

  const image = new Image()
  image.src = map
  return { resource: filter, ready: image.decode().then(() => true, () => false) }
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
    (filter) => filter.remove(),
  )
  const lenses = new Map<HTMLElement, LensState>()
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

  const settleTimers = new Map<HTMLElement, ReturnType<typeof setTimeout>>()
  const scheduleRefresh = (element: HTMLElement) => {
    const state = lenses.get(element)
    if (state?.key) clearLens(element, state)
    clearTimeout(settleTimers.get(element))
    settleTimers.set(element, setTimeout(() => {
      settleTimers.delete(element)
      refresh(element)
    }, RESIZE_SETTLE_MS))
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
    clearTimeout(settleTimers.get(element))
    settleTimers.delete(element)
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

  const preferenceQueries = ['(prefers-reduced-transparency: reduce)', '(forced-colors: active)']
    .map((query) => window.matchMedia?.(query))
    .filter((query): query is MediaQueryList => Boolean(query))
  const onPreferenceChange = () => {
    refracts = supportsGlassRefraction()
    document.documentElement.toggleAttribute('data-glass-refraction', refracts)
    for (const element of lenses.keys()) refresh(element)
  }
  preferenceQueries.forEach((query) => query.addEventListener('change', onPreferenceChange))

  root.querySelectorAll<HTMLElement>(LENS_SELECTOR).forEach(attach)

  return () => {
    mutationObserver.disconnect()
    resizeObserver.disconnect()
    settleTimers.forEach(timer => clearTimeout(timer))
    settleTimers.clear()
    preferenceQueries.forEach((query) => query.removeEventListener('change', onPreferenceChange))
    for (const [element, state] of lenses) clearLens(element, state)
    lenses.clear()
    registry.clear()
    document.documentElement.removeAttribute('data-glass-refraction')
    host?.parentElement?.remove()
  }
}
