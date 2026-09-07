import { computed } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { motionDuration, motionEase } from './runtime'

export function useOverlayMotion() {
  const reduce = useMediaQuery('(prefers-reduced-motion: reduce), (forced-colors: active)')
  return computed(() => ({
    initial: { opacity: 0, scale: reduce.value ? 1 : 0.96 },
    animate: { opacity: 1, scale: 1 },
    exit: { opacity: 0, scale: reduce.value ? 1 : 0.96, transition: { duration: reduce.value ? 0 : motionDuration.control / 1000 } },
    transition: { duration: reduce.value ? 0 : motionDuration.overlay / 1000, ease: [...motionEase] },
  }))
}
