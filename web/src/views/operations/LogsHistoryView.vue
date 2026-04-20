<script setup lang="ts">
import { computed, nextTick, onActivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import AppPage from '@/components/page/AppPage.vue'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import {
  areLocationQueriesEqual,
  buildLogsLocation,
  readLogWorkspaceState,
} from '@/lib/management-links'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { t } from '@/i18n'
import { toLocalDateTimeInput, useLogHistoryStore } from '@/stores/log-history'
import type { LogFilters } from '@/stores/log-state'
import type { LogSummary } from '@/types/api'
import { useLogDetailController } from '@/views/operations/useLogDetailController'

const route = useRoute()
const router = useRouter()
const historyStore = useLogHistoryStore()
const detailController = useLogDetailController()
const {
  currentDetail,
  error: detailError,
  loading: detailLoading,
  open: detailOpen,
  selectedLogId,
  selectedSummary,
} = detailController
const logsLayoutRef = ref<HTMLElement | null>(null)
const viewportRef = ref<{
  getScrollMetrics?: () => {
    clientHeight: number
    scrollHeight: number
    scrollTop: number
  }
  scrollToBottom: () => void
} | null>(null)
const autoFollowBottom = ref(false)
const routeSyncing = ref(false)

const {
  error,
  filters,
  hasOlder,
  initialized,
  items,
  loading,
  loadingOlder,
  timeRangeInput,
} = storeToRefs(historyStore)

const levelOptions = computed(() => ([
  { label: t('display.logLevels.debug'), value: 'debug' },
  { label: t('display.logLevels.info'), value: 'info' },
  { label: t('display.logLevels.warn'), value: 'warn' },
  { label: t('display.logLevels.error'), value: 'error' },
]))

function sameLogFilters(left: LogFilters, right: LogFilters) {
  return (left.level ?? '') === (right.level ?? '')
    && (left.source ?? '') === (right.source ?? '')
    && (left.protocol ?? '') === (right.protocol ?? '')
    && (left.pluginId ?? '') === (right.pluginId ?? '')
    && (left.requestId ?? '') === (right.requestId ?? '')
}

function toLocalInput(value: string) {
  if (!value) {
    return ''
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return ''
  }

  return toLocalDateTimeInput(parsed)
}

function currentRouteLogId() {
  return readLogWorkspaceState(route.query, { history: true }).logId
}

function shouldSyncViewportToLatest() {
  return route.name === 'logs-history'
    && items.value.length > 0
    && !currentRouteLogId()
}

async function syncViewportToLatest() {
  if (!shouldSyncViewportToLatest()) {
    return
  }

  autoFollowBottom.value = true
  await nextTick()
  viewportRef.value?.scrollToBottom()
  await nextTick()
  autoFollowBottom.value = false
}

async function replaceRouteState(nextLogId: string | null = selectedLogId.value) {
  const timeRange = historyStore.currentUtcRange()
  const target = buildLogsLocation({
    history: true,
    filters: filters.value,
    logId: nextLogId,
    startAt: timeRange.startAt ?? '',
    endAt: timeRange.endAt ?? '',
  })

  if (areLocationQueriesEqual(route.query, target.query ?? {})) {
    return
  }

  routeSyncing.value = true
  try {
    await router.replace(target)
  } finally {
    routeSyncing.value = false
  }
}

async function syncFromRoute() {
  if (route.name !== 'logs-history') {
    return
  }

  const routeState = readLogWorkspaceState(route.query, { history: true })
  const filtersChanged = !sameLogFilters(filters.value, routeState.filters)
  const nextStartLocal = toLocalInput(routeState.startAt)
  const nextEndLocal = toLocalInput(routeState.endAt)
  const hasExplicitTimeRange = Boolean(routeState.startAt && routeState.endAt)
  const timeRangeChanged = timeRangeInput.value.startLocal !== nextStartLocal
    || timeRangeInput.value.endLocal !== nextEndLocal

  if (filtersChanged) {
    filters.value = { ...routeState.filters }
  }

  if (hasExplicitTimeRange) {
    if (timeRangeChanged) {
      timeRangeInput.value = {
        startLocal: nextStartLocal,
        endLocal: nextEndLocal,
      }
    }
    if (filtersChanged || timeRangeChanged || !initialized.value) {
      await historyStore.applyFilters()
    }
  } else if (filtersChanged || !initialized.value) {
    historyStore.resetTimeRangeToDefault()
    await historyStore.refreshAnchor()
    await replaceRouteState(routeState.logId)
  }

  if (routeState.logId) {
    const targetSummary = items.value.find((item) => item.log_id === routeState.logId) ?? null
    if (targetSummary && selectedLogId.value !== routeState.logId) {
      await detailController.openDetail(targetSummary)
    }
    return
  }

  if (detailOpen.value) {
    detailController.closeDetail()
  }
}

async function activatePage() {
  if (!currentRouteLogId()) {
    autoFollowBottom.value = true
  }

  try {
    await syncFromRoute()
    await syncViewportToLatest()
  } catch {
    // store error drives the page
  }
}

async function refreshHistory() {
  autoFollowBottom.value = true

  try {
    await historyStore.refreshAnchor()
    await replaceRouteState()
    await syncViewportToLatest()
  } catch {
    // store error drives the page
  }
}

async function applyFilters() {
  autoFollowBottom.value = true

  try {
    await historyStore.applyFilters()
    await replaceRouteState(null)
    await syncViewportToLatest()
  } catch {
    // store error drives the page
  }
}

async function useRecentDay() {
  historyStore.resetTimeRangeToDefault()
  await refreshHistory()
}

async function useRecentDays(days: number) {
  historyStore.setTimeRange(days)
  await refreshHistory()
}

async function loadOlder() {
  if (!hasOlder.value || loadingOlder.value) {
    return
  }

  try {
    await historyStore.loadOlder()
  } catch {
    // store error drives the page
  }
}

function getLevelColor(level: string) {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'blue'
  return 'default'
}

async function openLogDetail(item: LogSummary) {
  await detailController.openDetail(item)
  await replaceRouteState(item.log_id)
}

async function closeLogDetail() {
  detailController.closeDetail()
  await replaceRouteState(null)
}

watch(
  () => route.query,
  () => {
    if (routeSyncing.value || route.name !== 'logs-history') {
      return
    }

    void syncFromRoute()
  },
)

onMounted(() => {
  void activatePage()
})

onActivated(() => {
  void activatePage()
})
</script>

<template>
  <AppPage :title="t('logs.historyTitle')" full-height>
    <template #extra>
      <a-button :loading="loading" @click="refreshHistory">
        {{ t('logs.history.refresh') }}
      </a-button>
    </template>

    <template #toolbar>
      <a-card :bordered="false" class="app-view-card logs-toolbar">
        <a-form layout="vertical" class="logs-filter-grid">
          <a-form-item :label="t('logs.filters.level')">
            <a-select
              v-model:value="filters.level"
              allow-clear
              :options="levelOptions"
              :placeholder="t('logs.filters.all')"
            />
          </a-form-item>
          <a-form-item :label="t('logs.filters.source')">
            <a-input v-model:value="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('logs.filters.protocol')">
            <a-select
              v-model:value="filters.protocol"
              allow-clear
              :options="[{ label: 'OneBot11', value: 'onebot11' }]"
              :placeholder="t('logs.filters.all')"
            />
          </a-form-item>
          <a-form-item :label="t('logs.filters.plugin')">
            <a-input v-model:value="filters.pluginId" :placeholder="t('logs.filters.pluginPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('logs.filters.requestId')">
            <a-input v-model:value="filters.requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('logs.history.startAt')">
            <a-input v-model:value="timeRangeInput.startLocal" type="datetime-local" />
          </a-form-item>
          <a-form-item :label="t('logs.history.endAt')">
            <a-input v-model:value="timeRangeInput.endLocal" type="datetime-local" />
          </a-form-item>
        </a-form>

        <div class="logs-toolbar__actions">
          <a-button @click="useRecentDay">{{ t('logs.history.lastDay') }}</a-button>
          <a-button @click="useRecentDays(7)">{{ t('logs.history.lastWeek') }}</a-button>
          <a-button @click="useRecentDays(30)">{{ t('logs.history.lastMonth') }}</a-button>
          <a-button @click="useRecentDays(180)">{{ t('logs.history.lastHalfYear') }}</a-button>
          <a-button type="primary" @click="applyFilters">{{ t('logs.filters.apply') }}</a-button>
        </div>
      </a-card>
    </template>

    <RetryPanel
      v-if="error && !initialized"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="refreshHistory"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <section
      v-else
      ref="logsLayoutRef"
      class="logs-layout"
    >
      <a-card :bordered="false" class="logs-feed-card">
        <template #title>
          <div class="logs-feed-card__title">
            <span>{{ t('logs.history.streamTitle') }}</span>
            <a-tag color="default">{{ t('logs.history.frozen') }}</a-tag>
          </div>
        </template>

        <VirtualDataViewport
          ref="viewportRef"
          :items="items"
          :item-height="96"
          :dynamic-item-height="true"
          :overscan="6"
          :follow-bottom="autoFollowBottom"
          :empty-label="t('display.empty')"
          :get-item-key="(item) => item.log_id"
          @reach-top="loadOlder"
        >
          <template #default="{ item }">
              <button
                type="button"
                class="logs-row"
                :class="{ 'is-selected': selectedLogId === item.log_id }"
                @click="openLogDetail(item)"
              >
              <div class="logs-row__meta">
                <div class="logs-row__time">{{ formatDateTime(item.timestamp) }}</div>
                <div class="logs-row__source">
                  <span>{{ item.source }}</span>
                  <span v-if="item.protocol" class="logs-row__protocol">{{ item.protocol }}</span>
                </div>
              </div>

              <div class="logs-row__main">
                <div class="logs-row__headline">
                  <a-tag size="small" :color="getLevelColor(item.level)">
                    {{ getLogLevelLabel(item.level) }}
                  </a-tag>
                  <span v-if="item.plugin_id" class="logs-row__sub">{{ item.plugin_id }}</span>
                  <span v-if="item.request_id" class="logs-row__sub">{{ item.request_id }}</span>
                </div>
                <p class="logs-row__message">{{ escapeUnsafeDisplayText(item.message) }}</p>
              </div>
            </button>
          </template>
        </VirtualDataViewport>
      </a-card>

      <ManagementLogDetailDrawer
        :open="detailOpen"
        :loading="detailLoading"
        :error="detailError"
        :summary="selectedSummary"
        :detail="currentDetail"
        memory-key="logs-history"
        scope="history"
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

.logs-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.logs-filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.logs-toolbar__actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.logs-feed-card,
.logs-feed-card :deep(.ant-card-body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-feed-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logs-row {
  width: 100%;
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 14px;
  border: none;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 78%, transparent);
  background: transparent;
  padding: 14px 16px;
  text-align: left;
  cursor: pointer;
}

.logs-row:hover,
.logs-row.is-selected {
  background: color-mix(in srgb, var(--app-primary) 8%, transparent);
}

.logs-row.is-selected {
  box-shadow: inset 3px 0 0 var(--app-primary);
  background: color-mix(in srgb, var(--app-primary) 5%, var(--surface-soft)) !important;
}

.logs-row__meta,
.logs-row__main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.logs-row__time,
.logs-row__source,
.logs-row__sub {
  font-family: "Cascadia Mono", "Consolas", monospace;
}

.logs-row__time {
  color: var(--text);
  font-size: 0.88rem;
}

.logs-row__source {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--muted);
  font-size: 0.78rem;
}

.logs-row__protocol {
  color: var(--app-primary);
}

.logs-row__headline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.logs-row__sub {
  color: var(--muted);
  font-size: 0.76rem;
}

.logs-row__message {
  margin: 0;
  color: var(--text);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  unicode-bidi: plaintext;
}

@media (max-width: 760px) {
  .logs-row {
    grid-template-columns: 1fr;
  }
}
</style>
