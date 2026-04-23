<script setup lang="ts">
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
      <div class="content-grid">
        <a-card :bordered="false">
          <template #title>
            <div class="card-header">
              <span>{{ t('plugins.sections.current') }}</span>
              <ManagementContextActions :actions="pluginWorkbenchActions" />
            </div>
          </template>

          <a-skeleton :loading="detailLoading" active>
            <a-descriptions :column="1" bordered size="small">
              <a-descriptions-item :label="t('plugins.fields.name')">{{ current?.name ?? t('display.empty') }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.role')">
                {{ getPluginRoleLabel(current?.role) }}
                <small v-if="current?.role"> · {{ current.role }}</small>
              </a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.registration')">
                {{ getPluginRegistrationStateLabel(current?.registration_state) }}
                <small v-if="current?.registration_state"> · {{ current.registration_state }}</small>
              </a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.desired')">
                {{ getPluginDesiredStateLabel(current?.desired_state) }}
                <small v-if="current?.desired_state"> · {{ current.desired_state }}</small>
              </a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.runtime')">
                {{ getPluginRuntimeStateLabel(current?.runtime_state) }}
                <small v-if="current?.runtime_state"> · {{ current.runtime_state }}</small>
              </a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.display')">
                {{ getPluginDisplayStateLabel(current?.display_state) }}
                <small v-if="current?.display_state"> · {{ current.display_state }}</small>
              </a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.trust')">{{ current?.trust?.label ?? t('display.empty') }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.sourceRoot')">{{ current?.source?.root ?? t('display.empty') }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.sourceRef')">{{ current?.source?.package_source_ref ?? current?.source?.package_source_type ?? t('display.empty') }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.conflicts')">
                <div v-if="current?.command_conflicts?.length" class="table-actions">
                  <a-tag v-for="command in current.command_conflicts" :key="command" color="warning">
                    {{ command }}
                  </a-tag>
                </div>
                <span v-else>{{ t('display.empty') }}</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-skeleton>
        </a-card>

        <a-card :bordered="false">
          <template #title>
            <div class="card-header">
              <span>{{ t('plugins.sections.permissions') }}</span>
              <div class="table-actions">
                <a-tag>{{ currentPermissions.length }}</a-tag>
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

          <a-alert
            v-if="isBuiltinPlugin"
            :message="t('plugins.builtinAutoGrantTitle')"
            type="success"
            :description="t('plugins.builtinAutoGrantBody')"
            show-icon
            class="section-gap"
          />

          <a-alert
            v-else-if="missingRequiredPermissions.length > 0"
            :message="t('plugins.permissionPendingTitle')"
            type="warning"
            :description="t('plugins.permissionPendingBody', { count: missingRequiredPermissions.length })"
            show-icon
            class="section-gap"
          />

          <a-skeleton :loading="grantBusy" active>
            <a-empty v-if="currentPermissions.length === 0" :description="t('plugins.empty.permissions')" />

            <div v-else class="permission-list">
              <article v-for="permission in currentPermissions" :key="permission.capability" class="permission-item">
                <div class="permission-item__main">
                  <div class="permission-item__title">
                    <strong>{{ permission.capability }}</strong>
                    <div class="table-actions">
                      <a-tag color="blue">{{ getPermissionRequirementLabel(permission.requirement) }}</a-tag>
                      <a-tag :color="permission.status === 'granted' ? 'success' : 'warning'">{{ getPermissionStatusLabel(permission.status) }}</a-tag>
                      <a-tag>{{ getPermissionSourceLabel(permission.source) }}</a-tag>
                    </div>
                  </div>
                  <small>{{ t('plugins.fields.grantedAt') }}：{{ formatDateTime(getGrantedAt(permission.capability)) }}</small>
                  <small>{{ t('plugins.fields.expiresAt') }}：{{ formatDateTime(permission.expires_at ?? undefined) }}</small>
                </div>

                <div class="table-actions">
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
      </div>

      <div class="content-grid">
        <a-card :bordered="false">
          <template #title>
            <div class="card-header">
              <span>{{ t('plugins.sections.package') }}</span>
            </div>
          </template>

          <a-skeleton :loading="detailLoading" active>
            <a-descriptions :column="1" bordered size="small">
              <a-descriptions-item :label="t('plugins.fields.version')">{{ getMetadataText(current?.version) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.type')">{{ getMetadataText(current?.type) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.runtimeFamily')">{{ getMetadataText(current?.runtime) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.entry')">{{ getMetadataText(current?.entry) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.author')">{{ getMetadataText(current?.author) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.license')">{{ getMetadataText(current?.license) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.sdkMinVersion')">{{ getMetadataText(current?.sdk_min_version) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.runtimeVersion')">{{ getMetadataText(current?.runtime_version) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.minCoreVersion')">{{ getMetadataText(current?.min_core_version) }}</a-descriptions-item>
              <a-descriptions-item :label="t('plugins.fields.dataSchemaVersion')">{{ getMetadataText(current?.data_schema_version) }}</a-descriptions-item>
            </a-descriptions>
          </a-skeleton>
        </a-card>

        <a-card :bordered="false">
          <template #title>
            <div class="card-header">
              <span>{{ t('plugins.sections.metadata') }}</span>
            </div>
          </template>

          <a-skeleton :loading="detailLoading" active>
            <div class="metadata-stack">
              <section class="metadata-section">
                <strong>{{ t('plugins.fields.description') }}</strong>
                <p>{{ getMetadataText(current?.description) }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.icon') }}</strong>
                <p>{{ getMetadataText(current?.icon) }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.repo') }}</strong>
                <a v-if="current?.repo" :href="current.repo" target="_blank" rel="noreferrer">{{ current.repo }}</a>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.homepage') }}</strong>
                <a v-if="current?.homepage" :href="current.homepage" target="_blank" rel="noreferrer">{{ current.homepage }}</a>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.keywords') }}</strong>
                <div v-if="hasItems(current?.keywords)" class="tag-list">
                  <a-tag v-for="keyword in current?.keywords" :key="keyword">{{ keyword }}</a-tag>
                </div>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.platforms') }}</strong>
                <div v-if="hasItems(current?.platforms)" class="tag-list">
                  <a-tag v-for="platform in current?.platforms" :key="platform">{{ platform }}</a-tag>
                </div>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.systemDependencies') }}</strong>
                <div v-if="hasItems(current?.system_dependencies)" class="tag-list">
                  <a-tag v-for="dependency in current?.system_dependencies" :key="dependency">{{ dependency }}</a-tag>
                </div>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.screenshots') }}</strong>
                <div v-if="hasItems(current?.screenshots)" class="screenshot-list">
                  <article v-for="screenshot in current?.screenshots" :key="screenshot.path" class="screenshot-item">
                    <span>{{ t('plugins.fields.screenshotPath') }}：{{ screenshot.path }}</span>
                    <span>{{ t('plugins.fields.screenshotAlt') }}：{{ getScreenshotAlt(screenshot) }}</span>
                  </article>
                </div>
                <p v-else>{{ t('display.empty') }}</p>
              </section>
            </div>
          </a-skeleton>
        </a-card>

        <a-card :bordered="false">
          <template #title>
            <div class="card-header">
              <span>{{ t('plugins.sections.runtimeConfig') }}</span>
            </div>
          </template>

          <a-skeleton :loading="detailLoading" active>
            <div class="metadata-stack">
              <section class="metadata-section">
                <strong>{{ t('plugins.fields.concurrency') }}</strong>
                <p>{{ current?.concurrency ?? t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.declaredCapabilities') }}</strong>
                <div v-if="hasItems(current?.declared_capabilities)" class="tag-list">
                  <a-tag v-for="capability in current?.declared_capabilities" :key="capability">{{ capability }}</a-tag>
                </div>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.dependencies') }}</strong>
                <pre v-if="hasObjectValue(current?.dependencies)" class="metadata-json">{{ getJsonPreview(current?.dependencies) }}</pre>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.scopes') }}</strong>
                <pre v-if="hasObjectValue(current?.scopes)" class="metadata-json">{{ getJsonPreview(current?.scopes) }}</pre>
                <p v-else>{{ t('display.empty') }}</p>
              </section>

              <section class="metadata-section">
                <strong>{{ t('plugins.fields.defaultConfig') }}</strong>
                <pre v-if="hasObjectValue(current?.default_config)" class="metadata-json">{{ getJsonPreview(current?.default_config) }}</pre>
                <p v-else>{{ t('display.empty') }}</p>
              </section>
            </div>
          </a-skeleton>
        </a-card>
      </div>

      <a-card :bordered="false">
        <template #title>
          <div class="card-header">
            <span>{{ t('plugins.sections.commands') }}</span>
            <a-tag>{{ current?.commands?.length ?? 0 }}</a-tag>
          </div>
        </template>

        <PluginCommandsPanel
          :commands="current?.commands ?? []"
          :command-conflicts="current?.command_conflicts ?? []"
          :command-prefix="commandPrefix"
        />
      </a-card>

      <a-card :bordered="false">
        <template #title>
          <div class="card-header">
            <span>{{ t('plugins.sections.console') }}</span>
            <div class="table-actions">
              <a-tag :color="getConsoleStatusColor(consoleSnapshot.status)">{{ getConnectionStatusLabel(consoleSnapshot.status) }}</a-tag>
              <a-button size="small" @click="socketStore.reconnectConsole()">{{ t('plugins.actions.reconnectConsole') }}</a-button>
              <a-button size="small" @click="clearConsole">{{ t('plugins.actions.clearConsole') }}</a-button>
            </div>
          </div>
        </template>

        <a-alert
          v-if="consoleSnapshot.lastError"
          :message="t('plugins.consoleUnavailable')"
          type="warning"
          :description="consoleSnapshot.lastError"
          show-icon
          class="section-gap"
        />

        <a-empty v-if="consoleFrames.length === 0" :description="t('plugins.empty.console')" />

        <div v-else ref="consoleScroller" class="console-terminal" aria-label="插件实时控制台">
          <div
            v-for="(frame, index) in consoleFrames"
            :key="getConsoleFrameKey(frame, index)"
            :class="['console-terminal-line', `is-${frame.stream}`, frame.stream === 'outbound' ? `is-${getConsoleLevel(frame)}` : null]"
          >
            <span class="console-meta">
              {{ formatDateTime(frame.timestamp) }} · {{ frame.stream }}
              <template v-if="frame.stream === 'outbound'"> · {{ getConsoleLevel(frame) }}</template>
            </span>
            <pre>{{ escapeUnsafeDisplayText(frame.text) }}</pre>
          </div>
        </div>
      </a-card>
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

.permission-list {
  display: grid;
  gap: 12px;
}

.metadata-stack {
  display: grid;
  gap: 14px;
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
  font-size: 0.95rem;
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
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
}

.permission-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  flex-wrap: wrap;
}

.permission-item__main {
  display: grid;
  gap: 8px;
}

.permission-item__title {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.permission-item small {
  color: var(--muted);
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

.console-terminal {
  min-height: 320px;
  max-height: 560px;
  overflow: auto;
  padding: 12px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  display: grid;
  gap: 8px;
}

.console-terminal-line {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-strong);
  color: var(--text);
  font-family: var(--font-mono);
  box-shadow: inset 2px 0 0 var(--border-accent);
}

.console-terminal-line.is-stderr {
  box-shadow: inset 2px 0 0 var(--border-danger);
}

.console-terminal-line.is-system {
  box-shadow: inset 2px 0 0 var(--border-warning);
}

.console-terminal-line.is-outbound {
  background: var(--surface-success);
}

.console-terminal-line.is-outbound.is-info {
  box-shadow: inset 2px 0 0 var(--border-success);
}

.console-terminal-line.is-outbound.is-warn {
  box-shadow: inset 2px 0 0 var(--border-warning);
}

.console-terminal-line.is-outbound.is-error {
  box-shadow: inset 2px 0 0 var(--border-danger);
}

.console-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.console-terminal-line pre {
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: var(--font-mono);
  line-height: 1.55;
  unicode-bidi: plaintext;
}
</style>
