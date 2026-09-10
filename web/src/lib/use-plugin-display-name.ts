import { computed, toValue, watch, type MaybeRefOrGetter } from 'vue'
import { usePluginsStore } from '@/stores/plugins'

// Resolve only a visible log's plugin. Historical records keep their ID when
// the package no longer exists, without repeatedly retrying the same failure.
export function usePluginDisplayName(pluginId: MaybeRefOrGetter<string | undefined>) {
  const plugins = usePluginsStore()
  watch(() => toValue(pluginId), id => {
    if (!id || plugins.getPluginDisplayName(id) !== id || plugins.detailErrorsByPluginId[id]) return
    void plugins.ensureDetail(id).catch(() => undefined)
  }, { immediate: true })
  return computed(() => {
    const id = toValue(pluginId)
    return id ? plugins.getPluginDisplayName(id) : ''
  })
}
