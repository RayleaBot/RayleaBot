<script setup lang="ts">
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppDrawer from '@/components/AppDrawer.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppTag from '@/components/AppTag.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  FilterIcon,
  EyeIcon,
  SettingsIcon,
  SearchIcon,
  PlusIcon,
  RefreshCwIcon,
} from '@lucide/vue'

import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import PluginPowerButton from '@/components/PluginPowerButton.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import AppTableToolbar from '@/components/AppTableToolbar.vue'
import AppStatusTag from '@/components/AppStatusTag.vue'
import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import PluginCommandsPanel from '@/components/PluginCommandsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  formatPluginVersion,
  getPluginRoleLabel,
  getPluginStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildPluginDetailLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { getPluginTrustLabel } from '@/lib/display'
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
const { actionPending, error, inspectionPending, installPending, loading, sortedItems, total, nextCursor, loadingMore } = storeToRefs(pluginsStore)
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
const filterDrawerVisible = ref(false)

const searchQuery = ref('')
const filterState = ref<'all' | 'running' | 'disabled' | 'alert'>('all')
const stateOptions = computed(() => [
  { value: 'all', label: t('plugins.filter.stateAll') },
  { value: 'running', label: t('plugins.stats.running') },
  { value: 'disabled', label: t('plugins.stats.disabled') },
  { value: 'alert', label: t('plugins.stats.alert') },
])
const sourceOptions = computed(() => [
  { value: 'all', label: t('plugins.filter.sourceAll') },
  { value: 'official', label: t('plugins.filter.sourceOfficial') },
  { value: 'community', label: t('plugins.filter.sourceCommunity') },
])
const filterSource = ref<'all' | 'official' | 'community'>('all')
const inspectionPermissionNames = computed(() => Object.keys(installInspection.value?.permissions ?? {}).sort())

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
  return record.trust?.level === 'official'
}

function getTrustLabel(record: (typeof sortedItems.value)[number]) {
  return getPluginTrustLabel(record.trust?.level)
}

function getTrustColor(record: (typeof sortedItems.value)[number]) {
  if (record.trust?.level === 'unverified') return 'warning'
  if (isOfficialPlugin(record)) return 'neutral'
  return 'neutral'
}

function getSourceTypeLabel(type?: string) {
  if (type === 'local_zip') return t('plugins.localZip')
  if (type === 'local_directory') return t('plugins.localDirectory')
  if (type === 'remote_url') return t('plugins.remoteUrl')
  if (type === 'catalog') return t('plugins.catalogSource')
  if (type === 'development') return t('plugins.developmentSource')
  return type || t('display.empty')
}

