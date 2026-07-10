import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from 'vue'

import { t } from '@/i18n'
import { usePluginConsoleStore, type ConsoleFrame } from '@/stores/plugin-console'
import { useSocketStore } from '@/stores/sockets'

export type PluginDetailInnerTab = 'summary' | 'commands' | 'console'

interface PluginConsolePanelOptions {
  pluginConsoleStore: ReturnType<typeof usePluginConsoleStore>
  pluginId: Readonly<Ref<string>>
  readyToRenderHeavyContent: Readonly<Ref<boolean>>
  socketStore: ReturnType<typeof useSocketStore>
}

export function usePluginConsolePanel(options: PluginConsolePanelOptions) {
  const { pluginConsoleStore, pluginId, readyToRenderHeavyContent, socketStore } = options
  const consoleFrames = computed(() => pluginConsoleStore.getConsole(pluginId.value))
  const consoleFrameCount = computed(() => consoleFrames.value.length)
  const consoleSnapshot = computed(() => socketStore.snapshots.pluginConsole)
  const consoleViewportRef = ref<{
    scrollToBottom: () => void
    scrollToOffset: (offset: number) => void
  } | null>(null)
  const consoleFollowBottom = ref(true)
  const activeDetailTab = ref<PluginDetailInnerTab>('summary')
  let consoleBottomSyncToken = 0

  function clearConsole() {
    pluginConsoleStore.clearConsole(pluginId.value)
  }

  function onConsoleViewportBottomChange(nextAtBottom: boolean) {
    consoleFollowBottom.value = nextAtBottom
    if (!nextAtBottom) {
      consoleBottomSyncToken += 1
    }
  }

  function getConsoleFrameKey(frame: ConsoleFrame, index: number) {
    if (frame.stream === 'outbound') {
      return frame.log_id
    }
    return `${frame.plugin_id}-${frame.stream}-${frame.timestamp}-${index}`
  }

  function getConsoleLevel(frame: ConsoleFrame) {
    return frame.stream === 'outbound' ? frame.level : ''
  }

  function getConsoleStreamLabel(stream: ConsoleFrame['stream']) {
    return t(`plugins.console.streams.${stream}`)
  }

  function getConsoleStreamColor(stream: ConsoleFrame['stream']) {
    if (stream === 'stderr') return 'error'
    if (stream === 'system') return 'warning'
    if (stream === 'outbound') return 'blue'
    return 'default'
  }

  function getConsoleLevelLabel(level: string) {
    if (level === 'debug' || level === 'info' || level === 'warn' || level === 'error') {
      return t(`plugins.console.levels.${level}`)
    }

    return level || t('display.empty')
  }

  function getConsoleLevelColor(level: string) {
    if (level === 'error') return 'error'
    if (level === 'warn') return 'warning'
    if (level === 'info') return 'blue'
    return 'default'
  }

  function getConsoleRequestId(frame: ConsoleFrame) {
    return frame.stream === 'outbound' ? frame.request_id ?? '' : ''
  }

  function getConsoleStatusColor(status: string) {
    if (status === 'authenticated') return 'success'
    if (status === 'reconnecting' || status === 'connecting') return 'warning'
    if (status === 'auth_failed') return 'error'
    return 'default'
  }

  function getConsoleSnapshotStatusColor(status: string) {
    if (status === 'authenticated') return 'var(--success)'
    if (status === 'reconnecting' || status === 'connecting') return 'var(--warning)'
    if (status === 'auth_failed') return 'var(--danger)'
    return 'var(--muted)'
  }

  function setActiveDetailTab(nextTab: PluginDetailInnerTab) {
    activeDetailTab.value = nextTab
    if (nextTab === 'console') {
      void syncConsoleViewportToBottom()
    }
  }

  async function syncConsoleViewportToBottom() {
    const syncToken = ++consoleBottomSyncToken
    consoleFollowBottom.value = true
    await nextTick()
    for (let attempt = 0; attempt < 4; attempt += 1) {
      if (syncToken !== consoleBottomSyncToken || activeDetailTab.value !== 'console') {
        return
      }

      await waitForAnimationFrame()
      if (syncToken !== consoleBottomSyncToken || activeDetailTab.value !== 'console') {
        return
      }

      consoleViewportRef.value?.scrollToBottom()
      await nextTick()
    }
  }

  async function waitForAnimationFrame() {
    if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
      await nextTick()
      return
    }

    await new Promise<void>((resolve) => {
      window.requestAnimationFrame(() => resolve())
    })
  }

  watch(
    () => consoleFrames.value.length,
    async () => {
      if (activeDetailTab.value === 'console' && consoleFollowBottom.value) {
        await nextTick()
        consoleViewportRef.value?.scrollToBottom()
      }
    },
  )

  watch(
    readyToRenderHeavyContent,
    (ready) => {
      if (ready && activeDetailTab.value === 'console') {
        void syncConsoleViewportToBottom()
      }
    },
    { immediate: true },
  )

  onBeforeUnmount(() => {
    consoleBottomSyncToken += 1
    socketStore.setConsolePlugin(null)
  })

  return {
    activeDetailTab,
    clearConsole,
    consoleFollowBottom,
    consoleFrameCount,
    consoleFrames,
    consoleSnapshot,
    consoleViewportRef,
    getConsoleFrameKey,
    getConsoleLevel,
    getConsoleLevelColor,
    getConsoleLevelLabel,
    getConsoleRequestId,
    getConsoleSnapshotStatusColor,
    getConsoleStatusColor,
    getConsoleStreamColor,
    getConsoleStreamLabel,
    onConsoleViewportBottomChange,
    setActiveDetailTab,
  }
}
