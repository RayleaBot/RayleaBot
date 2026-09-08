import { animate, animateMini } from 'motion-v'
import { nextTick } from 'vue'
import type { RouteLocationRaw, Router } from 'vue-router'

export const motionDuration = {
  control: 160,
  content: 200,
  overlay: 220,
} as const

export const motionEase = [0.16, 1, 0.3, 1] as const

export type PageMotionProfile = 'fade' | 'fade-slide' | 'none'
export type ThemeMotionOrigin = { x: number; y: number } | undefined
type ViewTransitionKind = 'route' | 'theme'
type RouteMotionListener = (active: boolean) => void

interface ActiveViewTransition {
  finish: () => void
  transition: ViewTransition
  cancelAnimation?: () => void
}

interface ScopedViewTransitionElement extends HTMLElement {
  startViewTransition?: (update: () => void | Promise<void>) => ViewTransition
}

interface ViewTransitionTarget {
  marker: HTMLElement
  start: (update: () => void | Promise<void>) => ViewTransition
}

let activeViewTransition: ActiveViewTransition | null = null
let viewTransitionSequence = 0
const routeMotionListeners = new Set<RouteMotionListener>()
const elementAnimations = new WeakMap<HTMLElement, ReturnType<typeof animate>>()

export function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined'
    && Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce), (forced-colors: active)').matches)
}

export function supportsViewTransitions(): boolean {
  return typeof document !== 'undefined'
    && typeof document.startViewTransition === 'function'
}

function resolveViewTransitionTarget(kind: ViewTransitionKind): ViewTransitionTarget | null {
  if (typeof document === 'undefined') {
    return null
  }
  if (kind === 'theme') {
    if (typeof document.startViewTransition !== 'function') {
      return null
    }
    return {
      marker: document.documentElement,
      start: (update) => document.startViewTransition(update),
    }
  }

  const workspace = document.querySelector<ScopedViewTransitionElement>('.admin-layout__content')
  if (!workspace || typeof workspace.startViewTransition !== 'function') {
    return null
  }
  return {
    marker: workspace,
    start: (update) => workspace.startViewTransition!(update),
  }
}

export function isManagedViewTransitionActive(): boolean {
  return activeViewTransition !== null
}

export function subscribeRouteMotion(listener: RouteMotionListener): () => void {
  routeMotionListeners.add(listener)
  return () => routeMotionListeners.delete(listener)
}

function notifyRouteMotion(active: boolean) {
  for (const listener of routeMotionListeners) {
    listener(active)
  }
}

function finishActiveViewTransition() {
  activeViewTransition?.cancelAnimation?.()
  activeViewTransition?.transition.skipTransition()
  activeViewTransition?.finish()
}

function startManagedViewTransition(
  kind: ViewTransitionKind,
  update: () => void | Promise<void>,
  profile: PageMotionProfile = 'fade',
): ViewTransition | null {
  finishActiveViewTransition()
  const target = resolveViewTransitionTarget(kind)
  if (!target || prefersReducedMotion() || profile === 'none') {
    void update()
    return null
  }

  const sequence = ++viewTransitionSequence
  target.marker.dataset.viewTransitionKind = kind
  target.marker.dataset.motionProfile = profile
  if (kind === 'route') {
    notifyRouteMotion(true)
  }

  let finished = false
  const finish = () => {
    if (finished) {
      return
    }
    finished = true
    if (kind === 'route') {
      notifyRouteMotion(false)
    }
    if (sequence === viewTransitionSequence) {
      delete target.marker.dataset.viewTransitionKind
      delete target.marker.dataset.motionProfile
      activeViewTransition = null
    }
  }

  const transition = target.start(async () => {
    await update()
    await nextTick()
  })
  activeViewTransition = { finish, transition }
  void transition.finished.catch(() => undefined).finally(finish)
  return transition
}

export function navigateWithMotion(
  router: Router,
  target: RouteLocationRaw,
  profile: PageMotionProfile,
) {
  let navigation: ReturnType<Router['push']> | null = null
  const update = () => {
    navigation = router.push(target)
    return navigation.then(() => undefined)
  }

  const transition = startManagedViewTransition('route', update, profile)
  if (!transition) {
    return navigation ?? router.push(target)
  }
  return transition.updateCallbackDone.then(() => navigation ?? undefined)
}

export function applyThemeWithMotion(update: () => void, origin?: ThemeMotionOrigin): ViewTransition | null {
  const transition = startManagedViewTransition('theme', update)
  if (!transition) return null
  void transition.ready.then(() => {
    if (activeViewTransition?.transition !== transition) return
    const x = Math.max(0, Math.min(window.innerWidth, origin?.x ?? window.innerWidth / 2))
    const y = Math.max(0, Math.min(window.innerHeight, origin?.y ?? window.innerHeight / 2))
    const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
    const options = { duration: origin ? .42 : .28, ease: motionEase, pseudoElement: '::view-transition-new(root)' }
    const controls = animateMini(document.documentElement, origin
      ? { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] }
      : { opacity: [0, 1] }, options)
    activeViewTransition.cancelAnimation = () => controls.cancel()
  }).catch(() => undefined)
  return transition
}

export function runRouteFallbackMotion(
  element: HTMLElement,
  phase: 'enter' | 'leave',
  profile: PageMotionProfile,
  done: () => void,
) {
  const current = elementAnimations.get(element)
  current?.cancel()
  elementAnimations.delete(element)

  if (
    phase === 'leave'
    || profile === 'none'
    || prefersReducedMotion()
    || isManagedViewTransitionActive()
    || typeof element.animate !== 'function'
  ) {
    queueMicrotask(done)
    return
  }

  const y = profile === 'fade-slide' ? 4 : 0
  const keyframes = {
    opacity: [0.88, 1],
    transform: [`translateY(${y}px)`, 'translateY(0)'],
  }
  const controls = animate(element, keyframes, {
    duration: motionDuration.content / 1000,
    ease: motionEase,
  })
  elementAnimations.set(element, controls)
  void controls.then(
    () => {
      elementAnimations.delete(element)
      done()
    },
    () => done(),
  )
}

export function cancelRouteFallbackMotion(element: HTMLElement) {
  elementAnimations.get(element)?.cancel()
  elementAnimations.delete(element)
}
