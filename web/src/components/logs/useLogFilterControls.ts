import { computed, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { t } from '@/i18n'
import { normalizeFilterValues, type LogFilters } from '@/stores/log-state'
import { usePluginsStore } from '@/stores/plugins'
import type { LogLevel } from '@/types/api'

export function useLogFilterControls(filters: Ref<LogFilters>) {
  const pluginsStore = usePluginsStore()
  const { sortedItems: pluginItems } = storeToRefs(pluginsStore)
  const selectedLevels = computed({
    get: () => filters.value.levels ?? [],
    set: (levels: LogLevel[]) => { filters.value.levels = levels },
  })
  const levelOptions = computed(() => ([
    { label: t('display.logLevels.debug'), value: 'debug' as LogLevel },
    { label: t('display.logLevels.info'), value: 'info' as LogLevel },
    { label: t('display.logLevels.warn'), value: 'warn' as LogLevel },
    { label: t('display.logLevels.error'), value: 'error' as LogLevel },
  ]))
  const selectedPluginIds = computed(() => normalizeFilterValues(filters.value.pluginIds))
  const pluginOptions = computed(() => {
    const options = pluginItems.value.map((plugin) => ({
      label: pluginsStore.getPluginLabel(plugin.id, plugin.name),
      value: plugin.id,
    }))
    const knownPluginIds = new Set(options.map((option) => option.value))

    for (const pluginId of selectedPluginIds.value) {
      if (!knownPluginIds.has(pluginId)) {
        options.push({ label: pluginsStore.getPluginLabel(pluginId), value: pluginId })
      }
    }

    return options
  })
  async function openPluginFilter() {
    if (pluginsStore.listLoaded) {
      return
    }

    try {
      await pluginsStore.fetchList()
    } catch {
      return
    }
  }



  return { selectedLevels, levelOptions, pluginOptions, openPluginFilter }
}
