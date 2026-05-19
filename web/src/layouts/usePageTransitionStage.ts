import { computed, inject, readonly, ref, type ComputedRef, type InjectionKey, type Ref } from 'vue'

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
