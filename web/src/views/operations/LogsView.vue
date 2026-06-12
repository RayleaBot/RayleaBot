<script setup lang="ts">
import { DownOutlined } from '@ant-design/icons-vue'
import { computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import AppPage from '@/components/page/AppPage.vue'
import { useToastFeedback } from '@/adapter/feedback'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import {
  areLocationQueriesEqual,
  buildLogsLocation,
  readLogWorkspaceState,
} from '@/lib/management-links'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { t } from '@/i18n'
import { normalizeFilterValues } from '@/stores/log-state'
import { useLogsStore } from '@/stores/logs'
import { usePluginsStore } from '@/stores/plugins'
import type { LogFilters } from '@/stores/log-state'
import type { LogLevel, LogSummary, PluginSummary } from '@/types/api'
import { useLogDetailController } from '@/views/operations/useLogDetailController'
import { useReadyToRenderHeavyContent } from '@/layouts/usePageTransitionStage'

const LOG_ROW_ESTIMATED_HEIGHT = 80

const route = useRoute()
const router = useRouter()
const logsStore = useLogsStore()
const pluginsStore = usePluginsStore()
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
  scrollToBottom: () => void
} | null>(null)
const routeSyncing = ref(false)
let activatePageTask: Promise<void> | null = null
let followOnNextActivation: boolean | null = null

