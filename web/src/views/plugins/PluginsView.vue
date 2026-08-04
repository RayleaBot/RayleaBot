<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import {
  FilterOutlined,
  SearchOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons-vue'

import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import PluginPowerButton from '@/components/PluginPowerButton.vue'
import AppTableToolbar from '@/components/AppTableToolbar.vue'
import AppStatusTag from '@/components/AppStatusTag.vue'
import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import PluginCommandsPanel from '@/components/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPluginRoleLabel,
  getPluginStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { PluginCommandSummary } from '@/types/api'
import { usePluginsStore } from '@/stores/plugins'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { usePluginInstallFlow } from './usePluginInstallFlow'

type HealthNoticeTone = '' | 'info' | 'warning' | 'danger'

interface PluginHealthNotice {
  label: string
  tone: HealthNoticeTone
}

const navigate = useMotionNavigation()
const pluginsStore = usePluginsStore()
const { actionPending, error, inspectionPending, installPending, loading, sortedItems } = storeToRefs(pluginsStore)
const {
  installDialogVisible,
  installForm,
  installInspection,
  resetInstallDialog,
  submitInstall,
  trustedCodeConfirmed,
} = usePluginInstallFlow(pluginsStore)
const summaryDrawerVisible = ref(false)
const summaryPluginId = ref<string | null>(null)
const expandedCommandPluginIds = ref(new Set<string>())
const filterDrawerVisible = ref(false)

const searchQuery = ref('')
const filterState = ref<'all' | 'running' | 'disabled' | 'alert'>('all')
const filterSource = ref<'all' | 'official' | 'community'>('all')

const pageErrorToast = computed(() => (
  error.value
    ? {
        key: `plugins-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))
useToastFeedback(pageErrorToast)

function isOfficialPlugin(record: (typeof sortedItems.value)[number]) {
  return record.trust?.level === 'official' || record.role === 'official'
}

function getTrustLabel(record: (typeof sortedItems.value)[number]) {
  if (isOfficialPlugin(record)) {
    return t('plugins.trustLabels.official')
  }
  if (record.trust?.level === 'unverified') {
    return t('plugins.trustLabels.unverified')
  }
  return record.trust?.label || t('plugins.trustLabels.thirdParty')
}

function getTrustColor(record: (typeof sortedItems.value)[number]) {
  if (isOfficialPlugin(record)) return 'blue'
  if (record.trust?.level === 'unverified') return 'warning'
  return 'default'
}

function getPluginInitials(name: string) {
  const normalized = name.trim()
  if (!normalized) return 'PL'
  if (/^[\u4e00-\u9fa5]/.test(normalized)) return normalized.slice(0, 2)
  const words = normalized.split(/[\s._-]+/).filter(Boolean)
  return words.length > 1
    ? `${words[0][0]}${words[1][0]}`.toUpperCase()
    : normalized.slice(0, 2).toUpperCase()
}

function getSourceTypeLabel(type?: string) {
  if (type === 'local_zip') return t('plugins.localZip')
  if (type === 'local_directory') return t('plugins.localDirectory')
  if (type === 'remote_url') return t('plugins.remoteUrl')
  if (type === 'catalog') return t('plugins.catalogSource')
  if (type === 'development') return t('plugins.developmentSource')
  return type || t('display.empty')
}

const filteredItems = computed(() => {
  return sortedItems.value.filter((item) => {
    // 1. Search Query
    if (searchQuery.value) {
      const q = searchQuery.value.toLowerCase().trim()
      const matchName = item.name?.toLowerCase().includes(q)
      const matchId = item.id?.toLowerCase().includes(q)
      const matchDesc = item.description?.toLowerCase().includes(q)
      if (!matchName && !matchId && !matchDesc) return false
    }

    // 2. Filter State
    if (filterState.value === 'running') {
      if (item.state !== 'running') return false
    } else if (filterState.value === 'disabled') {
      if (item.state !== 'disabled') return false
    } else if (filterState.value === 'alert') {
      const hasConflicts = (item.command_conflicts?.length ?? 0) > 0
      const hasIssue = item.state === 'failed' || item.state === 'invalid'
      if (!hasConflicts && !hasIssue) return false
    }

    // 3. Filter Source
    if (filterSource.value === 'official') {
      const isOfficial = isOfficialPlugin(item)
      if (!isOfficial) return false
    } else if (filterSource.value === 'community') {
      const isOfficial = isOfficialPlugin(item)
      if (isOfficial) return false
    }

    return true
  })
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

  if (row.state === 'failed') {
    notices.push({ label: t('plugins.health.runtimeIssue'), tone: 'danger' })
  } else if (row.state === 'invalid') {
    notices.push({ label: t('plugins.health.invalidManifest'), tone: 'danger' })
  } else if (row.state === 'enabled') {
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
  void navigate({ name: 'plugin-detail', params: { id } })
}

function openSummary(id: string) {
  summaryPluginId.value = id
  summaryDrawerVisible.value = true
}

function getToggleAction(state?: string) {
  return state === 'disabled' ? 'enable' : 'disable'
}

function isPluginLifecycleSwitching(state?: string) {
  return state === 'starting' || state === 'stopping'
}

function isToggleLoading(pluginId: string, state?: string) {
  return actionPending.value[pluginId] === 'enable' ||
    actionPending.value[pluginId] === 'disable' ||
    isPluginLifecycleSwitching(state)
}

function isReloadDisabled(state?: string) {
  return state === 'disabled' ||
    state === 'starting' ||
    state === 'stopping' ||
    state === 'invalid'
}

async function reloadPlugin(pluginId: string) {
  try {
    await pluginsStore.executeAction(pluginId, 'reload')
    notifySuccess(t('plugins.actionAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

</script>

<template>
  <AppPage :title="t('plugins.title')">
    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <div v-else class="plugins-page-content">
      <AppCard
        borderless
        class="plugins-card"
      >
        <AppTableToolbar class="plugins-toolbar">
          <template #left>
            <div class="toolbar-filters plugins-filter-desktop">
              <a-input
                v-model:value="searchQuery"
                :placeholder="t('plugins.filter.searchPlaceholder')"
                class="filter-search"
                allow-clear
              >
                <template #prefix>
                  <SearchOutlined class="search-icon" />
                </template>
              </a-input>

              <a-radio-group v-model:value="filterState" button-style="solid" class="filter-radio-group">
                <a-radio-button value="all">{{ t('plugins.filter.stateAll') }}</a-radio-button>
                <a-radio-button value="running">{{ t('plugins.stats.running') }}</a-radio-button>
                <a-radio-button value="disabled">{{ t('plugins.stats.disabled') }}</a-radio-button>
                <a-radio-button value="alert">{{ t('plugins.stats.alert') }}</a-radio-button>
              </a-radio-group>

              <a-select v-model:value="filterSource" class="filter-select" :dropdown-match-select-width="false">
                <a-select-option value="all">{{ t('plugins.filter.sourceAll') }}</a-select-option>
                <a-select-option value="official">{{ t('plugins.filter.sourceOfficial') }}</a-select-option>
                <a-select-option value="community">{{ t('plugins.filter.sourceCommunity') }}</a-select-option>
              </a-select>
            </div>
          </template>

          <template #right>
            <a-button class="plugins-filter-mobile-trigger" @click="filterDrawerVisible = true">
              <template #icon><FilterOutlined /></template>
              {{ t('plugins.filter.title') }}
            </a-button>
            <a-button type="primary" @click="installDialogVisible = true">
              <template #icon><PlusOutlined /></template>
              {{ t('plugins.install') }}
            </a-button>
          </template>
        </AppTableToolbar>

        <div class="plugins-grid-container">
          <AppEmptyState
            v-if="filteredItems.length === 0"
            icon="plugin"
            :title="t('plugins.empty.title')"
            :description="t('plugins.empty.description')"
            :action-label="t('plugins.install')"
            @action="installDialogVisible = true"
          />

          <div v-else class="plugins-grid" aria-label="插件列表">
            <article v-for="item in filteredItems" :key="item.id" class="plugin-grid-card">
              <header class="plugin-card__header">
                <div class="plugin-card__avatar" aria-hidden="true">
                  {{ getPluginInitials(item.name) }}
                </div>
                <div class="plugin-card__identity">
                  <button type="button" class="plugin-card__name" @click="openDetail(item.id)">
                    {{ item.name }}
                  </button>
                  <span class="plugin-card__id">{{ item.id }}</span>
                </div>
                <AppStatusTag :status="item.state" :label="getPluginStateLabel(item.state)" :aria-label="`状态：${getPluginStateLabel(item.state)}`" />
              </header>

              <p class="plugin-card__description" :title="getOptionalDisplayText(item.description)">
                {{ getOptionalDisplayText(item.description) }}
              </p>

              <div class="plugin-card__meta">
                <span>{{ getOptionalDisplayText(item.version) }}</span>
                <span>{{ getOptionalDisplayText(item.author) }}</span>
                <span class="plugin-card__source" :title="item.source?.root ?? t('display.empty')">
                  {{ item.source?.root ?? t('display.empty') }}
                </span>
                <span>{{ getSourceTypeLabel(item.source?.package_source_type) }}</span>
                <a-tag size="small" :color="getTrustColor(item)">{{ getTrustLabel(item) }}</a-tag>
              </div>

              <div v-if="getPluginHealthNotices(item).length > 0" class="plugin-health-notices">
                <a-tag
                  v-for="notice in getPluginHealthNotices(item)"
                  :key="notice.label"
                  size="small"
                  :color="getTagColor(notice.tone)"
                  :aria-label="`健康状态：${notice.label}`"
                >
                  {{ notice.label }}
                </a-tag>
              </div>

              <div class="plugin-card__commands">
                <div class="plugin-card__section-label">{{ t('plugins.fields.commands') }}</div>
                <div v-if="item.commands.length > 0" class="plugin-cell-commands">
                  <div
                    v-for="command in getVisibleCommands(item.id, item.commands)"
                    :key="`${item.id}-${command.name}`"
                    class="plugin-command-chip"
                  >
                    <a-tag
                      size="small"
                      :color="isConflictedCommand(command, item.command_conflicts) ? 'warning' : 'success'"
                      :aria-label="`指令：${command.name}`"
                    >
                      {{ command.name }}
                    </a-tag>
                    <a-tooltip v-if="command.aliases?.length" :title="getCommandAliasesText(command)">
                      <small>{{ t('plugins.commandAliasesCount', { count: command.aliases.length }) }}</small>
                    </a-tooltip>
                  </div>
                  <a-button
                    v-if="getOverflowCommandCount(item.commands) > 0"
                    class="plugin-command-expander"
                    size="small"
                    type="link"
                    :aria-expanded="isCommandsExpanded(item.id)"
                    :aria-label="isCommandsExpanded(item.id)
                      ? t('plugins.commandCollapseAria', { name: item.name })
                      : t('plugins.commandExpandAria', { name: item.name, count: getOverflowCommandCount(item.commands) })"
                    @click="toggleCommandExpansion(item.id)"
                  >
                    {{ isCommandsExpanded(item.id)
                      ? t('plugins.commandCollapse')
                      : t('plugins.commandOverflow', { count: getOverflowCommandCount(item.commands) }) }}
                  </a-button>
                </div>
                <span v-else class="plugin-command-empty">{{ t('plugins.empty.commands') }}</span>
              </div>

              <footer class="plugin-card__actions">
                <div class="plugin-card__action-buttons">
                  <a-button size="small" @click="openSummary(item.id)">{{ t('plugins.actions.summary') }}</a-button>
                  <a-button size="small" @click="openDetail(item.id)">{{ t('plugins.actions.detail') }}</a-button>
                  <a-button
                    size="small"
                    :data-testid="`plugin-reload-button-${item.id}`"
                    :loading="actionPending[item.id] === 'reload'"
                    :disabled="isReloadDisabled(item.state)"
                    @click="reloadPlugin(item.id)"
                  >
                    <template #icon><ReloadOutlined /></template>
                    {{ t('plugins.actions.reload') }}
                  </a-button>
                </div>
                <PluginPowerButton
                  compact
                  :checked="item.state !== 'disabled'"
                  :data-testid="`plugin-enable-button-${item.id}`"
                  :loading="isToggleLoading(item.id, item.state)"
                  :checked-label="t('plugins.actions.enable')"
                  :unchecked-label="t('plugins.actions.disable')"
                  @click="pluginsStore.executeAction(item.id, getToggleAction(item.state))"
                />
              </footer>
            </article>
          </div>
        </div>
      </AppCard>
    </div>

    <a-drawer
      v-model:open="filterDrawerVisible"
      placement="bottom"
      height="auto"
      :title="t('plugins.filter.title')"
      :destroy-on-close="true"
    >
      <div class="plugins-filter-drawer">
        <a-input
          v-model:value="searchQuery"
          :placeholder="t('plugins.filter.searchPlaceholder')"
          allow-clear
        >
          <template #prefix><SearchOutlined /></template>
        </a-input>
        <a-radio-group v-model:value="filterState" button-style="solid">
          <a-radio-button value="all">{{ t('plugins.filter.stateAll') }}</a-radio-button>
          <a-radio-button value="running">{{ t('plugins.stats.running') }}</a-radio-button>
          <a-radio-button value="disabled">{{ t('plugins.stats.disabled') }}</a-radio-button>
          <a-radio-button value="alert">{{ t('plugins.stats.alert') }}</a-radio-button>
        </a-radio-group>
        <a-select v-model:value="filterSource">
          <a-select-option value="all">{{ t('plugins.filter.sourceAll') }}</a-select-option>
          <a-select-option value="official">{{ t('plugins.filter.sourceOfficial') }}</a-select-option>
          <a-select-option value="community">{{ t('plugins.filter.sourceCommunity') }}</a-select-option>
        </a-select>
        <a-button type="primary" @click="filterDrawerVisible = false">完成</a-button>
      </div>
    </a-drawer>

    <a-modal
      v-model:open="installDialogVisible"
      :title="t('plugins.installDialogTitle')"
      :confirm-loading="inspectionPending || installPending"
      :ok-text="installInspection ? t('plugins.installSubmit') : '检查插件包'"
      :cancel-text="t('dashboard.previewCancel')"
      :ok-button-props="{
        disabled: !installForm.source.trim()
          || Boolean(installInspection && !trustedCodeConfirmed),
      }"
      @ok="submitInstall"
      @cancel="resetInstallDialog"
    >
      <a-form layout="vertical">
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

        <template v-if="installInspection">
          <a-alert
            type="warning"
            show-icon
            message="第三方插件是完全可信的本地代码"
            description="预编译 Go 插件进程使用当前用户权限运行。仅安装来源、平台、摘要和能力均符合预期的代码。"
          />

          <a-descriptions class="install-inspection" :column="1" bordered size="small">
            <a-descriptions-item label="插件">{{ installInspection.plugin.name }}（{{ installInspection.plugin.id }}）</a-descriptions-item>
            <a-descriptions-item label="版本">{{ installInspection.plugin.version }}</a-descriptions-item>
            <a-descriptions-item label="作者">{{ installInspection.plugin.author }}</a-descriptions-item>
            <a-descriptions-item label="许可证">{{ installInspection.plugin.license }}</a-descriptions-item>
            <a-descriptions-item label="来源">{{ installInspection.plugin.source_label }}</a-descriptions-item>
            <a-descriptions-item label="包摘要"><code>{{ installInspection.package_sha256 }}</code></a-descriptions-item>
            <a-descriptions-item label="目标平台">{{ installInspection.target_platform }}</a-descriptions-item>
            <a-descriptions-item label="后端"><code>{{ installInspection.backend.path }}</code> · {{ installInspection.backend.size }} bytes</a-descriptions-item>
            <a-descriptions-item label="管理页面">{{ installInspection.ui.enabled ? `${installInspection.ui.entry}（${installInspection.ui.file_count} 个文件）` : '无' }}</a-descriptions-item>
            <a-descriptions-item label="Artifact 校验">{{ installInspection.artifact.valid ? `v${installInspection.artifact.artifact_version} · ${installInspection.artifact.file_count} 个文件` : '未通过' }}</a-descriptions-item>
            <a-descriptions-item label="有效期">{{ installInspection.expires_at }}</a-descriptions-item>
          </a-descriptions>

          <div class="install-inspection-list">
            <strong>声明能力</strong>
            <div>
              <a-tag v-for="capability in installInspection.capabilities" :key="capability">{{ capability }}</a-tag>
              <span v-if="installInspection.capabilities.length === 0">未声明能力</span>
            </div>
          </div>

          <a-form-item>
            <a-checkbox v-model:checked="trustedCodeConfirmed">
              我已核对来源、目标平台、artifact 摘要和能力，并信任此代码使用本机当前用户权限运行。
            </a-checkbox>
          </a-form-item>
        </template>
      </a-form>
    </a-modal>

    <a-drawer
      v-model:open="summaryDrawerVisible"
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
            <a-descriptions-item :label="t('plugins.fields.state')">{{ getPluginStateLabel(summaryPlugin.state) }}</a-descriptions-item>
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
.plugins-page-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  flex: 1 1 auto;
  min-height: 0;
}

.plugins-toolbar {
  border-bottom: 1px solid var(--border);
  padding: var(--space-md) var(--space-lg);
  background: var(--surface);

  :deep(.app-table-toolbar-right) {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.filter-search {
  width: 260px;
  border-radius: 6px;

  :deep(.ant-input) {
    border-radius: 6px;
  }
  .search-icon {
    color: var(--muted);
  }
}

.filter-radio-group {
  :deep(.ant-radio-button-wrapper) {
    border-radius: 0;
    &:first-child {
      border-radius: 6px 0 0 6px;
    }
    &:last-child {
      border-radius: 0 6px 6px 0;
    }
  }
}

.filter-select {
  width: 140px;
  :deep(.ant-select-selector) {
    border-radius: 6px !important;
  }
}

.plugins-filter-mobile-trigger {
  display: none;
}

.plugins-filter-drawer {
  display: grid;
  gap: 16px;
}

.plugins-filter-drawer :deep(.ant-radio-group) {
  display: flex;
  flex-wrap: wrap;
}

@media (max-width: 639px) {
  .plugins-filter-desktop {
    display: none;
  }

  .plugins-filter-mobile-trigger {
    display: inline-flex;
  }

  .plugin-card__actions,
  .plugin-card__action-buttons {
    align-items: stretch;
    flex-direction: column;
  }

  .plugin-card__actions :deep(.ant-btn),
  .plugin-card__actions :deep(.plugin-holo-button) {
    width: 100%;
    min-height: 44px;
  }
}

.plugin-command-empty {
  font-size: 0.875rem;
  color: var(--muted);
  display: block;
}

.plugins-card {
  box-shadow: none;
}

.plugins-card :deep(.ant-card-body) {
  padding: 0;
}

.plugins-grid-container {
  min-height: 220px;
  padding: var(--space-lg);
  background: var(--surface-soft);
}

.plugins-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
  gap: var(--space-lg);
  align-items: start;
}

.plugin-grid-card {
  display: flex;
  min-width: 0;
  overflow: hidden;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
  transition: border-color 160ms var(--motion-easing), box-shadow 160ms var(--motion-easing);
}

.plugin-grid-card:hover {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-sm);
}

.plugin-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 16px 16px 12px;
}

.plugin-card__avatar {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--border-accent);
  border-radius: 10px;
  background: var(--surface-accent);
  color: var(--text-accent);
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.plugin-card__identity {
  display: grid;
  min-width: 0;
  flex: 1 1 auto;
  gap: 2px;
}

.plugin-card__name {
  overflow: hidden;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  font: inherit;
  font-size: 15px;
  font-weight: 650;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__name:hover,
.plugin-card__name:focus-visible {
  color: var(--text-accent);
}

.plugin-card__name:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 3px;
  border-radius: 4px;
}

.plugin-card__id {
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__description {
  display: -webkit-box;
  min-height: 44px;
  margin: 0;
  overflow: hidden;
  padding: 0 16px;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plugin-card__meta,
.plugin-health-notices {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px 10px;
}

.plugin-card__meta {
  margin: 12px 16px 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--surface-soft);
  color: var(--muted);
  font-size: 13px;
}

.plugin-card__meta > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__source {
  max-width: 180px;
}

.plugin-card__meta :deep(.ant-tag),
.plugin-health-notices :deep(.ant-tag) {
  margin-inline-end: 0;
}

.plugin-health-notices {
  padding: 10px 16px 0;
}

.plugin-card__commands {
  display: grid;
  flex: 1 1 auto;
  align-content: start;
  gap: 8px;
  margin-top: 14px;
  padding: 14px 16px 16px;
  border-top: 1px solid var(--border);
}

.plugin-card__section-label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
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
  gap: var(--space-xs);
  min-width: 0;
  flex: 0 1 auto;
}

.plugin-command-chip :deep(.ant-tag) {
  margin-inline-end: 0;
}

.plugin-command-chip small,
.plugin-command-empty {
  color: var(--muted);
  font-size: 13px;
}

.plugin-command-expander {
  height: 22px;
  padding: 0 6px;
  color: var(--muted);
  font-size: 0.875rem;
  line-height: 20px;
}

.plugin-command-expander:hover,
.plugin-command-expander:focus-visible {
  color: var(--primary);
}

.plugin-card__actions,
.plugin-card__action-buttons {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.plugin-card__actions {
  justify-content: space-between;
  padding: 12px 16px;
  border-top: 1px solid var(--border);
  background: var(--surface-soft);
}

.plugin-card__actions :deep(.plugin-holo-button) {
  flex: 0 0 auto;
}

.drawer-card {
  margin-top: 12px;
}

.install-inspection {
  margin-top: 16px;
}

.install-inspection code {
  overflow-wrap: anywhere;
  font-size: 13px;
}

.install-inspection-list {
  display: grid;
  gap: 8px;
  margin-top: 16px;
}

.install-inspection-list > div {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.drawer-section {
  padding: var(--space-lg) 0;
  border-bottom: 1px solid var(--border);
}

.mono-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  strong { font-size: 1rem; font-weight: 600; }
  small { font-family: var(--font-mono); font-size: 13px; color: var(--muted); }
}
</style>
