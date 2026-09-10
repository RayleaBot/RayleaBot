<script setup lang="ts">
import AppSelect from '@/components/AppSelect.vue'
import AppSearchInput from '@/components/AppSearchInput.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppTag from '@/components/AppTag.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Trash2Icon,
  PencilIcon,
  ExternalLinkIcon,
  PlusIcon,
  RotateCwIcon,
  SettingsIcon,
  RefreshCwIcon,
} from '@lucide/vue'

import { notifyError, notifySuccess, notifyWarning } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { formatPluginVersion } from '@/lib/display'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginStore, type PluginStoreSort } from '@/stores/plugin-store'
import type {
  PluginStoreEntry,
  PluginStoreInspectionResponse,
  PluginStoreSourceInput,
} from '@/types/api'

const store = usePluginStore()
const pluginsStore = usePluginsStore()
const { error, installing, items, loading, loadingMore, nextCursor, refreshing, source, sourceSaving, sources, total } = storeToRefs(store)

const query = ref('')
const sort = ref<PluginStoreSort>('recommended')
const sourceId = ref('official')
const confirmationOpen = ref(false)
const selectedPlugin = ref<PluginStoreEntry | null>(null)
const selectedInspection = ref<PluginStoreInspectionResponse | null>(null)
const sourceManagerOpen = ref(false)
const pendingSourceRemoval = ref<string | null>(null)
const sourceRemoving = ref(false)
const sourceOptions = computed(() => sources.value.map(item => ({ value: item.id, label: item.name })))
const sortOptions = computed(() => [
  { value: 'recommended', label: t('plugins.store.sort.recommended') },
  { value: 'name', label: t('plugins.store.sort.name') },
  { value: 'updated', label: t('plugins.store.sort.updated') },
])
const sourceEditorOpen = ref(false)
const editingSourceId = ref<string | null>(null)
const batchUpdating = ref(false)
const sourceForm = reactive<PluginStoreSourceInput>({ name: '', url: '' })
let pageActive = true
let activated = false
let refreshTimer: ReturnType<typeof setTimeout> | undefined

const permissionNames = computed(() => Object.keys(selectedInspection.value?.inspection.permissions ?? {}).sort())
const updateablePlugins = computed(() => items.value.filter(item => item.install_state === 'update_available'))
const selectedSource = computed(() => sources.value.find(item => item.id === sourceId.value) ?? source.value)

async function loadEntries() {
  try {
    await store.fetchEntries({ sourceId: sourceId.value, query: query.value, sort: sort.value })
  } catch {
    // The store owns the page error state.
  }
}

