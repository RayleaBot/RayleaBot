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
import { usePluginsStore } from '@/stores/plugins'
import type { ConsoleFrame } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'
import type { PluginDetail, PluginPermissionSummary } from '@/types/api'

type PermissionDialogMode = 'grant' | 'pending' | 'scope_changed'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const socketStore = useSocketStore()
const configStore = useConfigStore()

const { actionPending, current, detailLoading, grantsLoading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)

const pluginId = computed(() => String(route.params.id))
const currentPlugin = computed(() => current.value?.id === pluginId.value ? current.value : null)
const consoleFrames = computed(() => pluginsStore.getConsole(pluginId.value))
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
      pluginsStore.fetchOutboundConsoleHistory(requestedPluginId).catch(() => []),
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
  pluginsStore.clearConsole(pluginId.value)
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

function getPluginStateColor(status?: string | null) {
  if (!status) return 'default'
  if (status === 'failed' || status === 'error' || status === 'removed') return 'error'
  if (status === 'starting' || status === 'stopping' || status === 'enabling' || status === 'disabling' || status === 'retrying') return 'warning'
  if (status === 'installed' || status === 'enabled' || status === 'running' || status === 'discovered') return 'success'
  return 'default'
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
            <div class="plugin-detail-hero__mark" aria-hidden="true">{{ pluginInitial }}</div>
            <div class="plugin-detail-hero__copy">
              <div class="plugin-detail-hero__eyebrow">
                <a-tag>{{ getPluginRoleLabel(currentPlugin?.role) }}</a-tag>
                <a-tag>{{ currentPlugin?.trust?.label ?? t('display.empty') }}</a-tag>
              </div>
              <strong>{{ pluginDisplayName }}</strong>
              <span>{{ pluginId }}</span>
            </div>
          </div>

          <div class="plugin-detail-hero__tools">
            <ManagementContextActions :actions="pluginWorkbenchActions" />
          </div>

          <div class="plugin-detail-status-strip" :aria-label="t('plugins.sections.statusSummary')">
            <div v-for="item in statusSummaryItems" :key="item.key" class="plugin-detail-status-item">
              <span>{{ item.label }}</span>
              <a-tag :color="getPluginStateColor(item.raw)">
                {{ item.value }}
                <small v-if="item.raw"> · {{ item.raw }}</small>
              </a-tag>
            </div>
          </div>

          <dl class="plugin-detail-hero__facts">
            <div v-for="item in heroFacts" :key="item.key">
              <dt>{{ item.label }}</dt>
              <dd>{{ item.value }}</dd>
            </div>
          </dl>
        </section>
      </a-skeleton>

      <div class="plugin-detail-workspace">
        <main class="plugin-detail-main-column">
          <a-card :bordered="false" class="plugin-detail-section-card">
            <template #title>
              <div class="card-header">
                <span>{{ t('plugins.sections.commands') }}</span>
                <a-tag>{{ currentPlugin?.commands?.length ?? 0 }}</a-tag>
              </div>
            </template>

            <PluginCommandsPanel
              :commands="currentPlugin?.commands ?? []"
              :command-conflicts="currentPlugin?.command_conflicts ?? []"
              :command-prefix="commandPrefix"
            />
          </a-card>

          <a-card :bordered="false" class="plugin-detail-section-card">
            <template #title>
              <div class="card-header">
                <span>{{ t('plugins.sections.permissions') }}</span>
                <div class="table-actions">
                  <a-tag>{{ permissionSummaryLabel }}</a-tag>
                  <a-button
                    v-if="!isBuiltinPlugin && permissionCandidates.length > 0"
                    size="small"
                    type="primary"
                    @click="openPermissionDialog()"
                  >
                    {{ t('plugins.actions.reviewPermissions') }}
                  </a-button>
                </div>
              </div>
            </template>

            <div v-if="isBuiltinPlugin" class="plugin-detail-inline-state is-success">
              <strong>{{ t('plugins.builtinAutoGrantTitle') }}</strong>
              <span>{{ t('plugins.builtinAutoGrantBody') }}</span>
            </div>

            <div v-else-if="missingRequiredPermissions.length > 0" class="plugin-detail-inline-state is-warning">
              <strong>{{ t('plugins.permissionPendingTitle') }}</strong>
              <span>{{ t('plugins.permissionPendingBody', { count: missingRequiredPermissions.length }) }}</span>
            </div>

            <a-skeleton :loading="grantBusy" active>
              <a-empty v-if="currentPermissions.length === 0" :description="t('plugins.empty.permissions')" />

              <div v-else class="permission-list">
                <article v-for="permission in currentPermissions" :key="permission.capability" class="permission-item">
                  <div class="permission-item__capability">
                    <strong>{{ permission.capability }}</strong>
                    <div class="permission-item__tags">
                      <a-tag color="blue">{{ getPermissionRequirementLabel(permission.requirement) }}</a-tag>
                      <a-tag :color="permission.status === 'granted' ? 'success' : 'warning'">{{ getPermissionStatusLabel(permission.status) }}</a-tag>
                      <a-tag>{{ getPermissionSourceLabel(permission.source) }}</a-tag>
                    </div>
                  </div>

                  <div class="permission-item__time">
                    <span>
                      <small>{{ t('plugins.fields.grantedAt') }}</small>
                      {{ formatDateTime(getGrantedAt(permission.capability)) }}
                    </span>
                    <span>
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
          </a-card>

          <a-card :bordered="false" class="plugin-console-card">
            <template #title>
              <div class="plugin-console-header">
                <div class="plugin-console-title">
                  <span>{{ t('plugins.sections.console') }}</span>
                  <a-tag class="plugin-console-count">{{ t('plugins.console.outputCount', { count: consoleFrameCount }) }}</a-tag>
                </div>
                <div class="plugin-console-actions">
                  <a-tag :color="getConsoleStatusColor(consoleSnapshot.status)">{{ getConnectionStatusLabel(consoleSnapshot.status) }}</a-tag>
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
            </template>

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
                      <a-tag :color="getConsoleStreamColor(frame.stream)">{{ getConsoleStreamLabel(frame.stream) }}</a-tag>
                      <a-tag v-if="frame.stream === 'outbound'" :color="getConsoleLevelColor(getConsoleLevel(frame))">
                        {{ getConsoleLevelLabel(getConsoleLevel(frame)) }}
                      </a-tag>
                      <span v-if="getConsoleRequestId(frame)" class="console-request-id">{{ getConsoleRequestId(frame) }}</span>
                    </div>
                  </div>
                  <pre class="console-terminal-line__text">{{ escapeUnsafeDisplayText(frame.text) }}</pre>
                </article>
              </div>
            </div>
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
                    <a-tag v-for="capability in currentPlugin?.declared_capabilities" :key="capability">{{ capability }}</a-tag>
                  </div>
                  <p v-else>{{ t('display.empty') }}</p>
                </div>
              </section>

              <details class="plugin-detail-disclosure">
                <summary>
                  <span>{{ t('plugins.sections.details') }}</span>
                  <a-tag>{{ t('plugins.sections.metadata') }}</a-tag>
                </summary>

                <div class="plugin-detail-detail-stack">
                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.description') }}</strong>
                    <p>{{ getMetadataText(currentPlugin?.description) }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.icon') }}</strong>
                    <p>{{ getMetadataText(currentPlugin?.icon) }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.repo') }}</strong>
                    <a v-if="currentPlugin?.repo" :href="currentPlugin.repo" target="_blank" rel="noreferrer">{{ currentPlugin.repo }}</a>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.homepage') }}</strong>
                    <a v-if="currentPlugin?.homepage" :href="currentPlugin.homepage" target="_blank" rel="noreferrer">{{ currentPlugin.homepage }}</a>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.keywords') }}</strong>
                    <div v-if="hasItems(currentPlugin?.keywords)" class="tag-list">
                      <a-tag v-for="keyword in currentPlugin?.keywords" :key="keyword">{{ keyword }}</a-tag>
                    </div>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.platforms') }}</strong>
                    <div v-if="hasItems(currentPlugin?.platforms)" class="tag-list">
                      <a-tag v-for="platform in currentPlugin?.platforms" :key="platform">{{ platform }}</a-tag>
                    </div>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.systemDependencies') }}</strong>
                    <div v-if="hasItems(currentPlugin?.system_dependencies)" class="tag-list">
                      <a-tag v-for="dependency in currentPlugin?.system_dependencies" :key="dependency">{{ dependency }}</a-tag>
                    </div>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.dependencies') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.dependencies)" class="metadata-json">{{ getJsonPreview(currentPlugin?.dependencies) }}</pre>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.scopes') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.scopes)" class="metadata-json">{{ getJsonPreview(currentPlugin?.scopes) }}</pre>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.defaultConfig') }}</strong>
                    <pre v-if="hasObjectValue(currentPlugin?.default_config)" class="metadata-json">{{ getJsonPreview(currentPlugin?.default_config) }}</pre>
                    <p v-else>{{ t('display.empty') }}</p>
                  </section>

                  <section class="metadata-section">
                    <strong>{{ t('plugins.fields.screenshots') }}</strong>
                    <div v-if="hasItems(currentPlugin?.screenshots)" class="screenshot-list">
                      <article v-for="screenshot in currentPlugin?.screenshots" :key="screenshot.path" class="screenshot-item">
                        <span>{{ t('plugins.fields.screenshotPath') }}：{{ screenshot.path }}</span>
                        <span>{{ t('plugins.fields.screenshotAlt') }}：{{ getScreenshotAlt(screenshot) }}</span>
                      </article>
                    </div>
                    <p v-else>{{ t('display.empty') }}</p>
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
}

