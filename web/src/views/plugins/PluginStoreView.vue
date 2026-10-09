<script setup lang="ts">
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppTag from '@/components/AppTag.vue'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import {
  Trash2Icon,
  PencilIcon,
  ArrowUpRightIcon,
  ExternalLinkIcon,
  PlusIcon,
  SearchIcon,
  RotateCwIcon,
  SettingsIcon,
  RefreshCwIcon,
  CircleCheckIcon,
  PuzzleIcon,
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
import { buildPluginDetailLocation } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginStore, type PluginStoreSort } from '@/stores/plugin-store'
import type {
  PluginStoreDependency,
  PluginStoreEntry,
  PluginStoreSourceInput,
} from '@/types/api'
import PluginStoreInstallDialog from './PluginStoreInstallDialog.vue'
import { missingRequiredDependencies, pendingDependencies } from './plugin-store-dependencies'

const store = usePluginStore()
const pluginsStore = usePluginsStore()
const navigate = useMotionNavigation()
const { error, installing, items, loading, loadingMore, nextCursor, refreshing, source, sourceSaving, sources, total, sourcesTotal, sourcesNextCursor, sourcesLoadingMore, sourcesLoading, sourcesError } = storeToRefs(store)

const query = ref('')
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(query, () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { if (pageActive) void loadEntries() }, 300)
})
const sourceQuery = ref('')
watch(sourceQuery, () => { void store.fetchSources({ query: sourceQuery.value }).catch(() => undefined) })
const sort = ref<PluginStoreSort>('recommended')
const sourceId = ref('official')
const confirmationOpen = ref(false)
const selectedPlugin = ref<PluginStoreEntry | null>(null)
const sourceManagerOpen = ref(false)
const pendingSourceRemoval = ref<string | null>(null)
const sourceRemoving = ref(false)
const sourceOptions = computed(() => [...new Map([...sources.value, ...(source.value ? [source.value] : [])].map(item => [item.id, { value: item.id, label: item.name }])).values()])
const sortOptions = computed(() => [
  { value: 'recommended', label: t('plugins.store.sort.recommended') },
  { value: 'name', label: t('plugins.store.sort.name') },
  { value: 'updated', label: t('plugins.store.sort.updated') },
])
const sourceEditorOpen = ref(false)
const editingSourceId = ref<string | null>(null)
const batchUpdating = ref(false)
// Progress of an install that runs its prerequisite plugins first, keyed by the plugin the operator asked for.
const installPlans = ref<Record<string, { step: number; total: number }>>({})
const resolvingDependency = ref<string | null>(null)
const sourceForm = reactive<PluginStoreSourceInput>({ name: '', url: '' })
let pageActive = true
let activated = false
let refreshTimer: ReturnType<typeof setTimeout> | undefined

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
    await store.fetchSources({ query: sourceQuery.value })
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
  if (plugin.confirmation_reasons.length > 0 || pendingDependencies(plugin).length > 0) {
    selectedPlugin.value = plugin
    confirmationOpen.value = true
    return
  }
  try {
    await installEntry(plugin, false)
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  }
}

async function installEntry(plugin: PluginStoreEntry, trustedCodeConfirmed: boolean) {
  await store.install(plugin.id, {
    source_id: sourceId.value,
    trusted_code_confirmed: trustedCodeConfirmed,
  })
  notifySuccess(t('plugins.store.feedback.accepted'))
}

function confirmInstall(dependencyIds: string[]) {
  const plugin = selectedPlugin.value
  if (!plugin) return
  closeConfirmation()
  void runInstallPlan(plugin, dependencyIds)
}

