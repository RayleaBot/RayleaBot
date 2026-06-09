<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import {
  FieldTimeOutlined,
  ReloadOutlined,
  SyncOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { useThirdPartyMonitoringStore } from '@/stores/third-party-monitoring'
import type {
  BilibiliSourceStatusResponse,
  ThirdPartyMonitorItem,
  ThirdPartyMonitorService,
} from '@/types/api'

type StatusTone = 'normal' | 'success' | 'warning' | 'danger'

interface PlatformOption {
  label: string
  value: string
  disabled?: boolean
}

const store = useThirdPartyMonitoringStore()
const router = useRouter()
const {
  bilibiliStatus,
  error,
  items,
  loading,
  monitors,
  platform,
  restarting,
} = storeToRefs(store)

const avatarLoadFailures = reactive<Record<string, boolean>>({})
const coverLoadFailures = reactive<Record<string, boolean>>({})

const platformOptions = computed<PlatformOption[]>(() => [
  { label: 'Bilibili', value: 'bilibili' },
  { label: 'YouTube', value: 'youtube', disabled: true },
  { label: 'Twitch', value: 'twitch', disabled: true },
])
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

useToastFeedback(pageErrorToast)

onMounted(() => {
  void loadPage()
})

onUnmounted(() => {
  store.disposeMedia()
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

function handlePlatformChange(value: string | number) {
  if (value !== 'bilibili') {
    return
  }
  platform.value = 'bilibili'
  void loadPage()
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

function liveMetricText() {
  const live = bilibiliStatus.value?.live
  if (!live) {
    return t('display.empty')
  }
  if (live.failed_rooms > 0 && live.fallback_polling) {
    return t('builtinFeatures.thirdPartyMonitoring.liveFallbackMetric', { count: live.failed_rooms })
  }
  if (live.failed_rooms > 0 || live.last_error) {
    return t('builtinFeatures.thirdPartyMonitoring.liveFailedMetric', { count: live.failed_rooms })
  }
  return t('builtinFeatures.thirdPartyMonitoring.liveConnected', { count: live.connected_rooms })
}

function dynamicMetricText() {
  const dynamic = bilibiliStatus.value?.dynamic
  if (!dynamic) {
    return t('display.empty')
  }
  if (dynamic.last_error) {
    return t('builtinFeatures.thirdPartyMonitoring.dynamicErrorMetric')
  }
  return t('builtinFeatures.thirdPartyMonitoring.lastPoll', { time: displayTime(dynamic.last_poll_at) })
}

function accountMetricText() {
  return t('builtinFeatures.thirdPartyMonitoring.accountSummary', { count: accountCount.value })
}

function causeRetryText(value?: string | null) {
  return value
    ? t('builtinFeatures.thirdPartyMonitoring.retryAt', { time: displayTime(value) })
    : ''
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
    : displayTime(item.live.live_ended_at ?? item.live.updated_at)
}

function roomName(item: ThirdPartyMonitorItem) {
  return item.live.room_name || t('builtinFeatures.thirdPartyMonitoring.noRoomName')
}

function dynamicTitle(item: ThirdPartyMonitorItem) {
  return item.dynamic?.title || t('builtinFeatures.thirdPartyMonitoring.noDynamic')
}

function dynamicSummary(item: ThirdPartyMonitorItem) {
  return item.dynamic?.summary || t('builtinFeatures.thirdPartyMonitoring.noDynamicSummary')
}

function avatarFailed(uid: string) {
  avatarLoadFailures[uid] = true
}

function coverFailed(uid: string) {
  coverLoadFailures[uid] = true
}
</script>

<template>
  <AppPage :title="t('builtinFeatures.thirdPartyMonitoring.title')" :description="t('builtinFeatures.thirdPartyMonitoring.subtitle')">
    <template #extra>
      <div class="monitoring-actions">
        <a-button :loading="loading" @click="loadPage">
          <template #icon><ReloadOutlined /></template>
          {{ t('builtinFeatures.thirdPartyMonitoring.refresh') }}
        </a-button>
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
      <section :class="['monitoring-overview', `monitoring-overview--${statusTone}`]">
        <div class="monitoring-overview__top">
          <div class="platform-switch">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.platform') }}</span>
            <a-segmented
              :value="platform"
              :options="platformOptions"
              @change="handlePlatformChange"
            />
          </div>

          <div class="status-summary">
            <div>
              <span class="status-title">{{ t('builtinFeatures.thirdPartyMonitoring.sourceTitle') }}</span>
              <h2>{{ diagnosis?.headline || bilibiliStatus?.summary || t('builtinFeatures.thirdPartyMonitoring.sourceWaiting') }}</h2>
              <p>{{ diagnosis?.description || t('builtinFeatures.thirdPartyMonitoring.sourceWaiting') }}</p>
              <small>{{ t('builtinFeatures.thirdPartyMonitoring.updatedAt', { time: displayTime(diagnosis?.updated_at ?? monitors?.updated_at) }) }}</small>
            </div>
            <a-tag :color="statusTag.color">{{ statusTag.label }}</a-tag>
          </div>

          <div class="diagnosis-actions">
            <a-button v-if="openAccountsAction" type="primary" @click="openBilibiliAccounts">
              {{ openAccountsAction.label }}
            </a-button>
            <a-button :loading="loading" @click="loadPage">
              <template #icon><ReloadOutlined /></template>
              {{ t('builtinFeatures.thirdPartyMonitoring.refresh') }}
            </a-button>
            <a-button :loading="restarting" @click="restartSource">
              <template #icon><SyncOutlined /></template>
              {{ t('builtinFeatures.thirdPartyMonitoring.restartSource') }}
            </a-button>
          </div>
        </div>

        <div class="diagnosis-grid">
          <div class="diagnosis-column">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisCause') }}</span>
            <div v-if="diagnosis?.causes.length" class="diagnosis-list">
              <article v-for="cause in diagnosis.causes" :key="`${cause.scope}:${cause.code}:${cause.title}`">
                <strong>{{ cause.title }}</strong>
                <p>{{ cause.detail }}</p>
                <small v-if="causeRetryText(cause.retry_at)">{{ causeRetryText(cause.retry_at) }}</small>
                <small v-if="cause.last_error">{{ cause.last_error }}</small>
              </article>
            </div>
            <p v-else>{{ t('builtinFeatures.thirdPartyMonitoring.noDiagnosisCause') }}</p>
          </div>

          <div class="diagnosis-column">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisImpact') }}</span>
            <ul v-if="diagnosis?.impacts.length" class="diagnosis-points">
              <li v-for="impact in diagnosis.impacts" :key="impact">{{ impact }}</li>
            </ul>
            <p v-else>{{ t('builtinFeatures.thirdPartyMonitoring.noDiagnosisImpact') }}</p>
          </div>

          <div class="diagnosis-column">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisAction') }}</span>
            <ul v-if="diagnosis?.actions.length" class="diagnosis-points">
              <li v-for="action in diagnosis.actions" :key="`${action.kind}:${action.label}`">{{ action.label }}</li>
            </ul>
            <p v-else>{{ t('builtinFeatures.thirdPartyMonitoring.noDiagnosisAction') }}</p>
          </div>
        </div>

        <div class="monitoring-metrics">
          <div class="monitoring-metric">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.accountMetric') }}</span>
            <strong>{{ accountCount }}</strong>
            <small>{{ accountMetricText() }}</small>
          </div>
          <div class="monitoring-metric">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.liveMetric') }}</span>
            <strong>{{ liveCount }}/{{ bilibiliStatus?.live.watched_rooms ?? watchedUIDs.length }}</strong>
            <small>{{ liveMetricText() }}</small>
          </div>
          <div class="monitoring-metric">
            <span>{{ t('builtinFeatures.thirdPartyMonitoring.dynamicMetric') }}</span>
            <strong>{{ dynamicCount }}/{{ bilibiliStatus?.dynamic.watched_uids ?? watchedUIDs.length }}</strong>
            <small>{{ dynamicMetricText() }}</small>
          </div>
        </div>

        <div class="uid-panel">
          <span>{{ t('builtinFeatures.thirdPartyMonitoring.uidList') }}</span>
          <div v-if="watchedUIDs.length" class="uid-list">
            <a-tag v-for="uid in watchedUIDs" :key="uid">UID {{ uid }}</a-tag>
          </div>
          <span v-else class="uid-empty">{{ t('builtinFeatures.thirdPartyMonitoring.noUIDs') }}</span>
        </div>
      </section>

      <a-empty
        v-if="!items.length"
        :description="t('builtinFeatures.thirdPartyMonitoring.empty')"
        class="monitoring-empty"
      />

      <section v-else class="monitor-card-grid">
        <article v-for="item in items" :key="item.uid" class="monitor-card">
          <div class="monitor-card__media">
            <img
              v-if="mainImage(item) && !coverLoadFailures[item.uid]"
              :src="mainImage(item)"
              :alt="roomName(item)"
              @error="coverFailed(item.uid)"
            >
            <div v-else class="monitor-card__media-fallback">
              <FieldTimeOutlined />
            </div>
            <a-tag class="monitor-card__live-tag" :color="liveTag(item).color">{{ liveTag(item).label }}</a-tag>
          </div>

          <div class="monitor-card__body">
            <div class="monitor-card__identity">
              <a-avatar :size="48" class="monitor-avatar">
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
              <div>
                <strong>{{ item.username || item.uid }}</strong>
                <span>UID {{ item.uid }}</span>
              </div>
            </div>

            <div class="monitor-card__services">
              <a-tag v-for="service in item.services" :key="service">{{ serviceLabel(service) }}</a-tag>
            </div>

            <div class="monitor-card__dynamic">
              <span>{{ t('builtinFeatures.thirdPartyMonitoring.dynamicTitle') }}</span>
              <a
                v-if="item.dynamic?.url"
                :href="item.dynamic.url"
                target="_blank"
                rel="noreferrer"
              >
                {{ dynamicTitle(item) }}
              </a>
              <strong v-else>{{ dynamicTitle(item) }}</strong>
              <p>{{ dynamicSummary(item) }}</p>
              <small>{{ displayTime(item.dynamic?.published_at ?? item.dynamic?.observed_at) }}</small>
            </div>

            <dl class="monitor-card__facts">
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.roomName') }}</dt>
                <dd>
                  <a
                    v-if="item.live.room_url"
                    :href="item.live.room_url"
                    target="_blank"
                    rel="noreferrer"
                  >
                    {{ roomName(item) }}
                  </a>
                  <span v-else>{{ roomName(item) }}</span>
                </dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.roomId') }}</dt>
                <dd>{{ item.live.room_id || t('display.empty') }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.startedAt') }}</dt>
                <dd>{{ liveStartedText(item) }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.endedAt') }}</dt>
                <dd>{{ liveEndedText(item) }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.connection') }}</dt>
                <dd>{{ sourceStatusMeta(item.live.connection_state as BilibiliSourceStatusResponse['status']).label }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyMonitoring.updated') }}</dt>
                <dd>{{ displayTime(item.updated_at) }}</dd>
              </div>
            </dl>

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
.monitoring-actions,
.platform-switch,
.status-summary,
.diagnosis-actions,
.uid-panel,
.uid-list,
.monitor-card__identity,
.monitor-card__services {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.monitoring-actions {
  justify-content: flex-end;
  flex-wrap: wrap;
}

.third-party-monitoring {
  display: grid;
  gap: var(--space-lg);
  min-width: 0;
}

.monitoring-overview {
  display: grid;
  gap: var(--space-md);
  min-width: 0;
  padding: var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.monitoring-overview--warning {
  border-color: color-mix(in srgb, #d97706 42%, var(--border));
  background: color-mix(in srgb, #f59e0b 4%, var(--surface-strong));
}

.monitoring-overview--danger {
  border-color: color-mix(in srgb, #dc2626 42%, var(--border));
  background: color-mix(in srgb, #ef4444 4%, var(--surface-strong));
}

.monitoring-overview--success {
  border-color: color-mix(in srgb, #16a34a 24%, var(--border));
}

.monitoring-overview__top {
  display: grid;
  grid-template-columns: minmax(180px, 0.85fr) minmax(320px, 2fr) auto;
  gap: var(--space-md);
  align-items: stretch;
  min-width: 0;
}

.platform-switch,
.status-summary {
  min-width: 0;
  padding: var(--space-md);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.platform-switch {
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}

.platform-switch > span,
.status-title,
.diagnosis-column > span,
.uid-panel > span:first-child,
.monitoring-metric span,
.monitor-card__dynamic span,
.monitor-card__facts dt {
  color: var(--muted);
  font-size: 0.76rem;
}

.status-summary {
  justify-content: space-between;
  align-items: flex-start;
}

.status-summary > div {
  min-width: 0;
}

.status-title {
  display: block;
}

.status-summary h2 {
  margin: 4px 0 0;
  color: var(--text);
  font-size: 1.05rem;
  font-weight: 700;
  line-height: 1.35;
}

.status-summary p {
  max-width: 72ch;
  margin: 5px 0 0;
  color: var(--muted);
  font-size: 0.86rem;
  line-height: 1.55;
}

.status-summary small {
  display: block;
  margin-top: 5px;
  color: var(--muted);
  font-size: 0.74rem;
}

.diagnosis-actions {
  justify-content: flex-end;
  align-content: center;
  align-items: center;
  flex-wrap: wrap;
  min-width: 220px;
}

.diagnosis-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr) minmax(0, 0.85fr);
  gap: var(--space-sm);
  min-width: 0;
}

.diagnosis-column,
.uid-panel {
  min-width: 0;
  padding: var(--space-md);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.diagnosis-column {
  display: grid;
  align-content: start;
  gap: var(--space-sm);
}

.diagnosis-column p,
.diagnosis-points {
  margin: 0;
  color: var(--muted);
  font-size: 0.82rem;
  line-height: 1.5;
}

.diagnosis-list {
  display: grid;
  gap: var(--space-sm);
  min-width: 0;
}

.diagnosis-list article {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.diagnosis-list strong {
  color: var(--text);
  font-size: 0.9rem;
  font-weight: 650;
}

.diagnosis-list small {
  color: var(--muted);
  font-size: 0.74rem;
  overflow-wrap: anywhere;
}

.diagnosis-points {
  display: grid;
  gap: 5px;
  padding-left: 1.05rem;
}

.monitoring-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-sm);
  min-width: 0;
}

.monitoring-metric {
  min-width: 0;
  padding: var(--space-md);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.monitoring-metric strong {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: var(--text);
  font-size: 1.25rem;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitoring-metric small {
  display: block;
  margin-top: 3px;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.uid-panel {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.uid-list {
  flex-wrap: wrap;
  justify-content: flex-end;
  min-width: 0;
}

.uid-list :deep(.ant-tag) {
  margin-inline-end: 0;
}

.uid-empty {
  color: var(--muted);
}

.monitoring-empty {
  padding: var(--space-2xl);
  border: 1px dashed var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
}

.monitor-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 420px), 1fr));
  gap: var(--space-md);
  align-items: start;
}

.monitor-card {
  display: grid;
  grid-template-columns: minmax(152px, 0.42fr) minmax(0, 1fr);
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.monitor-card__media {
  position: relative;
  min-height: 210px;
  overflow: hidden;
  background: color-mix(in srgb, var(--text-accent) 8%, var(--surface-soft));
}

.monitor-card__media img {
  width: 100%;
  height: 100%;
  min-height: 210px;
  object-fit: cover;
}

.monitor-card__media-fallback {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  min-height: 210px;
  color: var(--muted);
  font-size: 1.6rem;
}

.monitor-card__live-tag {
  position: absolute;
  top: var(--space-sm);
  left: var(--space-sm);
  margin-inline-end: 0;
  box-shadow: var(--shadow-card);
}

.monitor-card__body {
  display: grid;
  gap: var(--space-md);
  min-width: 0;
  padding: var(--space-md);
}

.monitor-card__identity {
  min-width: 0;
}

.monitor-card__identity > div {
  display: grid;
  min-width: 0;
}

.monitor-card__identity strong,
.monitor-card__identity span,
.monitor-card__facts dd,
.monitor-card__dynamic a,
.monitor-card__dynamic strong,
.monitor-card__dynamic small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-card__identity strong {
  color: var(--text);
  font-size: 1rem;
  font-weight: 700;
}

.monitor-card__identity span {
  color: var(--muted);
  font-size: 0.78rem;
}

.monitor-avatar {
  flex: 0 0 auto;
  background: color-mix(in srgb, var(--text-accent) 16%, var(--surface-soft));
  color: var(--text-accent);
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

.monitor-card__services {
  flex-wrap: wrap;
}

.monitor-card__services :deep(.ant-tag) {
  margin-inline-end: 0;
}

.monitor-card__dynamic {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: var(--space-sm) var(--space-md);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
}

.monitor-card__dynamic a,
.monitor-card__dynamic strong {
  color: var(--text);
  font-weight: 650;
}

.monitor-card__dynamic p {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  color: var(--muted);
  font-size: 0.82rem;
  line-height: 1.5;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.monitor-card__dynamic small {
  color: var(--muted);
  font-size: 0.74rem;
}

.monitor-card__facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-sm);
  margin: 0;
}

.monitor-card__facts div {
  min-width: 0;
}

.monitor-card__facts dd {
  margin: 2px 0 0;
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 500;
}

.monitor-card__facts a {
  color: var(--text-accent);
}

.monitor-card__error {
  margin: 0;
  padding: 7px 9px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, #ef4444 7%, var(--surface));
  color: #b91c1c;
  font-size: 0.78rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

@media (max-width: 1100px) {
  .monitoring-overview__top,
  .diagnosis-grid {
    grid-template-columns: 1fr;
  }

  .diagnosis-actions {
    justify-content: flex-start;
    min-width: 0;
  }

  .uid-panel {
    grid-column: auto;
  }
}

@media (max-width: 760px) {
  .monitoring-actions,
  .diagnosis-actions,
  .uid-panel,
  .status-summary {
    align-items: stretch;
    flex-direction: column;
  }

  .monitoring-actions :deep(.ant-btn),
  .diagnosis-actions :deep(.ant-btn) {
    width: 100%;
  }

  .monitoring-metrics,
  .monitor-card,
  .monitor-card__facts {
    grid-template-columns: minmax(0, 1fr);
  }

  .monitor-card__media,
  .monitor-card__media img,
  .monitor-card__media-fallback {
    min-height: 180px;
  }

  .uid-list {
    justify-content: flex-start;
  }

  .status-summary h2,
  .monitoring-metric strong,
  .monitoring-metric small,
  .monitor-card__identity strong,
  .monitor-card__identity span,
  .monitor-card__facts dd,
  .monitor-card__dynamic a,
  .monitor-card__dynamic strong,
  .monitor-card__dynamic small {
    white-space: normal;
  }
}
</style>
