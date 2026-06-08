<script setup lang="ts">
import { computed, reactive, ref, onBeforeUnmount, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import {
  DeleteOutlined,
  EditOutlined,
  LinkOutlined,
  PlusOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  SaveOutlined,
  SyncOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { useThirdPartyAccountsStore } from '@/stores/third-party-accounts'
import type {
  BilibiliQRCodeLoginCreateResponse,
  BilibiliQRCodeLoginPollResponse,
  BilibiliQRCodeLoginState,
  BilibiliSourceStatusResponse,
  ThirdPartyAccountSummary,
  ThirdPartyCredentialState,
} from '@/types/api'

interface AccountDraft {
  account_id: string
  label: string
  enabled: boolean
  configured: boolean
  cookie: string
  isNew?: boolean
}

interface AccountDraftEntry {
  key: string
  draft: AccountDraft
}

type SourceTone = 'normal' | 'success' | 'warning' | 'danger'

interface SourceMetric {
  label: string
  detail?: string
  tone?: SourceTone
  value: string
}

interface QRLoginState {
  loginId: string
  qrcodeUrl: string
  expiresAt: string
  state: BilibiliQRCodeLoginState
  cookie: string
  accountNickname: string
  accountUid: string
}

const qrPollIntervalMs = 2000

const store = useThirdPartyAccountsStore()
const {
  bilibiliAccounts,
  bilibiliStatus,
  deletingAccountId,
  error,
  loading,
  qrcodeCreating,
  qrcodePollingLoginId,
  restarting,
  savingAccountId,
} = storeToRefs(store)

const drafts = reactive<Record<string, AccountDraft>>({})
const qrLogins = reactive<Record<string, QRLoginState>>({})
const avatarLoadFailures = reactive<Record<string, boolean>>({})
const editingAccountKey = ref<string>('')
const draftSequence = ref(0)
let qrPollTimer: number | undefined

const pageErrorToast = computed(() => (
  error.value && bilibiliStatus.value
    ? {
        key: `third-party-accounts-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))

useToastFeedback(pageErrorToast)

const fatalError = computed(() => error.value && !bilibiliStatus.value && bilibiliAccounts.value.length === 0)
const status = computed(() => bilibiliStatus.value)
const sourceStatusTag = computed(() => sourceStatusMeta(status.value?.status))
const hasAccounts = computed(() => bilibiliAccounts.value.length > 0)
const configuredAccountCount = computed(() => bilibiliAccounts.value.filter((account) => account.configured).length)
const enabledAccountCount = computed(() => bilibiliAccounts.value.filter((account) => account.enabled).length)
const activeDraftEntries = computed<AccountDraftEntry[]>(() => Object.entries(drafts)
  .filter(([, draft]) => draft.isNew)
  .map(([key, draft]) => ({ key, draft })))
const hasEditorCards = computed(() => activeDraftEntries.value.length > 0)
const sourceErrorText = computed(() => {
  const liveError = status.value?.live.last_error?.trim()
  const dynamicError = status.value?.dynamic.last_error?.trim()
  return liveError || dynamicError || t('builtinFeatures.thirdPartyAccounts.noError')
})
const statusTone = computed<SourceTone>(() => {
  switch (status.value?.status) {
    case 'connected':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'normal'
  }
})
const sourceMetrics = computed<SourceMetric[]>(() => {
  const failedRooms = status.value?.live.failed_rooms ?? 0
  return [
    {
      label: t('builtinFeatures.thirdPartyAccounts.liveMetric'),
      value: `${status.value?.live.connected_rooms ?? 0}/${status.value?.live.watched_rooms ?? 0}`,
      detail: status.value?.live.fallback_polling
        ? t('builtinFeatures.thirdPartyAccounts.liveFallbackMetric', { count: failedRooms })
        : t('builtinFeatures.thirdPartyAccounts.liveFailedMetric', { count: failedRooms }),
      tone: failedRooms > 0 || status.value?.live.fallback_polling ? 'warning' : 'normal',
    },
    {
      label: t('builtinFeatures.thirdPartyAccounts.dynamicMetric'),
      value: `${status.value?.dynamic.watched_uids ?? 0} UID`,
      detail: t('builtinFeatures.thirdPartyAccounts.lastPollMetric', { time: timeText(status.value?.dynamic.last_poll_at) }),
      tone: status.value?.dynamic.enabled === false ? 'warning' : 'normal',
    },
    {
      label: t('builtinFeatures.thirdPartyAccounts.accountMetric'),
      value: `${enabledAccountCount.value}/${bilibiliAccounts.value.length}`,
      detail: t('builtinFeatures.thirdPartyAccounts.configuredMetric', { count: configuredAccountCount.value }),
      tone: enabledAccountCount.value > 0 ? 'success' : 'normal',
    },
  ]
})

onMounted(() => {
  void loadPage()
})

onBeforeUnmount(() => {
  stopQRPolling()
})

async function loadPage() {
  try {
    await store.fetchAll()
  } catch {
    // store error state drives the page
  }
}

function draftFromAccount(account: ThirdPartyAccountSummary): AccountDraft {
  return {
    account_id: account.account_id,
    label: account.label || account.profile?.nickname || account.account_id,
    enabled: account.enabled,
    configured: account.configured,
    cookie: '',
  }
}

function addDraft() {
  const key = newDraftKey()
  drafts[key] = {
    account_id: nextAccountId(),
    label: 'Bilibili CK',
    enabled: true,
    configured: false,
    cookie: '',
    isNew: true,
  }
  editingAccountKey.value = key
}

function beginEdit(account: ThirdPartyAccountSummary) {
  const key = accountKey(account.account_id)
  drafts[key] = draftFromAccount(account)
  editingAccountKey.value = key
}

function cancelEdit(key: string) {
  delete drafts[key]
  delete qrLogins[key]
  if (editingAccountKey.value === key) {
    editingAccountKey.value = ''
  }
}

function isEditing(key: string) {
  return editingAccountKey.value === key
}

async function saveDraft(key: string) {
  const draft = drafts[key]
  if (!draft) {
    return
  }
  const accountId = normalizeAccountId(draft.account_id)
  if (!accountId) {
    notifyError(t('builtinFeatures.thirdPartyAccounts.accountIdRequired'))
    return
  }
  const label = draft.label.trim()
  if (!label) {
    notifyError(t('builtinFeatures.thirdPartyAccounts.labelRequired'))
    return
  }
  if (draft.isNew && bilibiliAccounts.value.some((account) => account.account_id === accountId)) {
    notifyError(t('builtinFeatures.thirdPartyAccounts.accountIdExists'))
    return
  }
  draft.account_id = accountId
  const cookie = draft.cookie.trim()
  if (!draft.configured && !cookie) {
    notifyError(t('builtinFeatures.thirdPartyAccounts.cookieRequired'))
    return
  }
  try {
    await store.saveBilibiliAccount(accountId, {
      label,
      enabled: draft.enabled,
      ...(cookie ? { cookie } : {}),
    })
    cancelEdit(key)
    notifySuccess(t('builtinFeatures.thirdPartyAccounts.saved'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

async function deleteAccount(account: ThirdPartyAccountSummary) {
  try {
    await store.deleteBilibiliAccount(account.account_id)
    cancelEdit(accountKey(account.account_id))
    notifySuccess(t('builtinFeatures.thirdPartyAccounts.deleted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function deleteDraft(key: string) {
  cancelEdit(key)
}

async function restartSource() {
  try {
    await store.restartBilibiliSource()
    notifySuccess(t('builtinFeatures.thirdPartyAccounts.restarted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

async function startQRCodeLogin(key: string) {
  try {
    const response = await store.createBilibiliQRCodeLogin()
    setQRLogin(key, response)
    scheduleQRPolling()
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function setQRLogin(key: string, response: BilibiliQRCodeLoginCreateResponse | BilibiliQRCodeLoginPollResponse) {
  const previous = qrLogins[key]
  qrLogins[key] = {
    loginId: response.login_id,
    qrcodeUrl: 'qrcode_url' in response ? response.qrcode_url : previous?.qrcodeUrl || '',
    expiresAt: response.expires_at,
    state: response.state,
    cookie: response.cookie || previous?.cookie || '',
    accountNickname: response.account?.nickname || previous?.accountNickname || '',
    accountUid: response.account?.uid || previous?.accountUid || '',
  }
  if (response.cookie && drafts[key]) {
    drafts[key].cookie = response.cookie
    if (response.account?.uid) {
      drafts[key].account_id = normalizeAccountId(response.account.uid)
    }
    if (response.account?.nickname) {
      drafts[key].label = response.account.nickname
    }
  }
}

function scheduleQRPolling() {
  if (qrPollTimer !== undefined) {
    return
  }
  qrPollTimer = window.setInterval(() => {
    void pollActiveQRLogins()
  }, qrPollIntervalMs)
  void pollActiveQRLogins()
}

function stopQRPolling() {
  if (qrPollTimer === undefined) {
    return
  }
  window.clearInterval(qrPollTimer)
  qrPollTimer = undefined
}

async function pollActiveQRLogins() {
  const active = Object.entries(qrLogins).filter(([, qr]) => qr.state === 'pending_scan' || qr.state === 'pending_confirm')
  if (active.length === 0) {
    stopQRPolling()
    return
  }
  await Promise.all(active.map(async ([key, qr]) => {
    try {
      const response = await store.pollBilibiliQRCodeLogin(qr.loginId)
      setQRLogin(key, response)
    } catch (err) {
      notifyError(getDisplayErrorMessage(err))
      delete qrLogins[key]
    }
  }))
}

function accountKey(value: string) {
  return `bilibili:${value}`
}

function newDraftKey() {
  draftSequence.value += 1
  return `bilibili-draft:${draftSequence.value}`
}

function nextAccountId() {
  const used = new Set([
    ...bilibiliAccounts.value.map((account) => normalizeAccountId(account.account_id)),
    ...activeDraftEntries.value.map((entry) => normalizeAccountId(entry.draft.account_id)),
  ])
  if (!used.has('primary')) {
    return 'primary'
  }
  let index = 2
  while (used.has(`bilibili-${index}`)) {
    index += 1
  }
  return `bilibili-${index}`
}

function normalizeAccountId(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9_.-]+/g, '').replace(/^[._-]+|[._-]+$/g, '').slice(0, 64)
}

function sourceStatusMeta(value?: BilibiliSourceStatusResponse['status']) {
  switch (value) {
    case 'connected':
      return { color: 'green', label: t('builtinFeatures.thirdPartyAccounts.sourceConnected') }
    case 'connecting':
      return { color: 'blue', label: t('builtinFeatures.thirdPartyAccounts.sourceConnecting') }
    case 'degraded':
      return { color: 'orange', label: t('builtinFeatures.thirdPartyAccounts.sourceDegraded') }
    case 'failed':
      return { color: 'red', label: t('builtinFeatures.thirdPartyAccounts.sourceFailed') }
    case 'disabled':
      return { color: 'default', label: t('builtinFeatures.thirdPartyAccounts.disabled') }
    case 'idle':
    default:
      return { color: 'default', label: t('builtinFeatures.thirdPartyAccounts.sourceIdle') }
  }
}

function credentialMeta(state?: ThirdPartyCredentialState) {
  switch (state) {
    case 'valid':
      return { color: 'green', label: t('builtinFeatures.thirdPartyAccounts.credentialValid') }
    case 'invalid':
      return { color: 'red', label: t('builtinFeatures.thirdPartyAccounts.credentialInvalid') }
    default:
      return { color: 'default', label: t('builtinFeatures.thirdPartyAccounts.credentialUnknown') }
  }
}

function qrStatus(qr?: QRLoginState) {
  switch (qr?.state) {
    case 'pending_confirm':
    case 'succeeded':
      return 'scanned'
    case 'expired':
      return 'expired'
    case 'pending_scan':
    default:
      return 'active'
  }
}

function qrStatusText(qr?: QRLoginState) {
  switch (qr?.state) {
    case 'pending_confirm':
      return t('builtinFeatures.thirdPartyAccounts.qrPendingConfirm')
    case 'expired':
      return t('builtinFeatures.thirdPartyAccounts.qrExpired')
    case 'succeeded':
      return t('builtinFeatures.thirdPartyAccounts.qrSucceeded')
    case 'pending_scan':
    default:
      return t('builtinFeatures.thirdPartyAccounts.qrPendingScan')
  }
}

function displayName(account: ThirdPartyAccountSummary) {
  return account.profile?.nickname || account.label || account.account_id
}

function displayUid(account: ThirdPartyAccountSummary) {
  return account.profile?.uid || account.account_id
}

function avatarText(account: ThirdPartyAccountSummary) {
  return displayName(account).slice(0, 1).toUpperCase()
}

function accountAvatarSrc(account: ThirdPartyAccountSummary) {
  const avatarURL = account.profile?.avatar_url?.trim()
  if (!avatarURL || avatarLoadFailures[avatarFailureKey(account)]) {
    return ''
  }
  return avatarURL
}

function markAvatarFailed(account: ThirdPartyAccountSummary) {
  avatarLoadFailures[avatarFailureKey(account)] = true
}

function avatarFailureKey(account: ThirdPartyAccountSummary) {
  return `${account.platform}:${account.account_id}:${account.profile?.avatar_url || ''}`
}

function timeText(value?: string | null) {
  return value ? formatDateTime(value) : t('builtinFeatures.thirdPartyAccounts.none')
}
</script>

<template>
  <AppPage :title="t('builtinFeatures.thirdPartyAccounts.title')" :description="t('builtinFeatures.thirdPartyAccounts.subtitle')">
    <RetryPanel
      v-if="fatalError"
      :title="t('errors.common.loadFailed')"
      :description="error || ''"
      :loading="loading"
      @retry="loadPage"
    />

    <div v-else class="third-party-layout">
      <section :class="['source-summary-strip', `source-summary-strip--${statusTone}`]">
        <div class="source-summary-main">
          <div class="source-summary-title-row">
            <span class="source-summary-title">
              <LinkOutlined />
              <span>{{ t('builtinFeatures.thirdPartyAccounts.sourceTitle') }}</span>
            </span>
            <a-tag :color="sourceStatusTag.color">{{ sourceStatusTag.label }}</a-tag>
          </div>
          <p>{{ status?.summary || t('builtinFeatures.thirdPartyAccounts.sourceWaiting') }}</p>
          <div :class="['source-summary-error', { 'is-empty': sourceErrorText === t('builtinFeatures.thirdPartyAccounts.noError') }]">
            {{ sourceErrorText }}
          </div>
        </div>

        <div class="source-metrics">
          <div v-for="metric in sourceMetrics" :key="metric.label" :class="['source-metric', `source-metric--${metric.tone || 'normal'}`]">
            <span>{{ metric.label }}</span>
            <strong>{{ metric.value }}</strong>
            <small>{{ metric.detail }}</small>
          </div>
        </div>

        <div class="source-summary-actions">
          <a-button :loading="loading" @click="loadPage">
            <template #icon><ReloadOutlined /></template>
            {{ t('builtinFeatures.thirdPartyAccounts.refresh') }}
          </a-button>
          <a-button :loading="restarting" @click="restartSource">
            <template #icon><SyncOutlined /></template>
            {{ t('builtinFeatures.thirdPartyAccounts.restartSource') }}
          </a-button>
          <a-button type="primary" @click="addDraft">
            <template #icon><PlusOutlined /></template>
            {{ t('builtinFeatures.thirdPartyAccounts.addBilibili') }}
          </a-button>
        </div>
      </section>

      <section class="accounts-panel">
        <div class="accounts-panel__header">
          <div>
            <h2>{{ t('builtinFeatures.thirdPartyAccounts.accountTitle') }}</h2>
            <p>{{ t('builtinFeatures.thirdPartyAccounts.accountSummary', { configured: configuredAccountCount, enabled: enabledAccountCount }) }}</p>
          </div>
        </div>

        <div v-if="!hasAccounts && !hasEditorCards" class="accounts-empty">
          <span>{{ t('builtinFeatures.thirdPartyAccounts.noAccounts') }}</span>
          <a-button type="primary" size="small" @click="addDraft">
            <template #icon><PlusOutlined /></template>
            {{ t('builtinFeatures.thirdPartyAccounts.addBilibili') }}
          </a-button>
        </div>

        <div v-else class="accounts-grid">
          <article
            v-for="entry in activeDraftEntries"
            :key="entry.key"
            class="account-card account-card--editing"
          >
            <div class="account-card__head">
              <a-avatar class="account-avatar account-avatar--draft" :size="48" data-testid="bilibili-account-avatar-fallback">
                <template #icon><UserOutlined /></template>
              </a-avatar>
              <div>
                <span class="account-platform">Bilibili</span>
                <strong>{{ entry.draft.label || entry.draft.account_id }}</strong>
                <small>{{ entry.draft.account_id }}</small>
              </div>
            </div>
            <div class="account-editor">
              <div class="account-editor-grid">
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.accountId')">
                  <a-input v-model:value="entry.draft.account_id" autocomplete="off" />
                </a-form-item>
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.label')">
                  <a-input v-model:value="entry.draft.label" autocomplete="off" />
                </a-form-item>
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.enabled')">
                  <a-switch
                    v-model:checked="entry.draft.enabled"
                    :checked-children="t('builtinFeatures.thirdPartyAccounts.enabled')"
                    :un-checked-children="t('builtinFeatures.thirdPartyAccounts.disabled')"
                  />
                </a-form-item>
              </div>
              <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.cookie')">
                <a-textarea
                  v-model:value="entry.draft.cookie"
                  :auto-size="{ minRows: 3, maxRows: 5 }"
                  autocomplete="off"
                  spellcheck="false"
                  placeholder="SESSDATA=..."
                />
              </a-form-item>
              <div v-if="qrLogins[entry.key]" class="qr-panel">
                <a-qrcode
                  :value="qrLogins[entry.key].qrcodeUrl"
                  :status="qrStatus(qrLogins[entry.key])"
                  :size="168"
                  bordered
                />
                <div>
                  <strong>{{ qrStatusText(qrLogins[entry.key]) }}</strong>
                  <p>{{ qrLogins[entry.key].accountNickname || t('builtinFeatures.thirdPartyAccounts.qrScanWithBilibili') }}</p>
                  <small>{{ t('builtinFeatures.thirdPartyAccounts.qrExpiresAt', { time: timeText(qrLogins[entry.key].expiresAt) }) }}</small>
                </div>
              </div>
              <div class="account-editor-actions">
                <a-button :loading="qrcodeCreating || qrcodePollingLoginId === qrLogins[entry.key]?.loginId" @click="startQRCodeLogin(entry.key)">
                  <template #icon><QrcodeOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.scanLogin') }}
                </a-button>
                <a-button danger @click="deleteDraft(entry.key)">
                  <template #icon><DeleteOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.cancel') }}
                </a-button>
                <a-button type="primary" :loading="savingAccountId === normalizeAccountId(entry.draft.account_id)" @click="saveDraft(entry.key)">
                  <template #icon><SaveOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.save') }}
                </a-button>
              </div>
            </div>
          </article>

          <article
            v-for="account in bilibiliAccounts"
            :key="account.account_id"
            :class="['account-card', { 'account-card--editing': isEditing(accountKey(account.account_id)) }]"
          >
            <div class="account-card__head">
              <a-avatar class="account-avatar" :size="52">
                <img
                  v-if="accountAvatarSrc(account)"
                  class="account-avatar__image"
                  data-testid="bilibili-account-avatar-image"
                  :src="accountAvatarSrc(account)"
                  :alt="displayName(account)"
                  draggable="false"
                  loading="lazy"
                  referrerpolicy="no-referrer"
                  @error="markAvatarFailed(account)"
                >
                <template v-else>
                  <span data-testid="bilibili-account-avatar-fallback">{{ avatarText(account) }}</span>
                </template>
              </a-avatar>
              <div>
                <span class="account-platform">Bilibili</span>
                <strong>{{ displayName(account) }}</strong>
                <small>UID {{ displayUid(account) }}</small>
              </div>
            </div>

            <div class="account-card__badges">
              <a-tag :color="account.enabled ? 'blue' : 'default'">
                {{ account.enabled ? t('builtinFeatures.thirdPartyAccounts.enabled') : t('builtinFeatures.thirdPartyAccounts.disabled') }}
              </a-tag>
              <a-tag :color="account.configured ? 'green' : 'default'">
                {{ account.configured ? t('builtinFeatures.thirdPartyAccounts.configured') : t('builtinFeatures.thirdPartyAccounts.notConfigured') }}
              </a-tag>
              <a-tag :color="credentialMeta(account.credential.state).color">
                {{ credentialMeta(account.credential.state).label }}
              </a-tag>
            </div>

            <dl class="account-card__facts">
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyAccounts.accountId') }}</dt>
                <dd>{{ account.account_id }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyAccounts.polling') }}</dt>
                <dd>{{ account.polling.enabled ? t('builtinFeatures.thirdPartyAccounts.enabled') : t('builtinFeatures.thirdPartyAccounts.disabled') }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyAccounts.lastUsedAt') }}</dt>
                <dd>{{ timeText(account.polling.last_used_at) }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyAccounts.credentialCheckedAt') }}</dt>
                <dd>{{ timeText(account.credential.checked_at) }}</dd>
              </div>
              <div>
                <dt>{{ t('builtinFeatures.thirdPartyAccounts.updatedAt', { time: '' }).trim() }}</dt>
                <dd>{{ timeText(account.updated_at) }}</dd>
              </div>
            </dl>

            <p v-if="account.credential.last_error" class="account-card__error">
              {{ account.credential.last_error }}
            </p>

            <div v-if="!isEditing(accountKey(account.account_id))" class="account-card__actions">
              <a-button size="small" @click="beginEdit(account)">
                <template #icon><EditOutlined /></template>
                {{ t('builtinFeatures.thirdPartyAccounts.edit') }}
              </a-button>
              <a-button danger size="small" :loading="deletingAccountId === account.account_id" @click="deleteAccount(account)">
                <template #icon><DeleteOutlined /></template>
                {{ t('builtinFeatures.thirdPartyAccounts.delete') }}
              </a-button>
            </div>

            <div v-else class="account-editor">
              <div class="account-editor-grid">
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.accountId')">
                  <a-input v-model:value="drafts[accountKey(account.account_id)].account_id" disabled autocomplete="off" />
                </a-form-item>
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.label')">
                  <a-input v-model:value="drafts[accountKey(account.account_id)].label" autocomplete="off" />
                </a-form-item>
                <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.enabled')">
                  <a-switch
                    v-model:checked="drafts[accountKey(account.account_id)].enabled"
                    :checked-children="t('builtinFeatures.thirdPartyAccounts.enabled')"
                    :un-checked-children="t('builtinFeatures.thirdPartyAccounts.disabled')"
                  />
                </a-form-item>
              </div>
              <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.cookie')" :extra="t('builtinFeatures.thirdPartyAccounts.keepCookie')">
                <a-textarea
                  v-model:value="drafts[accountKey(account.account_id)].cookie"
                  :auto-size="{ minRows: 3, maxRows: 5 }"
                  autocomplete="off"
                  spellcheck="false"
                  placeholder="SESSDATA=..."
                />
              </a-form-item>
              <div v-if="qrLogins[accountKey(account.account_id)]" class="qr-panel">
                <a-qrcode
                  :value="qrLogins[accountKey(account.account_id)].qrcodeUrl"
                  :status="qrStatus(qrLogins[accountKey(account.account_id)])"
                  :size="168"
                  bordered
                />
                <div>
                  <strong>{{ qrStatusText(qrLogins[accountKey(account.account_id)]) }}</strong>
                  <p>{{ qrLogins[accountKey(account.account_id)].accountNickname || t('builtinFeatures.thirdPartyAccounts.qrScanWithBilibili') }}</p>
                  <small>{{ t('builtinFeatures.thirdPartyAccounts.qrExpiresAt', { time: timeText(qrLogins[accountKey(account.account_id)].expiresAt) }) }}</small>
                </div>
              </div>
              <div class="account-editor-actions">
                <a-button :loading="qrcodeCreating || qrcodePollingLoginId === qrLogins[accountKey(account.account_id)]?.loginId" @click="startQRCodeLogin(accountKey(account.account_id))">
                  <template #icon><QrcodeOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.scanLogin') }}
                </a-button>
                <a-button danger :loading="deletingAccountId === account.account_id" @click="deleteAccount(account)">
                  <template #icon><DeleteOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.delete') }}
                </a-button>
                <a-button @click="cancelEdit(accountKey(account.account_id))">
                  {{ t('builtinFeatures.thirdPartyAccounts.cancel') }}
                </a-button>
                <a-button type="primary" :loading="savingAccountId === account.account_id" @click="saveDraft(accountKey(account.account_id))">
                  <template #icon><SaveOutlined /></template>
                  {{ t('builtinFeatures.thirdPartyAccounts.save') }}
                </a-button>
              </div>
            </div>
          </article>
        </div>
      </section>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.source-summary-actions,
.accounts-panel__header,
.source-summary-title,
.source-summary-title-row,
.account-card__badges,
.account-card__actions,
.account-editor-actions,
.qr-panel {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.third-party-layout {
  display: grid;
  gap: var(--space-lg);
  padding: var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.source-summary-strip {
  display: grid;
  grid-template-columns: minmax(240px, 4fr) minmax(320px, 5fr) minmax(220px, 3fr);
  align-items: center;
  gap: var(--space-lg);
  padding: var(--space-md) var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

.source-summary-strip--warning {
  border-color: color-mix(in srgb, #d97706 35%, var(--border));
  background: color-mix(in srgb, #f59e0b 4%, var(--surface-soft));
}

.source-summary-strip--danger {
  border-color: color-mix(in srgb, #dc2626 36%, var(--border));
  background: color-mix(in srgb, #ef4444 4%, var(--surface-soft));
}

.source-summary-strip--success {
  border-color: color-mix(in srgb, #16a34a 24%, var(--border));
}

.source-summary-main,
.source-summary-title-row,
.source-summary-title {
  min-width: 0;
}

.source-summary-title-row {
  flex-wrap: wrap;
}

.source-summary-title {
  color: var(--text);
  font-weight: 600;
}

.source-summary-main p {
  margin: var(--space-xs) 0 0;
  overflow: hidden;
  color: var(--muted);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-summary-error {
  margin-top: var(--space-xs);
  color: #b45309;
  font-size: 0.78rem;
  line-height: 1.4;
  overflow-wrap: anywhere;
}

.source-summary-error.is-empty {
  color: var(--muted);
}

.source-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-sm);
  min-width: 0;
}

.source-metric {
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
  border-radius: var(--radius-sm);
  background: var(--surface);
}

.source-metric span,
.source-metric small {
  display: block;
  color: var(--muted);
  font-size: 0.74rem;
  line-height: 1.35;
}

.source-metric strong {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: var(--text);
  font-size: 0.92rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-metric small {
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-metric--success strong {
  color: #15803d;
}

.source-metric--warning strong {
  color: #b45309;
}

.source-metric--danger strong {
  color: #b91c1c;
}

.source-summary-actions {
  justify-content: flex-end;
  flex-wrap: wrap;
  min-width: 0;
}

.accounts-panel {
  display: grid;
  gap: var(--space-md);
}

.accounts-panel__header {
  justify-content: space-between;
}

.accounts-panel__header h2 {
  margin: 0;
  color: var(--text);
  font-size: 1rem;
  font-weight: 650;
}

.accounts-panel__header p {
  margin: 3px 0 0;
  color: var(--muted);
  font-size: 0.8rem;
}

.accounts-empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  min-height: 64px;
  padding: var(--space-md);
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  color: var(--muted);
}

.accounts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 360px), 360px));
  gap: var(--space-md);
  align-items: start;
  justify-content: start;
}

.account-card {
  display: grid;
  align-content: start;
  gap: var(--space-md);
  min-width: 0;
  width: 100%;
  max-width: 100%;
  overflow: hidden;
  padding: var(--space-md);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface);
  transition: border-color 180ms ease, background-color 180ms ease;
}

.account-card--editing {
  border-color: var(--border-accent);
  background: color-mix(in srgb, var(--surface-soft) 80%, var(--surface));
}

.account-card__head {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: var(--space-md);
  min-width: 0;
}

.account-card__head > div {
  display: grid;
  min-width: 0;
}

.account-card__head strong,
.account-card__head small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-card__head strong {
  color: var(--text);
  font-size: 1rem;
  font-weight: 650;
}

.account-card__head small,
.account-card__facts dt {
  color: var(--muted);
}

.account-platform {
  color: var(--text-accent);
  font-size: 0.76rem;
  font-weight: 650;
}

.account-avatar {
  flex: 0 0 auto;
  background: color-mix(in srgb, var(--text-accent) 16%, var(--surface-soft));
  color: var(--text-accent);
  font-weight: 650;
}

.account-avatar--draft {
  color: var(--muted);
}

.account-avatar :deep(.ant-avatar-string) {
  inset: 0 !important;
  display: block;
  width: 100%;
  height: 100%;
  line-height: inherit;
  transform: none !important;
}

.account-avatar__image {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

.account-card__badges,
.account-card__actions,
.account-editor-actions {
  flex-wrap: wrap;
  min-width: 0;
}

.account-card__badges :deep(.ant-tag) {
  margin-inline-end: 0;
}

.account-card__facts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-sm);
  margin: 0;
}

.account-card__facts div {
  min-width: 0;
}

.account-card__facts dt {
  font-size: 0.74rem;
}

.account-card__facts dd {
  margin: 2px 0 0;
  overflow: hidden;
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-card__error {
  margin: 0;
  padding: 7px 9px;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, #ef4444 7%, var(--surface));
  color: #b91c1c;
  font-size: 0.78rem;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.account-card__actions,
.account-editor-actions {
  justify-content: flex-end;
}

.account-editor {
  display: grid;
  gap: var(--space-sm);
  min-width: 0;
  padding-top: var(--space-sm);
  border-top: 1px solid var(--border);
}

.account-editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-sm);
}

.account-editor :deep(.ant-form-item) {
  margin-bottom: 0;
  min-width: 0;
}

.account-editor :deep(.ant-form-item-control-input),
.account-editor :deep(.ant-form-item-control-input-content),
.account-editor :deep(.ant-input),
.account-editor :deep(.ant-input-affix-wrapper),
.account-editor :deep(.ant-switch),
.account-editor :deep(textarea) {
  max-width: 100%;
}

.account-editor :deep(.ant-input),
.account-editor :deep(.ant-input-affix-wrapper),
.account-editor :deep(textarea) {
  width: 100%;
}

.qr-panel {
  align-items: center;
  min-width: 0;
  padding: var(--space-sm);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-strong);
}

.qr-panel > div:last-child {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.qr-panel strong {
  color: var(--text);
}

.qr-panel p {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text);
}

.qr-panel small {
  overflow-wrap: anywhere;
  color: var(--muted);
}

@media (max-width: 960px) {
  .source-summary-strip {
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .source-summary-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 720px) {
  .third-party-layout {
    padding: var(--space-md);
  }

  .source-summary-strip {
    grid-template-columns: 1fr;
  }

  .source-summary-strip {
    padding: var(--space-md);
  }

  .source-metrics,
  .account-card__facts {
    grid-template-columns: 1fr;
  }

  .source-summary-main p,
  .source-metric strong,
  .source-metric small,
  .account-card__head strong,
  .account-card__head small,
  .account-card__facts dd {
    white-space: normal;
  }

  .source-summary-actions,
  .account-card__actions,
  .account-editor-actions,
  .accounts-empty,
  .qr-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .source-summary-actions :deep(.ant-btn),
  .account-card__actions :deep(.ant-btn),
  .account-editor-actions :deep(.ant-btn),
  .accounts-empty :deep(.ant-btn) {
    width: 100%;
  }
}

@media (max-width: 520px) {
  .accounts-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
