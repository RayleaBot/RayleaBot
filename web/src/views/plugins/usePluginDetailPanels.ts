import { computed, watch, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { t } from '@/i18n'
import {
  areLocationQueriesEqual,
  buildPluginDetailLocation,
  readPluginBotAccount,
  readPluginDetailPanel,
  readPluginManagementPage,
  type PluginDetailPanel,
} from '@/lib/management-links'
import type { BotAccountRef, PluginDetail } from '@/types/api'

type PluginPanelOption = { label: string; value: string }

export function usePluginDetailPanels(pluginId: Readonly<Ref<string>>, currentPlugin: Readonly<Ref<PluginDetail | null>>) {
  const route = useRoute()
  const router = useRouter()
  const requestedPanel = computed(() => readPluginDetailPanel(route.query))
  const requestedManagementPage = computed(() => readPluginManagementPage(route.query))
  const requestedBotAccount = computed(() => readPluginBotAccount(route.query))
  // The raw value, so that rewriting other parameters keeps even a malformed account in the address.
  const requestedBotKey = computed(() => {
    const value = route.query.bot
    return typeof value === 'string' ? value : null
  })
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
  const activePageScope = computed(() => (
    activePanel.value === 'management-ui' ? activeManagementPage.value?.scope ?? 'global' : null
  ))

  function pageScope(pageId: string | null | undefined) {
    return managementPages.value.find((page) => page.id === pageId)?.scope ?? 'global'
  }

  // Only bot_account pages carry an account; moving to the overview or a global page drops it.
  async function syncPanelQuery(nextPanel: PluginDetailPanel, managementPage?: string | null) {
    const target = buildPluginDetailLocation(pluginId.value, {
      panel: nextPanel,
      managementPage,
      bot: nextPanel === 'management-ui' && pageScope(managementPage) === 'bot_account' ? requestedBotKey.value : null,
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

  // A choice the user makes is a history entry; filling in a missing account replaces the address.
  async function selectBotAccount(account: BotAccountRef, options: { replace?: boolean } = {}) {
    const target = buildPluginDetailLocation(pluginId.value, {
      panel: 'management-ui',
      managementPage: activeManagementPage.value?.id,
      bot: account,
    })
    if (areLocationQueriesEqual(route.query, target.query ?? {})) {
      return
    }
    await (options.replace ? router.replace(target) : router.push(target))
  }

  watch(
    [requestedPanel, requestedManagementPage, currentPlugin, activeManagementPage, requestedBotAccount],
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
          return
        }
      }

      if (requestedBotAccount.value.present && (panel !== 'management-ui' || (plugin && pageScope(activePage?.id) !== 'bot_account'))) {
        void syncPanelQuery(panel, activePage?.id)
      }
    },
    { immediate: true },
  )

  return {
    activeManagementPage,
    activePageScope,
    activePanel,
    activePanelKey,
    managementPanelTitle,
    panelOptions,
    requestedBotAccount,
    selectBotAccount,
    setActivePanelKey,
  }
}
