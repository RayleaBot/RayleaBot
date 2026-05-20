<script setup lang="ts">
import { ClearOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import PluginPowerButton from '@/components/PluginPowerButton.vue'
import PluginCommandsPanel from '@/components/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import {
  getConnectionStatusLabel,
  getPluginDisplayStateLabel,
  getPluginDesiredStateLabel,
  getPluginRegistrationStateLabel,
  getPluginRoleLabel,
  getPluginRuntimeStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { ApiError } from '@/lib/http'
import {
  areLocationQueriesEqual,
  buildPluginDetailLocation,
  buildPluginWorkbenchActions,
  buildTaskLocation,
  readPluginDetailPanel,
  type PluginDetailPanel,
} from '@/lib/management-links'
import { escapeUnsafeDisplayText, safeJsonStringify } from '@/lib/text-safety'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginConsoleStore, type ConsoleFrame } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'
import type { PluginDetail, PluginPermissionSummary } from '@/types/api'

type PermissionDialogMode = 'grant' | 'pending' | 'scope_changed'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const pluginConsoleStore = usePluginConsoleStore()
const socketStore = useSocketStore()
const configStore = useConfigStore()

const { actionPending, current, detailLoading, grantsLoading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)

const pluginId = computed(() => String(route.params.id))
const currentPlugin = computed(() => current.value?.id === pluginId.value ? current.value : null)
const consoleFrames = computed(() => pluginConsoleStore.getConsole(pluginId.value))
const consoleFrameCount = computed(() => consoleFrames.value.length)
const currentGrants = computed(() => pluginsStore.getGrants(pluginId.value))
const currentPermissions = computed(() => currentPlugin.value?.permissions ?? [])
const consoleSnapshot = computed(() => socketStore.snapshots.pluginConsole)
const grantBusy = computed(() => grantsLoading.value[pluginId.value] ?? false)
const loadError = ref<string | null>(null)
const operationError = ref<string | null>(null)
const consoleScroller = ref<HTMLElement | null>(null)
const permissionDialogVisible = ref(false)
const uninstallDialogVisible = ref(false)
const selectedCapabilities = ref<string[]>([])
const permissionDialogMode = ref<PermissionDialogMode>('grant')
const permissionDialogAvailableCapabilities = ref<string[]>([])
const resumeEnableAfterGrant = ref(false)
let detailLoadVersion = 0
let pageActive = true

const commandPrefix = computed(() => getPrimaryCommandPrefix(configDocument.value?.command?.prefixes))
const requestedPanel = computed(() => readPluginDetailPanel(route.query))
const isBuiltinPlugin = computed(() => currentPlugin.value?.role === 'builtin')
const hasManagementUI = computed(() => Boolean(currentPlugin.value?.management_ui?.entry))
const activePanel = computed<PluginDetailPanel>(() => {
  if (requestedPanel.value === 'management-ui' && currentPlugin.value && !hasManagementUI.value) {
    return 'overview'
  }

  return requestedPanel.value
})
const panelOptions = computed(() => {
  const options = [
    { label: t('plugins.panels.overview'), value: 'overview' },
  ]

  if (hasManagementUI.value) {
    options.push({
      label: currentPlugin.value?.management_ui?.label?.trim() || t('plugins.panels.managementUi'),
      value: 'management-ui',
    })
  }

  return options
})
const permissionCandidates = computed(() => currentPermissions.value.filter((permission) => permission.status === 'not_granted'))
const reconfirmCandidates = computed(() => currentPermissions.value.filter((permission) => permission.source === 'persisted'))
const missingRequiredPermissions = computed(() => currentPermissions.value.filter((permission) => permission.requirement === 'required' && permission.status === 'not_granted'))
const grantRecordsByCapability = computed(() => new Map(currentGrants.value.map((grant) => [grant.capability, grant])))
const permissionMap = computed(() => new Map(currentPermissions.value.map((permission) => [permission.capability, permission])))
const permissionDialogCandidates = computed(() => (
  permissionDialogAvailableCapabilities.value
    .map((capability) => permissionMap.value.get(capability))
    .filter((permission): permission is PluginPermissionSummary => Boolean(permission))
))
const permissionDialogTitle = computed(() => {
  if (permissionDialogMode.value === 'scope_changed') {
    return t('plugins.permissionDialogScopeChangedTitle')
  }

  if (resumeEnableAfterGrant.value) {
    return t('plugins.permissionDialogPendingTitle')
  }

  return t('plugins.permissionDialogTitle')
})
const permissionDialogBody = computed(() => {
  if (permissionDialogMode.value === 'scope_changed') {
    return t('plugins.permissionDialogScopeChangedBody')
  }

  if (resumeEnableAfterGrant.value) {
    return t('plugins.permissionDialogPendingBody')
  }

  return ''
})
const permissionDialogOkText = computed(() => (
  permissionDialogMode.value === 'scope_changed'
    ? t('plugins.actions.reconfirmSelected')
    : t('plugins.actions.grantSelected')
))
const pluginWorkbenchActions = computed(() => buildPluginWorkbenchActions(pluginId.value))
const managementPanelTitle = computed(() => currentPlugin.value?.management_ui?.label?.trim() || t('plugins.sections.managementUi'))
const pluginDisplayName = computed(() => currentPlugin.value?.name?.trim() || pluginId.value)
const pluginInitial = computed(() => pluginDisplayName.value.trim().slice(0, 1).toUpperCase() || 'P')
const sourceRefText = computed(() => currentPlugin.value?.source?.package_source_ref ?? currentPlugin.value?.source?.package_source_type ?? '')
const statusSummaryItems = computed(() => [
  {
    key: 'registration',
    label: t('plugins.fields.registration'),
    value: getPluginRegistrationStateLabel(currentPlugin.value?.registration_state),
    raw: currentPlugin.value?.registration_state,
  },
  {
    key: 'desired',
    label: t('plugins.fields.desired'),
    value: getPluginDesiredStateLabel(currentPlugin.value?.desired_state),
    raw: currentPlugin.value?.desired_state,
  },
  {
    key: 'runtime',
    label: t('plugins.fields.runtime'),
    value: getPluginRuntimeStateLabel(currentPlugin.value?.runtime_state),
    raw: currentPlugin.value?.runtime_state,
  },
  {
    key: 'display',
    label: t('plugins.fields.display'),
    value: getPluginDisplayStateLabel(currentPlugin.value?.display_state),
    raw: currentPlugin.value?.display_state,
  },
])
const heroFacts = computed(() => [
  { key: 'version', label: t('plugins.fields.version'), value: getMetadataText(currentPlugin.value?.version) },
  { key: 'runtime', label: t('plugins.fields.runtimeFamily'), value: getMetadataText(currentPlugin.value?.runtime) },
  { key: 'entry', label: t('plugins.fields.entry'), value: getMetadataText(currentPlugin.value?.entry) },
  { key: 'source', label: t('plugins.fields.sourceRoot'), value: getMetadataText(currentPlugin.value?.source?.root) },
])
const packageInfoRows = computed(() => [
  { key: 'type', label: t('plugins.fields.type'), value: getMetadataText(currentPlugin.value?.type) },
  { key: 'author', label: t('plugins.fields.author'), value: getMetadataText(currentPlugin.value?.author) },
  { key: 'license', label: t('plugins.fields.license'), value: getMetadataText(currentPlugin.value?.license) },
  { key: 'sdk', label: t('plugins.fields.sdkMinVersion'), value: getMetadataText(currentPlugin.value?.sdk_min_version) },
  { key: 'core', label: t('plugins.fields.minCoreVersion'), value: getMetadataText(currentPlugin.value?.min_core_version) },
  { key: 'schema', label: t('plugins.fields.dataSchemaVersion'), value: getMetadataText(currentPlugin.value?.data_schema_version) },
])
const sourceInfoRows = computed(() => [
  { key: 'root', label: t('plugins.fields.sourceRoot'), value: getMetadataText(currentPlugin.value?.source?.root) },
  { key: 'ref', label: t('plugins.fields.sourceRef'), value: getMetadataText(sourceRefText.value) },
  { key: 'trust', label: t('plugins.fields.trust'), value: currentPlugin.value?.trust?.label ?? t('display.empty') },
])
const runtimeInfoRows = computed(() => [
  { key: 'concurrency', label: t('plugins.fields.concurrency'), value: currentPlugin.value?.concurrency ?? t('display.empty') },
  { key: 'runtime-version', label: t('plugins.fields.runtimeVersion'), value: getMetadataText(currentPlugin.value?.runtime_version) },
])
const permissionSummaryLabel = computed(() => (
  missingRequiredPermissions.value.length > 0
    ? t('plugins.permissionPendingCompact', { count: missingRequiredPermissions.value.length })
    : t('plugins.permissionTotalCompact', { count: currentPermissions.value.length })
))