const {
  atBottom,
  error,
  filters,
  hasOlder,
  initialized,
  items,
  loading,
  loadingOlder,
  pendingNewCount,
} = storeToRefs(logsStore)
const { sortedItems: pluginItems } = storeToRefs(pluginsStore)
const pageErrorToast = computed(() => (
  error.value
    ? {
        key: `logs-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))

const levelOptions = computed(() => ([
  { label: t('display.logLevels.debug'), value: 'debug' as LogLevel },
  { label: t('display.logLevels.info'), value: 'info' as LogLevel },
  { label: t('display.logLevels.warn'), value: 'warn' as LogLevel },
  { label: t('display.logLevels.error'), value: 'error' as LogLevel },
]))
const selectedPluginIds = computed(() => normalizeFilterValues(filters.value.pluginIds, filters.value.pluginId))
const pluginOptions = computed(() => {
  const options = pluginItems.value.map((plugin) => ({
    label: getPluginLabel(plugin),
    value: plugin.id,
  }))
  const knownPluginIds = new Set(options.map((option) => option.value))

  for (const pluginId of selectedPluginIds.value) {
    if (!knownPluginIds.has(pluginId)) {
      options.push({ label: pluginId, value: pluginId })
    }
  }

  return options
})

const followBottom = computed(() => atBottom.value)
const readyToRenderHeavyContent = useReadyToRenderHeavyContent()

useToastFeedback(pageErrorToast)

function whenReadyToRenderHeavyContent(): Promise<void> {
  if (readyToRenderHeavyContent.value) {
    return Promise.resolve()
  }

  return new Promise<void>((resolve) => {
    const stop = watch(readyToRenderHeavyContent, (value) => {
      if (value) {
        stop()
        resolve()
      }
    })
  })
}

function sameFilterValues(left: string[], right: string[]) {
  const normalizedLeft = [...left].sort((a, b) => a.localeCompare(b, 'zh-CN'))
  const normalizedRight = [...right].sort((a, b) => a.localeCompare(b, 'zh-CN'))
  return normalizedLeft.length === normalizedRight.length
    && normalizedLeft.every((item, index) => item === normalizedRight[index])
}

function sameLogFilters(left: LogFilters, right: LogFilters) {
  return sameFilterValues(normalizeFilterValues(left.levels, left.level), normalizeFilterValues(right.levels, right.level))
    && (left.source ?? '') === (right.source ?? '')
    && (left.protocol ?? '') === (right.protocol ?? '')
    && sameFilterValues(normalizeFilterValues(left.pluginIds, left.pluginId), normalizeFilterValues(right.pluginIds, right.pluginId))
    && (left.requestId ?? '') === (right.requestId ?? '')
}

function getPluginLabel(plugin: PluginSummary) {
  return `${plugin.name}（${plugin.id}）`
}

async function loadPluginOptions() {
  if (pluginsStore.items.length > 0 || pluginsStore.loading) {
    return
  }

  try {
    await pluginsStore.fetchList()
  } catch {
    return
  }
}

async function openPluginFilter() {
  await loadPluginOptions()
}

async function replaceRouteState(nextLogId: string | null = selectedLogId.value) {
  const target = buildLogsLocation({
    filters: filters.value,
    logId: nextLogId,
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
  if (route.name !== 'logs') {
    return
  }

  const routeState = readLogWorkspaceState(route.query)
  const filtersChanged = !sameLogFilters(filters.value, routeState.filters)

  if (filtersChanged) {
    filters.value = { ...routeState.filters }
  }

  if (filtersChanged) {
    await logsStore.applyFilters()
  } else {
    await logsStore.ensureLoaded()
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
  if (activatePageTask) {
    return activatePageTask
  }

  activatePageTask = (async () => {
    logsStore.setViewportActive(true)
    if (followOnNextActivation === true) {
      logsStore.setViewportAtBottom(true)
    }
    followOnNextActivation = null
    try {
      await syncFromRoute()
      await whenReadyToRenderHeavyContent()
      await nextTick()
      if (followBottom.value) {
        logsStore.acknowledgePendingNew()
        viewportRef.value?.scrollToBottom()
      }
    } catch {
      // store error drives the page
    }
  })()

  try {
    await activatePageTask
  } finally {
    activatePageTask = null
  }
}

async function applyFilters() {
  try {
    logsStore.setViewportAtBottom(true)
    await logsStore.applyFilters()
    await replaceRouteState(null)
    await whenReadyToRenderHeavyContent()
    await nextTick()
    viewportRef.value?.scrollToBottom()
  } catch {
    // store error drives the page
  }
}

async function loadOlder() {
  if (!hasOlder.value || loadingOlder.value) {
    return
  }

  try {
    await logsStore.loadOlder()
  } catch {
    // store error drives the page
  }
}

async function jumpToLatest() {
  logsStore.acknowledgePendingNew()
  logsStore.setViewportAtBottom(true)
  await whenReadyToRenderHeavyContent()
  await nextTick()
  viewportRef.value?.scrollToBottom()
}

function onViewportBottomChange(value: boolean) {
  logsStore.setViewportAtBottom(value)
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
    if (routeSyncing.value || route.name !== 'logs') {
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

onDeactivated(() => {
  followOnNextActivation = atBottom.value
  logsStore.setViewportActive(false)
})

onUnmounted(() => {
  logsStore.setViewportActive(false)
})
</script>

<template>
  <AppPage :title="t('logs.currentTitle')" full-height>
    <template #toolbar>
      <a-card
        :bordered="false"
        class="app-view-card logs-toolbar"
      >
        <a-form layout="vertical" class="logs-filter-grid">
          <a-form-item :label="t('logs.filters.level')">
            <a-select
              v-model:value="filters.levels"
              mode="multiple"
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
            <a-select
              v-model:value="filters.pluginIds"
              mode="multiple"
              allow-clear
              :options="pluginOptions"
              :placeholder="t('logs.filters.all')"
              @focus="openPluginFilter"
            />
          </a-form-item>
          <a-form-item :label="t('logs.filters.requestId')">
            <a-input v-model:value="filters.requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
          </a-form-item>
        </a-form>

        <div class="logs-toolbar__actions">
          <a-button type="primary" :aria-label="t('logs.filters.apply')" @click="applyFilters">{{ t('logs.filters.apply') }}</a-button>
        </div>
      </a-card>
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
      <a-card
        :bordered="false"
        class="logs-feed-card"
      >
        <template #title>
          <div class="logs-feed-card__title">
            <span>{{ t('logs.current.streamTitle') }}</span>
            <a-tag :color="atBottom ? 'success' : 'default'">
              {{ atBottom ? t('logs.current.following') : t('logs.current.paused') }}
            </a-tag>
          </div>
        </template>

        <div class="logs-feed-card__body">
          <a-skeleton
            v-if="!readyToRenderHeavyContent"
            active
            :paragraph="{ rows: 6 }"
          />
          <VirtualDataViewport
            v-else
            ref="viewportRef"
            :items="items"
            :item-height="LOG_ROW_ESTIMATED_HEIGHT"
            :dynamic-item-height="true"
            :overscan="6"
            :follow-bottom="followBottom"
            :bottom-threshold="0"
            :empty-label="t('display.empty')"
            :get-item-key="(item) => item.log_id"
            @reach-top="loadOlder"
            @at-bottom-change="onViewportBottomChange"
          >
            <template #default="{ item }">
                <button
                  type="button"
                  class="logs-row"
                  :class="{ 'is-selected': selectedLogId === item.log_id }"
                  :aria-label="`${getLogLevelLabel(item.level)} · ${item.source} · ${formatDateTime(item.timestamp)}`"
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

          <div v-if="!atBottom" class="logs-jump-latest">
            <a-badge :count="pendingNewCount || undefined" :offset="[-2, 4]">
              <a-tooltip
                :title="pendingNewCount > 0 ? t('logs.current.pendingNew', { count: pendingNewCount }) : t('logs.current.jumpToLatest')"
              >
                <a-button
                  type="primary"
                  shape="circle"
                  class="logs-jump-latest__button"
                  :aria-label="t('logs.current.jumpToLatest')"
                  @click="jumpToLatest"
                >
                  <template #icon>
                    <DownOutlined />
                  </template>
                </a-button>
              </a-tooltip>
            </a-badge>
          </div>
        </div>
      </a-card>

      <ManagementLogDetailDrawer
        v-if="detailOpen || selectedSummary"
        :open="detailOpen"
        :loading="detailLoading"
        :error="detailError"
        :summary="selectedSummary"
        :detail="currentDetail"
        memory-key="logs-current"
        scope="current_session"
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
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  box-shadow: var(--shadow-xs);
}

.logs-filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
  align-items: end;
}

.logs-toolbar__actions {
  display: flex;
  justify-content: flex-end;
  align-self: end;
}

.logs-feed-card,
.logs-feed-card :deep(.ant-card-body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-feed-card {
  box-shadow: var(--shadow-xs);
}

.logs-feed-card__body {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

.logs-feed-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
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

.logs-jump-latest :deep(.ant-badge) {
  pointer-events: auto;
}

.logs-jump-latest__button {
  box-shadow: 0 14px 30px color-mix(in srgb, var(--accent) 24%, transparent);
}

.logs-row {
  width: 100%;
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 14px;
  border: none;
  border-bottom: 1px solid var(--border);
  background: transparent;
  padding: 14px 16px;
  text-align: left;
  cursor: pointer;
}

.logs-row:hover,
.logs-row.is-selected {
  background: var(--surface-accent);
}

.logs-row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.logs-row.is-selected {
  box-shadow: inset 3px 0 0 var(--accent);
  background: var(--surface-accent) !important;
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
  font-family: var(--font-mono);
}

.logs-row__time {
  color: var(--muted);
  font-size: 0.82rem;
}

.logs-row__source {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--muted);
  font-size: 0.78rem;
}

.logs-row__protocol {
  color: var(--accent);
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
  font-size: 0.9rem;
  white-space: pre-wrap;
  word-break: break-all;
  unicode-bidi: plaintext;
}

@media (max-width: 760px) {
  .logs-jump-latest {
    right: 14px;
    bottom: 14px;
  }

  .logs-row {
    grid-template-columns: 1fr;
  }
}

@media (min-width: 961px) {
  .logs-layout.has-detail-window .logs-jump-latest {
    right: max(18px, calc(50% - 12px));
  }
}
</style>
