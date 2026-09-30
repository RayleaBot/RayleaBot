import { computed, shallowRef, ref, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'

import { t } from '@/i18n'
import { getPluginStateLabel } from '@/lib/display'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail, PluginState, PluginSummary } from '@/types/api'

type SidebarPluginItem = Pick<PluginSummary, 'id' | 'name'> & {
  icon?: PluginSummary['icon']
  state?: PluginState
  version?: PluginSummary['version']
}

type PluginManagementPage = NonNullable<PluginDetail['management_ui']>['pages'][number]

export type SidebarPluginNavigationEntry =
  | { key: string; kind: 'resource'; plugin: SidebarPluginItem }
  | { key: string; kind: 'overview'; plugin: SidebarPluginItem }
  | { key: string; kind: 'management'; page: PluginManagementPage; plugin: SidebarPluginItem }
  | { key: string; kind: 'retry'; plugin: SidebarPluginItem }

// The plugin center's installed-plugin list: the current page of plugins plus every plugin that
// was visited, each expandable into its overview and declared management pages.
export function useSidebarPluginNavigation(options: {
  activePluginId: Ref<string>
}) {
  const { activePluginId } = options
  const pluginsStore = usePluginsStore()
  const {
    current,
    detailErrorsByPluginId,
    detailLoadingByPluginId,
    detailsByPluginId,
  } = storeToRefs(pluginsStore)

  const pluginCollection = usePluginCollection()
  const { items: sortedItems, total } = pluginCollection
  const pluginFilter = ref('')
  const expandedPluginIds = shallowRef(new Set<string>())
  const visitedPluginIds = shallowRef(new Set<string>())
  watch(() => Object.keys(detailsByPluginId.value), ids => {
    visitedPluginIds.value = new Set([...visitedPluginIds.value, ...ids])
  }, { immediate: true })

  const activePlugin = computed(() => current.value?.id === activePluginId.value ? current.value : null)
  const activePluginSummary = computed(() => pluginsStore.knownItems.find(plugin => plugin.id === activePluginId.value))
  const activePluginName = computed(() => (
    activePlugin.value?.name?.trim()
    || activePluginSummary.value?.name?.trim()
    || pluginsStore.getPluginDisplayName(activePluginId.value)
  ))
  const navigationPlugins = computed<SidebarPluginItem[]>(() => {
    const visited = pluginsStore.knownItems.filter(plugin => visitedPluginIds.value.has(plugin.id))
    const remembered = new Map([...sortedItems.value, ...visited, ...Object.values(detailsByPluginId.value)].map(plugin => [plugin.id, plugin]))
    const items: SidebarPluginItem[] = [...remembered.values()].map(plugin => ({
      icon: plugin.icon,
      id: plugin.id,
      name: plugin.name,
      state: plugin.state,
      version: plugin.version,
    }))

    if (activePluginId.value && !items.some(plugin => plugin.id === activePluginId.value)) {
      items.push({
        icon: activePlugin.value?.icon,
        id: activePluginId.value,
        name: activePluginName.value,
        state: activePlugin.value?.state,
        version: activePlugin.value?.version,
      })
    }

    return items.sort((left, right) => left.id.localeCompare(right.id))
  })
  const showPluginFilter = computed(() => total.value >= 8 || Boolean(pluginFilter.value))
  const filteredPlugins = computed(() => {
    if (!pluginFilter.value.trim()) return navigationPlugins.value
    const matchingIds = new Set(sortedItems.value.map(plugin => plugin.id))
    return navigationPlugins.value.filter(plugin => plugin.id === activePluginId.value || matchingIds.has(plugin.id))
  })
  const pluginNavigationEntries = computed<SidebarPluginNavigationEntry[]>(() => (
    filteredPlugins.value.flatMap((plugin) => {
      const entries: SidebarPluginNavigationEntry[] = [{
        key: `plugin:${plugin.id}`,
        kind: 'resource',
        plugin,
      }]
      if (!isPluginContentVisible(plugin.id)) {
        return entries
      }

      entries.push({
        key: `plugin-page:${plugin.id}:overview`,
        kind: 'overview',
        plugin,
      })
      entries.push(...getPluginManagementPages(plugin.id).map(page => ({
        key: `plugin-page:${plugin.id}:management:${page.id}`,
        kind: 'management' as const,
        page,
        plugin,
      })))
      if (isPluginDetailUnavailable(plugin.id)) {
        entries.push({
          key: `plugin-page:${plugin.id}:retry`,
          kind: 'retry',
          plugin,
        })
      }
      return entries
    })
  ))

  watch(pluginFilter, (query, _, cleanup) => {
    pluginCollection.cancel()
    const timer = setTimeout(() => { void pluginCollection.load({ query }).catch(() => undefined) }, 250)
    cleanup(() => clearTimeout(timer))
  })

  watch(
    activePluginId,
    (pluginId) => {
      if (pluginId) {
        expandPlugin(pluginId)
      }
    },
    { immediate: true },
  )

  function isPluginExpanded(pluginId: string) {
    return expandedPluginIds.value.has(pluginId)
  }

  function expandPlugin(pluginId: string) {
    if (!expandedPluginIds.value.has(pluginId)) {
      expandedPluginIds.value = new Set([...expandedPluginIds.value, pluginId])
    }
    void pluginsStore.ensureDetail(pluginId).catch(() => undefined)
  }

  function togglePluginExpansion(pluginId: string) {
    if (expandedPluginIds.value.has(pluginId)) {
      const nextExpandedPluginIds = new Set(expandedPluginIds.value)
      nextExpandedPluginIds.delete(pluginId)
      expandedPluginIds.value = nextExpandedPluginIds
      return
    }

    expandPlugin(pluginId)
  }

  function getPluginDetail(pluginId: string) {
    return detailsByPluginId.value[pluginId] ?? (
      current.value?.id === pluginId ? current.value : null
    )
  }

  function getPluginManagementPages(pluginId: string) {
    return getPluginDetail(pluginId)?.management_ui?.pages ?? []
  }

  function isPluginDetailPending(pluginId: string) {
    return Boolean(detailLoadingByPluginId.value[pluginId])
  }

  function isPluginExpansionPending(pluginId: string) {
    return isPluginExpanded(pluginId)
      && isPluginDetailPending(pluginId)
      && !getPluginDetail(pluginId)
  }

  function isPluginContentVisible(pluginId: string) {
    return isPluginExpanded(pluginId) && !isPluginExpansionPending(pluginId)
  }

  function isPluginDetailUnavailable(pluginId: string) {
    return Boolean(detailErrorsByPluginId.value[pluginId])
      && !isPluginDetailPending(pluginId)
      && !getPluginDetail(pluginId)
  }

  function retryPluginList() {
    void pluginCollection.load({ query: pluginFilter.value }).catch(() => undefined)
  }

  function retryPluginDetail(pluginId: string) {
    void pluginsStore.ensureDetail(pluginId, { refresh: true }).catch(() => undefined)
  }

  function getPluginAriaLabel(plugin: SidebarPluginItem) {
    return plugin.state
      ? `${plugin.name}，${getPluginStateLabel(plugin.state)}`
      : plugin.name
  }

  function getPluginDisclosureLabel(plugin: SidebarPluginItem) {
    if (isPluginExpansionPending(plugin.id)) {
      return t('plugins.navigation.loadingPluginPages', { name: plugin.name })
    }

    return isPluginExpanded(plugin.id)
      ? t('plugins.navigation.collapsePluginPages', { name: plugin.name })
      : t('plugins.navigation.expandPluginPages', { name: plugin.name })
  }

  return {
    expandPlugin,
    filteredPlugins,
    getPluginAriaLabel,
    getPluginDisclosureLabel,
    isPluginContentVisible,
    isPluginExpansionPending,
    navigationPlugins,
    pluginCollection,
    pluginFilter,
    pluginNavigationEntries,
    retryPluginDetail,
    retryPluginList,
    showPluginFilter,
    togglePluginExpansion,
  }
}