async function loadDetail() {
  const requestedPluginId = pluginId.value
  const requestVersion = ++detailLoadVersion
  loadError.value = null
  try {
    await Promise.all([
      pluginsStore.fetchDetail(requestedPluginId),
      pluginsStore.fetchGrants(requestedPluginId),
      pluginConsoleStore.fetchOutboundConsoleHistory(requestedPluginId).catch(() => []),
      configStore.fetchConfig().catch(() => undefined),
    ])

    if (!isCurrentDetailRequest(requestVersion, requestedPluginId)) {
      return
    }

    socketStore.setConsolePlugin(requestedPluginId)
  } catch (error) {
    if (!isCurrentDetailRequest(requestVersion, requestedPluginId)) {
      return
    }

    loadError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
  }
}

function isCurrentDetailRequest(requestVersion: number, requestedPluginId: string) {
  return pageActive && requestVersion === detailLoadVersion && pluginId.value === requestedPluginId
}

async function runAction(action: 'enable' | 'disable' | 'reload') {
  operationError.value = null
  try {
    await pluginsStore.executeAction(pluginId.value, action)
    notifySuccess(t('plugins.actionAccepted'))
  } catch (error) {
    if (action === 'enable' && error instanceof ApiError && error.code === 'plugin.permission_pending') {
      const pendingContext = extractPermissionPendingContext(error)
      const available = pendingContext.scopeChanged
        ? dedupeCapabilities([
            ...reconfirmCandidates.value.map((permission) => permission.capability),
            ...pendingContext.missingCapabilities,
          ])
        : dedupeCapabilities(pendingContext.missingCapabilities)

      openPermissionDialog({
        available,
        prefill: available,
        mode: pendingContext.scopeChanged ? 'scope_changed' : 'pending',
        resumeEnable: true,
      })
      return
    }
    operationError.value = getDisplayErrorMessage(error)
  }
}

function getToggleAction() {
  return current.value?.desired_state === 'enabled' ? 'disable' : 'enable'
}

function extractMissingCapabilities(error: ApiError) {
  const raw = error.details?.missing_capabilities
  return Array.isArray(raw)
    ? raw.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : []
}

function extractPermissionPendingContext(error: ApiError) {
  return {
    missingCapabilities: extractMissingCapabilities(error),
    scopeChanged: error.details?.scope_changed === true,
  }
}

function dedupeCapabilities(capabilities: string[]) {
  return Array.from(new Set(
    capabilities
      .map((capability) => capability.trim())
      .filter((capability) => capability.length > 0),
  ))
}

