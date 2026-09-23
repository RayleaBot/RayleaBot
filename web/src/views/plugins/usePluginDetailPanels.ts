import { computed, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { t } from '@/i18n'
import {
  areLocationQueriesEqual,
  buildPluginDetailLocation,
  readPluginDetailPanel,
  readPluginManagementPage,
  type PluginDetailPanel,
} from '@/lib/management-links'
import type { PluginDetail } from '@/types/api'

type PluginPanelOption = { label: string; value: string }

export function usePluginDetailPanels(pluginId: Readonly<Ref<string>>, currentPlugin: Readonly<Ref<PluginDetail | null>>) {
  const route = useRoute()
  const router = useRouter()
  const requestedPanel = computed(() => readPluginDetailPanel(route.query))
  const requestedManagementPage = computed(() => readPluginManagementPage(route.query))
  const hasManagementUI = computed(() => (currentPlugin.value?.management_ui?.pages?.length ?? 0) > 0)
  const managementPages = computed(() => {
    const managementUI = currentPlugin.value?.management_ui
    return managementUI?.pages ?? []
  })
  const activeManagementPage = computed(() => {
    if (!hasManagementUI.value) {
      return null
    }

    return managementPages.value.find((page) => page.id === requestedManagementPage.value)
      ?? managementPages.value[0]
      ?? null
  })
  const activePanel = computed<PluginDetailPanel>(() => {
    if (requestedPanel.value === 'management-ui' && currentPlugin.value && !hasManagementUI.value) {
      return 'overview'
    }

    return requestedPanel.value
  })
  const activePanelKey = computed(() => (
    activePanel.value === 'management-ui' && activeManagementPage.value
      ? `management-ui:${activeManagementPage.value.id}`
      : activePanel.value
  ))
  const panelOptions = computed(() => {
    const options: PluginPanelOption[] = [
      { label: t('plugins.panels.overview'), value: 'overview' },
    ]

    if (hasManagementUI.value) {
      for (const page of managementPages.value) {
        options.push({
          label: page.label?.trim() || t('plugins.panels.managementUi'),
          value: `management-ui:${page.id}`,
        })
      }
    }

    return options
  })
  const managementPanelTitle = computed(() => activeManagementPage.value?.label?.trim() || t('plugins.sections.managementUi'))

  async function syncPanelQuery(nextPanel: PluginDetailPanel, managementPage?: string | null) {
    const target = buildPluginDetailLocation(pluginId.value, {
      panel: nextPanel,
      managementPage,
    })

    if (areLocationQueriesEqual(route.query, target.query ?? {})) {
      return
    }

    await router.replace(target)
  }

  async function setActivePanelKey(nextKey: string) {
    if (nextKey === 'overview') {
      await syncPanelQuery('overview')
      return
    }

    if (nextKey.startsWith('management-ui:')) {
      await syncPanelQuery(
        'management-ui',
        nextKey.slice('management-ui:'.length),
      )
      return
    }

    await syncPanelQuery('management-ui')
  }

  watch(
    [requestedPanel, requestedManagementPage, currentPlugin, activeManagementPage],
    ([panel, managementPage, plugin, activePage]) => {
      if (route.name !== 'plugin-detail') {
        return
      }

      if (panel === 'management-ui' && plugin && !hasManagementUI.value) {
        void syncPanelQuery('overview')
        return
      }

      if (panel === 'management-ui' && hasManagementUI.value && activePage) {
        const expectedPage = activePage.id
        if (managementPage !== expectedPage) {
          void syncPanelQuery('management-ui', expectedPage)
        }
      }
    },
    { immediate: true },
  )

  return {
    activeManagementPage,
    activePanel,
    activePanelKey,
    managementPanelTitle,
    panelOptions,
    setActivePanelKey,
  }
}
