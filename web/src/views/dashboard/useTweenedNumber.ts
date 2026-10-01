import { computed, onScopeDispose, ref, watch } from 'vue'

import { prefersReducedMotion } from '@/motion/runtime'

const TWEEN_MS = 450

// A live count rolls to its new value; a new subject (another period or connection) shows its number at once.
export function useTweenedNumber(source: () => number, subject: () => string) {
  const shown = ref(source())
  let frame = 0
  watch([source, subject], ([target, key], [, previousKey]) => {
    cancelAnimationFrame(frame)
    if (key !== previousKey || prefersReducedMotion() || typeof requestAnimationFrame !== 'function') {
      shown.value = target
      return
    }
    const from = shown.value
    const started = performance.now()
    const step = (time: number) => {
      const progress = Math.min(1, (time - started) / TWEEN_MS)
      shown.value = from + (target - from) * (1 - (1 - progress) ** 3)
      if (progress < 1) frame = requestAnimationFrame(step)
    }
    frame = requestAnimationFrame(step)
  })
  onScopeDispose(() => cancelAnimationFrame(frame))
  return computed(() => Math.round(shown.value))
}
