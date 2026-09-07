import { computed, onBeforeUnmount, ref, type ComputedRef, type InjectionKey } from 'vue'

const activeLayers = ref<symbol[]>([])
export const overlayLayerKey: InjectionKey<ComputedRef<number>> = Symbol('overlay-layer')

export function useOverlayLayer() {
  const id = Symbol('dialog')
  const layer = computed(() => 1200 + Math.max(0, activeLayers.value.indexOf(id)) * 20)
  function activate() {
    if (!activeLayers.value.includes(id)) activeLayers.value = [...activeLayers.value, id]
  }
  function release() { activeLayers.value = activeLayers.value.filter((entry) => entry !== id) }
  onBeforeUnmount(release)
  return { layer, activate, release }
}