async function loadInitialEntries() {
  try {
    await store.fetchSources()
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
  await loadEntries()
  try {
    await store.refreshSource(sourceId.value)
    await loadEntries()
  } catch {
    // Keep the last successful catalog visible; manual refresh remains available.
  }
}

async function changeSource() {
  query.value = ''
  await loadEntries()
  if (!source.value?.cached) {
    await refreshSource()
  }
}

async function refreshSource() {
  try {
    await store.refreshSource(sourceId.value)
    await loadEntries()
    notifySuccess(t('plugins.store.feedback.refreshed'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

async function requestInstall(plugin: PluginStoreEntry) {
  try {
    const inspection = await store.inspect(plugin.id, {
      source_id: sourceId.value,
    })
    if (!inspection.confirmation_required) {
      await acceptInspection(plugin, inspection, false)
      return
    }
    store.finishInspection(plugin.id)
    selectedPlugin.value = plugin
    selectedInspection.value = inspection
    confirmationOpen.value = true
  } catch (cause) {
    store.finishInspection(plugin.id)
    notifyError(getDisplayErrorMessage(cause))
  }
}

async function acceptInspection(
  plugin: PluginStoreEntry,
  inspection: PluginStoreInspectionResponse,
  trustedCodeConfirmed: boolean,
) {
  await store.install(plugin.id, {
    inspection_id: inspection.inspection.inspection_id,
    package_sha256: inspection.inspection.package_sha256,
    trusted_code_confirmed: trustedCodeConfirmed,
  })
  notifySuccess(t('plugins.store.feedback.accepted'))
}

async function confirmInstall() {
  if (!selectedPlugin.value || !selectedInspection.value) return
  try {
    const installation = acceptInspection(selectedPlugin.value, selectedInspection.value, true)
    closeConfirmation()
    await installation
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

function closeConfirmation() { confirmationOpen.value = false }
function resetConfirmation() {
  if (selectedPlugin.value) store.finishInspection(selectedPlugin.value.id)
  selectedPlugin.value = null
  selectedInspection.value = null
}

async function updateAll() {
  if (batchUpdating.value || updateablePlugins.value.length === 0) return
  batchUpdating.value = true
  let accepted = 0
  let skipped = 0
  try {
    for (const plugin of updateablePlugins.value) {
      try {
        const inspection = await store.inspect(plugin.id, {
          source_id: sourceId.value,
        })
        if (inspection.confirmation_required) {
          store.finishInspection(plugin.id)
          skipped++
          continue
        }
        await store.install(plugin.id, {
          inspection_id: inspection.inspection.inspection_id,
          package_sha256: inspection.inspection.package_sha256,
          trusted_code_confirmed: false,
        })
        accepted++
      } catch {
        store.finishInspection(plugin.id)
        skipped++
      }
    }
    if (accepted > 0) {
      notifySuccess(t('plugins.store.feedback.batchAccepted', { count: accepted }))
    }
    if (skipped > 0) {
      notifyWarning(t('plugins.store.feedback.batchSkipped', { count: skipped }))
    }
  } finally {
    batchUpdating.value = false
  }
}

function openNewSource() {
  editingSourceId.value = null
  sourceForm.name = ''
  sourceForm.url = ''
  sourceManagerOpen.value = false
  sourceEditorOpen.value = true
}

function openSourceEditor(id: string) {
  const item = sources.value.find(sourceItem => sourceItem.id === id)
  if (!item || item.official) return
  editingSourceId.value = item.id
  sourceForm.name = item.name
  sourceForm.url = item.url
  sourceManagerOpen.value = false
  sourceEditorOpen.value = true
}

async function saveSource() {
  const input = { name: sourceForm.name.trim(), url: sourceForm.url.trim() }
  if (!input.name || !input.url) return
  try {
    const saved = editingSourceId.value
      ? await store.saveSource(editingSourceId.value, input)
      : await store.createSource(input)
    sourceEditorOpen.value = false
    sourceId.value = saved.id
    await loadEntries()
    notifySuccess(t('plugins.store.sources.saved'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

async function removeSource(id: string) {
  if (sourceRemoving.value) return
  sourceRemoving.value = true
  try {
    await store.deleteSource(id)
    if (sourceId.value === id) {
      sourceId.value = 'official'
      await loadEntries()
    }
    pendingSourceRemoval.value = null
    notifySuccess(t('plugins.store.sources.removed'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  } finally {
    sourceRemoving.value = false
  }
}

function installActionLabel(plugin: PluginStoreEntry) {
  if (installing.value[plugin.id]) return t('plugins.store.actions.installing')
  switch (plugin.install_state) {
    case 'update_available': return t('plugins.store.actions.update')
    case 'installed': return t('plugins.store.actions.installed')
    case 'unpublished': return t('plugins.store.actions.unpublished')
    case 'incompatible': return t('plugins.store.actions.incompatible')
    default: return t('plugins.store.actions.install')
  }
}

function canInstall(plugin: PluginStoreEntry) {
  return !installing.value[plugin.id] && (plugin.install_state === 'available' || plugin.install_state === 'update_available')
}

async function loadMore() {
  try { await store.loadMore() } catch (cause) { notifyError(getDisplayErrorMessage(cause)) }
}

watch(() => pluginsStore.items.map(plugin => `${plugin.id}:${plugin.version}:${plugin.state}`).join('|'), () => {
  if (!pageActive || loading.value || Object.values(installing.value).some(Boolean)) return
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => { if (pageActive) void loadEntries() }, 150)
})
onActivated(() => { pageActive = true; if (activated) void loadEntries(); activated = true })
onDeactivated(() => { pageActive = false; clearTimeout(refreshTimer) })
onBeforeUnmount(() => { pageActive = false; clearTimeout(refreshTimer) })
onMounted(() => {
  void loadInitialEntries()
})
</script>

<template>
  <AppPage :title="t('plugins.store.title')" :show-header="false">
    <template #toolbar>
      <div class="store-toolbar">
        <AppSearchInput v-model="query" :placeholder="t('plugins.store.searchPlaceholder')" class="store-search" @search="loadEntries" />
        <AppSelect v-model="sourceId" :options="sourceOptions" :aria-label="t('plugins.fields.source')" wrapper-class="store-source" @update:model-value="changeSource" />
        <AppSelect v-model="sort" :options="sortOptions" aria-label="排序方式" wrapper-class="store-sort" @update:model-value="loadEntries" />
        <div class="store-actions">
          <AppTag v-if="selectedSource" :tone="selectedSource.official ? 'info' : 'neutral'">
            {{ selectedSource.official ? t('plugins.store.sources.official') : t('plugins.store.sources.custom') }}
          </AppTag>
          <AppTag v-if="selectedSource && !selectedSource.cached" tone="warning">
            {{ t('plugins.store.sources.notCached') }}
          </AppTag>
          <span class="store-count">{{ t('plugins.store.resultCount', { count: total }) }}</span>
          <AppButton v-if="updateablePlugins.length" :loading="batchUpdating" @click="updateAll">
            <template #icon><RefreshCwIcon /></template>
            {{ t('plugins.store.actions.updateAll', { count: updateablePlugins.length }) }}
          </AppButton>
          <AppButton data-testid="plugin-store-sources" @click="sourceManagerOpen = true">
            <template #icon><SettingsIcon /></template>
            {{ t('plugins.store.sources.manage') }}
          </AppButton>
          <AppButton :loading="refreshing" data-testid="plugin-store-refresh" @click="refreshSource">
            <template #icon><RotateCwIcon /></template>
            {{ t('plugins.store.actions.refresh') }}
          </AppButton>
        </div>
      </div>
    </template>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadEntries"
    />

    <template v-else>
      <AppSkeleton v-if="loading" :rows="8" />
      <AppEmptyState
        v-else-if="items.length === 0"
        :title="t('plugins.store.empty.title')"
        :description="t('plugins.store.empty.description')"
      />
      <div v-else class="store-grid">
        <AppCard v-for="plugin in items" :key="plugin.id" class="store-plugin-card" shadow="sm">
          <div class="plugin-card-header">
            <div class="plugin-identity">
              <PluginIcon :plugin-id="plugin.id" :remote-url="plugin.icon_url ?? ''" :source-key="sourceId" :version="plugin.latest_release?.version" :refresh-key="store.iconRevision" />
              <div class="plugin-heading">
                <div class="plugin-title-line">
                  <h2>{{ plugin.name }}</h2>
                  <AppTag v-if="plugin.recommended" tone="info">{{ t('plugins.store.recommended') }}</AppTag>
                  <AppTag v-if="plugin.category">{{ plugin.category }}</AppTag>
                </div>
                <span class="plugin-id">{{ plugin.id }}</span>
              </div>
            </div>
            <AppTooltip :title="t('plugins.store.repository')">
              <AppButton
                :href="plugin.repository_url"
                target="_blank"
                rel="noopener noreferrer"
                shape="circle"
                variant="ghost"
                :aria-label="t('plugins.store.repository')"
              >
                <template #icon><ExternalLinkIcon /></template>
              </AppButton>
            </AppTooltip>
          </div>

          <p class="plugin-summary">{{ plugin.summary }}</p>

          <div class="plugin-meta">
            <span>{{ t('plugins.store.publisher', { name: plugin.publisher.name }) }}</span>
            <span :title="t('plugins.fields.license')">{{ plugin.license }}</span>
            <span v-if="plugin.latest_release">{{ t('plugins.store.latestVersion', { version: formatPluginVersion(plugin.latest_release.version) }) }}</span>
          </div>

          <div class="plugin-card-footer">
            <span v-if="plugin.installed_version" class="installed-version">
              {{ t('plugins.store.installedVersion', { version: formatPluginVersion(plugin.installed_version) }) }}
            </span>
            <span v-else />
            <AppButton
              variant="default"
              :disabled="!canInstall(plugin)"
              :loading="installing[plugin.id]"
              :data-testid="`plugin-store-install-${plugin.id}`"
              @click="requestInstall(plugin)"
            >
              {{ installActionLabel(plugin) }}
            </AppButton>
          </div>
        </AppCard>
      </div>
      <div v-if="nextCursor" class="store-load-more"><AppButton :loading="loadingMore" :disabled="loading || loadingMore" @click="loadMore">{{ t('plugins.store.loadMore') }}</AppButton></div>
    </template>

    <AppDialog :open="confirmationOpen" :title="t('plugins.store.confirm.title')" :busy="Boolean(selectedPlugin && installing[selectedPlugin.id])" fallback-focus="[data-testid=plugin-store-refresh]" @close="closeConfirmation" @after-close="resetConfirmation">
      <AppAlert
        tone="warning"
        :title="t('plugins.store.confirm.warning')"
        :description="t('plugins.store.confirm.description', { name: selectedPlugin?.name ?? '' })"
      />
      <AppDetails v-if="selectedInspection" class="confirm-details">
        <AppDetailItem :label="t('plugins.fields.version')">
          {{ formatPluginVersion(selectedInspection.inspection.plugin.version) }}
        </AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.source')">
          {{ selectedInspection.inspection.plugin.source_label }}
        </AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.permissions')">
          <div class="permission-list">
            <AppTag v-for="permission in permissionNames" :key="permission">{{ permission }}</AppTag>
            <span v-if="permissionNames.length === 0">{{ t('plugins.store.confirm.noPermissions') }}</span>
          </div>
        </AppDetailItem>
      </AppDetails>
    <template #footer><div class="flex justify-end gap-3"><AppButton :disabled="Boolean(selectedPlugin && installing[selectedPlugin.id])" @click="closeConfirmation">{{ t('shell.cancel') }}</AppButton><AppButton variant="default" :loading="Boolean(selectedPlugin && installing[selectedPlugin.id])" @click="confirmInstall">{{ t('plugins.store.confirm.action') }}</AppButton></div></template>
    </AppDialog>

    <AppDialog :open="sourceManagerOpen" :title="t('plugins.store.sources.manage')" :width="720" fallback-focus="[data-testid=plugin-store-sources]" @close="sourceManagerOpen = false">
      <div class="source-manager-header">
        <p>{{ t('plugins.store.sources.description') }}</p>
        <AppButton variant="default" @click="openNewSource">
          <template #icon><PlusIcon /></template>
          {{ t('plugins.store.sources.add') }}
        </AppButton>
      </div>
      <div class="source-list">
        <div v-for="item in sources" :key="item.id" class="source-row">
          <div class="source-copy">
            <div class="source-title">
              <strong>{{ item.name }}</strong>
              <AppTag v-if="item.official" tone="info">{{ t('plugins.store.sources.official') }}</AppTag>
              <AppTag v-else>{{ t('plugins.store.sources.custom') }}</AppTag>
              <AppTag :tone="item.cached ? 'success' : 'neutral'">
                {{ item.cached ? t('plugins.store.sources.cached') : t('plugins.store.sources.notCached') }}
              </AppTag>
            </div>
            <code>{{ item.url }}</code>
          </div>
          <div v-if="!item.official" class="source-row-actions">
            <AppButton variant="ghost" @click="openSourceEditor(item.id)">
              <template #icon><PencilIcon /></template>
              {{ t('plugins.store.sources.edit') }}
            </AppButton>

              <AppButton variant="destructive" @click="pendingSourceRemoval = item.id">
                <template #icon><Trash2Icon /></template>
                {{ t('plugins.store.sources.remove') }}
              </AppButton>

          </div>
        </div>
      </div>

    </AppDialog>

    <AppDialog :open="sourceEditorOpen" :title="editingSourceId ? t('plugins.store.sources.edit') : t('plugins.store.sources.add')" :busy="sourceSaving" fallback-focus="[data-testid=plugin-store-sources]" @close="sourceEditorOpen = false">
      <div>
        <AppField floating :label="t('plugins.store.sources.name')">
          <AppInput v-model="sourceForm.name" :maxlength="120" />
        </AppField>
        <AppField floating :label="t('plugins.store.sources.url')">
          <AppInput v-model="sourceForm.url" placeholder="https://example.com/catalog.json" />
        </AppField>
      </div>
    <template #footer><div class="flex justify-end gap-3"><AppButton :disabled="sourceSaving" @click="sourceEditorOpen = false">{{ t('shell.cancel') }}</AppButton><AppButton variant="default" :loading="sourceSaving" :disabled="!sourceForm.name.trim() || !sourceForm.url.trim()" @click="saveSource">{{ t('plugins.store.sources.save') }}</AppButton></div></template>
    </AppDialog>
    <AppConfirmDialog :open="pendingSourceRemoval !== null" :title="t('plugins.store.sources.remove')" :description="t('plugins.store.sources.removeConfirm')" :busy="sourceRemoving" danger :confirm-text="t('plugins.store.sources.remove')" @confirm="pendingSourceRemoval && removeSource(pendingSourceRemoval)" @cancel="pendingSourceRemoval = null" />
  </AppPage>
</template>

<style scoped lang="scss">
.store-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.store-search { width: min(420px, 100%); }
.store-source { width: min(240px, 100%); }
.store-sort { width: 150px; }

.store-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-inline-start: auto;
}

.store-actions .app-tag { margin: 0; }

.store-count,
.plugin-id,
.plugin-meta,
.installed-version {
  color: var(--muted);
  font-size: 12px;
}

.store-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 340px), 1fr));
  gap: 16px;
}

.store-plugin-card :deep(.app-card__body) {
  display: flex;
  min-height: 244px;
  flex-direction: column;
}

.plugin-card-header,
.plugin-identity,
.plugin-title-line,
.plugin-meta,
.plugin-card-footer,
.source-manager-header,
.source-title,
.source-row,
.source-row-actions {
  display: flex;
  align-items: center;
}

.plugin-card-header,
.plugin-card-footer,
.source-manager-header,
.source-row {
  justify-content: space-between;
  gap: 12px;
}

.plugin-identity {
  min-width: 0;
  gap: 12px;
}

.plugin-heading { min-width: 0; }

.store-load-more { display: flex; justify-content: center; margin-top: var(--space-lg); }

.plugin-title-line {
  flex-wrap: wrap;
  gap: 6px;
}

.plugin-title-line h2 {
  margin: 0;
  color: var(--text);
  font-size: 17px;
  line-height: 1.35;
}

.plugin-summary {
  margin: 18px 0 14px;
  color: var(--text-secondary, var(--muted));
  line-height: 1.65;
}

.plugin-meta {
  flex-wrap: wrap;
  gap: 8px 14px;
}

.plugin-card-footer {
  margin-top: auto;
  padding-top: 20px;
}

.confirm-details { margin-top: 18px; }

.permission-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.permission-list .app-tag { margin: 0; }

.source-manager-header { margin-bottom: 16px; }
.source-manager-header p { margin: 0; color: var(--muted); }

.source-list {
  border-block-start: 1px solid var(--border);
}

.source-row {
  padding: 14px 0;
  border-block-end: 1px solid var(--border);
}

.source-copy {
  min-width: 0;
}

.source-title {
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 4px;
}

.source-title .app-tag { margin: 0; }

.source-copy code {
  display: block;
  max-width: 56ch;
  overflow: hidden;
  color: var(--muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-row-actions {
  flex: 0 0 auto;
  gap: 4px;
}

@media (max-width: 760px) {
  .store-search,
  .store-source,
  .store-sort {
    width: 100%;
  }

  .store-actions {
    width: 100%;
    margin-inline-start: 0;
  }

  .source-row {
    align-items: flex-start;
    flex-direction: column;
  }

  .source-row-actions {
    align-self: flex-end;
  }
}
</style>