.plugin-detail-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.plugin-detail-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 14px 18px;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--surface-strong) 94%, white) 0%, color-mix(in srgb, var(--surface-soft) 88%, transparent) 100%),
    var(--surface-strong);
  box-shadow: var(--shadow-xs);
}

.plugin-detail-hero__identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 14px;
}

.plugin-detail-hero__mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--accent) 12%, var(--surface-strong));
  color: var(--accent);
  font-size: 1.12rem;
  font-weight: 700;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 22%, transparent);
}

.plugin-detail-hero__copy {
  display: grid;
  min-width: 0;
  gap: 5px;
}

.plugin-detail-hero__copy strong {
  overflow: hidden;
  color: var(--text);
  font-size: 1.22rem;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-detail-hero__copy > span {
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-detail-hero__eyebrow,
.plugin-detail-hero__tools {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.plugin-detail-hero__tools {
  justify-content: flex-end;
}

.plugin-detail-status-strip {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(4, minmax(130px, 1fr));
  gap: 8px;
}

.plugin-detail-status-item {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-strong) 86%, transparent);
}

.plugin-detail-status-item > span {
  color: var(--muted);
  font-size: 0.76rem;
}

.plugin-detail-status-item :deep(.ant-tag) {
  width: fit-content;
  margin-inline-end: 0;
}

.plugin-detail-status-item small {
  font-family: var(--font-mono);
}

.plugin-detail-hero__facts {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px 16px;
  margin: 0;
}

.plugin-detail-hero__facts div,
.plugin-detail-kv-list div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.plugin-detail-hero__facts dt,
.plugin-detail-kv-list dt {
  color: var(--muted);
  font-size: 0.76rem;
}

.plugin-detail-hero__facts dd,
.plugin-detail-kv-list dd {
  min-width: 0;
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text);
  line-height: 1.45;
}

