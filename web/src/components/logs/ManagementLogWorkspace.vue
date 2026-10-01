<script setup lang="ts">
import { computed, ref } from 'vue'
import AppTag from '@/components/AppTag.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppCard from '@/components/AppCard.vue'
import AppJumpToLatest from '@/components/AppJumpToLatest.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import ManagementLogDetailDrawer from './ManagementLogDetailDrawer.vue'
import ManagementLogFilters from './ManagementLogFilters.vue'
import ManagementLogRow from './ManagementLogRow.vue'
import { useLogWorkspace, type LogViewport, type LogWorkspaceScope } from './useLogWorkspace'
import { managementTimeZone } from '@/lib/format'
import { useConfigStore } from '@/stores/config'
import { t } from '@/i18n'

const props = defineProps<{ scope: LogWorkspaceScope }>()
const history = props.scope === 'history'
const labelPrefix = history ? 'logs.history' : 'logs.current'
const viewportRef = ref<LogViewport | null>(null)
const {
  historyStore, draftFilters, timeRangeIssue, initialized, items, loading, error, detail,
  readyToRenderHeavyContent, atBottom, followBottom, pendingNewCount, showJumpToLatest,
  activatePage, useRecentDays, loadOlder, scrollToLatest,
  openLogDetail, closeLogDetail, onViewportBottomChange,
} = useLogWorkspace(props.scope, viewportRef)
const { currentDetail, error: detailError, loading: detailLoading, open: detailOpen,
  selectedLogId, selectedSummary } = detail
const logsLayoutRef = ref<HTMLElement | null>(null)
const configStore = useConfigStore()
const presetRanges = [
  { days: 1, label: t('logs.history.lastDay') },
  { days: 7, label: t('logs.history.lastWeek') },
  { days: 30, label: t('logs.history.lastMonth') },
  { days: 180, label: t('logs.history.lastHalfYear') },
]
// Logs older than the retention period are pruned, so no shortcut reaches past it; a retention that is not a preset
// gets its own shortcut, keeping every stored log one click away.
const recentRangeOptions = computed(() => {
  const retention = configStore.logRetentionDays
  const ranges = retention ? presetRanges.filter(range => range.days <= retention) : presetRanges
  const options = retention && !ranges.some(range => range.days === retention)
    ? [...ranges, { days: retention, label: t('logs.history.lastDays', { days: retention }) }]
    : ranges
  return options.map(range => ({ value: String(range.days), label: range.label }))
})
const pageDescription = computed(() => history && configStore.logRetentionDays
  ? t('logs.history.descriptionWithRetention', { days: configStore.logRetentionDays })
  : t(`${labelPrefix}.description`))
// No segment is selected while the list uses a hand-edited range.
const recentRange = computed(() => historyStore?.recentDays ? String(historyStore.recentDays) : '')
</script>

