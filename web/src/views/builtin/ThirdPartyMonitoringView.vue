<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import {
  CaretDownOutlined,
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
const diagnosisExpanded = ref(false)

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
const hasDiagnosisDetail = computed(() =>
  (diagnosis.value?.causes.length ?? 0) > 0 ||
  (diagnosis.value?.impacts.length ?? 0) > 0 ||
  (diagnosis.value?.actions.length ?? 0) > 0,
)

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

function toggleDiagnosis() {
  diagnosisExpanded.value = !diagnosisExpanded.value
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
      <!-- Slim status strip -->
      <section :class="['monitoring-strip', `monitoring-strip--${statusTone}`]">
        <div class="monitoring-strip__row">
          <div class="monitoring-strip__left">
            <span class="monitoring-strip__dot" :class="`monitoring-strip__dot--${statusTag.color}`" />
            <span class="monitoring-strip__label">{{ statusTag.label }}</span>
            <span class="monitoring-strip__summary">
              {{ diagnosis?.headline || bilibiliStatus?.summary || t('builtinFeatures.thirdPartyMonitoring.sourceWaiting') }}
            </span>
          </div>
          <div class="monitoring-strip__right">
            <span class="monitoring-strip__counts">
              <span>{{ accountCount }} CK</span>
              <span class="monitoring-strip__sep">·</span>
              <span>{{ liveCount }}/{{ bilibiliStatus?.live.watched_rooms ?? watchedUIDs.length }} {{ t('builtinFeatures.thirdPartyMonitoring.liveMetric') }}</span>
              <span class="monitoring-strip__sep">·</span>
              <span>{{ dynamicCount }}/{{ bilibiliStatus?.dynamic.watched_uids ?? watchedUIDs.length }} {{ t('builtinFeatures.thirdPartyMonitoring.dynamicMetric') }}</span>
            </span>
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
            <div v-if="diagnosis?.causes.length" class="diagnosis-chips">
              <span class="diagnosis-chips__label">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisCause') }}</span>
              <article v-for="cause in diagnosis.causes" :key="`${cause.scope}:${cause.code}`" class="diagnosis-chip">
                <strong>{{ cause.title }}</strong>
                <p>{{ cause.detail }}</p>
              </article>
            </div>
            <div v-if="diagnosis?.impacts.length" class="diagnosis-chips">
              <span class="diagnosis-chips__label">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisImpact') }}</span>
              <span v-for="impact in diagnosis.impacts" :key="impact" class="diagnosis-chip diagnosis-chip--inline">{{ impact }}</span>
            </div>
            <div v-if="diagnosisActions.length" class="diagnosis-chips">
              <span class="diagnosis-chips__label">{{ t('builtinFeatures.thirdPartyMonitoring.diagnosisAction') }}</span>
              <span v-for="action in diagnosisActions" :key="`${action.kind}:${action.label}`" class="diagnosis-chip diagnosis-chip--inline">{{ action.label }}</span>
            </div>
            <div v-if="openAccountsAction" class="diagnosis-chips__actions">
              <a-button type="primary" size="small" @click="openBilibiliAccounts">
                {{ openAccountsAction.label }}
              </a-button>
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
      <div v-if="!items.length" class="monitoring-empty">
        <div class="monitoring-empty__inner">
          <FieldTimeOutlined class="monitoring-empty__icon" />
          <p>{{ t('builtinFeatures.thirdPartyMonitoring.empty') }}</p>
          <a-button v-if="openAccountsAction" type="primary" @click="openBilibiliAccounts">
            {{ openAccountsAction.label }}
          </a-button>
        </div>
      </div>

      <!-- Monitor cards -->
      <section v-else class="monitor-card-grid">
        <article v-for="item in items" :key="item.uid" class="monitor-card">
          <!-- Landscape cover with gaussian blur transition -->
          <div class="monitor-card__cover-wrap">
            <div
              v-if="mainImage(item) && !coverLoadFailures[item.uid]"
              class="monitor-card__cover"
            >
              <img
                :src="mainImage(item)"
                :alt="roomName(item)"
                class="monitor-card__cover-img"
                @error="coverFailed(item.uid)"
              >
              <!-- Blurred extension layer -->
              <div class="monitor-card__cover-blur" aria-hidden="true">
                <img
                  :src="mainImage(item)"
                  alt=""
                  class="monitor-card__cover-blur-img"
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
                <span class="monitor-card__cover-room-id">房间 {{ item.live.room_id || '—' }}</span>
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
                <strong>{{ item.username || item.uid }}</strong>
                <span>UID {{ item.uid }}</span>
              </div>
              <div class="monitor-card__services">
                <a-tag v-for="service in item.services" :key="service" size="small">{{ serviceLabel(service) }}</a-tag>
              </div>
            </div>

            <!-- Dynamic -->
            <div v-if="item.dynamic" class="monitor-card__dynamic">
              <span class="monitor-card__dynamic-label">{{ t('builtinFeatures.thirdPartyMonitoring.dynamicTitle') }}</span>
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
              <p class="monitor-card__dynamic-summary">{{ dynamicSummary(item) }}</p>
              <small class="monitor-card__dynamic-time">{{ displayTime(item.dynamic.published_at ?? item.dynamic.observed_at) }}</small>
            </div>

            <!-- Live facts -->
            <dl class="monitor-card__facts">
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

/* ── Slim status strip ── */
.monitoring-strip {
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
  overflow: hidden;
}

.monitoring-strip--warning {
  border-color: color-mix(in srgb, var(--warning) 38%, var(--border));
  background: color-mix(in srgb, var(--warning) 3%, var(--surface-strong));
}

.monitoring-strip--danger {
  border-color: color-mix(in srgb, var(--danger) 38%, var(--border));
  background: color-mix(in srgb, var(--danger) 3%, var(--surface-strong));
}

.monitoring-strip__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  padding: 10px var(--space-md);
  min-height: 44px;
}

.monitoring-strip__left {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  min-width: 0;
  flex: 1 1 auto;
}

.monitoring-strip__right {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex: 0 0 auto;
  min-width: 0;
}

.monitoring-strip__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: 0 0 auto;
  background: var(--muted);

  &--green  { background: var(--success); }
  &--blue   { background: var(--accent); }
  &--orange { background: var(--warning); }
  &--red    { background: var(--danger); }
  &--default { background: var(--muted); }
}

