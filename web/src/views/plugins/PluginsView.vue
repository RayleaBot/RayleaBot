<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import PluginPowerButton from '@/components/PluginPowerButton.vue'
import AppTableToolbar from '@/components/AppTableToolbar.vue'
import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import PluginCommandsPanel from '@/components/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPluginDesiredStateLabel,
  getPluginDisplayStateLabel,
  getPluginRegistrationStateLabel,
  getPluginRoleLabel,
  getPluginRuntimeStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { PluginCommandSummary, PluginInstallRequest } from '@/types/api'
import { usePluginsStore } from '@/stores/plugins'

type HealthNoticeTone = '' | 'info' | 'warning' | 'danger'

interface PluginHealthNotice {
  label: string
  tone: HealthNoticeTone
}

const router = useRouter()
const pluginsStore = usePluginsStore()
const { actionPending, error, installPending, loading, sortedItems } = storeToRefs(pluginsStore)
const installDialogVisible = ref(false)
const installError = ref<string | null>(null)
const summaryDrawerVisible = ref(false)
const summaryPluginId = ref<string | null>(null)
const expandedCommandPluginIds = ref(new Set<string>())
const installForm = reactive<PluginInstallRequest>({
  source_type: 'local_zip',
  source: '',
})

const summaryPlugin = computed(() => sortedItems.value.find((item) => item.id === summaryPluginId.value) ?? null)
const tableColumns = computed(() => [
  { title: t('plugins.fields.plugin'), key: 'title', dataIndex: 'name', width: 240 },
  { title: t('plugins.fields.version'), key: 'version', dataIndex: 'version', width: 96 },
  { title: t('plugins.fields.author'), key: 'author', dataIndex: 'author', width: 140 },
  { title: t('plugins.fields.description'), key: 'description', dataIndex: 'description', width: 320 },
  { title: t('plugins.fields.source'), key: 'source', dataIndex: 'source', width: 220 },
  { title: t('plugins.fields.commands'), key: 'commands', dataIndex: 'commands', width: 300 },
  { title: t('plugins.fields.runtime'), key: 'runtime', dataIndex: 'runtime_state', width: 300 },
  { title: t('plugins.fields.actions'), key: 'actions', dataIndex: 'actions', width: 396 },
])

function getConflictNotice(count: number) {
  return t('plugins.health.commandConflicts', { count })
}

function getPluginHealthNotices(row: (typeof sortedItems.value)[number]) {
  const notices: PluginHealthNotice[] = []
  const conflicts = row.command_conflicts?.length ?? 0

  if (conflicts > 0) {
    notices.push({ label: getConflictNotice(conflicts), tone: 'warning' })
  }

  if (row.source?.verified === false || row.trust?.level === 'unverified') {
    notices.push({ label: t('plugins.health.unverifiedSource'), tone: 'info' })
  }

  if (row.registration_state === 'removed') {
    notices.push({ label: t('plugins.health.removed'), tone: 'danger' })
  }

  if (row.runtime_state === 'crashed' || row.runtime_state === 'dead_letter') {
    notices.push({ label: t('plugins.health.runtimeIssue'), tone: 'danger' })
  } else if (row.runtime_state === 'backoff') {
    notices.push({ label: t('plugins.health.retrying'), tone: 'warning' })
  } else if (row.desired_state === 'enabled' && row.runtime_state === 'stopped') {
    notices.push({ label: t('plugins.health.enabledButStopped'), tone: 'warning' })
  }

  return notices.slice(0, 3)
}

function isCommandsExpanded(pluginId: string) {
  return expandedCommandPluginIds.value.has(pluginId)
}

function getVisibleCommands(pluginId: string, commands: PluginCommandSummary[]) {
  return isCommandsExpanded(pluginId) ? commands : commands.slice(0, 3)
}

function getOverflowCommandCount(commands: PluginCommandSummary[]) {
  return Math.max(commands.length - 3, 0)
}

function toggleCommandExpansion(pluginId: string) {
  const next = new Set(expandedCommandPluginIds.value)
  if (next.has(pluginId)) {
    next.delete(pluginId)
  } else {
    next.add(pluginId)
  }
  expandedCommandPluginIds.value = next
}

function getCommandAliasesText(command: PluginCommandSummary) {
  return command.aliases?.length ? command.aliases.join(', ') : t('display.empty')
}