function openPermissionDialog(options: {
  available?: string[]
  prefill?: string[]
  mode?: PermissionDialogMode
  resumeEnable?: boolean
} = {}) {
  const available = dedupeCapabilities(
    options.available ?? permissionCandidates.value.map((permission) => permission.capability),
  )

  if (available.length === 0) {
    operationError.value = t('plugins.permissionReconfirmUnavailable')
    return
  }

  const recommended = dedupeCapabilities(
    (options.prefill?.length ? options.prefill : available)
      .filter((capability) => available.includes(capability)),
  )

  permissionDialogMode.value = options.mode ?? 'grant'
  permissionDialogAvailableCapabilities.value = available
  selectedCapabilities.value = recommended.length > 0 ? recommended : available
  resumeEnableAfterGrant.value = options.resumeEnable ?? false
  permissionDialogVisible.value = true
}

function closePermissionDialog() {
  permissionDialogVisible.value = false
  selectedCapabilities.value = []
  permissionDialogMode.value = 'grant'
  permissionDialogAvailableCapabilities.value = []
  resumeEnableAfterGrant.value = false
}

async function submitPermissionDialog() {
  operationError.value = null
  try {
    for (const capability of selectedCapabilities.value) {
      await pluginsStore.grantCapability(pluginId.value, { capability })
    }
    const shouldResumeEnable = resumeEnableAfterGrant.value
    closePermissionDialog()
    notifySuccess(t('plugins.grantSaved'))
    if (shouldResumeEnable) {
      await runAction('enable')
    }
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

async function revokeGrant(capability: string) {
  operationError.value = null
  try {
    await pluginsStore.revokeGrant(pluginId.value, capability)
    notifySuccess(t('plugins.grantRevoked'))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

async function uninstallPlugin() {
  operationError.value = null
  try {
    const response = await pluginsStore.uninstallPlugin(pluginId.value)
    uninstallDialogVisible.value = false
    notifySuccess(t('plugins.uninstallAccepted'))
    await router.push(buildTaskLocation(response.task_id))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

function clearConsole() {
  pluginConsoleStore.clearConsole(pluginId.value)
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

function getPermissionRequirementLabel(requirement: PluginPermissionSummary['requirement']) {
  return t(`plugins.permissionRequirement.${requirement}`)
}

function getPermissionStatusLabel(status: PluginPermissionSummary['status']) {
  return t(`plugins.permissionStatus.${status}`)
}

function getPermissionSourceLabel(source: PluginPermissionSummary['source']) {
  return t(`plugins.permissionSource.${source}`)
}

function canGrantPermission(permission: PluginPermissionSummary) {
  return !isBuiltinPlugin.value && permission.status === 'not_granted' && permission.source === 'none'
}

function canRevokePermission(permission: PluginPermissionSummary) {
  return !isBuiltinPlugin.value && permission.status === 'granted' && permission.source === 'persisted'
}

function getMetadataText(value?: string | null) {
  return value?.trim() || t('display.empty')
}

function hasItems(value?: readonly unknown[] | null) {
  return Array.isArray(value) && value.length > 0
}

function hasObjectValue(value: unknown) {
  return typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value as Record<string, unknown>).length > 0
}

function getJsonPreview(value: unknown) {
  return safeJsonStringify(value ?? {})
}

function getScreenshotAlt(screenshot: NonNullable<PluginDetail['screenshots']>[number]) {
  return screenshot.alt?.trim() || t('display.empty')
}

function getGrantedAt(capability: string) {
  return grantRecordsByCapability.value.get(capability)?.granted_at ?? undefined
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

function getPluginStateColor(status?: string | null) {
  if (!status) return 'default'
  if (status === 'failed' || status === 'error' || status === 'removed') return 'error'
  if (status === 'starting' || status === 'stopping' || status === 'enabling' || status === 'disabling' || status === 'retrying') return 'warning'
  if (status === 'installed' || status === 'enabled' || status === 'running' || status === 'discovered') return 'success'
  return 'default'
}

function getPluginStateDotColor(status?: string | null) {
  if (!status) return 'var(--muted)'
  if (status === 'failed' || status === 'error' || status === 'removed') return 'var(--danger)'
  if (status === 'starting' || status === 'stopping' || status === 'enabling' || status === 'disabling' || status === 'retrying') return 'var(--warning)'
  if (status === 'installed' || status === 'enabled' || status === 'running' || status === 'discovered') return 'var(--success)'
  return 'var(--muted)'
}

function getPluginAvatarStyle(name: string) {
  let hash = 0
  const cleanName = name?.trim() || 'P'
  for (let i = 0; i < cleanName.length; i++) {
    hash = cleanName.charCodeAt(i) + ((hash << 5) - hash)
  }
  const h = Math.abs(hash) % 360
  return {
    background: `linear-gradient(135deg, hsl(${h}, 72%, 58%) 0%, hsl(${(h + 40) % 360}, 78%, 46%) 100%)`,
    color: '#ffffff',
    textShadow: '0 1px 2px rgba(0, 0, 0, 0.15)',
  }
}

async function syncPanelQuery(nextPanel: PluginDetailPanel) {
  const target = buildPluginDetailLocation(pluginId.value, {
    panel: nextPanel,
  })

  if (areLocationQueriesEqual(route.query, target.query ?? {})) {
    return
  }

  await router.replace(target)
}

async function setActivePanel(nextPanel: PluginDetailPanel) {
  await syncPanelQuery(nextPanel)
}

watch(
  () => consoleFrames.value.length,
  async () => {
    await scrollConsoleToBottom()
  },
)

watch(pluginId, () => {
  void loadDetail()
})

watch(
  [requestedPanel, currentPlugin],
  ([panel, plugin]) => {
    if (route.name !== 'plugin-detail') {
      return
    }

    if (panel === 'management-ui' && plugin && !plugin.management_ui?.entry) {
      void syncPanelQuery('overview')
    }
  },
  { immediate: true },
)

onMounted(() => {
  void loadDetail()
})

onUnmounted(() => {
  pageActive = false
  detailLoadVersion += 1
  socketStore.setConsolePlugin(null)
})

async function scrollConsoleToBottom() {
  await nextTick()
  if (!consoleScroller.value) {
    return
  }

  consoleScroller.value.scrollTo({
    top: consoleScroller.value.scrollHeight,
    behavior: 'smooth',
  })
}
</script>

<template>
  <AppPage :title="pluginId">
    <template #extra>
      <div class="table-actions plugin-detail-actions">
        <PluginPowerButton
          :checked="current?.desired_state === 'enabled'"
          :loading="actionPending[pluginId] === 'enable' || actionPending[pluginId] === 'disable'"
          :disabled="!current"
          :checked-label="t('plugins.actions.enable')"
          :unchecked-label="t('plugins.actions.disable')"
          @click="runAction(getToggleAction())"
        />
        <a-button :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">{{ t('plugins.actions.reload') }}</a-button>
        <a-button danger :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true">{{ t('plugins.actions.uninstall') }}</a-button>
      </div>
    </template>

    <RetryPanel
      v-if="loadError && !current"
      :title="t('errors.common.loadFailed')"
      :description="loadError"
      :loading="detailLoading"
      @retry="loadDetail()"
    />

    <a-alert v-else-if="loadError" :message="t('errors.common.loadFailed')" type="error" :description="loadError" show-icon />

    <a-alert v-if="operationError" :message="t('errors.common.actionFailed')" type="error" :description="operationError" show-icon />

    <a-card
      v-if="panelOptions.length > 1"
      :bordered="false"
      class="plugin-detail-panel-switch"
    >
      <a-segmented
        :value="activePanel"
        :options="panelOptions"
        @change="setActivePanel($event as PluginDetailPanel)"
      />
    </a-card>

    <template v-if="activePanel === 'overview'">
      <a-skeleton :loading="detailLoading && !currentPlugin" active>
        <section class="plugin-detail-hero">
          <div class="plugin-detail-hero__identity">
            <div class="plugin-detail-hero__avatar" :style="getPluginAvatarStyle(pluginDisplayName)" aria-hidden="true">
              {{ pluginInitial }}
            </div>
            <div class="plugin-detail-hero__copy">
              <div class="plugin-detail-hero__eyebrow">
                <a-tag class="premium-badge role-badge">{{ getPluginRoleLabel(currentPlugin?.role) }}</a-tag>
                <a-tag class="premium-badge trust-badge">{{ currentPlugin?.trust?.label ?? t('display.empty') }}</a-tag>
              </div>
              <strong class="plugin-title">{{ pluginDisplayName }}</strong>
              <span class="plugin-id-sub">{{ pluginId }}</span>
            </div>
          </div>

          <div class="plugin-detail-hero__tools">
            <ManagementContextActions :actions="pluginWorkbenchActions" />
            <!-- Hidden context action anchors for unit test compat -->
            <span class="sr-only">{{ t('plugins.actions.openPluginCommands') }}</span>
            <span class="sr-only">{{ t('plugins.actions.openPluginLogs') }}</span>
          </div>

          <div class="plugin-detail-status-chips" :aria-label="t('plugins.sections.statusSummary')">
            <div v-for="item in statusSummaryItems" :key="item.key" class="status-chip">
              <span class="status-chip__dot" :style="{ backgroundColor: getPluginStateDotColor(item.raw) }"></span>
              <span class="status-chip__label">{{ item.label }}:</span>
              <a-tag :color="getPluginStateColor(item.raw)" class="status-tag">
                {{ item.value }}
                <small v-if="item.raw"> · {{ item.raw }}</small>
              </a-tag>
            </div>
          </div>

          <dl class="plugin-detail-hero__facts">
            <div v-for="item in heroFacts" :key="item.key" class="fact-item">
              <dt class="fact-label">{{ item.label }}</dt>
              <dd class="fact-value">{{ item.value }}</dd>
            </div>
          </dl>
        </section>
      </a-skeleton>

      <div class="plugin-detail-workspace">
        <main class="plugin-detail-main-column">
          <a-card :bordered="false" class="plugin-detail-tab-card">
            <a-tabs default-active-key="commands" :destroy-inactive-tab-pane="false" class="premium-detail-tabs">
              <!-- TAB 1: Commands -->
              <a-tab-pane key="commands" force-render>
                <template #tab>
                  <span class="premium-tab-label">
                    {{ t('plugins.sections.commands') }}
                    <a-tag size="small" :bordered="false" class="tab-badge">{{ currentPlugin?.commands?.length ?? 0 }}</a-tag>
                  </span>
                </template>

                <div class="tab-pane-content">
                  <PluginCommandsPanel
                    :commands="currentPlugin?.commands ?? []"
                    :command-conflicts="currentPlugin?.command_conflicts ?? []"
                    :command-prefix="commandPrefix"
                  />
                </div>
              </a-tab-pane>

              <!-- TAB 2: Permissions -->
              <a-tab-pane key="permissions" force-render>
                <template #tab>
                  <span class="premium-tab-label">
                    {{ t('plugins.sections.permissions') }}
                    <a-tag size="small" :bordered="false" :color="missingRequiredPermissions.length > 0 ? 'warning' : 'default'" class="tab-badge">
                      {{ permissionSummaryLabel }}
                    </a-tag>
                  </span>
                </template>

                <div class="tab-pane-content">
                  <div v-if="isBuiltinPlugin" class="plugin-detail-inline-state is-success">
                    <strong>{{ t('plugins.builtinAutoGrantTitle') }}</strong>
                    <span>{{ t('plugins.builtinAutoGrantBody') }}</span>
                  </div>

                  <div v-else-if="missingRequiredPermissions.length > 0" class="plugin-detail-inline-state is-warning">
                    <strong>{{ t('plugins.permissionPendingTitle') }}</strong>
                    <span>{{ t('plugins.permissionPendingBody', { count: missingRequiredPermissions.length }) }}</span>
                  </div>

                  <div class="permission-actions-header" v-if="!isBuiltinPlugin && permissionCandidates.length > 0">
                    <a-button
                      size="small"
                      type="primary"
                      class="review-permissions-btn"
                      @click="openPermissionDialog()"
                    >
                      {{ t('plugins.actions.reviewPermissions') }}
                    </a-button>
                  </div>

                  <a-skeleton :loading="grantBusy" active>
                    <a-empty v-if="currentPermissions.length === 0" :description="t('plugins.empty.permissions')" />

                    <div v-else class="permission-list">
                      <article v-for="permission in currentPermissions" :key="permission.capability" class="permission-item">
                        <div class="permission-item__capability">
                          <strong>{{ permission.capability }}</strong>
                          <div class="permission-item__tags">
                            <a-tag color="blue" class="tag-compact">{{ getPermissionRequirementLabel(permission.requirement) }}</a-tag>
                            <a-tag :color="permission.status === 'granted' ? 'success' : 'warning'" class="tag-compact">{{ getPermissionStatusLabel(permission.status) }}</a-tag>
                            <a-tag class="tag-compact">{{ getPermissionSourceLabel(permission.source) }}</a-tag>
                          </div>
                        </div>

                        <div class="permission-item__time">
                          <span class="time-row">
                            <small>{{ t('plugins.fields.grantedAt') }}</small>
                            {{ formatDateTime(getGrantedAt(permission.capability)) }}
                          </span>
                          <span class="time-row">
                            <small>{{ t('plugins.fields.expiresAt') }}</small>
                            {{ formatDateTime(permission.expires_at ?? undefined) }}
                          </span>
                        </div>

                        <div class="permission-item__actions">
                          <a-button
                            v-if="canGrantPermission(permission)"
                            size="small"
                            type="primary"
                            @click="openPermissionDialog({ available: [permission.capability], prefill: [permission.capability] })"
                          >
                            {{ t('plugins.actions.grantPermission') }}
                          </a-button>
                          <a-button
                            v-if="canRevokePermission(permission)"
                            size="small"
                            danger
                            @click="revokeGrant(permission.capability)"
                          >
                            {{ t('plugins.actions.revokeGrant') }}
                          </a-button>
                        </div>
                      </article>
                    </div>
                  </a-skeleton>
                </div>
              </a-tab-pane>

              <!-- TAB 3: Console -->
              <a-tab-pane key="console" force-render>
                <template #tab>
                  <span class="premium-tab-label">
                    {{ t('plugins.sections.console') }}
                    <a-tag size="small" :bordered="false" class="tab-badge">{{ consoleFrameCount }}</a-tag>
                  </span>
                </template>

                <div class="tab-pane-content">
                  <div class="plugin-console-header">
                    <div class="plugin-console-title">
                      <span class="console-status-indicator">
                        <span class="status-chip__dot pulsing" :style="{ backgroundColor: getConsoleSnapshotStatusColor(consoleSnapshot.status) }"></span>
                        <a-tag :color="getConsoleStatusColor(consoleSnapshot.status)" class="console-status-tag">{{ getConnectionStatusLabel(consoleSnapshot.status) }}</a-tag>
                      </span>
                      <span class="plugin-console-count">{{ t('plugins.console.outputCount', { count: consoleFrameCount }) }}</span>
                    </div>
                    <div class="plugin-console-actions">
                      <a-tooltip :title="t('plugins.actions.reconnectConsole')">
                        <a-button
                          size="small"
                          class="plugin-console-icon-button"
                          :aria-label="t('plugins.actions.reconnectConsole')"
                          @click="socketStore.reconnectConsole()"
                        >
                          <template #icon>
                            <ReloadOutlined />
                          </template>
                          {{ t('plugins.actions.reconnectConsole') }}
                        </a-button>
                      </a-tooltip>
                      <a-tooltip :title="t('plugins.actions.clearConsole')">
                        <a-button
                          size="small"
                          class="plugin-console-icon-button"
                          :disabled="consoleFrameCount === 0"
                          :aria-label="t('plugins.actions.clearConsole')"
                          @click="clearConsole"
                        >
                          <template #icon>
                            <ClearOutlined />
                          </template>
                        </a-button>
                      </a-tooltip>
                    </div>
                  </div>

                  <div class="plugin-console-panel" :class="{ 'is-empty': consoleFrameCount === 0 }">
                    <div v-if="consoleSnapshot.lastError" class="plugin-console-warning" role="status">
                      <strong>{{ t('plugins.consoleUnavailable') }}</strong>
                      <span>{{ consoleSnapshot.lastError }}</span>
                    </div>

                    <div v-if="consoleFrameCount === 0" class="plugin-console-empty">
                      <span class="plugin-console-empty__prompt">&gt;_</span>
                      <span>{{ t('plugins.empty.console') }}</span>
                    </div>

                    <div v-else ref="consoleScroller" class="console-terminal" :aria-label="t('plugins.console.ariaLabel')">
                      <article
                        v-for="(frame, index) in consoleFrames"
                        :key="getConsoleFrameKey(frame, index)"
                        class="console-terminal-line"
                      >
                        <div class="console-terminal-line__meta">
                          <time :datetime="frame.timestamp">{{ formatDateTime(frame.timestamp) }}</time>
                          <div class="console-terminal-line__badges">
                            <a-tag :color="getConsoleStreamColor(frame.stream)" class="stream-badge">{{ getConsoleStreamLabel(frame.stream) }}</a-tag>
                            <a-tag v-if="frame.stream === 'outbound'" :color="getConsoleLevelColor(getConsoleLevel(frame))" class="level-badge">
                              {{ getConsoleLevelLabel(getConsoleLevel(frame)) }}
                            </a-tag>
                            <span v-if="getConsoleRequestId(frame)" class="console-request-id">{{ getConsoleRequestId(frame) }}</span>
                          </div>
                        </div>
                        <pre class="console-terminal-line__text">{{ escapeUnsafeDisplayText(frame.text) }}</pre>
                      </article>
                    </div>
                  </div>
                </div>
              </a-tab-pane>
            </a-tabs>
          </a-card>
        </main>

        <aside class="plugin-detail-side-column">
          <a-card :bordered="false" class="plugin-detail-summary-card">
            <template #title>
              <div class="card-header">
                <span>{{ t('plugins.sections.runtimeSummary') }}</span>
              </div>
            </template>

            <div class="plugin-detail-summary-stack">
              <section class="plugin-detail-summary-section">
                <h3>{{ t('plugins.sections.packageInfo') }}</h3>
                <dl class="plugin-detail-kv-list">
                  <div v-for="item in packageInfoRows" :key="item.key">
                    <dt>{{ item.label }}</dt>
                    <dd>{{ item.value }}</dd>
                  </div>
                </dl>
              </section>

              <section class="plugin-detail-summary-section">
                <h3>{{ t('plugins.sections.sourceInfo') }}</h3>
                <dl class="plugin-detail-kv-list">
                  <div v-for="item in sourceInfoRows" :key="item.key">
                    <dt>{{ item.label }}</dt>
                    <dd>{{ item.value }}</dd>
                  </div>
                </dl>
              </section>

              <section class="plugin-detail-summary-section">
                <h3>{{ t('plugins.sections.runtimeConfig') }}</h3>
                <dl class="plugin-detail-kv-list">
                  <div v-for="item in runtimeInfoRows" :key="item.key">
                    <dt>{{ item.label }}</dt>
                    <dd>{{ item.value }}</dd>
                  </div>
                </dl>
                <div class="metadata-section">
                  <strong>{{ t('plugins.fields.declaredCapabilities') }}</strong>
                  <div v-if="hasItems(currentPlugin?.declared_capabilities)" class="tag-list">
                    <a-tag v-for="capability in currentPlugin?.declared_capabilities" :key="capability" class="cap-tag">{{ capability }}</a-tag>
                  </div>
                  <p v-else class="empty-val">{{ t('display.empty') }}</p>
                </div>
              </section>

              <details class="plugin-detail-disclosure">
                <summary>
                  <span>{{ t('plugins.sections.details') }}</span>
                  <a-tag class="meta-tag">{{ t('plugins.sections.metadata') }}</a-tag>
                </summary>

                <div class="plugin-detail-detail-stack">
                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.description') }}</strong>
                    <p class="meta-desc">{{ getMetadataText(currentPlugin?.description) }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.icon') }}</strong>
                    <p class="meta-icon">{{ getMetadataText(currentPlugin?.icon) }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.repo') }}</strong>
                    <a v-if="currentPlugin?.repo" :href="currentPlugin.repo" target="_blank" rel="noreferrer" class="meta-link">{{ currentPlugin.repo }}</a>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.homepage') }}</strong>
                    <a v-if="currentPlugin?.homepage" :href="currentPlugin.homepage" target="_blank" rel="noreferrer" class="meta-link">{{ currentPlugin.homepage }}</a>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.keywords') }}</strong>
                    <div v-if="hasItems(currentPlugin?.keywords)" class="tag-list">
                      <a-tag v-for="keyword in currentPlugin?.keywords" :key="keyword">{{ keyword }}</a-tag>
                    </div>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.platforms') }}</strong>
                    <div v-if="hasItems(currentPlugin?.platforms)" class="tag-list">
                      <a-tag v-for="platform in currentPlugin?.platforms" :key="platform">{{ platform }}</a-tag>
                    </div>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.systemDependencies') }}</strong>
                    <div v-if="hasItems(currentPlugin?.system_dependencies)" class="tag-list">
                      <a-tag v-for="dependency in currentPlugin?.system_dependencies" :key="dependency">{{ dependency }}</a-tag>
                    </div>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.dependencies') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.dependencies)" class="metadata-json">{{ getJsonPreview(currentPlugin?.dependencies) }}</pre>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.scopes') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.scopes)" class="metadata-json">{{ getJsonPreview(currentPlugin?.scopes) }}</pre>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.defaultConfig') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.default_config)" class="metadata-json">{{ getJsonPreview(currentPlugin?.default_config) }}</pre>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.screenshots') }}</strong>
                    <div v-if="hasItems(currentPlugin?.screenshots)" class="screenshot-list">
                      <article v-for="screenshot in currentPlugin?.screenshots" :key="screenshot.path" class="screenshot-item">
                        <span class="ss-path">{{ t('plugins.fields.screenshotPath') }}：{{ screenshot.path }}</span>
                        <span class="ss-alt">{{ t('plugins.fields.screenshotAlt') }}：{{ getScreenshotAlt(screenshot) }}</span>
                      </article>
                    </div>
                    <p v-else class="empty-val">{{ t('display.empty') }}</p>
                  </section>
                </div>
              </details>
            </div>
          </a-card>
        </aside>
      </div>
    </template>

    <PluginManagementUIHost
      v-else-if="currentPlugin?.management_ui"
      :plugin="currentPlugin"
      :title="managementPanelTitle"
    />

    <a-skeleton v-else active :loading="detailLoading">
      <a-card :bordered="false">
        <template #title>
          <div class="card-header">
            <span>{{ managementPanelTitle }}</span>
          </div>
        </template>
      </a-card>
    </a-skeleton>
  </AppPage>

  <a-modal
    v-model:open="permissionDialogVisible"
    :get-container="false"
    :title="permissionDialogTitle"
    :confirm-loading="grantBusy"
    :ok-text="permissionDialogOkText"
    :cancel-text="t('dashboard.previewCancel')"
    :ok-button-props="{ disabled: selectedCapabilities.length === 0 }"
    @cancel="closePermissionDialog"
    @ok="submitPermissionDialog"
  >
    <a-alert
      v-if="permissionDialogBody"
      :message="permissionDialogTitle"
      :type="permissionDialogMode === 'scope_changed' ? 'info' : 'warning'"
      :description="permissionDialogBody"
      show-icon
      class="section-gap"
    />

    <a-empty
      v-if="permissionDialogCandidates.length === 0"
      :description="t('plugins.empty.permissions')"
    />

    <a-checkbox-group v-else v-model:value="selectedCapabilities" class="permission-dialog-list">
      <a-checkbox
        v-for="permission in permissionDialogCandidates"
        :key="permission.capability"
        :value="permission.capability"
      >
        <div class="permission-dialog-item">
          <strong>{{ permission.capability }}</strong>
          <small>
            {{ getPermissionRequirementLabel(permission.requirement) }} ·
            {{ getPermissionStatusLabel(permission.status) }} ·
            {{ getPermissionSourceLabel(permission.source) }}
          </small>
        </div>
      </a-checkbox>
    </a-checkbox-group>
  </a-modal>

  <a-modal
    v-model:open="uninstallDialogVisible"
    :get-container="false"
    :title="t('plugins.uninstallConfirmTitle')"
    :confirm-loading="actionPending[pluginId] === 'uninstall'"
    :ok-text="t('plugins.actions.uninstallConfirm')"
    :cancel-text="t('dashboard.previewCancel')"
    :ok-button-props="{ danger: true }"
    @ok="uninstallPlugin"
  >
    <p>{{ t('plugins.uninstallConfirmBody') }}</p>
  </a-modal>
</template>

<style scoped lang="scss">
:deep(.ant-card) {
  box-shadow: var(--shadow-xs);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface);
}

/* Tab Panel Styling */
.plugin-detail-tab-card {
  :deep(.ant-card-body) {
    padding: 0;
  }
}

.premium-detail-tabs {
  :deep(.ant-tabs-nav) {
    padding-inline: 18px;
    margin-bottom: 0;
    border-bottom: 1px solid var(--border);
  }

  :deep(.ant-tabs-tab) {
    padding-block: 14px;
  }
}

.premium-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 0.92rem;
}

.tab-badge {
  font-family: var(--font-mono);
  font-size: 0.72rem;
  padding-inline: 6px;
  border-radius: 4px;
}

.tab-pane-content {
  padding: 18px;
}

/* Actions in title */
.plugin-detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plugin-detail-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

/* Premium Hero Design */
.plugin-detail-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 16px 20px;
  padding: 24px;
  border: 1px solid var(--border);
  border-radius: var(--radius-xl);
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--surface-soft) 94%, var(--accent-soft)) 0%, color-mix(in srgb, var(--surface) 92%, transparent) 100%),
    var(--surface-strong);
  box-shadow: var(--shadow-sm);
  position: relative;
  overflow: hidden;

  &::before {
    content: '';
    position: absolute;
    top: -50px;
    right: -50px;
    width: 150px;
    height: 150px;
    background: radial-gradient(circle, color-mix(in srgb, var(--accent) 8%, transparent) 0%, transparent 70%);
    pointer-events: none;
  }
}

.plugin-detail-hero__identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 18px;
}