.monitoring-strip__label {
  font-weight: 650;
  font-size: 0.88rem;
  color: var(--text);
  flex: 0 0 auto;
}

.monitoring-strip__summary {
  color: var(--muted);
  font-size: 0.84rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.monitoring-strip__counts {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--muted);
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.monitoring-strip__sep {
  color: color-mix(in srgb, var(--muted) 36%, transparent);
}

.monitoring-strip__toggle {
  color: var(--muted);
  transition: transform 0.25s ease;
  flex: 0 0 auto;

  &.is-expanded {
    transform: rotate(180deg);
  }
}

/* Expandable detail */
.monitoring-strip__detail {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.28s ease;

  &.is-open {
    grid-template-rows: 1fr;
  }
}

.monitoring-strip__detail-inner {
  overflow: hidden;
  display: grid;
  gap: var(--space-sm);
  padding: 0 var(--space-md) var(--space-md);
}

.diagnosis-chips {
  display: flex;
  align-items: flex-start;
  gap: var(--space-sm);
  flex-wrap: wrap;
  min-width: 0;
}

.diagnosis-chips__label {
  color: var(--muted);
  font-size: 0.74rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding-top: 3px;
  flex: 0 0 auto;
}

.diagnosis-chip {
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  border: 1px solid color-mix(in srgb, var(--border) 72%, transparent);
  min-width: 0;

  strong {
    display: block;
    color: var(--text);
    font-size: 0.84rem;
    font-weight: 650;
  }

  p {
    margin: 2px 0 0;
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.45;
  }
}

.diagnosis-chip--inline {
  color: var(--muted);
  font-size: 0.8rem;
}

.diagnosis-chips__actions {
  padding-top: 2px;
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

/* ── Empty state ── */
.monitoring-empty {
  display: grid;
  place-items: center;
  padding: var(--space-2xl) var(--space-lg);
  border: 1px dashed var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
}

.monitoring-empty__inner {
  display: grid;
  justify-items: center;
  gap: var(--space-md);
  text-align: center;
}

.monitoring-empty__icon {
  font-size: 2rem;
  color: var(--muted);
}

.monitoring-empty__inner p {
  margin: 0;
  color: var(--muted);
  font-size: 0.92rem;
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
  position: absolute;
  z-index: 2;
  inset: auto 0 0;
  padding: 28px var(--space-md) var(--space-md);
  display: grid;
  gap: 4px;
  background: linear-gradient(to top, rgba(0,0,0,0.62) 0%, rgba(0,0,0,0.28) 50%, transparent 100%);
}

.monitor-card__cover-info :deep(.ant-tag) {
  margin-inline-end: 0;
  justify-self: start;
  backdrop-filter: blur(6px);
}

.monitor-card__cover-room-name {
  margin: 0;
  font-size: 0.96rem;
  font-weight: 700;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #fff;
  text-shadow: 0 1px 3px rgba(0,0,0,0.5);

  a {
    color: inherit;

    &:hover {
      text-decoration: underline;
    }
  }
}

.monitor-card__cover-room-id {
  color: rgba(255,255,255,0.78);
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

  strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
    font-size: 0.94rem;
    font-weight: 700;
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
  }

  .monitoring-strip__counts {
    font-size: 0.74rem;
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
    gap: 6px;
    padding: 10px var(--space-sm);
  }

  .monitoring-strip__left {
    flex-wrap: wrap;
  }

  .monitoring-strip__right {
    width: 100%;
    justify-content: space-between;
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
</style>