const filteredItems = computed(() => sortedItems.value)
const listQuery = computed(() => ({ query: searchQuery.value, state: filterState.value === 'all' ? undefined : filterState.value, source: filterSource.value === 'all' ? undefined : filterSource.value }))
watch(listQuery, () => { void loadPlugins() })

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

  if (row.source?.verified === false && row.trust?.level !== 'unverified') {
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

function getOptionalDisplayText(value?: string | null) {
  const text = value?.trim()
  return text ? text : t('display.empty')
}

function getTagColor(tone: HealthNoticeTone) {
  if (tone === 'danger') return 'danger'
  if (tone === 'warning') return 'warning'
  if (tone === 'info') return 'neutral'
  return 'neutral'
}

async function loadPlugins() {
  try {
    await pluginsStore.fetchList(listQuery.value)
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

function openManagement(id: string) {
  void navigate(buildPluginDetailLocation(id, { panel: 'management-ui' }))
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
  <AppPage :title="t('plugins.title')" :show-header="false">
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
              <AppInput
                v-model="searchQuery" :maxlength="200"
                :placeholder="t('plugins.filter.searchPlaceholder')"
                wrapper-class="filter-search"
                allow-clear
              >
                <template #prefix>
                  <SearchIcon class="search-icon" />
                </template>
              </AppInput>

              <AppSegmented v-model="filterState" :options="stateOptions" :label="t('plugins.filter.title')" class="filter-radio-group" />

              <AppSelect v-model="filterSource" :options="sourceOptions" :aria-label="t('plugins.filter.sourceAll')" wrapper-class="filter-select" />
            </div>
          </template>

          <template #right>
            <AppButton class="plugins-filter-mobile-trigger" @click="filterDrawerVisible = true">
              <template #icon><FilterIcon /></template>
              {{ t('plugins.filter.title') }}
            </AppButton>
            <AppButton variant="default" @click="installDialogVisible = true">
              <template #icon><PlusIcon /></template>
              {{ t('plugins.install') }}
            </AppButton>
          </template>
        </AppTableToolbar>

        <div class="plugins-grid-container">
          <AppSkeleton v-if="loading && sortedItems.length === 0" :rows="6" />
          <AppEmptyState
            v-else-if="filteredItems.length === 0"
            icon="plugin"
            :title="t('plugins.empty.title')"
            :description="t('plugins.empty.description')"
            :action-label="t('plugins.install')"
            @action="installDialogVisible = true"
          />

          <div v-else class="plugins-grid" aria-label="插件列表">
            <article v-for="item in filteredItems" :key="item.id" class="plugin-grid-card">
              <header class="plugin-card__header">
                <PluginIcon :refresh-key="pluginsStore.iconRevision" :plugin-id="item.id" :icon="item.icon" :version="item.version" />
                <div class="plugin-card__identity">
                  <button type="button" class="plugin-card__name" :title="item.name" @click="openDetail(item.id)">
                    {{ item.name }}
                  </button>
                  <span v-if="item.version" class="plugin-card__version" :title="item.version">{{ formatPluginVersion(item.version) }}</span>
                </div>
                <AppStatusTag :status="item.state" :label="getPluginStateLabel(item.state)" :aria-label="`状态：${getPluginStateLabel(item.state)}`" />
              </header>

              <p class="plugin-card__description" :title="getOptionalDisplayText(item.description)">
                {{ getOptionalDisplayText(item.description) }}
              </p>

              <div class="plugin-card__meta">
                <span>{{ getSourceTypeLabel(item.source?.package_source_type) }}</span>
                <AppTag :tone="getTrustColor(item)">{{ getTrustLabel(item) }}</AppTag>
              </div>

              <div v-if="getPluginHealthNotices(item).length > 0" class="plugin-health-notices">
                <AppTag
                  v-for="notice in getPluginHealthNotices(item)"
                  :key="notice.label"
                  :tone="getTagColor(notice.tone)"
                  :aria-label="`健康状态：${notice.label}`"
                >
                  {{ notice.label }}
                </AppTag>
              </div>

              <footer class="plugin-card__actions">
                <div class="plugin-card__action-buttons">
                  <AppTooltip :title="t('plugins.actions.summary')">
                    <AppButton class="plugin-card__icon-action" variant="ghost" :aria-label="t('plugins.actions.summary')" @click="openSummary(item.id)">
                      <template #icon><EyeIcon /></template>
                    </AppButton>
                  </AppTooltip>
                  <AppButton
                    class="plugin-card__manage-action"
                    variant="ghost"
                    :aria-label="t('plugins.actions.manage')"
                    :data-testid="`plugin-manage-button-${item.id}`"
                    @click="openManagement(item.id)"
                  >
                    <template #icon><SettingsIcon /></template>
                    {{ t('plugins.actions.manage') }}
                  </AppButton>
                  <AppTooltip :title="t('plugins.actions.reload')">
                  <AppButton
                    class="plugin-card__icon-action"
                    variant="ghost"
                    :aria-label="t('plugins.actions.reload')"
                    :data-testid="`plugin-reload-button-${item.id}`"
                    :loading="actionPending[item.id] === 'reload'"
                    :disabled="isReloadDisabled(item.state)"
                    @click="reloadPlugin(item.id)"
                  >
                    <template #icon><RefreshCwIcon /></template>
                  </AppButton>
                  </AppTooltip>
                </div>
                <PluginPowerButton
                  icon-only
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
        <AppCollectionPagination :loaded="sortedItems.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore || loading" @more="pluginsStore.loadMore().catch(() => undefined)" />
      </AppCard>
    </div>

    <AppDrawer :open="filterDrawerVisible" placement="bottom" :title="t('plugins.filter.title')" @close="filterDrawerVisible = false">
      <div class="plugins-filter-drawer">
        <AppInput
          v-model="searchQuery" :maxlength="200"
          :placeholder="t('plugins.filter.searchPlaceholder')"
          allow-clear
        >
          <template #prefix><SearchIcon /></template>
        </AppInput>
        <AppSegmented v-model="filterState" :options="stateOptions" :label="t('plugins.filter.title')" class="filter-radio-group" />
        <AppSelect v-model="filterSource" :options="sourceOptions" :aria-label="t('plugins.filter.sourceAll')" wrapper-class="filter-select" />
        <AppButton variant="default" @click="filterDrawerVisible = false">完成</AppButton>
      </div>
    </AppDrawer>

    <AppDialog
      :open="installDialogVisible"
      :title="t('plugins.installDialogTitle')"
      :busy="inspectionPending || installPending"
      @close="installDialogVisible = false"
      @after-close="resetInstallDialog"
    >
      <div>
        <AppField floating :label="t('plugins.sourceType')">
          <AppSelect
            v-model="installForm.source_type"
            :options="[
              { label: t('plugins.localZip'), value: 'local_zip' },
              { label: t('plugins.localDirectory'), value: 'local_directory' },
              { label: t('plugins.remoteUrl'), value: 'remote_url' },
            ]"
          />
        </AppField>

        <AppField floating :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')">
          <AppInput v-model="installForm.source" />
        </AppField>

        <template v-if="installInspection">
          <AppAlert
            tone="warning"
            title="第三方插件是完全可信的本地代码"
            description="原生插件进程使用当前用户权限运行。仅安装来源、平台、摘要和权限均符合预期的代码。"
          />

          <AppDetails class="install-inspection">
            <AppDetailItem label="插件">{{ installInspection.plugin.name }}（{{ installInspection.plugin.id }}）</AppDetailItem>
            <AppDetailItem label="版本">{{ installInspection.plugin.version }}</AppDetailItem>
            <AppDetailItem label="作者">{{ installInspection.plugin.author }}</AppDetailItem>
            <AppDetailItem label="许可证">{{ installInspection.plugin.license }}</AppDetailItem>
            <AppDetailItem label="来源">{{ installInspection.plugin.source_label }}</AppDetailItem>
            <AppDetailItem label="包摘要"><code>{{ installInspection.package_sha256 }}</code></AppDetailItem>
            <AppDetailItem label="目标平台">{{ installInspection.target_platform }}</AppDetailItem>
            <AppDetailItem label="后端"><code>{{ installInspection.backend.path }}</code> · {{ installInspection.backend.size }} bytes</AppDetailItem>
            <AppDetailItem label="管理页面">{{ installInspection.ui.enabled ? `${installInspection.ui.entry}（${installInspection.ui.file_count} 个文件）` : '无' }}</AppDetailItem>
            <AppDetailItem label="Artifact 校验">{{ installInspection.artifact.valid ? `v${installInspection.artifact.artifact_version} · ${installInspection.artifact.file_count} 个文件` : '未通过' }}</AppDetailItem>
            <AppDetailItem label="有效期">{{ installInspection.expires_at }}</AppDetailItem>
          </AppDetails>

          <div class="install-inspection-list">
            <strong>声明权限</strong>
            <div>
              <AppTag v-for="permission in inspectionPermissionNames" :key="permission">{{ permission }}</AppTag>
              <span v-if="inspectionPermissionNames.length === 0">未声明额外权限</span>
            </div>
          </div>

          <div class="app-field-group">
            <AppCheckbox v-model="trustedCodeConfirmed">
              我已核对来源、目标平台、artifact 摘要和权限，并信任此代码使用本机当前用户权限运行。
            </AppCheckbox>
          </div>
        </template>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <AppButton :disabled="inspectionPending || installPending" @click="installDialogVisible = false">{{ t('dashboard.previewCancel') }}</AppButton>
          <AppButton variant="default" :loading="inspectionPending || installPending" :disabled="!installForm.source.trim() || Boolean(installInspection && !trustedCodeConfirmed)" @click="submitInstall">{{ installInspection ? t('plugins.installSubmit') : '检查插件包' }}</AppButton>
        </div>
      </template>
    </AppDialog>

    <AppDrawer :open="summaryDrawerVisible" :title="t('plugins.actions.summary')" :width="560" @close="summaryDrawerVisible = false">
      <template v-if="summaryPlugin">
        <div class="drawer-section drawer-section--dense">
          <div class="mono-list">
            <strong>{{ summaryPlugin.name }}</strong>
            <small>{{ summaryPlugin.id }}</small>
          </div>
        </div>

        <AppCard borderless class="drawer-card">
          <AppDetails>
            <AppDetailItem :label="t('plugins.fields.role')">{{ getPluginRoleLabel(summaryPlugin.role) }}</AppDetailItem>
            <AppDetailItem :label="t('plugins.fields.trust')">{{ getPluginTrustLabel(summaryPlugin.trust?.level) }}</AppDetailItem>
            <AppDetailItem :label="t('plugins.fields.state')">{{ getPluginStateLabel(summaryPlugin.state) }}</AppDetailItem>
            <AppDetailItem :label="t('plugins.fields.source')">{{ summaryPlugin.source?.root ?? t('display.empty') }}</AppDetailItem>
            <AppDetailItem :label="t('plugins.fields.sourceRef')">
              {{ summaryPlugin.source?.package_source_ref ?? summaryPlugin.source?.package_source_type ?? t('display.empty') }}
            </AppDetailItem>
            <AppDetailItem :label="t('plugins.fields.conflicts')">
              <div v-if="summaryPlugin.command_conflicts?.length" class="table-actions">
                <AppTag v-for="command in summaryPlugin.command_conflicts" :key="command" tone="warning">
                  {{ command }}
                </AppTag>
              </div>
              <span v-else>{{ t('display.empty') }}</span>
            </AppDetailItem>
          </AppDetails>
        </AppCard>

        <AppCard :title="t('plugins.sections.commands')" borderless class="drawer-card">
          <PluginCommandsPanel
            :commands="summaryPlugin.commands"
            :command-conflicts="summaryPlugin.command_conflicts"
          />
        </AppCard>
      </template>
    </AppDrawer>
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

  :deep(.app-input) {
    border-radius: 6px;
  }
  .search-icon {
    color: var(--muted);
  }
}

.filter-radio-group { flex-shrink: 0; }

.filter-select {
  width: 140px;
  :deep(.app-select) {
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

.plugins-filter-drawer :deep(.app-segmented) {
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


}

.plugins-card {
  box-shadow: none;
}

.plugins-card :deep(.app-card__body) {
  padding: 0;
}

.plugins-grid-container {
  min-height: 220px;
  padding: 16px 0 0;
  background: var(--bg);
}

.plugins-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-lg);
  align-items: stretch;
}

.plugin-grid-card {
  display: flex;
  min-width: 0;
  overflow: hidden;
  flex-direction: column;
  min-height: 224px;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: none;
  transition: border-color 160ms var(--motion-easing), box-shadow 160ms var(--motion-easing);
}

.plugin-grid-card:hover {
  border-color: var(--border-strong);
  box-shadow: none;
}

.plugin-card__header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 16px 16px 12px;
}

.plugin-card__identity {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  min-width: 0;
  flex: 1 1 auto;
  gap: 2px 8px;
}

.plugin-card__name {
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--text);
  cursor: pointer;
  font: inherit;
  font-size: 15px;
  font-weight: 600;
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
  outline-offset: var(--focus-outline-offset);
  border-radius: 4px;
}