function getOptionalDisplayText(value?: string | null) {
  const text = value?.trim()
  return text ? text : t('display.empty')
}

function isConflictedCommand(command: PluginCommandSummary, conflicts?: string[]) {
  return isPluginCommandConflicted(command, conflicts)
}

function getTagColor(tone: HealthNoticeTone) {
  if (tone === 'danger') return 'error'
  if (tone === 'warning') return 'warning'
  if (tone === 'info') return 'blue'
  return 'default'
}

function getRuntimeColor(state?: string) {
  if (state === 'running') return 'success'
  if (state === 'stopped') return 'default'
  return 'error'
}

async function loadPlugins() {
  try {
    await pluginsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadPlugins()
})

function openDetail(id: string) {
  void router.push({ name: 'plugin-detail', params: { id } })
}

function openSummary(id: string) {
  summaryPluginId.value = id
  summaryDrawerVisible.value = true
}

function getToggleAction(desiredState?: string) {
  return desiredState === 'enabled' ? 'disable' : 'enable'
}

async function reloadPlugin(pluginId: string) {
  try {
    await pluginsStore.executeAction(pluginId, 'reload')
    notifySuccess(t('plugins.actionAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

async function submitInstall() {
  installError.value = null
  try {
    const response = await pluginsStore.installPlugin(installForm)
    installDialogVisible.value = false
    installForm.source_type = 'local_zip'
    installForm.source = ''
    delete installForm.allow_install_scripts
    notifySuccess(t('plugins.installAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    installError.value = getDisplayErrorMessage(error)
  }
}
</script>

<template>
  <AppPage :title="t('plugins.title')" full-height>
    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <AppCard
      v-else
      borderless
      class="plugins-card"
      v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 0 } } }"
    >
      <AppTableToolbar>
        <template #right>
          <a-button type="primary" @click="installDialogVisible = true">
            {{ t('plugins.install') }}
          </a-button>
          <a-button :loading="loading" @click="loadPlugins()">
            {{ t('plugins.refresh') }}
          </a-button>
        </template>
      </AppTableToolbar>

      <a-alert v-if="installError" :message="t('errors.common.actionFailed')" type="error" :description="installError" show-icon class="plugins-alert" />

      <a-table
        class="plugins-data-table app-data-table"
        :columns="tableColumns"
        :data-source="sortedItems"
        :pagination="false"
        :row-key="(row) => row.id"
        :scroll="{ x: 2012 }"
      >
        <template #emptyText>
          <AppEmptyState
            icon="plugin"
            :title="t('plugins.empty.title')"
            :description="t('plugins.empty.description')"
            :action-label="t('plugins.install')"
            @action="installDialogVisible = true"
          />
        </template>

        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <div class="plugin-cell-identity">
              <strong class="plugin-name">{{ record.name }}</strong>
              <small class="plugin-id">{{ record.id }}</small>
            </div>
          </template>

          <template v-else-if="column.key === 'source'">
            <div class="plugin-cell-source">
              <div class="plugin-source-root" :title="record.source?.root ?? t('display.empty')">
                {{ record.source?.root ?? t('display.empty') }}
              </div>
              <div class="plugin-trust-label">
                {{ record.trust?.label ?? t('display.empty') }}
              </div>
            </div>
          </template>

          <template v-else-if="column.key === 'version'">
            <span class="plugin-cell-version">{{ getOptionalDisplayText(record.version) }}</span>
          </template>

          <template v-else-if="column.key === 'author'">
            <span class="plugin-cell-author" :title="getOptionalDisplayText(record.author)">
              {{ getOptionalDisplayText(record.author) }}
            </span>
          </template>

          <template v-else-if="column.key === 'description'">
            <span class="plugin-cell-description" :title="getOptionalDisplayText(record.description)">
              {{ getOptionalDisplayText(record.description) }}
            </span>
          </template>

          <template v-else-if="column.key === 'commands'">
            <div v-if="record.commands.length > 0" class="plugin-cell-commands">
              <div
                v-for="command in getVisibleCommands(record.id, record.commands)"
                :key="`${record.id}-${command.name}`"
                class="plugin-command-chip"
              >
                <a-tag
                  size="small"
                  :color="isConflictedCommand(command, record.command_conflicts) ? 'warning' : 'success'"
                  :aria-label="`指令：${command.name}`"
                >
                  {{ command.name }}
                </a-tag>
                <a-tooltip v-if="command.aliases?.length" :title="getCommandAliasesText(command)">
                  <small>{{ t('plugins.commandAliasesCount', { count: command.aliases.length }) }}</small>
                </a-tooltip>
              </div>
              <a-button
                v-if="getOverflowCommandCount(record.commands) > 0"
                class="plugin-command-expander"
                size="small"
                type="link"
                :aria-expanded="isCommandsExpanded(record.id)"
                :aria-label="isCommandsExpanded(record.id)
                  ? t('plugins.commandCollapseAria', { name: record.name })
                  : t('plugins.commandExpandAria', { name: record.name, count: getOverflowCommandCount(record.commands) })"
                @click="toggleCommandExpansion(record.id)"
              >
                {{ isCommandsExpanded(record.id)
                  ? t('plugins.commandCollapse')
                  : t('plugins.commandOverflow', { count: getOverflowCommandCount(record.commands) }) }}
              </a-button>
            </div>
            <span v-else class="plugin-command-empty">{{ t('plugins.empty.commands') }}</span>
          </template>

          <template v-else-if="column.key === 'runtime'">
            <div class="plugin-cell-status">
              <div class="plugin-status-badges">
                <a-tag size="small" color="blue" :aria-label="`期望状态：${getPluginDesiredStateLabel(record.desired_state)}`">{{ getPluginDesiredStateLabel(record.desired_state) }}</a-tag>
                <a-tag size="small" :color="getRuntimeColor(record.runtime_state)" :aria-label="`运行状态：${getPluginRuntimeStateLabel(record.runtime_state)}`">{{ getPluginRuntimeStateLabel(record.runtime_state) }}</a-tag>
              </div>
              <div v-if="getPluginHealthNotices(record).length > 0" class="plugin-health-notices">
                <a-tag
                  v-for="notice in getPluginHealthNotices(record)"
                  :key="notice.label"
                  size="small"
                  :color="getTagColor(notice.tone)"
                  :aria-label="`健康状态：${notice.label}`"
                >
                  {{ notice.label }}
                </a-tag>
              </div>
            </div>
          </template>

          <template v-else-if="column.key === 'actions'">
            <div class="plugin-cell-actions">
              <a-button size="small" @click="openSummary(record.id)">{{ t('plugins.actions.summary') }}</a-button>
              <a-button size="small" @click="openDetail(record.id)">{{ t('plugins.actions.detail') }}</a-button>

              <a-divider type="vertical" />

              <PluginPowerButton
                compact
                :checked="record.desired_state === 'enabled'"
                :data-testid="`plugin-enable-button-${record.id}`"
                :loading="actionPending[record.id] === 'enable' || actionPending[record.id] === 'disable'"
                :checked-label="t('plugins.actions.enable')"
                :unchecked-label="t('plugins.actions.disable')"
                @click="pluginsStore.executeAction(record.id, getToggleAction(record.desired_state))"
              />
              <a-button
                size="small"
                :data-testid="`plugin-reload-button-${record.id}`"
                :loading="actionPending[record.id] === 'reload'"
                @click="reloadPlugin(record.id)"
              >
                {{ t('plugins.actions.reload') }}
              </a-button>
            </div>
          </template>
        </template>
      </a-table>
    </AppCard>

    <a-modal
      v-model:open="installDialogVisible"
      :get-container="false"
      :title="t('plugins.installDialogTitle')"
      :confirm-loading="installPending"
      :ok-text="t('plugins.installSubmit')"
      :cancel-text="t('dashboard.previewCancel')"
      :ok-button-props="{ disabled: !installForm.source }"
      @ok="submitInstall"
    >
      <a-form layout="vertical">
        <a-alert v-if="installError" :message="t('errors.common.actionFailed')" type="error" :description="installError" show-icon class="plugins-alert" />

        <a-form-item :label="t('plugins.sourceType')">
          <a-select
            v-model:value="installForm.source_type"
            :options="[
              { label: t('plugins.localZip'), value: 'local_zip' },
              { label: t('plugins.localDirectory'), value: 'local_directory' },
              { label: t('plugins.remoteUrl'), value: 'remote_url' },
            ]"
          />
        </a-form-item>

        <a-form-item :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')">
          <a-input v-model:value="installForm.source" />
        </a-form-item>

        <a-form-item>
          <a-checkbox v-model:checked="installForm.allow_install_scripts">
            {{ t('plugins.allowScripts') }}
          </a-checkbox>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-drawer
      v-model:open="summaryDrawerVisible"
      :get-container="false"
      :title="t('plugins.actions.summary')"
      placement="right"
      width="min(560px, 92vw)"
    >
      <template v-if="summaryPlugin">
        <div class="drawer-section drawer-section--dense">
          <div class="mono-list">
            <strong>{{ summaryPlugin.name }}</strong>
            <small>{{ summaryPlugin.id }}</small>
          </div>
        </div>

        <AppCard borderless class="drawer-card">
          <a-descriptions :column="1" bordered size="small">
            <a-descriptions-item :label="t('plugins.fields.role')">{{ getPluginRoleLabel(summaryPlugin.role) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.trust')">{{ summaryPlugin.trust?.label ?? t('display.empty') }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.registration')">{{ getPluginRegistrationStateLabel(summaryPlugin.registration_state) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.desired')">{{ getPluginDesiredStateLabel(summaryPlugin.desired_state) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.runtime')">{{ getPluginRuntimeStateLabel(summaryPlugin.runtime_state) }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.display')">
              {{ getPluginDisplayStateLabel(summaryPlugin.display_state) }}
              <small v-if="summaryPlugin.display_state"> · {{ summaryPlugin.display_state }}</small>
            </a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.source')">{{ summaryPlugin.source?.root ?? t('display.empty') }}</a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.sourceRef')">
              {{ summaryPlugin.source?.package_source_ref ?? summaryPlugin.source?.package_source_type ?? t('display.empty') }}
            </a-descriptions-item>
            <a-descriptions-item :label="t('plugins.fields.conflicts')">
              <div v-if="summaryPlugin.command_conflicts?.length" class="table-actions">
                <a-tag v-for="command in summaryPlugin.command_conflicts" :key="command" size="small" color="warning">
                  {{ command }}
                </a-tag>
              </div>
              <span v-else>{{ t('display.empty') }}</span>
            </a-descriptions-item>
          </a-descriptions>
        </AppCard>

        <AppCard :title="t('plugins.sections.commands')" borderless class="drawer-card">
          <PluginCommandsPanel
            :commands="summaryPlugin.commands"
            :command-conflicts="summaryPlugin.command_conflicts"
          />
        </AppCard>
      </template>
    </a-drawer>
  </AppPage>
</template>

<style lang="scss" scoped>
.plugins-card {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  box-shadow: var(--shadow-xs);
}

.plugins-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  padding: 0;
}

.plugins-alert {
  margin: 12px 14px 0;
}

.plugins-data-table {
  flex: 1 1 auto;
  min-height: 0;
  border-radius: 0 0 var(--app-card-radius) var(--app-card-radius);
  overflow: hidden;
}

.plugins-data-table :deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

.plugin-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-name {
  font-size: 0.95rem;
  color: var(--text);
  font-weight: 600;
}

.plugin-id {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--muted);
}

.plugin-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.plugin-source-root {
  font-size: 0.88rem;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

.plugin-trust-label {
  font-size: 0.8rem;
  color: var(--muted);
}

.plugin-cell-version {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  color: var(--muted);
}

.plugin-cell-author {
  display: block;
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.86rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-cell-description {
  display: -webkit-box;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.86rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plugin-cell-status {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.plugin-status-badges,
.plugin-health-notices {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.plugin-cell-commands {
  display: flex;
  gap: 6px 8px;
  align-items: center;
  flex-wrap: wrap;
}

.plugin-command-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  flex: 0 1 auto;
}

.plugin-command-chip :deep(.ant-tag) {
  margin-inline-end: 0;
}

.plugin-command-chip small,
.plugin-command-empty {
  color: var(--muted);
  font-size: 0.8rem;
}

.plugin-command-expander {
  height: 22px;
  padding: 0 6px;
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 20px;
}

.plugin-command-expander:hover,
.plugin-command-expander:focus-visible {
  color: var(--primary);
}

.plugin-cell-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
}

.plugin-cell-actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.drawer-card {
  margin-top: 12px;
}

.drawer-section {
  padding: 16px 0;
  border-bottom: 1px solid var(--border);
}

.mono-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  strong { font-size: 1rem; font-weight: 600; }
  small { font-family: var(--font-mono); font-size: 0.8rem; color: var(--muted); }
}
</style>
