import { computed, shallowRef } from 'vue'
import { collectionURL, createCollectionPager, mergeCollectionItems } from '@/lib/collection-pager'
import { apiRequest } from '@/lib/http'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginListResponse, PluginSummary } from '@/types/api'

// Each consumer owns its query and cursor. Shared plugin state only remembers
// identities and live updates; visiting another page cannot replace this list.
export function usePluginCollection() {
  const plugins = usePluginsStore()
  const pageItems = shallowRef<PluginSummary[]>([])
  const pager = createCollectionPager<PluginListResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/plugins', query, cursor), { signal }),
    apply: (response, append) => {
      pageItems.value = append ? mergeCollectionItems(pageItems.value, response.items, plugin => plugin.id) : response.items
      plugins.rememberSummaries(response.items)
    },
  })
  const items = computed(() => {
    const known = new Map(plugins.knownItems.map(plugin => [plugin.id, plugin]))
    return pageItems.value.flatMap(plugin => known.has(plugin.id) ? [known.get(plugin.id)!] : [])
  })
  return { ...pager, items }
}