.plugin-card__version {
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
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
  gap: 6px 8px;
}

.plugin-card__meta {
  margin: 12px 16px 12px;
  min-height: 24px;
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: var(--muted);
  font-size: 13px;
}

.plugin-card__meta > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-card__meta :deep(.app-tag),
.plugin-health-notices :deep(.app-tag) {
  margin-inline-end: 0;
}

.plugin-health-notices {
  padding: 0 16px 12px;
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
  margin-top: auto;
  padding: 10px 12px;
  border-top: 1px solid var(--border);
  background: transparent;
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

.plugin-card__action-buttons { gap: 8px; }
.plugin-card__manage-action.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding-inline: 11px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  font-size: 14px;
  box-shadow: none;
}
.plugin-card__manage-action.app-button:hover:not(:disabled) { background: var(--surface-accent); color: var(--text-accent); }
.plugin-card__manage-action.app-button:active:not(:disabled) { transform: scale(.97); }
.plugin-card__icon-action.app-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  color: var(--text);
  background: var(--surface);
  border: 1px solid var(--border);
  font-size: 18px;
  border-radius: var(--radius-md);
  box-shadow: none;
}
.plugin-card__icon-action.app-button:hover:not(:disabled) { background: var(--surface-accent); color: var(--text); }
.plugin-card__icon-action.app-button:active:not(:disabled) { transform: scale(.94); }
.plugin-card__icon-action.app-button:disabled { color: var(--muted); opacity: .45; }
@media (min-width: 1800px) { .plugins-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
@media (min-width: 2300px) {
 .plugins-grid { grid-template-columns: repeat(5, minmax(0, 1fr)); }
 .plugin-grid-card { min-height: 240px; }
}
@media (max-width: 1199px) { .plugins-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 767px) { .plugins-grid { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 639px), (pointer: coarse) {
 .plugin-card__name { min-height: 44px; white-space: normal; line-height: 1.4; }
 .plugin-card__manage-action.app-button { min-height: 44px; }
 .plugin-card__icon-action.app-button { width: 44px; height: 44px; }
 .plugin-card__description { min-height: 0; }
}
</style>
