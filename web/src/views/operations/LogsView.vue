<script setup lang="ts">
import AppTag from '@/components/AppTag.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import { ChevronDownIcon } from '@lucide/vue'
import { computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'

import ManagementLogDetailDrawer from '@/components/logs/ManagementLogDetailDrawer.vue'
import ManagementLogAdvancedFilters from '@/components/logs/ManagementLogAdvancedFilters.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import AppPage from '@/components/page/AppPage.vue'
import { useToastFeedback } from '@/adapter/feedback'
import {
  areLocationQueriesEqual,
  buildLogsLocation,
  readLogWorkspaceState,
} from '@/lib/management-links'
import { t } from '@/i18n'
import { sameLogFilters } from '@/stores/log-state'
import { useLogFilterControls } from '@/components/logs/useLogFilterControls'
import ManagementLogRow from '@/components/logs/ManagementLogRow.vue'
import { useLogsStore } from '@/stores/logs'
import { useUiShellStore } from '@/stores/ui-shell'
import type { LogSummary } from '@/types/api'
import { useLogDetailController } from '@/views/operations/useLogDetailController'
import { useReadyToRenderHeavyContent } from '@/layouts/usePageTransitionStage'

const LOG_ROW_ESTIMATED_HEIGHT = 80
const LOG_BOTTOM_THRESHOLD = 24

const route = useRoute()
const router = useRouter()
const logsStore = useLogsStore()
const uiShellStore = useUiShellStore()
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
  isAtBottom: () => boolean
  scrollToBottom: () => void
} | null>(null)
const routeSyncing = ref(false)
const restoringLatest = ref(false)
let activatePageTask: Promise<void> | null = null

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
const pageErrorToast = computed(() => (
  error.value
    ? {
        key: `logs-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))

const { selectedLevels, levelOptions, pluginOptions, openPluginFilter } = useLogFilterControls(filters)


const readyToRenderHeavyContent = useReadyToRenderHeavyContent()
const followBottom = computed(() => atBottom.value)
const showJumpToLatest = computed(() => (
  readyToRenderHeavyContent.value
  && initialized.value
  && !restoringLatest.value
  && !atBottom.value
))

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

function waitForAnimationFrame() {
  if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
    return Promise.resolve()
  }

  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
}

async function scrollRealtimeLogsToLatest() {
  restoringLatest.value = true
  logsStore.setViewportAtBottom(true)
  logsStore.acknowledgePendingNew()

  try {
    await whenReadyToRenderHeavyContent()
    await nextTick()
    viewportRef.value?.scrollToBottom()
    await waitForAnimationFrame()
    viewportRef.value?.scrollToBottom()

    if (viewportRef.value?.isAtBottom() ?? true) {
      logsStore.setViewportAtBottom(true)
    }
  } finally {
    restoringLatest.value = false
  }
}

function syncLogsTabWithoutSelectedDetail() {
  const routeState = readLogWorkspaceState(route.query)
  const target = router.resolve(buildLogsLocation({
    filters: routeState.filters,
    logId: null,
  }))
  const tab = uiShellStore.tabs.find((item) => item.path === route.path)
  if (!tab || tab.fullPath === target.fullPath) {
    return
  }

  uiShellStore.upsertTab({
    ...tab,
    fullPath: target.fullPath,
  })
}

function deactivateRealtimeLogsPage(options: { syncTabDetail?: boolean } = {}) {
  logsStore.setViewportActive(false)
  logsStore.setViewportAtBottom(true)
  logsStore.acknowledgePendingNew()

  if (detailOpen.value || selectedSummary.value) {
    detailController.closeDetail()
  }

  if (options.syncTabDetail) {
    syncLogsTabWithoutSelectedDetail()
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

  if (route.name !== 'logs') {
    return
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
  void openPluginFilter()
  if (activatePageTask) {
    return activatePageTask
  }

  activatePageTask = (async () => {
    logsStore.setViewportActive(true)
    logsStore.setViewportAtBottom(true)
    try {
      await syncFromRoute()
      if (route.name !== 'logs') {
        return
      }
      await scrollRealtimeLogsToLatest()
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
    await scrollRealtimeLogsToLatest()
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
  await scrollRealtimeLogsToLatest()
}

function onViewportBottomChange(value: boolean) {
  if (restoringLatest.value && !value) {
    return
  }

  logsStore.setViewportAtBottom(value)
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

onBeforeRouteLeave(() => {
  deactivateRealtimeLogsPage({ syncTabDetail: true })
})

onDeactivated(() => {
  deactivateRealtimeLogsPage({ syncTabDetail: true })
})

onUnmounted(() => {
  deactivateRealtimeLogsPage()
})
</script>

<template>
  <AppPage :title="t('logs.currentTitle')" full-height>
    <template #toolbar>
      <AppCard
        borderless
        class="app-view-card logs-toolbar"
      >
        <div class="logs-filter-grid">
          <AppField :label="t('logs.filters.level')">
            <AppSelect
              v-model="selectedLevels"
              multiple
              clearable
              :options="levelOptions"
              :placeholder="t('logs.filters.all')"
            />
          </AppField>
          <AppField floating :label="t('logs.filters.source')">
            <AppInput v-model="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
          </AppField>
          <div class="logs-toolbar__actions">
            <ManagementLogAdvancedFilters
              v-model:protocol="filters.protocol"
              v-model:plugin-ids="filters.pluginIds"
              v-model:request-id="filters.requestId"
              :plugin-options="pluginOptions"
              @plugin-focus="openPluginFilter"
            />
            <AppButton class="logs-toolbar__apply" variant="default" :aria-label="t('logs.filters.apply')" @click="applyFilters">{{ t('logs.filters.apply') }}</AppButton>
          </div>
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
      <AppCard
        borderless
        class="logs-feed-card"
      >
        <template #title>
          <div class="logs-feed-card__title">
            <span>{{ t('logs.current.streamTitle') }}</span>
            <AppTag :tone="atBottom ? 'success' : 'neutral'">
              {{ atBottom ? t('logs.current.following') : t('logs.current.paused') }}
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
            :item-height="LOG_ROW_ESTIMATED_HEIGHT"
            :dynamic-item-height="true"
            :overscan="6"
            :follow-bottom="followBottom"
            :bottom-threshold="LOG_BOTTOM_THRESHOLD"
            :empty-label="t('logs.current.empty')"
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
                  @click="jumpToLatest"
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
        memory-key="logs-current"
        scope="current_session"
        :host-element="logsLayoutRef"
        @close="closeLogDetail"
      />
    </section>
  </AppPage>
</template>

<style lang="scss" scoped>
.logs-jump-latest__count { position: absolute; top: -8px; right: -8px; min-width: 22px; padding: 2px 5px; border-radius: 12px; background: var(--surface-danger); color: var(--text-danger); font-size: 12px; }
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

.logs-filter-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: end;
  width: 100%;
}

.logs-filter-grid :deep(.app-field) {
  flex: 1 1 200px;
  max-width: 320px;
  margin-bottom: 0;
}

.logs-filter-grid :deep(.app-field:first-child) {
  max-width: 220px;
}

.logs-toolbar__actions {
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  margin-inline-start: auto;
}

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

.logs-jump-latest :deep(.logs-jump-latest__control) {
  pointer-events: auto;
}

.logs-jump-latest__button {
  box-shadow: 0 14px 30px color-mix(in srgb, var(--accent) 24%, transparent);
}

@media (max-width: 760px) {
  .logs-filter-grid :deep(.app-field) {
    flex-basis: 100%;
    max-width: none;
  }

  .logs-toolbar__actions {
    flex: 1 1 100%;
    align-items: stretch;
    justify-content: flex-start;
    margin-inline-start: 0;
  }

  .logs-toolbar__apply {
    flex: 1 1 auto;
  }

  .logs-jump-latest {
    right: 14px;
    bottom: 14px;
  }

}

@media (min-width: 961px) {
  .logs-layout.has-detail-window .logs-jump-latest {
    right: max(18px, calc(50% - 12px));
  }
}
</style>