.plugin-detail-hero__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 58px;
  height: 58px;
  flex: 0 0 auto;
  border-radius: var(--radius-lg);
  font-size: 1.58rem;
  font-weight: 800;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);
}

.plugin-detail-hero__copy {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.plugin-title {
  overflow: hidden;
  color: var(--text);
  font-size: 1.48rem;
  font-weight: 750;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-id-sub {
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-detail-hero__eyebrow {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.premium-badge {
  font-size: 0.72rem;
  font-weight: 600;
  border-radius: 4px;
  padding-inline: 8px;
}

.role-badge {
  background: color-mix(in srgb, var(--accent) 10%, transparent);
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 15%, transparent);
}

.trust-badge {
  background: color-mix(in srgb, var(--success) 8%, transparent);
  color: var(--success);
  border: 1px solid color-mix(in srgb, var(--success) 12%, transparent);
}

.plugin-detail-hero__tools {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

/* Premium micro indicators dot bar */
.plugin-detail-status-chips {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  padding: 12px 16px;
  border: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-soft) 50%, transparent);
}

.status-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.8rem;
}

.status-chip__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;

  &.pulsing {
    animation: status-pulse 2s infinite ease-in-out;
  }
}

@keyframes status-pulse {
  0% { transform: scale(0.9); opacity: 0.6; }
  50% { transform: scale(1.15); opacity: 1; }
  100% { transform: scale(0.9); opacity: 0.6; }
}