.plugin-detail-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 380px);
  align-items: start;
  gap: 12px;
}

.plugin-detail-main-column,
.plugin-detail-side-column,
.plugin-detail-summary-stack,
.plugin-detail-detail-stack,
.permission-list {
  display: grid;
  gap: 12px;
}

.plugin-detail-section-card :deep(.ant-card-body),
.plugin-detail-summary-card :deep(.ant-card-body) {
  padding-top: 12px;
}

.plugin-detail-summary-section {
  display: grid;
  gap: 10px;
}

.plugin-detail-summary-section + .plugin-detail-summary-section {
  padding-top: 12px;
  border-top: 1px solid var(--border);
}

.plugin-detail-summary-section h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.9rem;
  line-height: 1.35;
}

.plugin-detail-kv-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 14px;
  margin: 0;
}

.metadata-section {
  display: grid;
  gap: 8px;
}

.metadata-section p,
.metadata-section a {
  margin: 0;
  word-break: break-word;
}

.metadata-section strong {
  font-size: 0.88rem;
}

.metadata-json {
  margin: 0;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.6;
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.screenshot-list {
  display: grid;
  gap: 10px;
}

.screenshot-item {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
}

.plugin-detail-disclosure {
  border-top: 1px solid var(--border);
  padding-top: 12px;
}

.plugin-detail-disclosure summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  cursor: pointer;
  color: var(--text);
  font-weight: 600;
  list-style: none;
}