// Prerequisites go first, one at a time, so the server sees each of them installed before the plugin that needs
// them; a failed prerequisite stops the plan before the plugin itself.
async function runInstallPlan(plugin: PluginStoreEntry, dependencyIds: string[]) {
  const dependencies = pendingDependencies(plugin).filter(dependency => dependencyIds.includes(dependency.id))
  const total = dependencies.length + 1
  const setStep = (step: number) => { installPlans.value = { ...installPlans.value, [plugin.id]: { step, total } } }
  try {
    for (const [index, dependency] of dependencies.entries()) {
      setStep(index + 1)
      try {
        await store.install(dependency.id, { source_id: sourceId.value, trusted_code_confirmed: true })
      } catch (cause) {
        notifyError(t('plugins.store.feedback.dependencyFailed', { name: dependency.name, plugin: plugin.name, reason: getDisplayErrorMessage(cause) }))
        return
      }
    }
    setStep(total)
    await store.install(plugin.id, { source_id: sourceId.value, trusted_code_confirmed: plugin.confirmation_reasons.length > 0 })
    notifySuccess(dependencies.length > 0
      ? t('plugins.store.feedback.installedWithDependencies', { names: [...dependencies.map(dependency => dependency.name), plugin.name].join('、') })
      : t('plugins.store.feedback.accepted'))
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  } finally {
    const { [plugin.id]: _finished, ...rest } = installPlans.value
    installPlans.value = rest
  }
}

// An installed plugin can still point to a prerequisite it suggests; installing it starts from that plugin's own
// store entry so its confirmation and dependencies are handled as usual.
async function installDependency(dependency: PluginStoreDependency) {
  if (resolvingDependency.value) return
  resolvingDependency.value = dependency.id
  try {
    const detail = await store.fetchDetail(dependency.id, sourceId.value)
    await requestInstall(detail.plugin)
  } catch (cause) {
    notifyError(getDisplayErrorMessage(cause))
  } finally {
    resolvingDependency.value = null
  }
}

function dependencyText(dependency: PluginStoreDependency) {
  const installed = dependency.state === 'installed'
  const key = dependency.requirement === 'required'
    ? installed ? 'requiredInstalled' : 'required'
    : installed ? 'recommendedInstalled' : 'recommended'
  return t(`plugins.store.dependencies.${key}`, { name: dependency.name })
}

function closeConfirmation() { confirmationOpen.value = false }
function resetConfirmation() {
  selectedPlugin.value = null
}

