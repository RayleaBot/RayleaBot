<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDownIcon } from '@lucide/vue'
import AppTag from '@/components/AppTag.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import ManagementLogDetailDrawer from './ManagementLogDetailDrawer.vue'
import ManagementLogFilters from './ManagementLogFilters.vue'
import ManagementLogRow from './ManagementLogRow.vue'
import { useLogWorkspace, type LogViewport, type LogWorkspaceScope } from './useLogWorkspace'
import { managementTimeZone } from '@/lib/format'
import { t } from '@/i18n'

const props = defineProps<{ scope: LogWorkspaceScope }>()
const history = props.scope === 'history'
const labelPrefix = history ? 'logs.history' : 'logs.current'
const viewportRef = ref<LogViewport | null>(null)
const {
  historyStore, draftFilters, filtersPending, initialized, items, loading, error, detail,
  readyToRenderHeavyContent, atBottom, followBottom, pendingNewCount, showJumpToLatest,
  activatePage, applyFilters, useRecentDays, loadOlder, scrollToLatest,
  openLogDetail, closeLogDetail, onViewportBottomChange,
} = useLogWorkspace(props.scope, viewportRef)
const { currentDetail, error: detailError, loading: detailLoading, open: detailOpen,
  selectedLogId, selectedSummary } = detail
const logsLayoutRef = ref<HTMLElement | null>(null)
const recentRangeOptions = [
  { value: '1', label: t('logs.history.lastDay') },
  { value: '7', label: t('logs.history.lastWeek') },
  { value: '30', label: t('logs.history.lastMonth') },
  { value: '180', label: t('logs.history.lastHalfYear') },
]
// No segment is selected while the list uses a hand-edited range.
const recentRange = computed(() => historyStore?.recentDays ? String(historyStore.recentDays) : '')
</script>

<template>
  <AppPage :title="t(history ? 'logs.historyTitle' : 'logs.currentTitle')" :description="t(`${labelPrefix}.description`)" full-height>
    <template #toolbar>
      <AppCard borderless class="app-view-card logs-toolbar">
        <ManagementLogFilters v-model="draftFilters" :history="history" :pending="filtersPending" @apply="applyFilters">
          <template v-if="historyStore" #fields>
            <!-- The time zone sits in the label, so every field keeps the same height and the actions stay on the control row. -->
            <AppField :label="t('logs.history.startAt')">
              <template #label>{{ t('logs.history.startAt') }} <span class="logs-toolbar__zone">{{ managementTimeZone() }}</span></template>
              <AppInput v-model="historyStore.timeRangeInput.startLocal" type="datetime-local" />
            </AppField>
            <AppField :label="t('logs.history.endAt')">
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

          <div v-if="showJumpToLatest" class="logs-jump-latest">
            <div class="logs-jump-latest__control">
              <AppTooltip
                :title="pendingNewCount > 0 ? t('logs.current.pendingNew', { count: pendingNewCount }) : t('logs.current.jumpToLatest')"
              >
                <AppButton
                  variant="default"
                  size="icon"
                  class="logs-jump-latest__button"
                  :aria-label="t('logs.current.jumpToLatest')"
                  @click="scrollToLatest"
                >
                  <template #icon>
                    <ChevronDownIcon />
                  </template>
                  <span v-if="pendingNewCount" class="logs-jump-latest__count">{{ pendingNewCount }}</span>
                </AppButton>
              </AppTooltip>
            </div>
          </div>
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

.logs-jump-latest {
  position: absolute;
  right: 18px;
  bottom: 18px;
  z-index: 2;
  display: flex;
  justify-content: flex-end;
  pointer-events: none;
}

.logs-jump-latest__control { pointer-events: auto; }
.logs-jump-latest__button { box-shadow: var(--shadow-floating); }
// The count of unread entries is information, not an error: a white badge on the blue button.
.logs-jump-latest__count {
  position: absolute;
  top: -8px;
  right: -8px;
  min-width: 22px;
  padding: 2px 5px;
  border-radius: 12px;
  background: var(--surface-raised);
  color: var(--brand-foreground);
  box-shadow: var(--shadow-xs);
  font-size: 12px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.logs-layout.has-detail-window .logs-jump-latest { right: max(18px, calc(50% - 12px)); }
</style>
