<script setup lang="ts">
import { computed, reactive, ref, onBeforeUnmount, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  QrcodeOutlined,
  SaveOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import {
  thirdPartyPlatformLabels,
  thirdPartyPlatformOrder,
  useThirdPartyAccountsStore,
  type ThirdPartyPlatform,
} from '@/stores/third-party-accounts'
import type {
  ThirdPartyAccountProfile,
  ThirdPartyAccountSummary,
  ThirdPartyCredentialState,
  ThirdPartyQRCodeLoginCreateResponse,
  ThirdPartyQRCodeLoginPollResponse,
  ThirdPartyQRCodeLoginState,
} from '@/types/api'

interface AccountDraft {
  platform: ThirdPartyPlatform
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

interface PlatformSection {
  platform: ThirdPartyPlatform
  label: string
  addLabel: string
  accounts: ThirdPartyAccountSummary[]
  draftEntries: AccountDraftEntry[]
  configuredCount: number
  enabledCount: number
  supportsQRCode: boolean
  cookiePlaceholder: string
}

interface QRLoginState {
  platform: ThirdPartyPlatform
  loginId: string
  qrcodeUrl: string
  expiresAt: string
  state: ThirdPartyQRCodeLoginState
  accountNickname: string
  accountUid: string
  accountAvatarUrl: string
}

const qrPollIntervalMs = 2000

const store = useThirdPartyAccountsStore()
const {
  accounts,
  accountsByPlatform,
  deletingAccountId,
  error,
  loading,
  qrcodeCreating,
  qrcodePollingLoginId,
  savingAccountId,
} = storeToRefs(store)

const drafts = reactive<Record<string, AccountDraft>>({})
const qrLogins = reactive<Record<string, QRLoginState>>({})
const avatarLoadFailures = reactive<Record<string, boolean>>({})
const editingAccountKey = ref<string>('')
const draftSequence = ref(0)
let qrPollTimer: number | undefined

const pageErrorToast = computed(() => (
  error.value && accounts.value.length > 0
    ? {
        key: `third-party-accounts-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))

useToastFeedback(pageErrorToast)

const fatalError = computed(() => error.value && accounts.value.length === 0)
const hasAccounts = computed(() => accounts.value.length > 0)
const configuredAccountCount = computed(() => accounts.value.filter((account) => account.configured).length)
const enabledAccountCount = computed(() => accounts.value.filter((account) => account.enabled).length)
const activeDraftEntries = computed<AccountDraftEntry[]>(() => Object.entries(drafts)
  .filter(([, draft]) => draft.isNew)
  .map(([key, draft]) => ({ key, draft })))
const hasEditorCards = computed(() => activeDraftEntries.value.length > 0)
const platformSections = computed<PlatformSection[]>(() => thirdPartyPlatformOrder.map((platform) => {
  const platformAccounts = accountsByPlatform.value[platform] || []
  return {
    platform,
    label: platformLabel(platform),
    addLabel: addAccountLabel(platform),
    accounts: platformAccounts,
    draftEntries: activeDraftEntries.value.filter((entry) => entry.draft.platform === platform),
    configuredCount: platformAccounts.filter((account) => account.configured).length,
    enabledCount: platformAccounts.filter((account) => account.enabled).length,
    supportsQRCode: supportsQRCode(platform),
    cookiePlaceholder: platform === 'bilibili' ? 'SESSDATA=...' : 'Cookie',
  }
}))

onMounted(() => {
  void loadPage()
})

onBeforeUnmount(() => {
  stopQRPolling()
  store.disposeMedia()
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
    platform: account.platform,
    account_id: account.account_id,
    label: account.label || account.profile?.nickname || account.account_id,
    enabled: account.enabled,
    configured: account.configured,
    cookie: '',
  }
}

function addDraft(platform: ThirdPartyPlatform = 'bilibili') {
  const key = newDraftKey(platform)
  drafts[key] = {
    platform,
    account_id: nextAccountId(platform),
    label: `${platformLabel(platform)} Cookie`,
    enabled: true,
    configured: false,
    cookie: '',
    isNew: true,
  }
  editingAccountKey.value = key
}

function beginEdit(account: ThirdPartyAccountSummary) {
  const key = accountKey(account)
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
  if (draft.isNew && (accountsByPlatform.value[draft.platform] || []).some((account) => account.account_id === accountId)) {
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
    const profile = qrLoginProfile(qrLogins[key])
    await store.saveAccount(draft.platform, accountId, {
      label,
      enabled: draft.enabled,
      ...(cookie ? { cookie } : {}),
      ...(cookie && profile ? { profile } : {}),
    })
    cancelEdit(key)
    notifySuccess(t('builtinFeatures.thirdPartyAccounts.saved'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

async function deleteAccount(account: ThirdPartyAccountSummary) {
  try {
    await store.deleteAccount(account.platform, account.account_id)
    cancelEdit(accountKey(account))
    notifySuccess(t('builtinFeatures.thirdPartyAccounts.deleted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function deleteDraft(key: string) {
  cancelEdit(key)
}

async function startQRCodeLogin(key: string) {
  const platform = drafts[key]?.platform
  if (!platform || !supportsQRCode(platform)) {
    return
  }
  try {
    const response = await store.createQRCodeLogin(platform)
    setQRLogin(key, response)
    scheduleQRPolling()
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function setQRLogin(key: string, response: ThirdPartyQRCodeLoginCreateResponse | ThirdPartyQRCodeLoginPollResponse) {
  const previous = qrLogins[key]
  const account = 'account' in response ? response.account : null
  qrLogins[key] = {
    platform: response.platform,
    loginId: response.login_id,
    qrcodeUrl: 'qrcode_url' in response ? response.qrcode_url : previous?.qrcodeUrl || '',
    expiresAt: response.expires_at,
    state: response.state,
    accountNickname: account?.profile?.nickname || account?.label || previous?.accountNickname || '',
    accountUid: account?.profile?.uid || account?.account_id || previous?.accountUid || '',
    accountAvatarUrl: account?.profile?.avatar_url || previous?.accountAvatarUrl || '',
  }
  if (response.state === 'succeeded' && account && drafts[key]) {
    drafts[key].account_id = normalizeAccountId(account.account_id)
    drafts[key].label = account.label || account.profile?.nickname || drafts[key].label
    drafts[key].configured = true
  }
}

function qrLoginProfile(qr?: QRLoginState): ThirdPartyAccountProfile | null {
  const uid = qr?.accountUid.trim() || ''
  const nickname = qr?.accountNickname.trim() || ''
  const avatarUrl = qr?.accountAvatarUrl.trim() || ''
  if (!uid || !nickname || !avatarUrl) {
    return null
  }
  return {
    uid,
    nickname,
    avatar_url: avatarUrl,
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
      const response = await store.pollQRCodeLogin(qr.platform, qr.loginId)
      setQRLogin(key, response)
    } catch (err) {
      notifyError(getDisplayErrorMessage(err))
      delete qrLogins[key]
    }
  }))
}

function accountKey(account: Pick<ThirdPartyAccountSummary, 'platform' | 'account_id'>) {
  return operationKey(account.platform, account.account_id)
}

function newDraftKey(platform: ThirdPartyPlatform) {
  draftSequence.value += 1
  return `${platform}-draft:${draftSequence.value}`
}

function nextAccountId(platform: ThirdPartyPlatform) {
  const used = new Set([
    ...(accountsByPlatform.value[platform] || []).map((account) => normalizeAccountId(account.account_id)),
    ...activeDraftEntries.value
      .filter((entry) => entry.draft.platform === platform)
      .map((entry) => normalizeAccountId(entry.draft.account_id)),
  ])
  if (!used.has('primary')) {
    return 'primary'
  }
  let index = 2
  while (used.has(`${platform}-${index}`)) {
    index += 1
  }
  return `${platform}-${index}`
}

function operationKey(platform: ThirdPartyPlatform, accountId: string) {
  return `${platform}:${accountId}`
}

function platformLabel(platform: ThirdPartyPlatform) {
  return thirdPartyPlatformLabels[platform] || platform
}

function addAccountLabel(platform: ThirdPartyPlatform) {
  switch (platform) {
    case 'bilibili':
      return t('builtinFeatures.thirdPartyAccounts.addBilibili')
    case 'weibo':
      return t('builtinFeatures.thirdPartyAccounts.addWeibo')
    case 'douyin':
      return t('builtinFeatures.thirdPartyAccounts.addDouyin')
    case 'netease_music':
      return t('builtinFeatures.thirdPartyAccounts.addNeteaseMusic')
    default:
      return t('builtinFeatures.thirdPartyAccounts.addAccount')
  }
}

function supportsQRCode(platform: ThirdPartyPlatform) {
  return platform === 'bilibili' || platform === 'weibo' || platform === 'douyin' || platform === 'netease_music'
}

function normalizeAccountId(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9_.-]+/g, '').replace(/^[._-]+|[._-]+$/g, '').slice(0, 64)
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

function qrScanPrompt(platform?: ThirdPartyPlatform) {
  switch (platform) {
    case 'weibo':
      return t('builtinFeatures.thirdPartyAccounts.qrScanWithWeibo')
    case 'douyin':
      return t('builtinFeatures.thirdPartyAccounts.qrScanWithDouyin')
    case 'netease_music':
      return t('builtinFeatures.thirdPartyAccounts.qrScanWithNeteaseMusic')
    case 'bilibili':
    default:
      return t('builtinFeatures.thirdPartyAccounts.qrScanWithBilibili')
  }
}

function displayName(account: ThirdPartyAccountSummary) {
  return account.profile?.nickname || account.label || account.account_id
}

function displayUid(account: ThirdPartyAccountSummary) {
  const value = account.profile?.uid || account.account_id
  switch (account.platform) {
    case 'bilibili':
    case 'weibo':
      return `UID ${value}`
    case 'douyin':
      return `抖音号 ${value}`
    case 'netease_music':
      return `ID ${value}`
    default:
      return value
  }
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
      <section class="accounts-panel">
        <div class="accounts-panel__header">
          <div>
            <h2>{{ t('builtinFeatures.thirdPartyAccounts.accountTitle') }}</h2>
            <p>{{ t('builtinFeatures.thirdPartyAccounts.accountSummary', { configured: configuredAccountCount, enabled: enabledAccountCount }) }}</p>
          </div>
        </div>

        <div v-if="!hasAccounts && !hasEditorCards" class="accounts-empty">
          <span>{{ t('builtinFeatures.thirdPartyAccounts.noAccounts') }}</span>
          <div class="accounts-empty__actions">
            <a-button
              v-for="section in platformSections"
              :key="section.platform"
              type="primary"
              size="small"
              @click="addDraft(section.platform)"
            >
              <template #icon><PlusOutlined /></template>
              {{ section.addLabel }}
            </a-button>
          </div>
        </div>

        <div v-if="hasAccounts || hasEditorCards" class="platform-stack">
          <section
            v-for="section in platformSections"
            :key="section.platform"
            class="platform-section"
          >
            <div class="platform-section__header">
              <div>
                <h3>{{ section.label }}</h3>
                <p>{{ t('builtinFeatures.thirdPartyAccounts.accountSummary', { configured: section.configuredCount, enabled: section.enabledCount }) }}</p>
              </div>
              <a-button type="primary" size="small" @click="addDraft(section.platform)">
                <template #icon><PlusOutlined /></template>
                {{ section.addLabel }}
              </a-button>
            </div>

            <div v-if="section.accounts.length === 0 && section.draftEntries.length === 0" class="accounts-empty accounts-empty--compact">
              <span>{{ t('builtinFeatures.thirdPartyAccounts.noPlatformAccounts', { platform: section.label }) }}</span>
            </div>

            <div v-else class="accounts-grid">
              <article
                v-for="entry in section.draftEntries"
                :key="entry.key"
                class="account-card account-card--editing"
              >
                <div class="account-card__head">
                  <a-avatar class="account-avatar account-avatar--draft" :size="48" data-testid="bilibili-account-avatar-fallback">
                    <template #icon><UserOutlined /></template>
                  </a-avatar>
                  <div>
                    <span class="account-platform">{{ section.label }}</span>
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
                      :placeholder="section.cookiePlaceholder"
                    />
                  </a-form-item>
                  <div v-if="section.supportsQRCode && qrLogins[entry.key]" class="qr-panel">
                    <a-qrcode
                      :value="qrLogins[entry.key].qrcodeUrl"
                      :status="qrStatus(qrLogins[entry.key])"
                      :size="168"
                      bordered
                    />
                    <div>
                      <strong>{{ qrStatusText(qrLogins[entry.key]) }}</strong>
                      <p>{{ qrLogins[entry.key].accountNickname || qrScanPrompt(qrLogins[entry.key].platform) }}</p>
                      <small>{{ t('builtinFeatures.thirdPartyAccounts.qrExpiresAt', { time: timeText(qrLogins[entry.key].expiresAt) }) }}</small>
                    </div>
                  </div>
                  <div class="account-editor-actions">
                    <a-button v-if="section.supportsQRCode" :loading="qrcodeCreating || qrcodePollingLoginId === qrLogins[entry.key]?.loginId" @click="startQRCodeLogin(entry.key)">
                      <template #icon><QrcodeOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.scanLogin') }}
                    </a-button>
                    <a-button danger @click="deleteDraft(entry.key)">
                      <template #icon><DeleteOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.cancel') }}
                    </a-button>
                    <a-button type="primary" :loading="savingAccountId === operationKey(entry.draft.platform, normalizeAccountId(entry.draft.account_id))" @click="saveDraft(entry.key)">
                      <template #icon><SaveOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.save') }}
                    </a-button>
                  </div>
                </div>
              </article>

              <article
                v-for="account in section.accounts"
                :key="accountKey(account)"
                :class="['account-card', { 'account-card--editing': isEditing(accountKey(account)) }]"
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
                    <span class="account-platform">{{ platformLabel(account.platform) }}</span>
                    <strong>{{ displayName(account) }}</strong>
                    <small>{{ displayUid(account) }}</small>
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

                <div v-if="!isEditing(accountKey(account))" class="account-card__actions">
                  <a-button size="small" @click="beginEdit(account)">
                    <template #icon><EditOutlined /></template>
                    {{ t('builtinFeatures.thirdPartyAccounts.edit') }}
                  </a-button>
                  <a-button danger size="small" :loading="deletingAccountId === operationKey(account.platform, account.account_id)" @click="deleteAccount(account)">
                    <template #icon><DeleteOutlined /></template>
                    {{ t('builtinFeatures.thirdPartyAccounts.delete') }}
                  </a-button>
                </div>

                <div v-else class="account-editor">
                  <div class="account-editor-grid">
                    <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.accountId')">
                      <a-input v-model:value="drafts[accountKey(account)].account_id" disabled autocomplete="off" />
                    </a-form-item>
                    <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.label')">
                      <a-input v-model:value="drafts[accountKey(account)].label" autocomplete="off" />
                    </a-form-item>
                    <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.enabled')">
                      <a-switch
                        v-model:checked="drafts[accountKey(account)].enabled"
                        :checked-children="t('builtinFeatures.thirdPartyAccounts.enabled')"
                        :un-checked-children="t('builtinFeatures.thirdPartyAccounts.disabled')"
                      />
                    </a-form-item>
                  </div>
                  <a-form-item :label="t('builtinFeatures.thirdPartyAccounts.cookie')" :extra="t('builtinFeatures.thirdPartyAccounts.keepCookie')">
                    <a-textarea
                      v-model:value="drafts[accountKey(account)].cookie"
                      :auto-size="{ minRows: 3, maxRows: 5 }"
                      autocomplete="off"
                      spellcheck="false"
                      :placeholder="section.cookiePlaceholder"
                    />
                  </a-form-item>
                  <div v-if="section.supportsQRCode && qrLogins[accountKey(account)]" class="qr-panel">
                    <a-qrcode
                      :value="qrLogins[accountKey(account)].qrcodeUrl"
                      :status="qrStatus(qrLogins[accountKey(account)])"
                      :size="168"
                      bordered
                    />
                    <div>
                      <strong>{{ qrStatusText(qrLogins[accountKey(account)]) }}</strong>
                      <p>{{ qrLogins[accountKey(account)].accountNickname || qrScanPrompt(qrLogins[accountKey(account)].platform) }}</p>
                      <small>{{ t('builtinFeatures.thirdPartyAccounts.qrExpiresAt', { time: timeText(qrLogins[accountKey(account)].expiresAt) }) }}</small>
                    </div>
                  </div>
                  <div class="account-editor-actions">
                    <a-button v-if="section.supportsQRCode" :loading="qrcodeCreating || qrcodePollingLoginId === qrLogins[accountKey(account)]?.loginId" @click="startQRCodeLogin(accountKey(account))">
                      <template #icon><QrcodeOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.scanLogin') }}
                    </a-button>
                    <a-button danger :loading="deletingAccountId === operationKey(account.platform, account.account_id)" @click="deleteAccount(account)">
                      <template #icon><DeleteOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.delete') }}
                    </a-button>
                    <a-button @click="cancelEdit(accountKey(account))">
                      {{ t('builtinFeatures.thirdPartyAccounts.cancel') }}
                    </a-button>
                    <a-button type="primary" :loading="savingAccountId === operationKey(account.platform, account.account_id)" @click="saveDraft(accountKey(account))">
                      <template #icon><SaveOutlined /></template>
                      {{ t('builtinFeatures.thirdPartyAccounts.save') }}
                    </a-button>
                  </div>
                </div>
              </article>
            </div>
          </section>
        </div>
      </section>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.accounts-panel__header,
.accounts-panel__actions,
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

.accounts-panel {
  display: grid;
  gap: var(--space-md);
}

.accounts-panel__header {
  justify-content: space-between;
}

.accounts-panel__actions {
  justify-content: flex-end;
  flex-wrap: wrap;
  min-width: 0;
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

.accounts-empty--compact {
  min-height: 48px;
}

.accounts-empty__actions,
.platform-stack,
.platform-section {
  display: grid;
  gap: var(--space-sm);
}

.accounts-empty__actions {
  grid-template-columns: repeat(auto-fit, minmax(140px, max-content));
  justify-content: end;
}

.platform-stack {
  gap: var(--space-lg);
}

.platform-section {
  min-width: 0;
}

.platform-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  min-width: 0;
}

.platform-section__header h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.96rem;
  font-weight: 650;
}

.platform-section__header p {
  margin: 3px 0 0;
  color: var(--muted);
  font-size: 0.78rem;
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

@media (max-width: 720px) {
  .third-party-layout {
    padding: var(--space-md);
  }

  .account-card__facts {
    grid-template-columns: 1fr;
  }

  .account-card__head strong,
  .account-card__head small,
  .account-card__facts dd {
    white-space: normal;
  }

  .accounts-panel__header,
  .accounts-panel__actions,
  .accounts-empty__actions,
  .platform-section__header,
  .account-card__actions,
  .account-editor-actions,
  .accounts-empty,
  .qr-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .accounts-panel__actions :deep(.ant-btn),
  .accounts-empty__actions :deep(.ant-btn),
  .platform-section__header :deep(.ant-btn),
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
