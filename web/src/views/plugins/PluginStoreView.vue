<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import {
  DeleteOutlined,
  EditOutlined,
  GithubOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
  SyncOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess, notifyWarning } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { usePluginStore, type PluginStoreSort } from '@/stores/plugin-store'
import type {
  PluginStoreEntry,
  PluginStoreInspectionResponse,
  PluginStoreSourceInput,
} from '@/types/api'

const store = usePluginStore()
const { error, installing, items, loading, refreshing, source, sourceSaving, sources, total } = storeToRefs(store)

const query = ref('')
const sort = ref<PluginStoreSort>('recommended')
const sourceId = ref('official')
const confirmationOpen = ref(false)
const selectedPlugin = ref<PluginStoreEntry | null>(null)
const selectedInspection = ref<PluginStoreInspectionResponse | null>(null)
const sourceManagerOpen = ref(false)
const sourceEditorOpen = ref(false)
const editingSourceId = ref<string | null>(null)
const batchUpdating = ref(false)
const sourceForm = reactive<PluginStoreSourceInput>({ name: '', url: '' })
const failedIcons = ref<Set<string>>(new Set())

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
    await acceptInspection(selectedPlugin.value, selectedInspection.value, true)
    closeConfirmation()
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

function closeConfirmation() {
  if (selectedPlugin.value) {
    store.finishInspection(selectedPlugin.value.id)
  }
  confirmationOpen.value = false
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
  try {
    await store.deleteSource(id)
    if (sourceId.value === id) {
      sourceId.value = 'official'
      await loadEntries()
    }
    notifySuccess(t('plugins.store.sources.removed'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

function installActionLabel(plugin: PluginStoreEntry) {
  switch (plugin.install_state) {
    case 'update_available': return t('plugins.store.actions.update')
    case 'installed': return t('plugins.store.actions.installed')
    case 'unpublished': return t('plugins.store.actions.unpublished')
    case 'incompatible': return t('plugins.store.actions.incompatible')
    default: return t('plugins.store.actions.install')
  }
}

function canInstall(plugin: PluginStoreEntry) {
  return plugin.install_state === 'available' || plugin.install_state === 'update_available'
}

function markIconFailed(pluginId: string) {
  failedIcons.value = new Set(failedIcons.value).add(pluginId)
}

onMounted(() => {
  void loadInitialEntries()
})
</script>

<template>
  <AppPage :title="t('plugins.store.title')" :show-header="false">
    <template #toolbar>
      <div class="store-toolbar">
        <a-input-search
          v-model:value="query"
          :placeholder="t('plugins.store.searchPlaceholder')"
          allow-clear
          class="store-search"
          @search="loadEntries"
        >
          <template #prefix><SearchOutlined /></template>
        </a-input-search>
        <a-select v-model:value="sourceId" class="store-source" @change="changeSource">
          <a-select-option v-for="item in sources" :key="item.id" :value="item.id">
            {{ item.name }}
          </a-select-option>
        </a-select>
        <a-select v-model:value="sort" class="store-sort" @change="loadEntries">
          <a-select-option value="recommended">{{ t('plugins.store.sort.recommended') }}</a-select-option>
          <a-select-option value="name">{{ t('plugins.store.sort.name') }}</a-select-option>
          <a-select-option value="updated">{{ t('plugins.store.sort.updated') }}</a-select-option>
        </a-select>
        <div class="store-actions">
          <a-tag v-if="selectedSource" :color="selectedSource.official ? 'blue' : 'default'">
            {{ selectedSource.official ? t('plugins.store.sources.official') : t('plugins.store.sources.custom') }}
          </a-tag>
          <a-tag v-if="selectedSource && !selectedSource.cached" color="warning">
            {{ t('plugins.store.sources.notCached') }}
          </a-tag>
          <span class="store-count">{{ t('plugins.store.resultCount', { count: total }) }}</span>
          <a-button v-if="updateablePlugins.length" :loading="batchUpdating" @click="updateAll">
            <template #icon><SyncOutlined /></template>
            {{ t('plugins.store.actions.updateAll', { count: updateablePlugins.length }) }}
          </a-button>
          <a-button @click="sourceManagerOpen = true">
            <template #icon><SettingOutlined /></template>
            {{ t('plugins.store.sources.manage') }}
          </a-button>
          <a-button :loading="refreshing" data-testid="plugin-store-refresh" @click="refreshSource">
            <template #icon><ReloadOutlined /></template>
            {{ t('plugins.store.actions.refresh') }}
          </a-button>
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
      <a-skeleton v-if="loading" active :paragraph="{ rows: 8 }" />
      <AppEmptyState
        v-else-if="items.length === 0"
        :title="t('plugins.store.empty.title')"
        :description="t('plugins.store.empty.description')"
      />
      <div v-else class="store-grid">
        <AppCard v-for="plugin in items" :key="plugin.id" class="store-plugin-card" shadow="sm">
          <div class="plugin-card-header">
            <div class="plugin-identity">
              <div class="plugin-mark">
                <img
                  v-if="plugin.icon_url && !failedIcons.has(plugin.id)"
                  :src="plugin.icon_url"
                  alt=""
                  referrerpolicy="no-referrer"
                  @error="markIconFailed(plugin.id)"
                />
                <span v-else>{{ plugin.name.slice(0, 2).toUpperCase() }}</span>
              </div>
              <div class="plugin-heading">
                <div class="plugin-title-line">
                  <h2>{{ plugin.name }}</h2>
                  <a-tag v-if="plugin.recommended" color="processing">{{ t('plugins.store.recommended') }}</a-tag>
                  <a-tag v-if="plugin.category">{{ plugin.category }}</a-tag>
                </div>
                <span class="plugin-id">{{ plugin.id }}</span>
              </div>
            </div>
            <a-tooltip :title="t('plugins.store.repository')">
              <a-button
                :href="plugin.repository_url"
                target="_blank"
                rel="noopener noreferrer"
                shape="circle"
                type="text"
                :aria-label="t('plugins.store.repository')"
              >
                <template #icon><GithubOutlined /></template>
              </a-button>
            </a-tooltip>
          </div>

          <p class="plugin-summary">{{ plugin.summary }}</p>

          <div class="plugin-meta">
            <span>{{ plugin.publisher.name }}</span>
            <span>{{ plugin.license }}</span>
            <span v-if="plugin.latest_release">v{{ plugin.latest_release.version }}</span>
          </div>

          <div class="plugin-card-footer">
            <span v-if="plugin.installed_version" class="installed-version">
              {{ t('plugins.store.installedVersion', { version: plugin.installed_version }) }}
            </span>
            <span v-else />
            <a-button
              type="primary"
              :disabled="!canInstall(plugin)"
              :loading="installing[plugin.id]"
              :data-testid="`plugin-store-install-${plugin.id}`"
              @click="requestInstall(plugin)"
            >
              {{ installActionLabel(plugin) }}
            </a-button>
          </div>
        </AppCard>
      </div>
    </template>

    <a-modal
      v-model:open="confirmationOpen"
      :title="t('plugins.store.confirm.title')"
      :confirm-loading="selectedPlugin ? installing[selectedPlugin.id] : false"
      :ok-text="t('plugins.store.confirm.action')"
      :cancel-text="t('shell.cancel')"
      @ok="confirmInstall"
      @cancel="closeConfirmation"
    >
      <a-alert
        type="warning"
        show-icon
        :message="t('plugins.store.confirm.warning')"
        :description="t('plugins.store.confirm.description', { name: selectedPlugin?.name ?? '' })"
      />
      <a-descriptions v-if="selectedInspection" class="confirm-details" :column="1" size="small">
        <a-descriptions-item :label="t('plugins.fields.version')">
          {{ selectedInspection.inspection.plugin.version }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('plugins.fields.source')">
          {{ selectedInspection.inspection.plugin.source_label }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('plugins.fields.permissions')">
          <div class="permission-list">
            <a-tag v-for="permission in permissionNames" :key="permission">{{ permission }}</a-tag>
            <span v-if="permissionNames.length === 0">{{ t('plugins.store.confirm.noPermissions') }}</span>
          </div>
        </a-descriptions-item>
      </a-descriptions>
    </a-modal>

    <a-modal
      v-model:open="sourceManagerOpen"
      :title="t('plugins.store.sources.manage')"
      :footer="null"
      width="720px"
    >
      <div class="source-manager-header">
        <p>{{ t('plugins.store.sources.description') }}</p>
        <a-button type="primary" @click="openNewSource">
          <template #icon><PlusOutlined /></template>
          {{ t('plugins.store.sources.add') }}
        </a-button>
      </div>
      <div class="source-list">
        <div v-for="item in sources" :key="item.id" class="source-row">
          <div class="source-copy">
            <div class="source-title">
              <strong>{{ item.name }}</strong>
              <a-tag v-if="item.official" color="blue">{{ t('plugins.store.sources.official') }}</a-tag>
              <a-tag v-else>{{ t('plugins.store.sources.custom') }}</a-tag>
              <a-tag :color="item.cached ? 'success' : 'default'">
                {{ item.cached ? t('plugins.store.sources.cached') : t('plugins.store.sources.notCached') }}
              </a-tag>
            </div>
            <code>{{ item.url }}</code>
          </div>
          <div v-if="!item.official" class="source-row-actions">
            <a-button type="text" @click="openSourceEditor(item.id)">
              <template #icon><EditOutlined /></template>
              {{ t('plugins.store.sources.edit') }}
            </a-button>
            <a-popconfirm :title="t('plugins.store.sources.removeConfirm')" @confirm="removeSource(item.id)">
              <a-button type="text" danger>
                <template #icon><DeleteOutlined /></template>
                {{ t('plugins.store.sources.remove') }}
              </a-button>
            </a-popconfirm>
          </div>
        </div>
      </div>
    </a-modal>

    <a-modal
      v-model:open="sourceEditorOpen"
      :title="editingSourceId ? t('plugins.store.sources.edit') : t('plugins.store.sources.add')"
      :confirm-loading="sourceSaving"
      :ok-text="t('plugins.store.sources.save')"
      :cancel-text="t('shell.cancel')"
      :ok-button-props="{ disabled: !sourceForm.name.trim() || !sourceForm.url.trim() }"
      @ok="saveSource"
    >
      <a-form layout="vertical">
        <a-form-item :label="t('plugins.store.sources.name')">
          <a-input v-model:value="sourceForm.name" :maxlength="120" />
        </a-form-item>
        <a-form-item :label="t('plugins.store.sources.url')">
          <a-input v-model:value="sourceForm.url" placeholder="https://example.com/catalog.json" />
        </a-form-item>
      </a-form>
    </a-modal>
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

.store-actions .ant-tag { margin: 0; }

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

.store-plugin-card :deep(.ant-card-body) {
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

.plugin-mark {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  overflow: hidden;
  place-items: center;
  border-radius: var(--radius-lg);
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface));
  font-weight: 700;
}

.plugin-mark img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

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

.permission-list .ant-tag { margin: 0; }

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

.source-title .ant-tag { margin: 0; }

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
