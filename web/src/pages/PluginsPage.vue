<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

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
const installForm = reactive<PluginInstallRequest>({
  source_type: 'local_zip',
  source: '',
})
const summaryPlugin = computed(() => sortedItems.value.find((item) => item.id === summaryPluginId.value) ?? null)

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

function getVisibleCommands(commands: PluginCommandSummary[]) {
  return commands.slice(0, 3)
}

function getOverflowCommandCount(commands: PluginCommandSummary[]) {
  return Math.max(commands.length - 3, 0)
}

function getCommandAliasesText(command: PluginCommandSummary) {
  return command.aliases?.length ? command.aliases.join(', ') : t('display.empty')
}

function isConflictedCommand(command: PluginCommandSummary, conflicts?: string[]) {
  return isPluginCommandConflicted(command, conflicts)
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

async function submitInstall() {
  installError.value = null
  try {
    const response = await pluginsStore.installPlugin(installForm)
    installDialogVisible.value = false
    installForm.source_type = 'local_zip'
    installForm.source = ''
    delete installForm.allow_install_scripts
    ElMessage.success(t('plugins.installAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    installError.value = getDisplayErrorMessage(error)
  }
}
</script>

<template>
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('plugins.title') }}</h1>
      </div>

      <div class="table-actions">
        <el-button type="primary" @click="installDialogVisible = true">
          {{ t('plugins.install') }}
        </el-button>
        <el-button :loading="loading" @click="loadPlugins()">
          {{ t('plugins.refresh') }}
        </el-button>
      </div>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <el-alert v-if="installError" :title="t('errors.common.actionFailed')" type="error" :description="installError" show-icon />

    <el-table
      v-else
      :data="sortedItems"
      style="width: 100%;"
      class="plugins-data-table"
      :empty-text="t('display.empty')"
    >
      <el-table-column :label="t('plugins.title')" min-width="260">
        <template #default="{ row }">
          <div class="plugin-cell-identity">
            <strong class="plugin-name">{{ row.name }}</strong>
            <small class="plugin-id">{{ row.id }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('plugins.fields.source')" min-width="200">
        <template #default="{ row }">
          <div class="plugin-cell-source">
            <div class="plugin-source-root" :title="row.source?.root ?? t('display.empty')">
              {{ row.source?.root ?? t('display.empty') }}
            </div>
            <div class="plugin-trust-label">
              {{ row.trust?.label ?? t('display.empty') }}
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('plugins.fields.commands')" min-width="250">
        <template #default="{ row }">
          <div v-if="row.commands.length > 0" class="plugin-cell-commands">
            <div
              v-for="command in getVisibleCommands(row.commands)"
              :key="`${row.id}-${command.name}`"
              class="plugin-command-chip"
            >
              <el-tag
                size="small"
                effect="plain"
                :type="isConflictedCommand(command, row.command_conflicts) ? 'warning' : 'success'"
              >
                {{ command.name }}
              </el-tag>
              <el-tooltip
                v-if="command.aliases?.length"
                :content="getCommandAliasesText(command)"
                placement="top"
              >
                <small>{{ t('plugins.commandAliasesCount', { count: command.aliases.length }) }}</small>
              </el-tooltip>
            </div>
            <small v-if="getOverflowCommandCount(row.commands) > 0" class="plugin-command-overflow">
              {{ t('plugins.commandOverflow', { count: getOverflowCommandCount(row.commands) }) }}
            </small>
          </div>
          <span v-else class="plugin-command-empty">{{ t('plugins.empty.commands') }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('plugins.fields.runtime')" min-width="300">
        <template #default="{ row }">
          <div class="plugin-cell-status">
            <div class="plugin-status-badges">
              <el-tag size="small" type="info" effect="plain">{{ getPluginDesiredStateLabel(row.desired_state) }}</el-tag>
              <el-tag size="small" :type="row.runtime_state === 'running' ? 'success' : (row.runtime_state === 'stopped' ? 'info' : 'danger')" effect="light">{{ getPluginRuntimeStateLabel(row.runtime_state) }}</el-tag>
            </div>
            <div v-if="getPluginHealthNotices(row).length > 0" class="plugin-health-notices">
              <el-tag
                v-for="notice in getPluginHealthNotices(row)"
                :key="notice.label"
                size="small"
                effect="plain"
                :type="notice.tone"
              >
                {{ notice.label }}
              </el-tag>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column fixed="right" min-width="420" align="right">
        <template #default="{ row }">
          <div class="plugin-cell-actions">
            <el-button size="small" plain @click="openSummary(row.id)">{{ t('plugins.actions.summary') }}</el-button>
            <el-button size="small" plain @click="openDetail(row.id)">{{ t('plugins.actions.detail') }}</el-button>
            
            <el-divider direction="vertical" />

            <el-button 
              size="small" 
              type="success" 
              plain 
              :loading="actionPending[row.id] === 'enable'" 
              :disabled="row.desired_state === 'enabled'"
              @click="pluginsStore.executeAction(row.id, 'enable')"
            >
              {{ t('plugins.actions.enable') }}
            </el-button>
            <el-button 
              size="small" 
              type="warning" 
              plain 
              :loading="actionPending[row.id] === 'reload'" 
              @click="pluginsStore.executeAction(row.id, 'reload')"
            >
              {{ t('plugins.actions.reload') }}
            </el-button>
            <el-button 
              size="small" 
              type="danger" 
              plain 
              :loading="actionPending[row.id] === 'disable'" 
              :disabled="row.desired_state === 'disabled'"
              @click="pluginsStore.executeAction(row.id, 'disable')"
            >
              {{ t('plugins.actions.disable') }}
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog v-model="installDialogVisible" :title="t('plugins.installDialogTitle')" width="520px">
    <el-form label-position="top">
      <el-alert v-if="installError" :title="t('errors.common.actionFailed')" type="error" :description="installError" show-icon class="section-gap" />

      <el-form-item :label="t('plugins.sourceType')">
        <el-select v-model="installForm.source_type">
          <el-option :label="t('plugins.localZip')" value="local_zip" />
          <el-option :label="t('plugins.localDirectory')" value="local_directory" />
          <el-option :label="t('plugins.remoteUrl')" value="remote_url" />
        </el-select>
      </el-form-item>

      <el-form-item :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')">
        <el-input v-model="installForm.source" />
      </el-form-item>

      <el-form-item>
        <el-checkbox v-model="installForm.allow_install_scripts">
          {{ t('plugins.allowScripts') }}
        </el-checkbox>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="table-actions">
        <el-button @click="installDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="primary" :loading="installPending" :disabled="!installForm.source" @click="submitInstall">
          {{ t('plugins.installSubmit') }}
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="summaryDrawerVisible" :title="t('plugins.actions.summary')" width="min(560px, 92vw)">
    <template v-if="summaryPlugin">
      <div class="drawer-section drawer-section--dense">
        <div class="mono-list">
          <strong>{{ summaryPlugin.name }}</strong>
          <small>{{ summaryPlugin.id }}</small>
        </div>
      </div>

      <el-descriptions :column="1" border>
        <el-descriptions-item :label="t('plugins.fields.role')">{{ getPluginRoleLabel(summaryPlugin.role) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.trust')">{{ summaryPlugin.trust?.label ?? t('display.empty') }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.registration')">{{ getPluginRegistrationStateLabel(summaryPlugin.registration_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.desired')">{{ getPluginDesiredStateLabel(summaryPlugin.desired_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.runtime')">{{ getPluginRuntimeStateLabel(summaryPlugin.runtime_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.display')">
          {{ getPluginDisplayStateLabel(summaryPlugin.display_state) }}
          <small v-if="summaryPlugin.display_state"> · {{ summaryPlugin.display_state }}</small>
        </el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.source')">{{ summaryPlugin.source?.root ?? t('display.empty') }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.sourceRef')">
          {{ summaryPlugin.source?.package_source_ref ?? summaryPlugin.source?.package_source_type ?? t('display.empty') }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.conflicts')">
          <div v-if="summaryPlugin.command_conflicts?.length" class="table-actions">
            <el-tag v-for="command in summaryPlugin.command_conflicts" :key="command" size="small" type="warning">
              {{ command }}
            </el-tag>
          </div>
          <span v-else>{{ t('display.empty') }}</span>
        </el-descriptions-item>
      </el-descriptions>

      <el-card class="plugin-command-summary-card section-gap">
        <template #header>
          <strong>{{ t('plugins.sections.commands') }}</strong>
        </template>
        <PluginCommandsPanel
          :commands="summaryPlugin.commands"
          :command-conflicts="summaryPlugin.command_conflicts"
        />
      </el-card>
    </template>
  </el-dialog>
</template>

<style lang="scss" scoped>
.plugins-data-table {
  border-radius: 22px;
  overflow: hidden;
  box-shadow: 0 14px 32px rgba(18, 32, 38, 0.06);
  border: 1px solid rgba(22, 33, 39, 0.08);

  :deep(.el-table__inner-wrapper) {
    background: rgba(247, 250, 246, 0.88);
  }
  
  :deep(.el-table__header-wrapper th) {
    background-color: transparent !important;
    border-bottom: 1px solid rgba(22, 33, 39, 0.08);
    color: var(--muted);
    font-size: 0.85rem;
    font-weight: 600;
    padding: 16px 8px;
  }

  :deep(.el-table__row) {
    background-color: transparent;
    transition: background-color 150ms ease;
    
    td {
      border-bottom: 1px solid rgba(22, 33, 39, 0.04);
      padding: 12px 8px;
    }

    &:hover {
      background-color: rgba(255, 255, 255, 0.6);
      td {
        background-color: transparent !important;
      }
    }
  }

  :deep(.el-table__body-wrapper) {
    background-color: transparent;
  }
}

.plugin-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
  
  .plugin-name {
    font-size: 0.98rem;
    color: var(--text);
    font-weight: 600;
  }
  
  .plugin-id {
    font-family: "Cascadia Mono", "Consolas", monospace;
    font-size: 0.8rem;
    color: var(--muted);
  }
}

.plugin-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;
  
  .plugin-source-root {
    font-size: 0.9rem;
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
}

.plugin-cell-status {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;

  .plugin-status-badges {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .plugin-health-notices {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
}

.plugin-cell-commands {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.plugin-command-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.plugin-command-chip small,
.plugin-command-overflow,
.plugin-command-empty {
  color: var(--muted);
  font-size: 0.8rem;
}

.plugin-command-summary-card {
  margin-top: 16px;
  border-radius: 20px;
}

.plugin-cell-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  
  .el-button {
    margin: 0;
  }
  
  .plugin-more-btn {
    padding: 6px;
    font-weight: bold;
    letter-spacing: 2px;
  }
}
</style>
