<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import {
  CaretDownOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  FieldTimeOutlined,
  InfoCircleOutlined,
  KeyOutlined,
  LinkOutlined,
  NotificationOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SyncOutlined,
  ThunderboltOutlined,
  ToolOutlined,
  UserOutlined,
  VideoCameraOutlined,
  WarningOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { useSocketStore } from '@/stores/sockets'
import { useThirdPartyMonitoringStore } from '@/stores/third-party-monitoring'
import type {
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorService,
} from '@/types/api'

type StatusTone = 'normal' | 'success' | 'warning' | 'danger'

const store = useThirdPartyMonitoringStore()
const socketStore = useSocketStore()
const router = useRouter()
const {
  bilibiliStatus,
  error,
  items,
  lastRefreshedAt,
  loading,
  monitors,
  restarting,
} = storeToRefs(store)

const avatarLoadFailures = reactive<Record<string, boolean>>({})
const coverLoadFailures = reactive<Record<string, boolean>>({})
const coverLoaded = reactive<Record<string, boolean>>({})
const diagnosisExpanded = ref(false)
const stripRefreshed = ref(false)
const cardRefreshed = reactive<Record<string, boolean>>({})
const metricBumped = reactive<Record<string, boolean>>({})
const cardsEntered = ref(false)

let stripRefreshTimer: number | null = null
let cardsEnteredTimer: number | null = null
const cardRefreshTimers = new Map<string, number>()
const metricBumpTimers = new Map<string, number>()

const fatalError = computed(() => error.value && !monitors.value)
const pageErrorToast = computed(() => (
  error.value && monitors.value
    ? {
        key: `third-party-monitoring-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))
const statusTag = computed(() => sourceStatusMeta(bilibiliStatus.value?.status))
const statusTone = computed<StatusTone>(() => statusToneFromDiagnosis(bilibiliStatus.value?.diagnosis.level))
const diagnosis = computed(() => bilibiliStatus.value?.diagnosis ?? null)
const diagnosisActions = computed(() => diagnosis.value?.actions ?? [])
const openAccountsAction = computed(() => diagnosisActions.value.find((action) => action.kind === 'open_accounts'))
const watchedUIDs = computed(() => items.value.map((item) => item.uid))
const liveCount = computed(() => items.value.filter((item) => item.live.is_live).length)
const dynamicCount = computed(() => items.value.filter((item) => item.dynamic).length)
const accountCount = computed(() => bilibiliStatus.value?.accounts.length ?? 0)
const hasDiagnosisDetail = computed(() =>
  (diagnosis.value?.causes.length ?? 0) > 0 ||
  (diagnosis.value?.impacts.length ?? 0) > 0 ||
  (diagnosis.value?.actions.length ?? 0) > 0,
)
const eventsStatus = computed(() => socketStore.snapshots.events.status)
const realtimeConnected = computed(() => {
  const status = eventsStatus.value
  return status === 'connected' || status === 'authenticated'
})
const hasSkeleton = computed(() => loading.value && !monitors.value)
const isReconnecting = computed(() => !realtimeConnected.value && !!monitors.value)

useToastFeedback(pageErrorToast)

watch(lastRefreshedAt, () => {
  if (!monitors.value || loading.value) {
    return
  }
  stripRefreshed.value = true
  if (stripRefreshTimer !== null) {
    window.clearTimeout(stripRefreshTimer)
  }
  stripRefreshTimer = window.setTimeout(() => {
    stripRefreshed.value = false
    stripRefreshTimer = null
  }, 600)
})

watch(items, (newItems, oldItems) => {
  if (!oldItems || oldItems.length === 0) {
    return
  }
  const oldMap = new Map(oldItems.map((item) => [item.uid, item]))
  for (const item of newItems) {
    const old = oldMap.get(item.uid)
    if (old && JSON.stringify(item) !== JSON.stringify(old)) {
      cardRefreshed[item.uid] = true
      const existingTimer = cardRefreshTimers.get(item.uid)
      if (existingTimer !== undefined) {
        window.clearTimeout(existingTimer)
      }
      const timer = window.setTimeout(() => {
        cardRefreshed[item.uid] = false
        cardRefreshTimers.delete(item.uid)
      }, 600)
      cardRefreshTimers.set(item.uid, timer)
    }
  }
})

watch([liveCount, dynamicCount, accountCount], (newVals, oldVals) => {
  const keys = ['liveCount', 'dynamicCount', 'accountCount']
  for (let i = 0; i < keys.length; i++) {
    if (oldVals[i] !== undefined && newVals[i] !== oldVals[i]) {
      metricBumped[keys[i]!] = true
      const key = keys[i]!
      const existingTimer = metricBumpTimers.get(key)
      if (existingTimer !== undefined) {
        window.clearTimeout(existingTimer)
      }
      const timer = window.setTimeout(() => {
        metricBumped[key] = false
        metricBumpTimers.delete(key)
      }, 500)
      metricBumpTimers.set(key, timer)
    }
  }
})

onMounted(() => {
  store.activate()
  void loadPage()
  cardsEnteredTimer = window.setTimeout(() => {
    cardsEntered.value = true
    cardsEnteredTimer = null
  }, 50)
})

onUnmounted(() => {
  store.deactivate()
  if (stripRefreshTimer !== null) {
    window.clearTimeout(stripRefreshTimer)
    stripRefreshTimer = null
  }
  if (cardsEnteredTimer !== null) {
    window.clearTimeout(cardsEnteredTimer)
    cardsEnteredTimer = null
  }
  for (const timer of cardRefreshTimers.values()) {
    window.clearTimeout(timer)
  }
  cardRefreshTimers.clear()
  for (const timer of metricBumpTimers.values()) {
    window.clearTimeout(timer)
  }
  metricBumpTimers.clear()
})

async function loadPage() {
  try {
    await store.fetchAll()
  } catch {
    // store error state drives the page
  }
}

async function restartSource() {
  try {
    await store.restartBilibiliSource()
    notifySuccess(t('builtinFeatures.thirdPartyMonitoring.restarted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function toggleDiagnosis() {
  diagnosisExpanded.value = !diagnosisExpanded.value
}

function liveIndicatorMeta() {
  switch (eventsStatus.value) {
    case 'connected':
    case 'authenticated':
      return { color: 'green', tone: 'live', label: t('builtinFeatures.thirdPartyMonitoring.realtime') }
    case 'connecting':
      return { color: 'blue', tone: 'connecting', label: t('builtinFeatures.thirdPartyMonitoring.realtimeConnecting') }
    case 'reconnecting':
      return { color: 'warning', tone: 'reconnecting', label: t('builtinFeatures.thirdPartyMonitoring.realtimeReconnecting') }
    case 'auth_failed':
      return { color: 'danger', tone: 'error', label: t('builtinFeatures.thirdPartyMonitoring.realtimeAuthFailed') }
    case 'disconnected':
    default:
      return { color: 'default', tone: 'disconnected', label: t('builtinFeatures.thirdPartyMonitoring.realtimeDisconnected') }
  }
}

function sourceStatusMeta(value?: BilibiliSourceStatusResponse['status']) {
  switch (value) {
    case 'connected':
      return { color: 'green', label: t('builtinFeatures.thirdPartyMonitoring.sourceConnected') }
    case 'connecting':
      return { color: 'blue', label: t('builtinFeatures.thirdPartyMonitoring.sourceConnecting') }
    case 'degraded':
      return { color: 'orange', label: t('builtinFeatures.thirdPartyMonitoring.sourceDegraded') }
    case 'failed':
      return { color: 'red', label: t('builtinFeatures.thirdPartyMonitoring.sourceFailed') }
    case 'disabled':
      return { color: 'default', label: t('builtinFeatures.thirdPartyMonitoring.disabled') }
    default:
      return { color: 'default', label: t('builtinFeatures.thirdPartyMonitoring.sourceIdle') }
  }
}

function statusToneFromDiagnosis(value?: BilibiliSourceStatusResponse['diagnosis']['level']): StatusTone {
  switch (value) {
    case 'normal':
      return 'success'
    case 'attention':
      return 'warning'
    case 'action_required':
      return 'danger'
    default:
      return 'normal'
  }
}

async function openBilibiliAccounts() {
  await router.push(openAccountsAction.value?.target || { name: 'third-party-accounts' })
}

function liveTag(item: ThirdPartyMonitorItem) {
  if (item.live.is_live) {
    return { color: 'green', label: t('builtinFeatures.thirdPartyMonitoring.liveOn') }
  }
  if (visibleLiveError(item)) {
    return { color: 'red', label: t('builtinFeatures.thirdPartyMonitoring.liveError') }
  }
  return { color: 'default', label: t('builtinFeatures.thirdPartyMonitoring.liveOff') }
}

function serviceLabel(service: ThirdPartyMonitorService | string) {
  const key = `builtinFeatures.thirdPartyMonitoring.services.${service}`
  return t(key)
}

function mainImage(item: ThirdPartyMonitorItem) {
  return item.live.cover_url || item.dynamic?.images?.[0]?.url || ''
}

function visibleLiveError(item: ThirdPartyMonitorItem) {
  const value = item.live.last_error.trim()
  if (!value) {
    return ''
  }
  const normalized = value.toLowerCase()
  return normalized.includes('risk_control') || normalized.includes('code -352') ? '' : value
}

function displayTime(value?: string | null) {
  return value ? formatDateTime(value) : t('display.empty')
}

function liveStartedText(item: ThirdPartyMonitorItem) {
  return item.live.is_live
    ? displayTime(item.live.live_started_at)
    : t('display.empty')
}

function liveEndedText(item: ThirdPartyMonitorItem) {
  return item.live.is_live
    ? t('display.empty')
    : displayTime(item.live.live_ended_at)
}

function roomName(item: ThirdPartyMonitorItem) {
  return item.live.room_name || t('builtinFeatures.thirdPartyMonitoring.noRoomName')
}

function dynamicTitle(item: ThirdPartyMonitorItem) {
  return item.dynamic?.title || t('builtinFeatures.thirdPartyMonitoring.noDynamic')
}

function dynamicSummary(item: ThirdPartyMonitorItem) {
  return item.dynamic?.summary?.trim() || ''
}

function avatarFailed(uid: string) {
  avatarLoadFailures[uid] = true
}

function coverFailed(uid: string) {
  coverLoadFailures[uid] = true
}

function coverLoadedOk(uid: string) {
  coverLoaded[uid] = true
}

function cardTone(item: ThirdPartyMonitorItem): string {
  if (visibleLiveError(item)) {
    return 'error'
  }
  if (item.live.is_live && item.dynamic) {
    return 'live-dynamic'
  }
  if (item.live.is_live) {
    return 'live-only'
  }
  if (item.dynamic) {
    return 'dynamic-only'
  }
  return 'idle'
}

function dynamicIcon(service?: string) {
  switch (service) {
    case 'video':
      return 'play'
    case 'image_text':
      return 'image'
    case 'article':
      return 'article'
    case 'repost':
      return 'repost'
    default:
      return 'default'
  }
}
</script>

<template>
  <AppPage :title="t('builtinFeatures.thirdPartyMonitoring.title')" :description="t('builtinFeatures.thirdPartyMonitoring.subtitle')">
    <template #extra>
      <div class="monitoring-actions">
        <a-button :loading="restarting" @click="restartSource">
          <template #icon><SyncOutlined /></template>
          {{ t('builtinFeatures.thirdPartyMonitoring.restartSource') }}
        </a-button>
      </div>
    </template>

    <RetryPanel
      v-if="fatalError"
      :title="t('errors.common.loadFailed')"
      :description="error || ''"
      :loading="loading"
      @retry="loadPage"
    />

    <div v-else class="third-party-monitoring">
      <!-- Skeleton -->
      <div v-if="hasSkeleton" class="monitoring-skeleton">
        <div class="monitoring-skeleton__strip" />
        <div class="monitoring-skeleton__cards">
          <div v-for="i in 3" :key="i" class="monitoring-skeleton__card">
            <div class="monitoring-skeleton__cover" />
            <div class="monitoring-skeleton__body">
              <div class="monitoring-skeleton__identity">
                <div class="monitoring-skeleton__avatar" />
                <div class="monitoring-skeleton__name" />
              </div>
              <div class="monitoring-skeleton__line" />
              <div class="monitoring-skeleton__line monitoring-skeleton__line--short" />
            </div>
          </div>
        </div>
      </div>

      <!-- Status bar -->
      <section
        v-if="!hasSkeleton"
        :class="[
          'monitoring-strip',
          `monitoring-strip--${statusTone}`,
          { 'is-reconnecting': isReconnecting, 'is-refreshed': stripRefreshed },
        ]"
      >
        <div class="monitoring-strip__row">
          <div class="monitoring-strip__left">
            <div class="monitoring-strip__status">
              <span class="monitoring-strip__pulse" :class="`monitoring-strip__pulse--${statusTag.color}`">
                <span class="monitoring-strip__dot" :class="`monitoring-strip__dot--${statusTag.color}`" />
              </span>
              <span class="monitoring-strip__label">{{ statusTag.label }}</span>
            </div>
            <span class="monitoring-strip__summary">
              {{ diagnosis?.headline || bilibiliStatus?.summary || t('builtinFeatures.thirdPartyMonitoring.sourceWaiting') }}
            </span>
          </div>
          <div class="monitoring-strip__right">
            <span
              :class="[
                'monitoring-strip__live',
                `monitoring-strip__live--${liveIndicatorMeta().tone}`,
              ]"
              data-testid="third-party-monitoring-live-indicator"
            >
              <span class="monitoring-strip__live-dot" aria-hidden="true" />
              {{ liveIndicatorMeta().label }}
            </span>
            <div class="monitoring-strip__metrics">
              <div
                class="metric-badge"
                :class="{ 'is-bumped': metricBumped['accountCount'] }"
              >
                <KeyOutlined class="metric-badge__icon" />
                <span class="metric-badge__value">{{ accountCount }}</span>
                <span class="metric-badge__label">CK</span>
              </div>
              <div
                class="metric-badge"
                :class="{
                  'metric-badge--warning': liveCount < (bilibiliStatus?.live.watched_rooms ?? watchedUIDs.length),
                  'is-bumped': metricBumped['liveCount'],
                }"
              >
                <VideoCameraOutlined class="metric-badge__icon" />
                <span class="metric-badge__value">{{ liveCount }}/{{ bilibiliStatus?.live.watched_rooms ?? watchedUIDs.length }}</span>
                <span class="metric-badge__label">{{ t('builtinFeatures.thirdPartyMonitoring.liveMetric') }}</span>
              </div>
              <div
                class="metric-badge"
                :class="{
                  'metric-badge--warning': dynamicCount < (bilibiliStatus?.dynamic.watched_uids ?? watchedUIDs.length),
                  'is-bumped': metricBumped['dynamicCount'],
                }"
              >
                <NotificationOutlined class="metric-badge__icon" />
                <span class="metric-badge__value">{{ dynamicCount }}/{{ bilibiliStatus?.dynamic.watched_uids ?? watchedUIDs.length }}</span>
                <span class="metric-badge__label">{{ t('builtinFeatures.thirdPartyMonitoring.dynamicMetric') }}</span>
              </div>
            </div>
            <a-button
              v-if="hasDiagnosisDetail"
              type="text"
              size="small"
              :class="['monitoring-strip__toggle', { 'is-expanded': diagnosisExpanded }]"
              @click="toggleDiagnosis"
            >
              <CaretDownOutlined />
            </a-button>
          </div>
        </div>

        <!-- Expandable diagnosis detail -->
        <div :class="['monitoring-strip__detail', { 'is-open': diagnosisExpanded }]">
          <div class="monitoring-strip__detail-inner">
            <div
              :class="[
                'diagnosis-grid',
                { 'diagnosis-grid--single-col': !diagnosis?.causes.length || (!diagnosis?.impacts.length && !diagnosisActions.length) }
              ]"
            >
              <!-- Left Column: Causes -->
              <div v-if="diagnosis?.causes.length" class="diagnosis-grid__col-causes">
                <div class="diagnosis-section">
                  <div class="diagnosis-section__header">
                    <InfoCircleOutlined class="diagnosis-section__header-icon" />
                    <span class="diagnosis-section__header-title">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisCause') }}</span>
                  </div>
                  <div class="diagnosis-causes">
                    <article
                      v-for="cause in diagnosis.causes"
                      :key="`${cause.scope}:${cause.code}`"
                      class="diagnosis-cause-card"
                    >
                      <div class="diagnosis-cause-card__icon">
                        <WarningOutlined v-if="statusTone === 'warning'" />
                        <CloseCircleOutlined v-else-if="statusTone === 'danger'" />
                        <CheckCircleOutlined v-else />
                      </div>
                      <div class="diagnosis-cause-card__content">
                        <strong>{{ cause.title }}</strong>
                        <p>{{ cause.detail }}</p>
                      </div>
                    </article>
                  </div>
                </div>
              </div>

              <!-- Right Column: Impacts and Actions -->
              <div v-if="diagnosis?.impacts.length || diagnosisActions.length" class="diagnosis-grid__col-meta">
                <!-- Impacts -->
                <div v-if="diagnosis?.impacts.length" class="diagnosis-section">
                  <div class="diagnosis-section__header">
                    <ThunderboltOutlined class="diagnosis-section__header-icon" />
                    <span class="diagnosis-section__header-title">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisImpact') }}</span>
                  </div>
                  <div class="diagnosis-impact-list">
                    <span v-for="impact in diagnosis.impacts" :key="impact" class="impact-tag">
                      <CheckCircleOutlined class="impact-tag__icon" />
                      {{ impact }}
                    </span>
                  </div>
                </div>

                <!-- Actions -->
                <div v-if="diagnosisActions.length" class="diagnosis-section">
                  <div class="diagnosis-section__header">
                    <ToolOutlined class="diagnosis-section__header-icon" />
                    <span class="diagnosis-section__header-title">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisAction') }}</span>
                  </div>
                  <div class="diagnosis-action-list">
                    <span
                      v-for="action in diagnosisActions"
                      :key="`${action.kind}:${action.label}`"
                      class="action-tag"
                    >
                      {{ action.label }}
                    </span>
                    <a-button
                      v-if="openAccountsAction"
                      type="primary"
                      size="small"
                      class="diagnosis-action-btn"
                      @click="openBilibiliAccounts"
                    >
                      {{ openAccountsAction.label }}
                    </a-button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- UID tag flow -->
      <div v-if="watchedUIDs.length" class="uid-strip">
        <span class="uid-strip__label">{{ t('builtinFeatures.thirdPartyMonitoring.uidList') }}</span>
        <div class="uid-strip__tags">
          <a-tag v-for="uid in watchedUIDs" :key="uid" size="small">UID {{ uid }}</a-tag>
        </div>
      </div>

      <!-- Empty state -->
      <AppEmptyState
        v-if="!loading && !items.length"
        icon="search"
        :title="t('builtinFeatures.thirdPartyMonitoring.empty')"
        :action-label="openAccountsAction?.label"
        @action="openBilibiliAccounts"
      />

      <!-- Monitor cards -->
      <section v-if="items.length" class="monitor-card-grid">
        <article
          v-for="(item, index) in items"
          :key="item.uid"
          :class="[
            'monitor-card',
            `monitor-card--${cardTone(item)}`,
            { 'is-refreshed': cardRefreshed[item.uid], 'is-entered': cardsEntered },
            { 'is-reconnecting': isReconnecting },
          ]"
          :style="{ transitionDelay: `${index * 40}ms` }"
        >
          <!-- Landscape cover with gaussian blur transition -->
          <div class="monitor-card__cover-wrap">
            <div
              v-if="mainImage(item) && !coverLoadFailures[item.uid]"
              class="monitor-card__cover"
            >
              <div
                v-if="!coverLoaded[item.uid]"
                class="monitor-card__cover-skeleton"
                aria-hidden="true"
              />
              <img
                :src="mainImage(item)"
                :alt="roomName(item)"
                :class="['monitor-card__cover-img', { 'is-loaded': coverLoaded[item.uid] }]"
                @load="coverLoadedOk(item.uid)"
                @error="coverFailed(item.uid)"
              >
              <!-- Blurred extension layer -->
              <div class="monitor-card__cover-blur" aria-hidden="true">
                <img
                  :src="mainImage(item)"
                  alt=""
                  :class="['monitor-card__cover-blur-img', { 'is-loaded': coverLoaded[item.uid] }]"
                >
              </div>
              <!-- Live info overlay on blurred zone -->
              <div class="monitor-card__cover-info">
                <a-tag :color="liveTag(item).color" size="small">{{ liveTag(item).label }}</a-tag>
                <h3 class="monitor-card__cover-room-name">
                  <a
                    v-if="item.live.room_url"
                    :href="item.live.room_url"
                    target="_blank"
                    rel="noreferrer"
                  >{{ roomName(item) }}</a>
                  <span v-else>{{ roomName(item) }}</span>
                </h3>
                <span class="monitor-card__cover-room-id">{{ t('builtinFeatures.thirdPartyMonitoring.roomId', { id: item.live.room_id || t('display.empty') }) }}</span>
              </div>
            </div>
            <div v-else class="monitor-card__cover monitor-card__cover--fallback">
              <div class="monitor-card__cover-fb-inner">
                <FieldTimeOutlined />
                <span>{{ t('builtinFeatures.thirdPartyMonitoring.noRoomName') }}</span>
              </div>
            </div>
          </div>

          <!-- Card body -->
          <div class="monitor-card__body">
            <div class="monitor-card__identity">
              <a-avatar :size="40" class="monitor-avatar">
                <img
                  v-if="item.avatar_url && !avatarLoadFailures[item.uid]"
                  class="monitor-avatar__image"
                  :src="item.avatar_url"
                  :alt="item.username"
                  data-testid="third-party-monitor-avatar-image"
                  draggable="false"
                  loading="lazy"
                  referrerpolicy="no-referrer"
                  @error="avatarFailed(item.uid)"
                >
                <UserOutlined v-else />
              </a-avatar>
              <div class="monitor-card__identity-text">
                <a
                  v-if="item.profile_url"
                  :href="item.profile_url"
                  target="_blank"
                  rel="noreferrer"
                  class="monitor-card__profile-link"
                >
                  {{ item.username || item.uid }}
                </a>
                <strong v-else>{{ item.username || item.uid }}</strong>
                <span>UID {{ item.uid }}</span>
              </div>
              <div class="monitor-card__services">
                <a-tag
                  v-for="service in item.services"
                  :key="service"
                  size="small"
                  :class="['service-tag', `service-tag--${service}`]"
                >
                  {{ serviceLabel(service) }}
                </a-tag>
              </div>
            </div>

            <!-- Dynamic -->
            <div v-if="item.dynamic" class="monitor-card__dynamic">
              <div class="monitor-card__dynamic-header">
                <span class="monitor-card__dynamic-icon">
                  <PlayCircleOutlined v-if="dynamicIcon(item.dynamic.service) === 'play'" />
                  <NotificationOutlined v-else-if="dynamicIcon(item.dynamic.service) === 'image'" />
                  <InfoCircleOutlined v-else-if="dynamicIcon(item.dynamic.service) === 'article'" />
                  <SyncOutlined v-else-if="dynamicIcon(item.dynamic.service) === 'repost'" />
                  <NotificationOutlined v-else />
                </span>
                <span class="monitor-card__dynamic-label">{{ t('builtinFeatures.thirdPartyMonitoring.dynamicTitle') }}</span>
              </div>
              <a
                v-if="item.dynamic.url"
                :href="item.dynamic.url"
                target="_blank"
                rel="noreferrer"
                class="monitor-card__dynamic-link"
              >
                {{ dynamicTitle(item) }}
              </a>
              <strong v-else class="monitor-card__dynamic-title">{{ dynamicTitle(item) }}</strong>
              <p v-if="dynamicSummary(item)" class="monitor-card__dynamic-summary">{{ dynamicSummary(item) }}</p>
              <div class="monitor-card__dynamic-footer">
                <small class="monitor-card__dynamic-time">{{ displayTime(item.dynamic.published_at ?? item.dynamic.observed_at) }}</small>
              </div>
            </div>

            <!-- Live facts -->
            <dl class="monitor-card__facts">
              <div>
                <dt>
                  <FieldTimeOutlined class="monitor-card__fact-icon" />
                  {{ t('builtinFeatures.thirdPartyMonitoring.startedAt') }}
                </dt>
                <dd :class="{ 'is-live': item.live.is_live }">{{ liveStartedText(item) }}</dd>
              </div>
              <div>
                <dt>
                  <ClockCircleOutlined class="monitor-card__fact-icon" />
                  {{ t('builtinFeatures.thirdPartyMonitoring.endedAt') }}
                </dt>
                <dd>{{ liveEndedText(item) }}</dd>
              </div>
              <div>
                <dt>
                  <LinkOutlined class="monitor-card__fact-icon" />
                  {{ t('builtinFeatures.thirdPartyMonitoring.connection') }}
                </dt>
                <dd>{{ sourceStatusMeta(item.live.connection_state as BilibiliSourceStatusResponse['status']).label }}</dd>
              </div>
              <div>
                <dt>
                  <ReloadOutlined class="monitor-card__fact-icon" />
                  {{ t('builtinFeatures.thirdPartyMonitoring.updated') }}
                </dt>
                <dd>{{ displayTime(lastRefreshedAt) }}</dd>
              </div>
            </dl>

            <!-- Live error -->
            <p v-if="visibleLiveError(item)" class="monitor-card__error">
              {{ visibleLiveError(item) }}
            </p>
          </div>
        </article>
      </section>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
/* ── Actions bar ── */
.monitoring-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: var(--space-sm);
  flex-wrap: wrap;
}

/* ── Page grid ── */
.third-party-monitoring {
  display: grid;
  gap: var(--space-md);
  min-width: 0;
}

/* ── Status bar ── */
.monitoring-strip {
  position: relative;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);

  &--success {
    border-color: color-mix(in srgb, var(--success) 18%, var(--border));
    background: linear-gradient(
      135deg,
      color-mix(in srgb, var(--success) 3%, var(--surface-strong)) 0%,
      var(--surface-strong) 80%
    );
  }

  &--warning {
    border-color: color-mix(in srgb, var(--warning) 24%, var(--border));
    background: linear-gradient(
      135deg,
      color-mix(in srgb, var(--warning) 4%, var(--surface-strong)) 0%,
      var(--surface-strong) 80%
    );
  }

  &--danger {
    border-color: color-mix(in srgb, var(--danger) 24%, var(--border));
    background: linear-gradient(
      135deg,
      color-mix(in srgb, var(--danger) 4%, var(--surface-strong)) 0%,
      var(--surface-strong) 80%
    );
  }

  &--normal {
    border-color: color-mix(in srgb, var(--accent) 18%, var(--border));
    background: linear-gradient(
      135deg,
      color-mix(in srgb, var(--accent) 3%, var(--surface-strong)) 0%,
      var(--surface-strong) 80%
    );
  }
}

.monitoring-strip__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  padding: var(--space-md);
  min-height: 48px;
}

.monitoring-strip__left {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-width: 0;
  flex: 1 1 auto;
}

.monitoring-strip__status {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

/* Pulse animation wrapper */
.monitoring-strip__pulse {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  flex: 0 0 auto;

  &::before {
    content: '';
    position: absolute;
    inset: -3px;
    border-radius: 50%;
    opacity: 0;
    animation: status-pulse 2.4s ease-in-out infinite;
  }

  &--green::before  { background: var(--success); }
  &--blue::before   { background: var(--accent); }
  &--orange::before { background: var(--warning); }
  &--red::before    { background: var(--danger); }
  &--default::before { background: var(--muted); }
}

@keyframes status-pulse {
  0%   { transform: scale(0.6); opacity: 0; }
  30%  { transform: scale(1); opacity: 0.25; }
  60%  { transform: scale(1.4); opacity: 0; }
  100% { transform: scale(1.4); opacity: 0; }
}

@media (prefers-reduced-motion: reduce) {
  .monitoring-strip__pulse::before,
  .monitoring-strip__live-dot {
    animation: none;
  }
}

.monitoring-strip__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: 0 0 auto;
  background: var(--muted);
  position: relative;
  z-index: 1;

  &--green  { background: var(--success); }
  &--blue   { background: var(--accent); }
  &--orange { background: var(--warning); }
  &--red    { background: var(--danger); }
  &--default { background: var(--muted); }
}

.monitoring-strip__label {
  font-weight: 700;
  font-size: 0.9rem;
  color: var(--text);
  flex: 0 0 auto;
  letter-spacing: -0.01em;
}

.monitoring-strip__summary {
  color: var(--muted);
  font-size: 0.82rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  padding-left: 6px;
  border-left: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.monitoring-strip__right {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex: 0 0 auto;
  min-width: 0;
}

/* Realtime indicator */
.monitoring-strip__live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
  color: var(--muted);
  font-size: 0.74rem;
  white-space: nowrap;
}

.monitoring-strip__live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex: 0 0 auto;
  background: var(--success);
  animation: live-dot-breathe 2.4s ease-in-out infinite;
}

.monitoring-strip__live--live {
  color: var(--success);

  .monitoring-strip__live-dot {
    background: var(--success);
    animation: live-dot-breathe 2.4s ease-in-out infinite;
  }
}

.monitoring-strip__live--connecting {
  color: var(--accent);

  .monitoring-strip__live-dot {
    background: var(--accent);
    animation: none;
  }
}

.monitoring-strip__live--reconnecting {
  color: var(--warning);

  .monitoring-strip__live-dot {
    background: var(--warning);
    animation: live-dot-breathe 1.2s ease-in-out infinite;
  }
}

.monitoring-strip__live--error {
  color: var(--danger);

  .monitoring-strip__live-dot {
    background: var(--danger);
    animation: none;
  }
}

.monitoring-strip__live--disconnected {
  color: var(--muted);

  .monitoring-strip__live-dot {
    background: var(--muted);
    animation: none;
  }
}

@keyframes live-dot-breathe {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.35; }
}

/* Metric badges */
.monitoring-strip__metrics {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
}

.metric-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  font-size: 0.76rem;
  color: var(--muted);
  white-space: nowrap;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

  &:hover {
    border-color: var(--border-strong);
    background: color-mix(in srgb, var(--text) 4%, var(--surface-soft));
    color: var(--text);
  }

  &__icon {
    font-size: 0.82rem;
    opacity: 0.8;
  }

  &__value {
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    color: var(--text);
    transition: color 0.25s ease;
  }

  &__label {
    font-size: 0.72rem;
    opacity: 0.85;
  }

  &--warning {
    border-color: color-mix(in srgb, var(--warning) 25%, var(--border));
    background: color-mix(in srgb, var(--warning) 8%, var(--surface-soft));
    color: var(--warning);

    .metric-badge__value {
      color: var(--warning);
    }

    &:hover {
      border-color: var(--warning);
      background: color-mix(in srgb, var(--warning) 12%, var(--surface-soft));
      color: var(--warning);
    }
  }
}

.monitoring-strip__toggle {
  color: var(--muted);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.2s ease, color 0.2s ease;
  flex: 0 0 auto;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: none;
  background: transparent;
  cursor: pointer;

  &:hover {
    background-color: var(--surface-soft);
    color: var(--text);
  }

  &.is-expanded {
    transform: rotate(180deg);
  }
}

/* Expandable detail */
.monitoring-strip__detail {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.28s ease, visibility 0.28s ease;
  overflow: hidden;
  visibility: hidden;

  &.is-open {
    grid-template-rows: 1fr;
    visibility: visible;
  }
}

.monitoring-strip__detail-inner {
  overflow: hidden;
  display: grid;
  gap: var(--space-md);
  padding: 0 var(--space-md) var(--space-md);
  min-height: 0;
}

/* Diagnosis Grid */
.diagnosis-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-md);
  align-items: start;
}

@media (min-width: 960px) {
  .diagnosis-grid:not(.diagnosis-grid--single-col) {
    grid-template-columns: 1.6fr 1fr;
    gap: var(--space-lg);
  }
}

.diagnosis-grid__col-meta {
  display: grid;
  gap: var(--space-md);
}

/* Diagnosis sections */
.diagnosis-section {
  display: grid;
  gap: var(--space-sm);
}

.diagnosis-section__header {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.diagnosis-section__header-icon {
  font-size: 0.88rem;
  opacity: 0.8;
}

.diagnosis-section__header-title {
  padding-top: 1px;
}

/* Cause cards */
.diagnosis-causes {
  display: grid;
  gap: var(--space-sm);
}

.diagnosis-cause-card {
  display: flex;
  align-items: flex-start;
  gap: var(--space-sm);
  padding: var(--space-md);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  min-width: 0;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);

  &:hover {
    border-color: var(--border-strong);
    background: color-mix(in srgb, var(--text) 2%, var(--surface-soft));
  }

  &__icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--success) 8%, var(--surface));
    color: var(--success);
    font-size: 0.92rem;
    flex: 0 0 auto;
  }

  &__content {
    flex: 1 1 auto;
    min-width: 0;

    strong {
      display: block;
      color: var(--text);
      font-size: 0.86rem;
      font-weight: 650;
      line-height: 1.4;
    }

    p {
      margin: 3px 0 0;
      color: var(--muted);
      font-size: 0.78rem;
      line-height: 1.5;
    }
  }
}

.monitoring-strip--warning {
  .diagnosis-cause-card {
    border-color: color-mix(in srgb, var(--warning) 12%, var(--border));
    background: color-mix(in srgb, var(--warning) 2%, var(--surface-soft));
  }
  .diagnosis-cause-card__icon {
    background: color-mix(in srgb, var(--warning) 8%, var(--surface));
    color: var(--warning);
  }
}

.monitoring-strip--danger {
  .diagnosis-cause-card {
    border-color: color-mix(in srgb, var(--danger) 12%, var(--border));
    background: color-mix(in srgb, var(--danger) 2%, var(--surface-soft));
  }
  .diagnosis-cause-card__icon {
    background: color-mix(in srgb, var(--danger) 8%, var(--surface));
    color: var(--danger);
  }
}

/* Impact tags */
.diagnosis-impact-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.impact-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--success) 5%, var(--surface-soft));
  border: 1px solid color-mix(in srgb, var(--success) 15%, var(--border));
  color: color-mix(in srgb, var(--success) 85%, var(--text));
  font-size: 0.76rem;
  line-height: 1.4;
  transition: all 0.2s ease;

  &__icon {
    font-size: 0.82rem;
    color: var(--success);
  }
}

.monitoring-strip--warning .impact-tag {
  background: color-mix(in srgb, var(--warning) 5%, var(--surface-soft));
  border-color: color-mix(in srgb, var(--warning) 15%, var(--border));
  color: color-mix(in srgb, var(--warning) 85%, var(--text));

  .impact-tag__icon {
    color: var(--warning);
  }
}

.monitoring-strip--danger .impact-tag {
  background: color-mix(in srgb, var(--danger) 5%, var(--surface-soft));
  border-color: color-mix(in srgb, var(--danger) 15%, var(--border));
  color: color-mix(in srgb, var(--danger) 85%, var(--text));

  .impact-tag__icon {
    color: var(--danger);
  }
}

/* Action tags */
.diagnosis-action-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.action-tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: 0.76rem;
  line-height: 1.4;
}

.diagnosis-action-btn {
  border-radius: var(--radius-md);
  margin-left: 2px;
}

/* ── UID strip ── */
.uid-strip {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-width: 0;
}

.uid-strip__label {
  color: var(--muted);
  font-size: 0.76rem;
  flex: 0 0 auto;
}

.uid-strip__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.uid-strip__tags :deep(.ant-tag) {
  margin-inline-end: 0;
}

/* ── Card grid ── */
.monitor-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 480px), 1fr));
  gap: var(--space-md);
  align-items: start;
}

/* ── Monitor card ── */
.monitor-card {
  display: grid;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
  transition: transform 0.22s ease, box-shadow 0.22s ease, border-color 0.22s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-elevated);
    border-color: var(--border-strong);
  }
}

/* ── Cover area ── */
.monitor-card__cover-wrap {
  position: relative;
  min-width: 0;
}

.monitor-card__cover {
  position: relative;
  display: grid;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-soft));
}

.monitor-card__cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.4s ease;

  .monitor-card:hover & {
    transform: scale(1.04);
  }
}

/* Gaussian blur extension layer */
.monitor-card__cover-blur {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
}

.monitor-card__cover-blur-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  filter: blur(18px) saturate(1.3);
  opacity: 0.52;
  transform: scale(1.08);
  mask-image: linear-gradient(to bottom, transparent 60%, rgba(0,0,0,0.85) 78%, rgba(0,0,0,1) 100%);
  -webkit-mask-image: linear-gradient(to bottom, transparent 60%, rgba(0,0,0,0.85) 78%, rgba(0,0,0,1) 100%);
}

/* Live info overlay */
.monitor-card__cover-info {
  --cover-overlay-strong: rgba(0, 0, 0, 0.62);
  --cover-overlay-soft: rgba(0, 0, 0, 0.28);
  --cover-overlay-text: #fff;
  position: absolute;
  z-index: 2;
  inset: auto 0 0;
  padding: 28px var(--space-md) var(--space-md);
  display: grid;
  gap: 4px;
  background: linear-gradient(to top, var(--cover-overlay-strong) 0%, var(--cover-overlay-soft) 50%, transparent 100%);
}

.monitor-card__cover-info :deep(.ant-tag) {
  margin-inline-end: 0;
  justify-self: start;
  backdrop-filter: blur(6px);
  transition: color 0.25s ease, background-color 0.25s ease, border-color 0.25s ease;
}

.monitor-card__cover-room-name {
  margin: 0;
  font-size: 0.96rem;
  font-weight: 700;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--cover-overlay-text);
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);

  a {
    color: inherit;

    &:hover {
      text-decoration: underline;
    }
  }
}

.monitor-card__cover-room-id {
  color: color-mix(in srgb, var(--cover-overlay-text) 78%, transparent);
  font-size: 0.76rem;
  font-variant-numeric: tabular-nums;
}

/* Cover fallback */
.monitor-card__cover--fallback {
  display: grid;
  place-items: center;
}

.monitor-card__cover-fb-inner {
  display: grid;
  justify-items: center;
  gap: var(--space-sm);
  color: var(--muted);
  font-size: 1.6rem;

  span {
    font-size: 0.84rem;
    font-family: var(--font-sans);
  }
}

/* ── Card body ── */
.monitor-card__body {
  display: grid;
  gap: var(--space-md);
  padding: var(--space-md);
  min-width: 0;
}

.monitor-card__identity {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-width: 0;
}

.monitor-card__identity-text {
  display: grid;
  min-width: 0;
  flex: 1 1 auto;

  strong,
  .monitor-card__profile-link {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-size: 0.94rem;
    font-weight: 700;
  }

  .monitor-card__profile-link:hover {
    color: var(--accent);
  }

  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--muted);
    font-size: 0.74rem;
  }
}

.monitor-card__services {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  flex: 0 0 auto;

  :deep(.ant-tag) {
    margin-inline-end: 0;
  }
}

.monitor-avatar {
  flex: 0 0 auto;
  background: color-mix(in srgb, var(--accent) 16%, var(--surface-soft));
  color: var(--accent);
}

.monitor-avatar :deep(.ant-avatar-string) {
  inset: 0 !important;
  display: block;
  width: 100%;
  height: 100%;
  line-height: inherit;
  transform: none !important;
}

.monitor-avatar__image {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

/* Dynamic */
.monitor-card__dynamic {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: var(--space-sm) var(--space-md);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.monitor-card__dynamic-label {
  color: var(--muted);
  font-size: 0.72rem;
}

.monitor-card__dynamic-link,
.monitor-card__dynamic-title {
  color: var(--text);
  font-weight: 650;
  font-size: 0.86rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-card__dynamic-summary {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.8rem;
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.monitor-card__dynamic-time {
  color: var(--muted);
  font-size: 0.72rem;
}

/* Facts */
.monitor-card__facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-sm);
  margin: 0;
}

.monitor-card__facts div {
  min-width: 0;
}

.monitor-card__facts dt {
  color: var(--muted);
  font-size: 0.72rem;
}

.monitor-card__facts dd {
  margin: 2px 0 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text);
  font-size: 0.82rem;
  font-weight: 500;
}

/* Error */
.monitor-card__error {
  margin: 0;
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--danger) 7%, var(--surface));
  color: var(--danger);
  font-size: 0.76rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

/* ── Responsive ── */
@media (max-width: 960px) {
  .monitoring-strip__row {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-sm);
  }

  .monitoring-strip__right {
    width: 100%;
    justify-content: space-between;
  }

  .monitor-card-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .monitoring-actions,
  .uid-strip {
    flex-direction: column;
    align-items: stretch;
  }

  .monitoring-actions :deep(.ant-btn) {
    width: 100%;
  }

  .monitoring-strip__row {
    gap: 8px;
    padding: 10px var(--space-sm) 10px calc(var(--space-sm) + 3px);
  }

  .monitoring-strip__left {
    flex-wrap: wrap;
  }

  .monitoring-strip__summary {
    width: 100%;
    padding-left: 0;
    border-left: none;
    padding-top: 2px;
  }

  .monitoring-strip__right {
    width: 100%;
    justify-content: space-between;
  }

  .monitoring-strip__metrics {
    flex-wrap: wrap;
  }

  .metric-badge {
    padding: 2px 8px;
    font-size: 0.72rem;
  }

  .monitoring-strip__detail-inner {
    padding: 0 var(--space-sm) var(--space-sm) calc(var(--space-sm) + 3px);
    gap: var(--space-sm);
  }

  .diagnosis-cause-card {
    padding: 8px 10px;

    &__icon {
      width: 24px;
      height: 24px;
      font-size: 0.84rem;
    }

    &__content strong {
      font-size: 0.82rem;
    }

    &__content p {
      font-size: 0.74rem;
    }
  }

  .monitor-card__facts {
    grid-template-columns: 1fr;
  }

  .monitor-card__identity {
    flex-wrap: wrap;
  }

  .monitor-card__services {
    width: 100%;
  }
}

/* Skeleton */
.monitoring-skeleton {
  display: grid;
  gap: var(--space-md);
}

.monitoring-skeleton__strip {
  height: 56px;
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  border: 1px solid var(--border);
  overflow: hidden;
  position: relative;

  &::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent 0%,
      color-mix(in srgb, var(--text) 4%, transparent) 50%,
      transparent 100%
    );
    animation: skeleton-shimmer 1.6s ease-in-out infinite;
  }
}

.monitoring-skeleton__cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 480px), 1fr));
  gap: var(--space-md);
}

.monitoring-skeleton__card {
  display: grid;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  overflow: hidden;
}

.monitoring-skeleton__cover {
  aspect-ratio: 16 / 9;
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-soft));
  position: relative;
  overflow: hidden;

  &::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent 0%,
      color-mix(in srgb, var(--text) 4%, transparent) 50%,
      transparent 100%
    );
    animation: skeleton-shimmer 1.6s ease-in-out infinite;
  }
}

.monitoring-skeleton__body {
  display: grid;
  gap: var(--space-sm);
  padding: var(--space-md);
}

.monitoring-skeleton__identity {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.monitoring-skeleton__avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--accent) 12%, var(--surface-soft));
  flex: 0 0 auto;
}

.monitoring-skeleton__name {
  height: 16px;
  width: 120px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.monitoring-skeleton__line {
  height: 12px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);

  &--short {
    width: 60%;
  }
}

@keyframes skeleton-shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

@media (prefers-reduced-motion: reduce) {
  .monitoring-skeleton__strip::after,
  .monitoring-skeleton__cover::after {
    animation: none;
  }
}

/* ── Status bar reconnecting & refresh states ── */
.monitoring-strip.is-reconnecting {
  border-style: dashed;
  border-color: color-mix(in srgb, var(--warning) 30%, var(--border));

  .metric-badge {
    opacity: 0.6;
  }
}

.monitoring-strip.is-refreshed {
  animation: strip-refresh-pulse 0.6s ease;
}

@keyframes strip-refresh-pulse {
  0% { filter: brightness(1); }
  40% { filter: brightness(1.06); }
  100% { filter: brightness(1); }
}

/* ── Metric badge bump ── */
.metric-badge.is-bumped .metric-badge__value {
  animation: metric-bump 0.5s ease;
}

@keyframes metric-bump {
  0% { transform: scale(1); }
  30% { transform: scale(1.15); color: var(--accent); }
  100% { transform: scale(1); }
}

/* ── Card tone coloring ── */
.monitor-card {
  position: relative;

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 3px;
    border-radius: var(--radius-lg) var(--radius-lg) 0 0;
    background: var(--border);
    z-index: 3;
    transition: background 0.3s ease;
  }
}

.monitor-card--live-dynamic::before {
  background: linear-gradient(90deg, var(--success) 0%, var(--accent) 100%);
}

.monitor-card--live-only::before {
  background: var(--success);
}

.monitor-card--dynamic-only::before {
  background: var(--accent);
}

.monitor-card--error::before {
  background: var(--danger);
}

.monitor-card--idle::before {
  background: var(--muted);
}

/* ── Card refresh flash ── */
.monitor-card.is-refreshed {
  animation: card-refresh-flash 0.6s ease;
}

@keyframes card-refresh-flash {
  0% { border-color: var(--border); }
  40% { border-color: color-mix(in srgb, var(--accent) 50%, var(--border)); }
  100% { border-color: var(--border); }
}

/* ── Card reconnecting state ── */
.monitor-card.is-reconnecting {
  &::before {
    background: var(--warning);
    animation: reconnecting-dash 1.2s linear infinite;
  }
}

@keyframes reconnecting-dash {
  0% { opacity: 0.5; }
  50% { opacity: 1; }
  100% { opacity: 0.5; }
}

/* ── Card enter animation ── */
.monitor-card {
  opacity: 0;
  transform: translateY(12px);
  transition:
    opacity 0.35s ease,
    transform 0.35s ease,
    box-shadow 0.22s ease,
    border-color 0.22s ease;

  &.is-entered {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .monitor-card {
    opacity: 1;
    transform: none;
    transition: box-shadow 0.22s ease, border-color 0.22s ease;
  }
}

/* ── Cover image load fade ── */
.monitor-card__cover-img {
  opacity: 0;
  transition: opacity 0.35s ease, transform 0.4s ease;

  &.is-loaded {
    opacity: 1;
  }
}

.monitor-card__cover-blur-img {
  opacity: 0;
  transition: opacity 0.35s ease;

  &.is-loaded {
    opacity: 0.52;
  }
}

.monitor-card__cover-skeleton {
  position: absolute;
  inset: 0;
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-soft));
  overflow: hidden;

  &::after {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent 0%,
      color-mix(in srgb, var(--text) 3%, transparent) 50%,
      transparent 100%
    );
    animation: skeleton-shimmer 1.6s ease-in-out infinite;
  }
}

/* ── Service tag colors ── */
.service-tag {
  transition: all 0.2s ease;

  &--live {
    background: color-mix(in srgb, var(--accent) 8%, var(--surface-soft));
    border-color: color-mix(in srgb, var(--accent) 20%, var(--border));
    color: color-mix(in srgb, var(--accent) 80%, var(--text));
  }

  &--video {
    background: color-mix(in srgb, var(--success) 8%, var(--surface-soft));
    border-color: color-mix(in srgb, var(--success) 20%, var(--border));
    color: color-mix(in srgb, var(--success) 80%, var(--text));
  }

  &--image_text {
    background: color-mix(in srgb, var(--warning) 8%, var(--surface-soft));
    border-color: color-mix(in srgb, var(--warning) 20%, var(--border));
    color: color-mix(in srgb, var(--warning) 80%, var(--text));
  }

  &--article {
    background: color-mix(in srgb, var(--info) 8%, var(--surface-soft));
    border-color: color-mix(in srgb, var(--info) 20%, var(--border));
    color: color-mix(in srgb, var(--info) 80%, var(--text));
  }

  &--repost {
    background: color-mix(in srgb, var(--muted) 8%, var(--surface-soft));
    border-color: color-mix(in srgb, var(--muted) 20%, var(--border));
    color: color-mix(in srgb, var(--muted) 90%, var(--text));
  }
}

/* ── Dynamic section redesign ── */
.monitor-card__dynamic-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}

.monitor-card__dynamic-icon {
  color: var(--accent);
  font-size: 0.82rem;
  opacity: 0.85;
  display: inline-flex;
  align-items: center;
}

.monitor-card__dynamic-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 2px;
}

/* ── Live facts icon ── */
.monitor-card__fact-icon {
  font-size: 0.78rem;
  opacity: 0.65;
  margin-right: 4px;
  vertical-align: -0.1em;
}

.monitor-card__facts dd.is-live {
  color: var(--accent);
  font-weight: 600;
}
</style>
