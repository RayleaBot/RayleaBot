<script setup lang="ts">
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { SearchIcon, PlusIcon } from '@lucide/vue'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppTableToolbar from '@/components/AppTableToolbar.vue'
import PluginCard from '@/components/plugins/PluginCard.vue'
import PluginSummaryDrawer from '@/components/plugins/PluginSummaryDrawer.vue'
import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildPluginDetailLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { usePluginsStore } from '@/stores/plugins'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import PluginInstallDialog from './PluginInstallDialog.vue'

const navigate = useMotionNavigation()
const pluginsStore = usePluginsStore()
const { actionPending, error, loading, sortedItems, total, nextCursor, loadingMore } = storeToRefs(pluginsStore)
const installDialogVisible = ref(false)
const summaryPluginId = ref<string | null>(null)
const summaryDrawerVisible = ref(false)

const searchQuery = ref('')
const filterState = ref<'all' | 'running' | 'disabled' | 'alert'>('all')
const filterSource = ref<'all' | 'official' | 'community'>('all')
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

// The retry panel already explains a failed first load; the toast only reports a failed refresh over shown plugins.
useToastFeedback(computed(() => (
  error.value && sortedItems.value.length > 0
    ? {
        key: `plugins-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
)))

// Filtering happens on the server; every filter change reloads from the first page.
const listQuery = computed(() => ({
  query: searchQuery.value,
  state: filterState.value === 'all' ? undefined : filterState.value,
  source: filterSource.value === 'all' ? undefined : filterSource.value,
}))
watch(listQuery, () => { void loadPlugins() })

const summaryPlugin = computed(() => sortedItems.value.find((item) => item.id === summaryPluginId.value) ?? null)
// With a search or filter in effect an empty result means "no match", not "nothing installed".
const filtersActive = computed(() => Boolean(searchQuery.value.trim()) || filterState.value !== 'all' || filterSource.value !== 'all')

function clearFilters() {
  searchQuery.value = ''
  filterState.value = 'all'
  filterSource.value = 'all'
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

function openSummary(id: string) {
  summaryPluginId.value = id
  summaryDrawerVisible.value = true
}

async function runPluginAction(pluginId: string, action: 'enable' | 'disable' | 'reload') {
  try {
    await pluginsStore.executeAction(pluginId, action)
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
      <AppTableToolbar class="plugins-toolbar">
        <template #left>
          <div class="toolbar-filters">
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
          <AppButton variant="default" @click="installDialogVisible = true">
            <template #icon><PlusIcon /></template>
            {{ t('plugins.install') }}
          </AppButton>
        </template>
      </AppTableToolbar>

      <div class="plugins-grid-container">
        <div v-if="loading && sortedItems.length === 0" class="plugins-grid" aria-hidden="true">
          <AppSkeletonCard v-for="index in 8" :key="index" show-header :rows="2" />
        </div>
        <AppEmptyState
          v-else-if="sortedItems.length === 0 && filtersActive"
          icon="search"
          :title="t('plugins.empty.filteredTitle')"
          :description="t('plugins.empty.filteredDescription')"
          :action-label="t('plugins.empty.clearFilters')"
          @action="clearFilters"
        />
        <AppEmptyState
          v-else-if="sortedItems.length === 0"
          icon="plugin"
          :title="t('plugins.empty.title')"
          :description="t('plugins.empty.description')"
          :action-label="t('plugins.install')"
          @action="installDialogVisible = true"
        />

        <div v-else class="plugins-grid" :aria-label="t('plugins.title')">
          <PluginCard
            v-for="item in sortedItems"
            :key="item.id"
            :plugin="item"
            :pending-action="actionPending[item.id]"
            @detail="navigate(buildPluginDetailLocation(item.id))"
            @summary="openSummary(item.id)"
            @manage="navigate(buildPluginDetailLocation(item.id, { panel: 'management-ui' }))"
            @reload="runPluginAction(item.id, 'reload')"
            @toggle="runPluginAction(item.id, item.state === 'disabled' ? 'enable' : 'disable')"
          />
        </div>
      </div>
      <AppCollectionPagination :loaded="sortedItems.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore || loading" @more="pluginsStore.loadMore().catch(() => undefined)" />
    </div>

    <PluginInstallDialog v-model:open="installDialogVisible" />

    <PluginSummaryDrawer :open="summaryDrawerVisible" :plugin="summaryPlugin" @close="summaryDrawerVisible = false" />
  </AppPage>
</template>

<style lang="scss" scoped>
@use '@/styles/breakpoints.generated' as bp;
.plugins-page-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  flex: 1 1 auto;
  min-height: 0;
}

// Filters sit directly on the page above the grid of plugin boxes.
.plugins-toolbar {
  padding: 0;
  border-bottom: 0;
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.filter-search {
  width: 260px;

  .search-icon {
    width: 16px;
    height: 16px;
    color: var(--muted);
  }
}

.filter-radio-group { flex-shrink: 0; }

.filter-select {
  width: 140px;
}

.plugins-page-content :deep(.collection-pagination) {
  padding-inline: 4px;
}

.plugins-grid-container {
  min-height: 220px;
}

.plugins-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: var(--space-lg);
  align-items: stretch;
}

@media (min-width: #{bp.$fiveColumns}) { .plugins-grid { grid-template-columns: repeat(5, minmax(0, 1fr)); } }
</style>