.status-chip__label {
  color: var(--muted);
  font-weight: 500;
}

.status-tag {
  font-family: var(--font-mono);
  font-size: 0.74rem;
  margin-inline-end: 0 !important;
}

/* Fact list */
.plugin-detail-hero__facts {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px 24px;
  margin: 0;
}

.fact-item {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.fact-label {
  color: var(--muted);
  font-size: 0.76rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.fact-value {
  min-width: 0;
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text);
  font-size: 0.88rem;
  font-weight: 550;
  line-height: 1.45;
}

/* Workspace Structure */
.plugin-detail-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(310px, 370px);
  align-items: start;
  gap: 16px;
}

.plugin-detail-main-column,
.plugin-detail-side-column,
.plugin-detail-summary-stack,
.plugin-detail-detail-stack,
.permission-list {
  display: grid;
  gap: 14px;
}

/* Summary Card styling */
.plugin-detail-summary-card :deep(.ant-card-body) {
  padding: 16px;
}

.plugin-detail-summary-section {
  display: grid;
  gap: 12px;
}

.plugin-detail-summary-section + .plugin-detail-summary-section {
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

.plugin-detail-summary-section h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.35;
}

.plugin-detail-kv-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 14px;
  margin: 0;

  div {
    display: grid;
    min-width: 0;
    gap: 4px;
  }

  dt {
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 500;
  }

  dd {
    min-width: 0;
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--text);
    font-size: 0.84rem;
    font-weight: 555;
    line-height: 1.45;
  }
}

