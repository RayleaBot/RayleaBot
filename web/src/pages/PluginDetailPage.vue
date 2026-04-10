<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

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
  loadError.value = null
  try {
    await Promise.all([
      pluginsStore.fetchDetail(pluginId.value),
      pluginsStore.fetchGrants(pluginId.value),
      pluginsStore.fetchOutboundConsoleHistory(pluginId.value).catch(() => []),
      configStore.fetchConfig().catch(() => undefined),
    ])
    socketStore.setConsolePlugin(pluginId.value)
  } catch (error) {
    loadError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
  }
}

async function runAction(action: 'enable' | 'disable' | 'reload') {
  operationError.value = null
  try {
    await pluginsStore.executeAction(pluginId.value, action)
    ElMessage.success(t('plugins.actionAccepted'))
  } catch (error) {
    if (action === 'enable' && error instanceof ApiError && error.code === 'plugin.permission_pending') {
      openPermissionDialog(extractMissingCapabilities(error), true)
      return
    }
    operationError.value = getDisplayErrorMessage(error)
  }
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
    ElMessage.success(t('plugins.grantSaved'))
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
    ElMessage.success(t('plugins.grantRevoked'))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

async function uninstallPlugin() {
  operationError.value = null
  try {
    const response = await pluginsStore.uninstallPlugin(pluginId.value)
    uninstallDialogVisible.value = false
    ElMessage.success(t('plugins.uninstallAccepted'))
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
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <h1>{{ pluginId }}</h1>
      </div>

      <div class="table-actions">
        <el-button type="success" :loading="actionPending[pluginId] === 'enable'" @click="runAction('enable')">{{ t('plugins.actions.enable') }}</el-button>
        <el-button type="warning" :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">{{ t('plugins.actions.reload') }}</el-button>
        <el-button type="danger" plain :loading="actionPending[pluginId] === 'disable'" @click="runAction('disable')">{{ t('plugins.actions.disable') }}</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true">{{ t('plugins.actions.uninstall') }}</el-button>
      </div>
    </section>

    <RetryPanel
      v-if="loadError && !current"
      :title="t('errors.common.loadFailed')"
      :description="loadError"
      :loading="detailLoading"
      @retry="loadDetail()"
    />

    <el-alert v-else-if="loadError" :title="t('errors.common.loadFailed')" type="error" :description="loadError" show-icon />

    <el-alert v-if="operationError" :title="t('errors.common.actionFailed')" type="error" :description="operationError" show-icon />

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('plugins.sections.current') }}</span>
          </div>
        </template>

        <el-skeleton :loading="detailLoading" animated>
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="t('plugins.fields.name')">{{ current?.name ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.role')">
              {{ getPluginRoleLabel(current?.role) }}
              <small v-if="current?.role"> · {{ current.role }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.registration')">
              {{ getPluginRegistrationStateLabel(current?.registration_state) }}
              <small v-if="current?.registration_state"> · {{ current.registration_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.desired')">
              {{ getPluginDesiredStateLabel(current?.desired_state) }}
              <small v-if="current?.desired_state"> · {{ current.desired_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.runtime')">
              {{ getPluginRuntimeStateLabel(current?.runtime_state) }}
              <small v-if="current?.runtime_state"> · {{ current.runtime_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.display')">
              {{ getPluginDisplayStateLabel(current?.display_state) }}
              <small v-if="current?.display_state"> · {{ current.display_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.trust')">{{ current?.trust?.label ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.sourceRoot')">{{ current?.source?.root ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.sourceRef')">{{ current?.source?.package_source_ref ?? current?.source?.package_source_type ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.conflicts')">
              <div v-if="current?.command_conflicts?.length" class="table-actions">
                <el-tag v-for="command in current.command_conflicts" :key="command" size="small" type="warning">
                  {{ command }}
                </el-tag>
              </div>
              <span v-else>{{ t('display.empty') }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-skeleton>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('plugins.sections.permissions') }}</span>
            <div class="table-actions">
              <el-tag size="small">{{ currentPermissions.length }}</el-tag>
              <el-button
                v-if="!isBuiltinPlugin && permissionCandidates.length > 0"
                size="small"
                type="primary"
                @click="openPermissionDialog()"
              >
                {{ t('plugins.actions.reviewPermissions') }}
              </el-button>
            </div>
          </div>
        </template>

        <el-alert
          v-if="isBuiltinPlugin"
          :title="t('plugins.builtinAutoGrantTitle')"
          type="success"
          :description="t('plugins.builtinAutoGrantBody')"
          show-icon
          class="section-gap"
        />

        <el-alert
          v-else-if="missingRequiredPermissions.length > 0"
          :title="t('plugins.permissionPendingTitle')"
          type="warning"
          :description="t('plugins.permissionPendingBody', { count: missingRequiredPermissions.length })"
          show-icon
          class="section-gap"
        />

        <el-skeleton :loading="grantBusy" animated>
          <el-empty v-if="currentPermissions.length === 0" :description="t('plugins.empty.permissions')" />

          <div v-else class="permission-list">
            <article v-for="permission in currentPermissions" :key="permission.capability" class="permission-item">
              <div class="permission-item__main">
                <div class="permission-item__title">
                  <strong>{{ permission.capability }}</strong>
                  <div class="table-actions">
                    <el-tag size="small" type="info">{{ getPermissionRequirementLabel(permission.requirement) }}</el-tag>
                    <el-tag size="small" :type="permission.status === 'granted' ? 'success' : 'warning'">{{ getPermissionStatusLabel(permission.status) }}</el-tag>
                    <el-tag size="small" effect="plain">{{ getPermissionSourceLabel(permission.source) }}</el-tag>
                  </div>
                </div>
                <small>{{ t('plugins.fields.grantedAt') }}：{{ formatDateTime(getGrantedAt(permission.capability)) }}</small>
                <small>{{ t('plugins.fields.expiresAt') }}：{{ formatDateTime(permission.expires_at ?? undefined) }}</small>
              </div>

              <div class="table-actions">
                <el-button
                  v-if="canGrantPermission(permission)"
                  size="small"
                  type="primary"
                  @click="openPermissionDialog([permission.capability])"
                >
                  {{ t('plugins.actions.grantPermission') }}
                </el-button>
                <el-button
                  v-if="canRevokePermission(permission)"
                  size="small"
                  type="danger"
                  plain
                  @click="revokeGrant(permission.capability)"
                >
                  {{ t('plugins.actions.revokeGrant') }}
                </el-button>
              </div>
            </article>
          </div>
        </el-skeleton>
      </el-card>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('plugins.sections.commands') }}</span>
          <el-tag size="small">{{ current?.commands?.length ?? 0 }}</el-tag>
        </div>
      </template>

      <PluginCommandsPanel
        :commands="current?.commands ?? []"
        :command-conflicts="current?.command_conflicts ?? []"
        :command-prefix="commandPrefix"
      />
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('plugins.sections.console') }}</span>
          <div class="table-actions">
            <el-tag size="small">{{ getConnectionStatusLabel(consoleSnapshot.status) }}</el-tag>
            <el-button size="small" plain @click="socketStore.reconnectConsole()">{{ t('plugins.actions.reconnectConsole') }}</el-button>
            <el-button size="small" plain @click="clearConsole">{{ t('plugins.actions.clearConsole') }}</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="consoleSnapshot.lastError"
        :title="t('plugins.consoleUnavailable')"
        type="warning"
        :description="consoleSnapshot.lastError"
        show-icon
        class="section-gap"
      />

      <el-empty v-if="consoleFrames.length === 0" :description="t('plugins.empty.console')" />

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
          <pre>{{ frame.text }}</pre>
        </div>
      </div>
    </el-card>
  </div>

  <el-dialog v-model="permissionDialogVisible" :title="permissionDialogTitle" width="480px">
    <el-alert
      v-if="resumeEnableAfterGrant"
      :title="t('plugins.permissionPendingTitle')"
      type="warning"
      :description="t('plugins.permissionDialogPendingBody')"
      show-icon
      class="section-gap"
    />

    <el-empty
      v-if="permissionCandidates.length === 0"
      :description="t('plugins.empty.permissions')"
    />

    <el-checkbox-group v-else v-model="selectedCapabilities" class="permission-dialog-list">
      <el-checkbox
        v-for="permission in permissionCandidates"
        :key="permission.capability"
        :label="permission.capability"
        :value="permission.capability"
      >
        <div class="permission-dialog-item">
          <strong>{{ permission.capability }}</strong>
          <small>
            {{ getPermissionRequirementLabel(permission.requirement) }} ·
            {{ getPermissionStatusLabel(permission.status) }}
          </small>
        </div>
      </el-checkbox>
    </el-checkbox-group>

    <template #footer>
      <div class="table-actions">
        <el-button @click="permissionDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button
          type="primary"
          :loading="grantBusy"
          :disabled="selectedCapabilities.length === 0"
          @click="submitPermissionDialog"
        >
          {{ t('plugins.actions.grantSelected') }}
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="uninstallDialogVisible" :title="t('plugins.uninstallConfirmTitle')" width="420px">
    <p>{{ t('plugins.uninstallConfirmBody') }}</p>

    <template #footer>
      <div class="table-actions">
        <el-button @click="uninstallDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallPlugin">
          {{ t('plugins.actions.uninstallConfirm') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped lang="scss">
.permission-list {
  display: grid;
  gap: 12px;
}

.permission-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 18px;
  background: rgba(247, 250, 246, 0.88);
  border: 1px solid rgba(22, 33, 39, 0.08);
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
  padding: 14px;
  border-radius: 20px;
  background:
    linear-gradient(180deg, rgba(14, 20, 25, 0.98), rgba(18, 26, 33, 0.98)),
    radial-gradient(circle at top right, rgba(86, 198, 255, 0.08), transparent 24%);
  border: 1px solid rgba(110, 204, 255, 0.12);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  display: grid;
  gap: 10px;
}

.console-terminal-line {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  color: #e8eff6;
  box-shadow: inset 2px 0 0 rgba(88, 196, 255, 0.48);
}

.console-terminal-line.is-stderr {
  box-shadow: inset 2px 0 0 rgba(255, 104, 104, 0.72);
}

.console-terminal-line.is-system {
  box-shadow: inset 2px 0 0 rgba(255, 187, 74, 0.68);
}

.console-terminal-line.is-outbound {
  background: rgba(80, 200, 120, 0.06);
}

.console-terminal-line.is-outbound.is-info {
  box-shadow: inset 2px 0 0 rgba(95, 214, 132, 0.76);
}

.console-terminal-line.is-outbound.is-warn,
.console-terminal-line.is-outbound.is-error {
  box-shadow: inset 2px 0 0 rgba(255, 187, 74, 0.72);
}

.console-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  color: #8ca4b3;
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.78rem;
}

.console-terminal-line pre {
  margin: 0;
  color: #f5f8fb;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: "Cascadia Mono", "Consolas", monospace;
  line-height: 1.55;
}
</style>
