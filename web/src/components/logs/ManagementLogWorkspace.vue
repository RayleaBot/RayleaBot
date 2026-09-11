<script setup lang="ts">
import { ref, useId } from 'vue'
import { ChevronDownIcon, FilterIcon } from '@lucide/vue'
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
  historyStore, filters, initialized, items, loading, error, detail,
  readyToRenderHeavyContent, atBottom, followBottom, pendingNewCount, showJumpToLatest,
  activatePage, applyFilters, useRecentDays, loadOlder, scrollToLatest,
  openLogDetail, closeLogDetail, onViewportBottomChange,
} = useLogWorkspace(props.scope, viewportRef)
const { currentDetail, error: detailError, loading: detailLoading, open: detailOpen,
  selectedLogId, selectedSummary } = detail
const logsLayoutRef = ref<HTMLElement | null>(null)
const filtersExpanded = ref(false)
const filterPanelRef = ref<HTMLElement | null>(null)
const filterToggleRef = ref<HTMLButtonElement | null>(null)
const filterPanelId = useId()
const recentRanges = [
  { days: 1, label: 'logs.history.lastDay' },
  { days: 7, label: 'logs.history.lastWeek' },
  { days: 30, label: 'logs.history.lastMonth' },
  { days: 180, label: 'logs.history.lastHalfYear' },
]

function collapseMobileFilters() {
  if (!filterToggleRef.value?.getClientRects().length) return
  const restoreFocus = filterPanelRef.value?.contains(document.activeElement)
  filtersExpanded.value = false
  if (restoreFocus) filterToggleRef.value?.focus()
}

async function submitFilters() {
  if (await applyFilters()) collapseMobileFilters()
}

async function selectRecentRange(days: number) {
  if (await useRecentDays(days)) collapseMobileFilters()
}
</script>

<template>
  <AppPage :title="t(history ? 'logs.historyTitle' : 'logs.currentTitle')" full-height>
    <template #toolbar>
      <AppCard borderless class="app-view-card logs-toolbar" :class="{ 'logs-toolbar--history': history }">
        <button
          v-if="history"
          ref="filterToggleRef"
          type="button"
          class="logs-filter-toggle"
          :aria-expanded="filtersExpanded"
          :aria-controls="filterPanelId"
          @click="filtersExpanded = !filtersExpanded"
        >
          <FilterIcon aria-hidden="true" />
          <span>{{ t('logs.filters.panel') }}</span>
          <ChevronDownIcon class="logs-filter-toggle__chevron" :class="{ 'is-expanded': filtersExpanded }" aria-hidden="true" />
        </button>
        <div :id="filterPanelId" ref="filterPanelRef" :class="{ 'logs-filter-panel': history, 'is-expanded': filtersExpanded }">
          <ManagementLogFilters v-model="filters" :history="history" @apply="submitFilters">
            <template v-if="historyStore" #fields>
              <AppField :label="t('logs.history.startAt')" :hint="managementTimeZone()">
                <AppInput v-model="historyStore.timeRangeInput.startLocal" type="datetime-local" />
              </AppField>
              <AppField :label="t('logs.history.endAt')" :hint="managementTimeZone()">
                <AppInput v-model="historyStore.timeRangeInput.endLocal" type="datetime-local" />
              </AppField>
            </template>
            <template v-if="history" #actions>
              <AppButton v-for="range in recentRanges" :key="range.days" @click="selectRecentRange(range.days)">
                {{ t(range.label) }}
              </AppButton>
            </template>
          </ManagementLogFilters>
        </div>
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
            <AppTag :tone="!history && atBottom ? 'success' : 'neutral'">
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
            :item-height="80"
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
@use '@/styles/breakpoints.generated' as bp;
.logs-layout {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  gap: 12px;
  overflow: hidden;
}

.logs-toolbar {
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  box-shadow: none;
}

.logs-toolbar :deep(.app-card__body) {
  padding: 12px 14px;
}

.logs-filter-toggle { display: none; }

.logs-feed-card,
.logs-feed-card :deep(.app-card__body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-feed-card {
  box-shadow: none;
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

@media (max-width: #{bp.$compactStore}) {
  .logs-toolbar--history :deep(.app-card__body) { padding: 6px 12px; }
  .logs-filter-toggle {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-height: 44px;
    padding: 0 2px;
    border: 0;
    background: transparent;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }
  .logs-filter-toggle__chevron { margin-inline-start: auto; color: var(--muted); }
  .logs-filter-toggle__chevron.is-expanded { transform: rotate(180deg); }
  .logs-filter-panel { display: none; }
  .logs-filter-panel.is-expanded { display: block; max-height: 55dvh; overflow: auto; padding: 8px 2px 10px; }

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
.logs-jump-latest__button { box-shadow: 0 14px 30px color-mix(in srgb, var(--accent) 24%, transparent); }
.logs-jump-latest__count {
  position: absolute;
  top: -8px;
  right: -8px;
  min-width: 22px;
  padding: 2px 5px;
  border-radius: 12px;
  background: var(--surface-danger);
  color: var(--text-danger);
  font-size: 12px;
}

@media (max-width: #{bp.$compactStore}) {
  .logs-jump-latest { right: 14px; bottom: 14px; }
}

@media (min-width: #{bp.$protocolPanel + 1px}) {
  .logs-layout.has-detail-window .logs-jump-latest { right: max(18px, calc(50% - 12px)); }
}
</style>