async function updateAll() {
  if (batchUpdating.value || updateablePlugins.value.length === 0) return
  batchUpdating.value = true
  let accepted = 0
  let skipped = 0
  try {
    for (const plugin of updateablePlugins.value) {
      if (plugin.confirmation_reasons.length > 0 || missingRequiredDependencies(plugin).length > 0) {
        skipped++
        continue
      }
      try {
        await store.install(plugin.id, {
          source_id: sourceId.value,
          trusted_code_confirmed: false,
        })
        accepted++
      } catch {
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

function cancelSourceEditor() {
  sourceEditorOpen.value = false
  sourceManagerOpen.value = true
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

// A release cannot be installed when RayleaBot's own version is unknown (a source checkout), when that version is too
// old, or when no package fits this platform; each has a different remedy, and upgrading RayleaBot only helps one.
function incompatibilityReason(plugin: PluginStoreEntry) {
  const release = plugin.latest_release
  if (!release) return ''
  if (release.incompatible_reason === 'core_version_unknown') return t('plugins.store.coreVersionUnknown')
  if (!release.compatible) return t('plugins.store.requiresCore', { version: formatPluginVersion(release.min_core_version) })
  if (!release.asset_available) return t('plugins.store.noPlatformAsset')
  return ''
}

// Only a store release plainly newer than the installed one is an update; an unreadable version makes no claim.
function isPlainlyNewer(candidate: string, current: string) {
  const parse = (version: string) => /^v?(\d+)\.(\d+)\.(\d+)$/.exec(version.trim())?.slice(1).map(Number)
  const next = parse(candidate)
  const installed = parse(current)
  if (!next || !installed) return false
  for (let index = 0; index < 3; index += 1) {
    if (next[index] !== installed[index]) return next[index]! > installed[index]!
  }
  return false
}

// An installed plugin whose newer store release cannot be installed shows no update button, so it says why.
function blockedUpdateReason(plugin: PluginStoreEntry) {
  const release = plugin.latest_release
  if (plugin.install_state !== 'installed' || !release || !plugin.installed_version) return ''
  if (!isPlainlyNewer(release.version, plugin.installed_version)) return ''
  const reason = incompatibilityReason(plugin)
  return reason ? t('plugins.store.updateBlocked', { reason }) : ''
}

function installActionLabel(plugin: PluginStoreEntry) {
  const plan = installPlans.value[plugin.id]
  if (plan && plan.total > 1) return t('plugins.store.actions.installingStep', plan)
  if (installing.value[plugin.id] || plan) return t('plugins.store.actions.installing')
  switch (plugin.install_state) {
    case 'update_available': return t('plugins.store.actions.update')
    case 'installed': return t('plugins.store.actions.installed')
    case 'unpublished': return t('plugins.store.actions.unpublished')
    case 'incompatible': return incompatibilityReason(plugin) || t('plugins.store.actions.incompatible')
    default: return t('plugins.store.actions.install')
  }
}

function canInstall(plugin: PluginStoreEntry) {
  return !installing.value[plugin.id] && !installPlans.value[plugin.id] && (plugin.install_state === 'available' || plugin.install_state === 'update_available')
}

async function loadMore() {
  try { await store.loadMore() } catch (cause) { notifyError(getDisplayErrorMessage(cause)) }
}

watch(() => pluginsStore.items.map(plugin => `${plugin.id}:${plugin.version}:${plugin.state}`).join('|'), () => {
  if (!pageActive || loading.value || Object.values(installing.value).some(Boolean) || Object.keys(installPlans.value).length > 0) return
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => { if (pageActive) void loadEntries() }, 150)
})
onActivated(() => { pageActive = true; if (activated) void loadEntries(); activated = true })
onDeactivated(() => { pageActive = false; clearTimeout(refreshTimer); clearTimeout(searchTimer) })
onBeforeUnmount(() => { pageActive = false; clearTimeout(refreshTimer); clearTimeout(searchTimer) })
onMounted(() => {
  void loadInitialEntries()
})
</script>

<template>
  <AppPage :title="t('plugins.store.title')">
    <template #toolbar>
      <div class="store-toolbar">
        <AppInput v-model="query" :maxlength="200" :placeholder="t('plugins.store.searchPlaceholder')" :aria-label="t('plugins.store.searchPlaceholder')" wrapper-class="store-search" allow-clear>
          <template #prefix><SearchIcon class="store-search__icon" /></template>
        </AppInput>
        <AppSelect v-model="sourceId" :options="sourceOptions" :aria-label="t('plugins.fields.source')" wrapper-class="store-source" @update:model-value="changeSource" />
        <AppSelect v-model="sort" :options="sortOptions" :aria-label="t('plugins.store.sortLabel')" wrapper-class="store-sort" @update:model-value="loadEntries" />
        <div class="store-actions">
          <AppTag v-if="selectedSource">
            {{ selectedSource.official ? t('plugins.store.sources.official') : t('plugins.store.sources.custom') }}
          </AppTag>
          <AppTag v-if="selectedSource && !selectedSource.cached">
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
      <!-- A reload keeps the loaded catalog on screen; the skeleton only stands in while there is nothing to show. -->
      <div v-if="loading && items.length === 0" class="store-grid" aria-hidden="true">
        <AppSkeletonCard v-for="index in 8" :key="index" show-header :rows="2" />
      </div>
      <!-- Without a search the empty list says whether the source was never loaded or is simply empty. -->
      <AppEmptyState
        v-else-if="items.length === 0 && query.trim()"
        :title="t('plugins.store.empty.title')"
        :description="t('plugins.store.empty.description')"
      />
      <AppEmptyState
        v-else-if="items.length === 0 && !source?.cached"
        :title="t('plugins.store.empty.notLoadedTitle')"
        :description="t('plugins.store.empty.notLoadedDescription')"
        :action-label="t('plugins.store.actions.refresh')"
        @action="refreshSource"
      />
      <AppEmptyState
        v-else-if="items.length === 0"
        :title="t('plugins.store.empty.sourceEmptyTitle')"
        :description="t('plugins.store.empty.sourceEmptyDescription')"
      />
      <div v-else class="store-grid">
        <AppCard v-for="plugin in items" :key="plugin.id" class="store-plugin-card" shadow="sm">
          <div class="plugin-card-header">
            <div class="plugin-identity">
              <PluginIcon :plugin-id="plugin.id" :remote-url="plugin.icon_url ?? ''" :source-key="sourceId" :version="plugin.latest_release?.version" :refresh-key="store.iconRevision" />
              <div class="plugin-heading">
                <div class="plugin-title-line">
                  <h2>{{ plugin.name }}</h2>
                  <AppTag v-if="plugin.recommended">{{ t('plugins.store.recommended') }}</AppTag>
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
                size="icon"
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
          <ul v-if="plugin.latest_release?.dependencies.length" class="plugin-dependencies" :aria-label="t('plugins.store.dependencies.label')">
            <li
              v-for="dependency in plugin.latest_release.dependencies"
              :key="dependency.id"
              :data-state="dependency.state"
              :data-required="dependency.requirement === 'required' || undefined"
            >
              <CircleCheckIcon v-if="dependency.state === 'installed'" aria-hidden="true" />
              <PuzzleIcon v-else aria-hidden="true" />
              <span>{{ dependencyText(dependency) }}</span>
              <AppButton
                v-if="plugin.install_state === 'installed' && dependency.state === 'installable'"
                variant="link"
                size="xs"
                :loading="resolvingDependency === dependency.id || installing[dependency.id]"
                :data-testid="`plugin-store-install-dependency-${dependency.id}`"
                @click="installDependency(dependency)"
              >
                {{ t('plugins.store.dependencies.install') }}
              </AppButton>
            </li>
          </ul>
          <p v-if="blockedUpdateReason(plugin)" class="plugin-update-note">{{ blockedUpdateReason(plugin) }}</p>

          <div class="plugin-card-footer">
            <span v-if="plugin.installed_version" class="installed-version">
              {{ t('plugins.store.installedVersion', { version: formatPluginVersion(plugin.installed_version) }) }}
            </span>
            <span v-else />
            <AppButton
              v-if="plugin.install_state === 'installed'"
              :data-testid="`plugin-store-open-${plugin.id}`"
              @click="navigate(buildPluginDetailLocation(plugin.id))"
            >
              <template #icon><ArrowUpRightIcon /></template>
              {{ t('plugins.store.actions.viewInstalled') }}
            </AppButton>
            <AppTag
              v-else-if="plugin.install_state === 'unpublished' || plugin.install_state === 'incompatible'"
              :tone="plugin.install_state === 'incompatible' ? 'warning' : 'neutral'"
            >
              {{ installActionLabel(plugin) }}
            </AppTag>
            <AppButton
              v-else
              variant="default"
              :disabled="!canInstall(plugin)"
              :loading="installing[plugin.id] || Boolean(installPlans[plugin.id])"
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

    <PluginStoreInstallDialog :open="confirmationOpen" :plugin="selectedPlugin" @close="closeConfirmation" @after-close="resetConfirmation" @confirm="confirmInstall" />

    <AppDialog :open="sourceManagerOpen" :title="t('plugins.store.sources.manage')" :width="720" fallback-focus="[data-testid=plugin-store-sources]" @close="sourceManagerOpen = false">
      <div class="source-manager-header">
        <p>{{ t('plugins.store.sources.description') }}</p>
        <AppButton variant="default" @click="openNewSource">
          <template #icon><PlusIcon /></template>
          {{ t('plugins.store.sources.add') }}
        </AppButton>
      </div>
      <AppInput v-model="sourceQuery" :maxlength="200" :aria-label="t('plugins.store.sources.searchLabel')" :placeholder="t('plugins.store.sources.searchPlaceholder')" allow-clear />
      <p v-if="sourcesError" role="alert">{{ sourcesError }}</p>
      <div class="source-list">
        <div v-for="item in sources" :key="item.id" class="source-row">
          <div class="source-copy">
            <div class="source-title">
              <strong>{{ item.name }}</strong>
              <AppTag v-if="item.official">{{ t('plugins.store.sources.official') }}</AppTag>
              <AppTag v-else>{{ t('plugins.store.sources.custom') }}</AppTag>
              <AppTag :tone="item.cached ? 'success' : 'neutral'">
                {{ item.cached ? t('plugins.store.sources.cached', { count: item.entry_count }) : t('plugins.store.sources.notCached') }}
              </AppTag>
              <AppTag v-if="item.id === sourceId" tone="info">{{ t('plugins.store.sources.current') }}</AppTag>
            </div>
            <code :title="item.url">{{ item.url }}</code>
          </div>
          <div class="source-row-actions">
            <AppButton v-if="item.id !== sourceId" @click="sourceId = item.id; sourceManagerOpen = false; changeSource()">{{ t('plugins.store.sources.select') }}</AppButton>
            <template v-if="!item.official">
            <AppButton variant="ghost" @click="openSourceEditor(item.id)">
              <template #icon><PencilIcon /></template>
              {{ t('plugins.store.sources.edit') }}
            </AppButton>

              <AppButton variant="destructive" @click="pendingSourceRemoval = item.id">
                <template #icon><Trash2Icon /></template>
                {{ t('plugins.store.sources.remove') }}
              </AppButton>

            </template>
          </div>
        </div>
      </div>
      <AppCollectionPagination :loaded="sources.length" :total="sourcesTotal" :next-cursor="sourcesNextCursor" :loading="sourcesLoadingMore || sourcesLoading" @more="store.loadMoreSources().catch(() => undefined)" />

    </AppDialog>

    <AppDialog :open="sourceEditorOpen" :title="editingSourceId ? t('plugins.store.sources.editTitle') : t('plugins.store.sources.add')" :busy="sourceSaving" fallback-focus="[data-testid=plugin-store-sources]" @close="cancelSourceEditor">
      <div>
        <AppField floating :label="t('plugins.store.sources.name')">
          <AppInput v-model="sourceForm.name" :maxlength="120" />
        </AppField>
        <AppField floating :label="t('plugins.store.sources.url')" :hint="t('plugins.store.sources.urlHint')">
          <AppInput v-model="sourceForm.url" placeholder="https://example.com/catalog.json" />
        </AppField>
      </div>
    <template #footer><div class="flex justify-end gap-3"><AppButton :disabled="sourceSaving" @click="cancelSourceEditor">{{ t('shell.cancel') }}</AppButton><AppButton variant="default" :loading="sourceSaving" :disabled="!sourceForm.name.trim() || !sourceForm.url.trim()" @click="saveSource">{{ t('plugins.store.sources.save') }}</AppButton></div></template>
    </AppDialog>
    <AppConfirmDialog :open="pendingSourceRemoval !== null" :title="t('plugins.store.sources.removeTitle')" :description="t('plugins.store.sources.removeConfirm')" :busy="sourceRemoving" danger :confirm-text="t('plugins.store.sources.remove')" @confirm="pendingSourceRemoval && removeSource(pendingSourceRemoval)" @cancel="pendingSourceRemoval = null" />
  </AppPage>
</template>

<style scoped lang="scss">
.store-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

:deep(.store-search) { width: min(420px, 100%); }
.store-search__icon { width: 16px; height: 16px; color: var(--muted); }
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
  font-size: 15px;
  font-weight: 700;
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

.plugin-dependencies {
  display: grid;
  gap: 2px;
  margin: 12px 0 0;
  padding: 0;
  color: var(--muted);
  font-size: 12px;
  list-style: none;
}

.plugin-dependencies li {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 24px;
}

.plugin-dependencies svg {
  flex: none;
  width: 14px;
  height: 14px;
}

.plugin-dependencies li[data-required]:not([data-state=installed]) { color: var(--text); }
.plugin-dependencies li[data-state=installed] svg { color: var(--text-success); }

.plugin-update-note {
  margin: 0;
  color: var(--text-warning);
  font-size: var(--font-size-sm);
}

.plugin-card-footer {
  margin-top: auto;
  padding-top: 20px;
}

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
</style>