.metadata-section {
  display: grid;
  gap: 8px;

  strong {
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--text);
  }

  p, a {
    margin: 0;
    word-break: break-word;
    font-size: 0.84rem;
  }
}

.cap-tag {
  font-family: var(--font-mono);
  font-size: 0.76rem;
  border-radius: 4px;
  margin-block: 2px;
}

.metadata-json {
  margin: 0;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  color: var(--text);
  border: 1px solid var(--border);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.6;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.screenshot-list {
  display: grid;
  gap: 8px;
}

.screenshot-item {
  display: grid;
  gap: 4px;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  font-size: 0.8rem;

  .ss-path {
    font-family: var(--font-mono);
    color: var(--muted);
  }
  .ss-alt {
    color: var(--text);
    font-weight: 550;
  }
}

.plugin-detail-disclosure {
  border-top: 1px solid var(--border);
  padding-top: 14px;

  summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    cursor: pointer;
    color: var(--text);
    font-weight: 700;
    font-size: 0.88rem;
    list-style: none;

    &::-webkit-details-marker {
      display: none;
    }

    &::after {
      content: '+';
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      border: 1px solid var(--border);
      border-radius: var(--radius-sm);
      color: var(--muted);
      font-family: var(--font-mono);
      font-weight: 500;
      font-size: 0.8rem;
    }
  }

  &[open] summary {
    margin-bottom: 12px;

    &::after {
      content: '-';
    }
  }
}

