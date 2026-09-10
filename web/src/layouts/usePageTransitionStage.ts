import { computed, inject, onActivated, onDeactivated, onScopeDispose, readonly, ref, watch, type ComputedRef, type InjectionKey, type Ref } from 'vue'

export type PageTransitionStage = 'entering' | 'idle'

export const PAGE_TRANSITION_STAGE_KEY: InjectionKey<Readonly<Ref<PageTransitionStage>>>
  = Symbol('pageTransitionStage')

const defaultStage = readonly(ref<PageTransitionStage>('idle'))

export function usePageTransitionStage(): Readonly<Ref<PageTransitionStage>> {
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
