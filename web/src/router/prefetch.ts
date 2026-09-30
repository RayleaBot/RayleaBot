import type { RouteComponent, Router } from 'vue-router'

type LazyRouteComponent = () => Promise<RouteComponent>

type IdleScheduler = {
  request: (callback: () => void) => number
  cancel: (handle: number) => void
}

const START_DELAY_MS = 1500

function idleScheduler(): IdleScheduler {
  if (typeof window.requestIdleCallback === 'function') {
    return {
      request: callback => window.requestIdleCallback(callback, { timeout: 1000 }),
      cancel: handle => window.cancelIdleCallback(handle),
    }
  }
  return {
    request: callback => window.setTimeout(callback, 200),
    cancel: handle => window.clearTimeout(handle),
  }
}

/** Lazy page components of the given routes, excluding development-only pages. */
export function lazyRouteComponents(router: Router): LazyRouteComponent[] {
  const loaders = new Set<LazyRouteComponent>()
  for (const record of router.getRoutes()) {
    if (record.path.startsWith('/__dev')) continue
    for (const component of Object.values(record.components ?? {})) {
      // Route records hold page SFCs as objects; a function here is a `() => import()` loader.
      if (typeof component === 'function') loaders.add(component as LazyRouteComponent)
    }
  }
  return [...loaders]
}

/**
 * Warms each lazy page chunk once the workspace is idle, one page per idle callback, so the first visit to
 * a page does not download and evaluate its chunk inside the page transition. Skipped when the browser
 * asks to save data; a failed prefetch is left for the real navigation to report.
 */
export function prefetchRouteComponents(router: Router): () => void {
  if (typeof window === 'undefined') return () => undefined
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection
  if (connection?.saveData) return () => undefined

  const scheduler = idleScheduler()
  const pending = lazyRouteComponents(router)
  let handle: number | null = null
  let stopped = false

  const next = () => {
    handle = null
    const loader = pending.shift()
    if (stopped || !loader) return
    void loader().catch(() => undefined).finally(() => {
      if (!stopped) handle = scheduler.request(next)
    })
  }

  const startTimer = window.setTimeout(() => { handle = scheduler.request(next) }, START_DELAY_MS)
  return () => {
    stopped = true
    window.clearTimeout(startTimer)
    if (handle !== null) scheduler.cancel(handle)
  }
}
