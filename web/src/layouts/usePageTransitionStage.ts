import { computed, inject, onActivated, onBeforeUnmount, onDeactivated, onMounted, onScopeDispose, provide, readonly, ref, watch, type ComputedRef, type InjectionKey, type Ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'

import {
  cancelRouteFallbackMotion,
  isManagedViewTransitionActive,
  runRouteFallbackMotion,
  subscribeRouteMotion,
  type PageMotionProfile,
} from '@/motion/runtime'

export type PageTransitionStage = 'entering' | 'idle'

export const PAGE_TRANSITION_STAGE_KEY: InjectionKey<Readonly<Ref<PageTransitionStage>>>
  = Symbol('pageTransitionStage')

const defaultStage = readonly(ref<PageTransitionStage>('idle'))

// The layout reports whether the entering page is still animating so heavy pages can wait.
export function providePageTransitionStage(profile: Ref<PageMotionProfile>) {
  const reducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)')
  const skipStage = computed(() => profile.value === 'none' || reducedMotion.value)
  const activeStage = ref<PageTransitionStage>('idle')
  provide(PAGE_TRANSITION_STAGE_KEY, readonly(computed(() => skipStage.value ? 'idle' : activeStage.value)))

  let unsubscribe: (() => void) | null = null
  onMounted(() => {
    unsubscribe = subscribeRouteMotion((active) => { activeStage.value = active ? 'entering' : 'idle' })
  })
  onBeforeUnmount(() => {
    unsubscribe?.()
    unsubscribe = null
  })

  return {
    onBeforeEnter() {
      activeStage.value = skipStage.value ? 'idle' : 'entering'
    },
    onEnter(element: Element, done: () => void) {
      runRouteFallbackMotion(element as HTMLElement, 'enter', profile.value, done)
    },
    onLeave(element: Element, done: () => void) {
      runRouteFallbackMotion(element as HTMLElement, 'leave', profile.value, done)
    },
    onAfterEnter() {
      if (!isManagedViewTransitionActive()) activeStage.value = 'idle'
    },
    onCancelled(element: Element) {
      cancelRouteFallbackMotion(element as HTMLElement)
      activeStage.value = 'idle'
    },
  }
}

function usePageTransitionStage(): Readonly<Ref<PageTransitionStage>> {
  return inject(PAGE_TRANSITION_STAGE_KEY, defaultStage)
}

export function useReadyToRenderHeavyContent(): ComputedRef<boolean> {
  const stage = usePageTransitionStage()
  return computed(() => stage.value === 'idle')
}

export function useHeavyContentGate() {
  const ready = useReadyToRenderHeavyContent()
  const pending = new Set<(ready: boolean) => void>()
  let active = true
  function settle(value: boolean) {
    for (const resolve of pending) resolve(value)
    pending.clear()
  }
  watch(ready, value => { if (value) settle(active) })
  onActivated(() => { active = true })
  const cancel = () => { active = false; settle(false) }
  onDeactivated(cancel)
  onScopeDispose(cancel)
  function waitUntilReady(): Promise<boolean> {
    if (!active || ready.value) return Promise.resolve(active)
    return new Promise(resolve => { pending.add(resolve) })
  }
  return { readyToRenderHeavyContent: ready, waitUntilReady }
}
