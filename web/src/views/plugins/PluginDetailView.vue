<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
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
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { ConsoleFrame } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'
import type { PluginPermissionSummary } from '@/types/api'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const socketStore = useSocketStore()
const configStore = useConfigStore()

const { actionPending, current, detailLoading, grantsLoading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)

const pluginId = computed(() => String(route.params.id))
const consoleFrames = computed(() => pluginsStore.getConsole(pluginId.value))
const currentGrants = computed(() => pluginsStore.getGrants(pluginId.value))
const currentPermissions = computed(() => current.value?.permissions ?? [])
const consoleSnapshot = computed(() => socketStore.snapshots.pluginConsole)
const grantBusy = computed(() => grantsLoading.value[pluginId.value] ?? false)
const loadError = ref<string | null>(null)
const operationError = ref<string | null>(null)
const consoleScroller = ref<HTMLElement | null>(null)
const permissionDialogVisible = ref(false)
const uninstallDialogVisible = ref(false)
const selectedCapabilities = ref<string[]>([])
const resumeEnableAfterGrant = ref(false)
let detailLoadVersion = 0
let pageActive = true

const commandPrefix = computed(() => getPrimaryCommandPrefix(configDocument.value?.command?.prefixes))
const isBuiltinPlugin = computed(() => current.value?.role === 'builtin')
const permissionCandidates = computed(() => currentPermissions.value.filter((permission) => permission.status === 'not_granted'))
const missingRequiredPermissions = computed(() => currentPermissions.value.filter((permission) => permission.requirement === 'required' && permission.status === 'not_granted'))
const grantRecordsByCapability = computed(() => new Map(currentGrants.value.map((grant) => [grant.capability, grant])))
const permissionDialogTitle = computed(() => (
  resumeEnableAfterGrant.value
    ? t('plugins.permissionDialogPendingTitle')
    : t('plugins.permissionDialogTitle')
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
      openPermissionDialog(extractMissingCapabilities(error), true)
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

function openPermissionDialog(prefill: string[] = [], resumeEnable = false) {
  const available = new Set(permissionCandidates.value.map((permission) => permission.capability))
  const recommended = (prefill.length > 0 ? prefill : Array.from(available))
    .filter((capability) => available.has(capability))
  selectedCapabilities.value = recommended
  resumeEnableAfterGrant.value = resumeEnable
  permissionDialogVisible.value = true
}

async function submitPermissionDialog() {
  operationError.value = null
  try {
    for (const capability of selectedCapabilities.value) {
      await pluginsStore.grantCapability(pluginId.value, { capability })
    }
    permissionDialogVisible.value = false
    const shouldResumeEnable = resumeEnableAfterGrant.value
    selectedCapabilities.value = []
    resumeEnableAfterGrant.value = false
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
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
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

function getGrantedAt(capability: string) {
  return grantRecordsByCapability.value.get(capability)?.granted_at ?? undefined
}

function getConsoleStatusColor(status: string) {
  if (status === 'authenticated') return 'success'
  if (status === 'reconnecting' || status === 'connecting') return 'warning'
  if (status === 'auth_failed') return 'error'
  return 'default'
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

    <div class="content-grid">
      <a-card :bordered="false">
        <template #title>
          <div class="card-header">
            <span>{{ t('plugins.sections.current') }}</span>
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
                  @click="openPermissionDialog([permission.capability])"
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
  </AppPage>

  <a-modal
    v-model:open="permissionDialogVisible"
    :get-container="false"
    :title="permissionDialogTitle"
    :confirm-loading="grantBusy"
    :ok-text="t('plugins.actions.grantSelected')"
    :cancel-text="t('dashboard.previewCancel')"
    :ok-button-props="{ disabled: selectedCapabilities.length === 0 }"
    @ok="submitPermissionDialog"
  >
    <a-alert
      v-if="resumeEnableAfterGrant"
      :message="t('plugins.permissionPendingTitle')"
      type="warning"
      :description="t('plugins.permissionDialogPendingBody')"
      show-icon
      class="section-gap"
    />

    <a-empty
      v-if="permissionCandidates.length === 0"
      :description="t('plugins.empty.permissions')"
    />

    <a-checkbox-group v-else v-model:value="selectedCapabilities" class="permission-dialog-list">
      <a-checkbox
        v-for="permission in permissionCandidates"
        :key="permission.capability"
        :value="permission.capability"
      >
        <div class="permission-dialog-item">
          <strong>{{ permission.capability }}</strong>
          <small>
            {{ getPermissionRequirementLabel(permission.requirement) }} ·
            {{ getPermissionStatusLabel(permission.status) }}
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
.plugin-detail-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.permission-list {
  display: grid;
  gap: 12px;
}

.permission-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border-radius: 10px;
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
  border-radius: 10px;
  background: var(--surface-soft);
  border: 1px solid var(--border);
  display: grid;
  gap: 8px;
}

.console-terminal-line {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--surface-strong);
  color: var(--text);
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--accent) 52%, transparent);
}

.console-terminal-line.is-stderr {
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--danger) 70%, transparent);
}

.console-terminal-line.is-system {
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--warning) 70%, transparent);
}

.console-terminal-line.is-outbound {
  background: color-mix(in srgb, var(--success) 6%, var(--surface-strong));
}

.console-terminal-line.is-outbound.is-info {
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--success) 72%, transparent);
}

.console-terminal-line.is-outbound.is-warn,
.console-terminal-line.is-outbound.is-error {
  box-shadow: inset 2px 0 0 color-mix(in srgb, var(--warning) 72%, transparent);
}

.console-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  color: var(--muted);
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.78rem;
}

.console-terminal-line pre {
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: "Cascadia Mono", "Consolas", monospace;
  line-height: 1.55;
  unicode-bidi: plaintext;
}
</style>