<template>
  <AppPage :title="t(history ? 'logs.historyTitle' : 'logs.currentTitle')" :description="pageDescription" full-height>
    <template #toolbar>
      <AppCard borderless class="app-view-card logs-toolbar">
        <ManagementLogFilters v-model="draftFilters" :history="history">
          <template v-if="historyStore" #fields>
            <!-- The time zone sits in the label, so every field keeps the same height and the actions stay on the control row.
                 A range that cannot be queried is explained under the field to fix and is not applied. -->
            <AppField class="logs-toolbar__time" :label="t('logs.history.startAt')" :error="timeRangeIssue?.field === 'start' ? timeRangeIssue.message : undefined">
              <template #label>{{ t('logs.history.startAt') }} <span class="logs-toolbar__zone">{{ managementTimeZone() }}</span></template>
              <AppInput v-model="historyStore.timeRangeInput.startLocal" type="datetime-local" />
            </AppField>
            <AppField class="logs-toolbar__time" :label="t('logs.history.endAt')" :error="timeRangeIssue?.field === 'end' ? timeRangeIssue.message : undefined">
              <template #label>{{ t('logs.history.endAt') }} <span class="logs-toolbar__zone">{{ managementTimeZone() }}</span></template>
              <AppInput v-model="historyStore.timeRangeInput.endLocal" type="datetime-local" />
            </AppField>
          </template>
          <template v-if="history" #actions>
            <!-- Looks like a segmented control but stays buttons: choosing the current range again re-anchors it to now. -->
            <div class="logs-range-group" role="group" :aria-label="t('logs.history.quickRange')">
              <button
                v-for="range in recentRangeOptions"
                :key="range.value"
                type="button"
                class="logs-range-group__item"
                :aria-pressed="recentRange === range.value"
                @click="useRecentDays(Number(range.value))"
              >
                {{ range.label }}
              </button>
            </div>
          </template>
        </ManagementLogFilters>
      </AppCard>
    </template>

    <RetryPanel
      v-if="error && !initialized"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="activatePage"
    />

    <section
      v-else
      ref="logsLayoutRef"
      class="logs-layout"
      :class="{ 'has-detail-window': detailOpen }"
    >
      <AppCard borderless class="logs-feed-card">
        <template #title>
          <div class="logs-feed-card__title">
            <span>{{ t(`${labelPrefix}.streamTitle`) }}</span>
            <!-- Following is a mode of the list, not a health state, so it stays neutral like the paused state. -->
            <AppTag>
              {{ t(history ? 'logs.history.frozen' : atBottom ? 'logs.current.following' : 'logs.current.paused') }}
            </AppTag>
          </div>
        </template>

        <div class="logs-feed-card__body">
          <AppSkeleton
            v-if="!readyToRenderHeavyContent"
            :rows="6"
          />
          <VirtualDataViewport
            v-else
            ref="viewportRef"
            :items="items"
            :item-height="44"
            :dynamic-item-height="true"
            :overscan="6"
            :follow-bottom="followBottom"
            :bottom-threshold="24"
            :empty-label="t(`${labelPrefix}.empty`)"
            :get-item-key="(item) => item.log_id"
            @reach-top="loadOlder"
            @at-bottom-change="onViewportBottomChange"
          >
            <template #default="{ item }">
              <ManagementLogRow :item="item" :selected="selectedLogId === item.log_id" @select="openLogDetail" />
            </template>
          </VirtualDataViewport>

          <AppJumpToLatest
            v-if="showJumpToLatest"
            class="logs-jump-latest"
            :label="t('logs.current.jumpToLatest')"
            :pending-label="t('logs.current.pendingNew', { count: pendingNewCount })"
            :count="pendingNewCount"
            @jump="scrollToLatest"
          />
        </div>
      </AppCard>

      <ManagementLogDetailDrawer
        :open="detailOpen"
        :loading="detailLoading"
        :error="detailError"
        :summary="selectedSummary"
        :detail="currentDetail"
        :memory-key="history ? 'logs-history' : 'logs-current'"
        :scope="props.scope"
        :host-element="logsLayoutRef"
        @close="closeLogDetail"
      />
    </section>
  </AppPage>
</template>

<style lang="scss" scoped>
.logs-layout {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  gap: 12px;
  overflow: hidden;
}

.logs-toolbar :deep(.app-card__body) {
  padding: 12px 14px;
}

.logs-toolbar__zone {
  color: var(--muted);
  font-size: 12px;
  font-weight: 400;
}

.logs-toolbar__time {
  width: 220px;
}

.logs-range-group {
  display: flex;
  gap: 2px;
  padding: 3px;
  border-radius: 999px;
  background: var(--surface-soft);
}

.logs-range-group__item {
  min-height: 34px;
  padding: 6px 14px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  font-size: 13px;
  white-space: nowrap;
  cursor: pointer;
  transition: background-color var(--motion-fast), color var(--motion-fast);
}

.logs-range-group__item:hover {
  color: var(--text);
}

.logs-range-group__item[aria-pressed=true] {
  background: var(--surface-raised);
  color: var(--text);
  font-weight: 700;
  box-shadow: var(--shadow-xs);
}

.logs-range-group__item:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

@media (pointer: coarse) {
  .logs-range-group__item { min-height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .logs-range-group__item { transition: none; }
}

@media (forced-colors: active) {
  .logs-range-group { border: 1px solid CanvasText; }
  .logs-range-group__item[aria-pressed=true] { outline: 2px solid Highlight; outline-offset: -2px; }
}

.logs-feed-card,
.logs-feed-card :deep(.app-card__body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-feed-card :deep(.data-viewport) {
  border: 0;
  border-radius: 0;
}

.logs-feed-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logs-feed-card__body {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

.logs-layout.has-detail-window .logs-jump-latest { right: max(18px, calc(50% - 12px)); }
</style>