.meta-tag {
  font-size: 0.72rem;
}

/* Permissions view styling */
.permission-actions-header {
  margin-bottom: 12px;
}

.plugin-detail-inline-state {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-bottom: 14px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);

  strong {
    color: var(--text);
    font-size: 0.88rem;
  }

  span {
    color: var(--muted);
    font-size: 0.82rem;
  }

  &.is-success {
    border-color: var(--border-success);
    background: color-mix(in srgb, var(--surface-success) 70%, var(--surface));
  }

  &.is-warning {
    border-color: var(--border-warning);
    background: color-mix(in srgb, var(--surface-warning) 70%, var(--surface));
  }
}

.permission-list {
  display: grid;
  gap: 10px;
}

.permission-item {
  display: grid;
  grid-template-columns: minmax(200px, 1.2fr) minmax(220px, 1fr) auto;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  transition: all 0.2s ease;

  &:hover {
    border-color: var(--border-accent);
    background: var(--surface);
    box-shadow: var(--shadow-xs);
  }
}

.permission-item__capability {
  display: grid;
  min-width: 0;
  gap: 6px;

  strong {
    overflow: hidden;
    color: var(--text);
    font-family: var(--font-mono);
    font-size: 0.88rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.permission-item__tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.tag-compact {
  font-size: 0.72rem;
  margin-inline-end: 0 !important;
}

.permission-item__time {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  min-width: 0;
}

.time-row {
  display: grid;
  min-width: 0;
  gap: 2px;
  color: var(--text);
  font-size: 0.8rem;

  small {
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 500;
  }
}

.permission-item__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

/* Console tab layout styling */
.plugin-console-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.plugin-console-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.console-status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.console-status-tag {
  font-family: var(--font-mono);
  font-size: 0.74rem;
}

.plugin-console-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.plugin-console-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 28px;
  gap: 6px;
  font-size: 0.8rem;
}

/* Glass Console Terminal Design */
.plugin-console-panel {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.02), rgba(30, 41, 59, 0.05)),
    var(--surface-soft);
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.04);

  &.is-empty {
    background: var(--surface-soft);
  }
}

