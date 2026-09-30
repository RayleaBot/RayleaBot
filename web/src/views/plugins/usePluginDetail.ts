import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useConfigStore } from '@/stores/config'
import { usePluginConsoleStore } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'

export function usePluginDetail(pluginId: Readonly<Ref<string>>) {
  const router = useRouter()
  const pluginsStore = usePluginsStore()
  const pluginConsoleStore = usePluginConsoleStore()
  const socketStore = useSocketStore()
  const configStore = useConfigStore()
  const { actionPending, current, detailLoading } = storeToRefs(pluginsStore)
  const currentPlugin = computed(() => current.value?.id === pluginId.value ? current.value : null)
  const loadError = ref<string | null>(null)
  const operationError = ref<string | null>(null)
  const uninstallDialogVisible = ref(false)
  let detailLoadVersion = 0
  let contextVersion = 0
  let pageActive = true

  useToastFeedback(computed(() => {
    const message = operationError.value || loadError.value
    if (!message) return null
    const source = operationError.value ? 'operation' : 'load'
    return { key: `plugin-detail-${source}:${message}`, level: 'error' as const, message }
  }))

  function captureContext() {
    const id = pluginId.value
    const version = contextVersion
    return { id, isCurrent: () => pageActive && version === contextVersion && id === pluginId.value }
  }

  async function loadDetail() {
    const context = captureContext()
    const requestVersion = ++detailLoadVersion
    loadError.value = null
    try {
      await Promise.all([
        pluginsStore.fetchDetail(context.id),
        pluginConsoleStore.fetchOutboundConsoleHistory(context.id).catch(() => []),
        configStore.fetchConfig().catch(() => undefined),
      ])
      if (context.isCurrent() && requestVersion === detailLoadVersion) {
        socketStore.setConsolePlugin(context.id)
      }
    } catch (error) {
      if (context.isCurrent() && requestVersion === detailLoadVersion) {
        loadError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
      }
    }
  }

  async function runAction(action: 'enable' | 'disable' | 'reload') {
    if (!currentPlugin.value || actionPending.value[pluginId.value]) return
    const context = captureContext()
    operationError.value = null
    try {
      const plugin = await pluginsStore.executeAction(context.id, action)
      if (context.isCurrent()) notifySuccess(t(`plugins.actionResult.${action}`, { name: plugin.name || context.id }))
    } catch (error) {
      if (context.isCurrent()) operationError.value = getDisplayErrorMessage(error)
    }
  }

  function getToggleAction() {
    return currentPlugin.value?.state === 'disabled' ? 'enable' : 'disable'
  }

  async function uninstallPlugin() {
    if (!currentPlugin.value || actionPending.value[pluginId.value]) return
    const context = captureContext()
    operationError.value = null
    try {
      await pluginsStore.uninstallPlugin(context.id, () => {
        if (context.isCurrent()) uninstallDialogVisible.value = false
      })
      if (!context.isCurrent()) return
      uninstallDialogVisible.value = false
      notifySuccess(t('plugins.uninstallAccepted'))
      await router.replace('/plugins')
    } catch (error) {
      if (context.isCurrent()) operationError.value = getDisplayErrorMessage(error)
    }
  }

  watch(pluginId, () => {
    contextVersion += 1
    operationError.value = null
    uninstallDialogVisible.value = false
    void loadDetail()
  }, { flush: 'sync' })

  onMounted(() => { void loadDetail() })
  onBeforeUnmount(() => {
    pageActive = false
    contextVersion += 1
    socketStore.setConsolePlugin(null)
  })

  return {
    actionPending, currentPlugin, detailLoading, loadError, operationError,
    getToggleAction, loadDetail, runAction, uninstallDialogVisible, uninstallPlugin,
  }
}
