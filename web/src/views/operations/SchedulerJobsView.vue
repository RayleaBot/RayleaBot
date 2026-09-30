<script setup lang="ts">
import { SearchIcon } from '@lucide/vue'

import { computed } from 'vue'
import AppCard from '@/components/AppCard.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppInput from '@/components/AppInput.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import SchedulerJobDetailDialog from './SchedulerJobDetailDialog.vue'
import SchedulerJobTable from './SchedulerJobTable.vue'
import { useSchedulerJobDetail } from './useSchedulerJobDetail'
import { useSchedulerJobsPage } from './useSchedulerJobsPage'

const {
  schedulerStore, pluginName, now,
  error, loading, sortedItems, triggeringJobId, total, nextCursor, loadingMore,
  searchQuery, statusFilter, sortBy, loadSchedulerJobs, triggerJob, getNextRunRelativeText,
} = useSchedulerJobsPage()

// With a search or a status filter in effect, an empty list means "no match", not "no jobs".
const filtersActive = computed(() => Boolean(searchQuery.value.trim()) || statusFilter.value !== 'all')
function clearFilters() {
  searchQuery.value = ''
  statusFilter.value = 'all'
}

const {
  closeJobDetail,
  currentJob,
  detailVisible,
  finishJobDetailClose,
  showJobDetail,
} = useSchedulerJobDetail()
</script>

<template>
  <AppPage :title="t('scheduler.title')" :description="t('scheduler.description')">
    <div class="scheduler-page-container">
      <div class="scheduler-filter-card app-box">
        <div class="filter-left">
          <AppInput
            v-model="searchQuery"
            :maxlength="200"
            :placeholder="t('scheduler.searchPlaceholder')"
            allow-clear
            wrapper-class="filter-search-input"
            :aria-label="t('scheduler.searchLabel')"
          >
            <template #prefix>
              <SearchIcon class="search-icon" />
            </template>
          </AppInput>

          <AppSegmented
            v-model="statusFilter"
            :options="[
              { label: t('scheduler.filterAll'), value: 'all' },
              { label: t('scheduler.healthy'), value: 'success' },
              { label: t('scheduler.filterError'), value: 'error' },
            ]"
            class="filter-segmented"
            :aria-label="t('scheduler.filterLabel')"
          />
        </div>

        <div class="filter-right">
          <div class="sort-wrapper">
            <span class="sort-label">{{ t('scheduler.sortCaption') }}</span>
            <AppSelect v-model="sortBy" wrapper-class="sort-select" :aria-label="t('scheduler.sortLabel')" :options="[
              { value: 'name', label: t('scheduler.sortName') },
              { value: 'last_run', label: t('scheduler.sortLastRun') },
              { value: 'duration', label: t('scheduler.sortDuration') },
            ]" />
          </div>
        </div>
      </div>

      <div class="scheduler-content-stage">
        <RetryPanel
          v-if="error && sortedItems.length === 0"
          :title="t('errors.common.loadFailed')"
          :description="error"
          :loading="loading"
          @retry="loadSchedulerJobs"
        />

        <AppCard v-else-if="loading && sortedItems.length === 0">
          <AppSkeleton :rows="6" />
        </AppCard>

        <AppEmptyState
          v-else-if="sortedItems.length === 0 && filtersActive"
          icon="search"
          :title="t('scheduler.empty.filteredTitle')"
          :description="t('scheduler.empty.filteredDescription')"
          :action-label="t('scheduler.empty.clearFilters')"
          @action="clearFilters"
        />

        <AppEmptyState
          v-else-if="sortedItems.length === 0"
          icon="box"
          :title="t('scheduler.empty.title')"
          :description="t('scheduler.empty.description')"
        />

        <SchedulerJobTable
          v-else
          :jobs="sortedItems"
          :now="now"
          :triggering-job-id="triggeringJobId"
          @view="showJobDetail"
          @trigger="triggerJob"
        />
      </div>
    </div>

    <AppCollectionPagination :loaded="sortedItems.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore || loading" @more="schedulerStore.loadMore().catch(() => undefined)" />

    <SchedulerJobDetailDialog
      :open="detailVisible"
      :job="currentJob"
      :plugin-name="currentJob ? pluginName(currentJob) : ''"
      :next-run-text="currentJob?.next_run ? getNextRunRelativeText(currentJob.next_run) : ''"
      @close="closeJobDetail"
      @after-close="finishJobDetailClose"
    />
  </AppPage>
</template>

<style lang="scss" scoped>
.lucide { width: 16px; height: 16px; flex-shrink: 0; }
.scheduler-page-container {
  display: grid;
  gap: var(--space-md);
}

.scheduler-content-stage {
  display: grid;
}

.scheduler-filter-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-md);
  padding: 14px 18px;

  .filter-left,
  .filter-right {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    flex-wrap: wrap;
  }

  .filter-search-input {
    width: 300px;

    .search-icon {
      color: var(--muted);
    }
  }

  .sort-wrapper {
    display: flex;
    align-items: center;
    gap: var(--space-sm);

    .sort-label {
      font-size: 13px;
      color: var(--muted);
      white-space: nowrap;
    }

    // The select sizes to its longest option; a fixed width wrapped "按任务字母排序" onto two lines.
    .sort-select {
      width: auto;
      min-width: 150px;
      font-size: 13px;
      font-weight: 500;
      color: var(--text);
    }

    .sort-select :deep(.app-select__value) {
      white-space: nowrap;
    }
  }
}
</style>