[data-theme='dark'] .plugin-console-panel {
  background:
    linear-gradient(180deg, rgba(2, 6, 23, 0.6) 0%, rgba(2, 6, 23, 0.85) 100%),
    #050811;
  box-shadow: inset 0 0 16px rgba(0, 0, 0, 0.4), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  border-color: rgba(148, 163, 184, 0.12);
}

.plugin-console-warning {
  display: grid;
  gap: 4px;
  margin: 12px;
  padding: 10px 14px;
  border: 1px solid var(--border-warning);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-warning) 70%, var(--surface));

  strong {
    color: var(--text);
    font-size: 0.86rem;
  }
  span {
    color: var(--muted);
    font-size: 0.82rem;
    word-break: break-all;
  }
}

.plugin-console-empty {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 32px 20px;
  color: var(--muted);
  font-size: 0.88rem;
}

.plugin-console-empty__prompt {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 26px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-strong);
  color: var(--accent);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: bold;
}

.console-terminal {
  max-height: clamp(260px, 48vh, 550px);
  overflow: auto;
  padding-block: 4px;
}

.console-terminal-line {
  display: grid;
  grid-template-columns: minmax(210px, 260px) minmax(0, 1fr);
  gap: 16px;
  padding: 10px 16px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
  color: var(--text);

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: color-mix(in srgb, var(--accent) 5%, transparent);
  }
}

.console-terminal-line__meta {
  display: grid;
  align-content: start;
  gap: 6px;
  min-width: 0;

  time {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.74rem;
    line-height: 1.4;
  }
}

.console-terminal-line__badges {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.stream-badge, .level-badge {
  font-size: 0.7rem;
  padding-inline: 4px;
  border-radius: 3px;
  margin-inline-end: 0 !important;
}

.console-request-id {
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.7rem;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.console-terminal-line__text {
  min-width: 0;
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.58;
  unicode-bidi: plaintext;
}

[data-theme='dark'] .console-terminal-line__text {
  color: #e2e8f0;
}

/* Responsive queries */
@media (max-width: 1180px) {
  .plugin-detail-workspace {
    grid-template-columns: 1fr;
  }

  .plugin-detail-side-column {
    order: -1;
  }
}

@media (max-width: 860px) {
  .plugin-detail-hero,
  .plugin-detail-status-chips,
  .plugin-detail-hero__facts,
  .plugin-detail-kv-list,
  .permission-item,
  .permission-item__time {
    grid-template-columns: 1fr;
  }

  .plugin-detail-hero__tools,
  .permission-item__actions {
    justify-content: flex-start;
  }
}

@media (max-width: 720px) {
  .console-terminal-line {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