.plugin-detail-disclosure summary::-webkit-details-marker {
  display: none;
}

.plugin-detail-disclosure summary::after {
  content: '+';
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--muted);
  font-family: var(--font-mono);
  font-weight: 500;
}

.plugin-detail-disclosure[open] summary {
  margin-bottom: 12px;
}

.plugin-detail-disclosure[open] summary::after {
  content: '-';
}

.plugin-detail-inline-state {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.plugin-detail-inline-state strong {
  color: var(--text);
  font-size: 0.88rem;
}

.plugin-detail-inline-state span {
  color: var(--muted);
  font-size: 0.82rem;
}

.plugin-detail-inline-state.is-success {
  border-color: var(--border-success);
  background: color-mix(in srgb, var(--surface-success) 72%, var(--surface-strong));
}

.plugin-detail-inline-state.is-warning {
  border-color: var(--border-warning);
  background: color-mix(in srgb, var(--surface-warning) 72%, var(--surface-strong));
}

.permission-item {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(240px, 0.8fr) auto;
  align-items: center;
  gap: 12px;
  padding: 11px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--surface-soft) 72%, transparent);
}

.permission-item:hover {
  background: color-mix(in srgb, var(--accent) 5%, var(--surface-soft));
}

.permission-item__capability,
.permission-item__time {
  display: grid;
  min-width: 0;
  gap: 6px;
}

.permission-item__capability strong {
  overflow: hidden;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 0.86rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.permission-item__tags,
.permission-item__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.permission-item__time {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.permission-item__time span {
  display: grid;
  min-width: 0;
  gap: 2px;
  color: var(--text);
  font-size: 0.8rem;
}

.permission-item__time small {
  color: var(--muted);
  font-size: 0.72rem;
}

.permission-item__actions {
  justify-content: flex-end;
}

.permission-dialog-list {
  display: grid;
  gap: 12px;
}

.permission-dialog-item {
  display: grid;
  gap: 4px;
}

.permission-dialog-item small {
  color: var(--muted);
}

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
  .plugin-detail-status-strip,
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

.plugin-console-card :deep(.ant-card-body) {
  padding-top: 12px;
}

.plugin-console-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.plugin-console-title,
.plugin-console-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.plugin-console-title {
  min-width: 0;
}

.plugin-console-count {
  margin-inline-end: 0;
  color: var(--muted);
  font-weight: 500;
}

.plugin-console-actions {
  justify-content: flex-end;
}

.plugin-console-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.plugin-console-panel {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-strong) 96%, transparent), color-mix(in srgb, var(--surface-soft) 82%, transparent)),
    var(--surface-strong);
}

.plugin-console-panel.is-empty {
  background: color-mix(in srgb, var(--surface-soft) 92%, transparent);
}

.plugin-console-warning {
  display: grid;
  gap: 4px;
  margin: 12px 12px 0;
  padding: 10px 12px;
  border: 1px solid var(--border-warning);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--surface-warning) 72%, var(--surface-strong));
}

.plugin-console-warning strong {
  color: var(--text);
  font-size: 0.86rem;
}

.plugin-console-warning span {
  color: var(--muted);
  font-size: 0.82rem;
  word-break: break-word;
}

.plugin-console-empty {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 24px 16px;
  color: var(--muted);
}

.plugin-console-empty__prompt {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 28px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-strong);
  color: var(--accent);
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.console-terminal {
  max-height: clamp(260px, 42vh, 520px);
  overflow: auto;
}

.console-terminal-line {
  display: grid;
  grid-template-columns: minmax(210px, 280px) minmax(0, 1fr);
  gap: 12px;
  padding: 11px 14px;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}

.console-terminal-line:last-child {
  border-bottom: none;
}

.console-terminal-line:hover {
  background: color-mix(in srgb, var(--accent) 5%, transparent);
}

.console-terminal-line__meta {
  display: grid;
  align-content: start;
  gap: 6px;
  min-width: 0;
}

.console-terminal-line__meta time {
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.76rem;
  line-height: 1.4;
}

.console-terminal-line__badges {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.console-request-id {
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.72rem;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.console-terminal-line__text {
  min-width: 0;
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.58;
  unicode-bidi: plaintext;
}

@media (max-width: 720px) {
  .plugin-console-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .console-terminal-line {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
