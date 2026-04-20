<script setup lang="ts">
import { MotionDirective as vMotion } from '@vueuse/motion'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime, formatRateLimit } from '@/lib/format'
import { buildCommandsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { useGovernanceStore } from '@/stores/governance'
import type {
  BlacklistEntry,
  CommandPermissionLevel,
  GovernanceEntryType,
} from '@/types/api'

const router = useRouter()
const governanceStore = useGovernanceStore()

const {
  blacklist,
  blacklistError,
  blacklistLoading,
  whitelist,
  whitelistError,
  whitelistLoading,
  commandPolicy,
  commandPolicyError,
  loading,
  error: governanceError,
  hasData,
} = storeToRefs(governanceStore)

const blacklistActionError = ref<string | null>(null)
const whitelistActionError = ref<string | null>(null)
const blacklistMutating = ref(false)
const whitelistMutating = ref(false)
const whitelistConfirmVisible = ref(false)

const blacklistScopeFilter = ref<'all' | 'user' | 'group'>('all')
const whitelistScopeFilter = ref<'all' | 'user' | 'group'>('all')
const blacklistDraftScope = ref<GovernanceEntryType>('user')
const whitelistDraftScope = ref<GovernanceEntryType>('user')

const blacklistPagination = reactive({ current: 1, pageSize: 10 })
const whitelistPagination = reactive({ current: 1, pageSize: 10 })

const blacklistDrafts = reactive<Record<GovernanceEntryType, { reason: string; targetId: string }>>({
  user: { targetId: '', reason: '' },
  group: { targetId: '', reason: '' },
})

const whitelistDrafts = reactive<Record<GovernanceEntryType, { reason: string; targetId: string }>>({
  user: { targetId: '', reason: '' },
  group: { targetId: '', reason: '' },
})

const pageErrorMessage = computed(() => (
  governanceError.value
  ?? commandPolicyError.value
  ?? blacklistError.value
  ?? whitelistError.value
))
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && !hasData.value)

const userBlacklistEntries = computed(() => blacklist.value?.user_entries ?? [])
const groupBlacklistEntries = computed(() => blacklist.value?.group_entries ?? [])
const totalBlacklistEntries = computed(() => userBlacklistEntries.value.length + groupBlacklistEntries.value.length)

const userWhitelistEntries = computed(() => whitelist.value?.user_entries ?? [])
const groupWhitelistEntries = computed(() => whitelist.value?.group_entries ?? [])
const totalWhitelistEntries = computed(() => userWhitelistEntries.value.length + groupWhitelistEntries.value.length)
const whitelistEnabled = computed(() => whitelist.value?.enabled ?? false)
const showWhitelistEmptyWarning = computed(() => whitelistEnabled.value && totalWhitelistEntries.value === 0)

const blacklistRegionError = computed(() => blacklistActionError.value ?? blacklistError.value)
const whitelistRegionError = computed(() => whitelistActionError.value ?? whitelistError.value)

function sortEntries(entries: BlacklistEntry[]) {
  return [...entries].sort((a, b) => {
    if (!a.created_at || !b.created_at) return 0
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
}

const filteredBlacklistEntries = computed(() => {
  const entries = blacklistScopeFilter.value === 'all'
    ? [...userBlacklistEntries.value, ...groupBlacklistEntries.value]
    : blacklistScopeFilter.value === 'user'
      ? userBlacklistEntries.value
      : groupBlacklistEntries.value
  return sortEntries(entries)
})

const filteredWhitelistEntries = computed(() => {
  const entries = whitelistScopeFilter.value === 'all'
    ? [...userWhitelistEntries.value, ...groupWhitelistEntries.value]
    : whitelistScopeFilter.value === 'user'
      ? userWhitelistEntries.value
      : groupWhitelistEntries.value
  return sortEntries(entries)
})

const paginatedBlacklistEntries = computed(() => {
  const start = (blacklistPagination.current - 1) * blacklistPagination.pageSize
  return filteredBlacklistEntries.value.slice(start, start + blacklistPagination.pageSize)
})

const paginatedWhitelistEntries = computed(() => {
  const start = (whitelistPagination.current - 1) * whitelistPagination.pageSize
  return filteredWhitelistEntries.value.slice(start, start + whitelistPagination.pageSize)
})

watch(filteredBlacklistEntries, (entries) => {
  const maxPage = Math.ceil(entries.length / blacklistPagination.pageSize) || 1
  if (blacklistPagination.current > maxPage) {
    blacklistPagination.current = maxPage
  }
})

watch(filteredWhitelistEntries, (entries) => {
  const maxPage = Math.ceil(entries.length / whitelistPagination.pageSize) || 1
  if (whitelistPagination.current > maxPage) {
    whitelistPagination.current = maxPage
  }
})

const summaryItems = computed(() => [
  {
    key: 'default-permission',
    label: t('governance.summary.defaultPermission'),
    tone: 'accent',
    value: getCommandPermissionLabel(commandPolicy.value?.default_level),
    meta: t('governance.summary.defaultPermissionMeta'),
  },
  {
    key: 'user-cooldown',
    label: t('governance.summary.userCooldown'),
    tone: 'neutral',
    value: formatRateLimit(commandPolicy.value?.cooldown.user_command_rate_limit),
    meta: t('governance.summary.userCooldownMeta', {
      value: commandPolicy.value?.cooldown.user_command_rate_limit ?? t('display.empty'),
    }),
  },
  {
    key: 'group-cooldown',
    label: t('governance.summary.groupCooldown'),
    tone: 'neutral',
    value: formatRateLimit(commandPolicy.value?.cooldown.group_command_rate_limit),
    meta: t('governance.summary.groupCooldownMeta', {
      value: commandPolicy.value?.cooldown.group_command_rate_limit ?? t('display.empty'),
    }),
  },
  {
    key: 'cooldown-reply',
    label: t('governance.summary.cooldownReply'),
    tone: commandPolicy.value?.cooldown.cooldown_reply ? 'success' : 'neutral',
    value: commandPolicy.value?.cooldown.cooldown_reply
      ? t('governance.summary.cooldownReplyEnabled')
      : t('governance.summary.cooldownReplyDisabled'),
    meta: t('governance.summary.cooldownReplyDescription'),
  },
  {
    key: 'blacklist-count',
    label: t('governance.summary.blacklistCount'),
    tone: 'warning',
    value: String(totalBlacklistEntries.value),
    meta: t('governance.cards.blacklistTitle'),
  },
  {
    key: 'whitelist-status',
    label: t('governance.summary.whitelistStatus'),
    tone: whitelistEnabled.value ? (showWhitelistEmptyWarning.value ? 'warning' : 'success') : 'neutral',
    value: whitelistEnabled.value ? t('governance.summary.whitelistEnabled') : t('governance.summary.whitelistDisabled'),
    meta: t('governance.summary.whitelistCount', { count: totalWhitelistEntries.value }),
  },
])

const scopeOptions = computed(() => [
  { label: t('governance.scopes.user'), value: 'user' },
  { label: t('governance.scopes.group'), value: 'group' },
])

const scopeFilterOptions = computed(() => [
  { label: t('governance.filters.all'), value: 'all' },
  { label: t('governance.scopes.user'), value: 'user' },
  { label: t('governance.scopes.group'), value: 'group' },
])

const tableColumns = computed(() => [
  { title: t('governance.table.columns.type'), key: 'type', dataIndex: 'entry_type', width: 90, align: 'center' as const },
  { title: t('governance.table.columns.targetId'), key: 'targetId', dataIndex: 'target_id', width: 200 },
  { title: t('governance.table.columns.reason'), key: 'reason', dataIndex: 'reason' },
  { title: t('governance.table.columns.createdAt'), key: 'createdAt', dataIndex: 'created_at', width: 170 },
  { title: t('governance.table.columns.actions'), key: 'actions', width: 90, align: 'center' as const, fixed: 'right' as const },
])

function cardMotion(delay: number) {
  return {
    initial: { opacity: 0, y: 12 },
    enter: { opacity: 1, y: 0, transition: { duration: 320, delay: delay * 60, ease: 'easeOut' } },
  }
}

function getCommandPermissionLabel(level: CommandPermissionLevel | null | undefined) {
  switch (level) {
    case 'everyone':
      return t('commands.permissions.everyone')
    case 'group_admin':
      return t('commands.permissions.groupAdmin')
    case 'super_admin':
      return t('commands.permissions.superAdmin')
    default:
      return t('display.empty')
  }
}

function getEntryTypeLabel(type: GovernanceEntryType) {
  return type === 'user' ? t('governance.scopes.user') : t('governance.scopes.group')
}

function getEntryTypeTagColor(type: GovernanceEntryType) {
  return type === 'user' ? 'blue' : 'purple'
}

function getEntryDraft(collection: Record<GovernanceEntryType, { reason: string; targetId: string }>, entryType: GovernanceEntryType) {
  return collection[entryType]
}

function resetEntryDraft(collection: Record<GovernanceEntryType, { reason: string; targetId: string }>, entryType: GovernanceEntryType) {
  collection[entryType].targetId = ''
  collection[entryType].reason = ''
}

function validateEntryDraft(collection: Record<GovernanceEntryType, { reason: string; targetId: string }>, entryType: GovernanceEntryType) {
  const targetId = collection[entryType].targetId.trim()
  const reason = collection[entryType].reason.trim()

  if (!targetId || !reason) {
    return null
  }

  return {
    entry_type: entryType,
    target_id: targetId,
    reason,
  }
}

async function loadGovernance() {
  try {
    await governanceStore.refresh()
  } catch {
    // store state drives the page
  }
}

async function addBlacklistEntry() {
  const payload = validateEntryDraft(blacklistDrafts, blacklistDraftScope.value)
  if (!payload) {
    blacklistActionError.value = t('governance.validation.entryRequired')
    return
  }

  blacklistMutating.value = true
  blacklistActionError.value = null
  try {
    await governanceStore.addBlacklistEntry(payload)
    resetEntryDraft(blacklistDrafts, blacklistDraftScope.value)
    blacklistPagination.current = 1
    notifySuccess(t('governance.feedback.blacklistSaved'))
  } catch (error) {
    blacklistActionError.value = getDisplayErrorMessage(error)
  } finally {
    blacklistMutating.value = false
  }
}

async function removeBlacklistEntry(entry: BlacklistEntry) {
  blacklistMutating.value = true
  blacklistActionError.value = null
  try {
    await governanceStore.removeBlacklistEntry(entry.entry_type, entry.target_id)
    notifySuccess(t('governance.feedback.blacklistRemoved'))
  } catch (error) {
    blacklistActionError.value = getDisplayErrorMessage(error)
  } finally {
    blacklistMutating.value = false
  }
}

async function addWhitelistEntry() {
  const payload = validateEntryDraft(whitelistDrafts, whitelistDraftScope.value)
  if (!payload) {
    whitelistActionError.value = t('governance.validation.entryRequired')
    return
  }

  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.addWhitelistEntry(payload)
    resetEntryDraft(whitelistDrafts, whitelistDraftScope.value)
    whitelistPagination.current = 1
    notifySuccess(t('governance.feedback.whitelistSaved'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

async function removeWhitelistEntry(entry: BlacklistEntry) {
  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.removeWhitelistEntry(entry.entry_type, entry.target_id)
    notifySuccess(t('governance.feedback.whitelistRemoved'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

async function applyWhitelistEnabled(enabled: boolean) {
  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.setWhitelistEnabled(enabled)
    notifySuccess(t(enabled ? 'governance.feedback.whitelistEnabled' : 'governance.feedback.whitelistDisabled'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

function handleWhitelistToggle(checked: boolean) {
  if (checked && !whitelistEnabled.value && totalWhitelistEntries.value === 0) {
    whitelistConfirmVisible.value = true
    return
  }

  void applyWhitelistEnabled(checked)
}

async function confirmEmptyWhitelistEnable() {
  whitelistConfirmVisible.value = false
  await applyWhitelistEnabled(true)
}

onMounted(() => {
  void loadGovernance()
})
</script>

<template>
  <AppPage :title="t('governance.title')" :description="t('governance.subtitle')">
    <template #extra>
      <a-button :loading="loading" type="primary" @click="loadGovernance()">
        {{ t('governance.refresh') }}
      </a-button>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('errors.common.loadFailed')"
      :description="pageErrorMessage ?? t('errors.common.loadFailed')"
      :loading="loading"
      @retry="loadGovernance()"
    />

    <div v-else class="governance-page__stack">
      <AppCard
        v-motion="cardMotion(0)"
        borderless
        class="governance-summary-card"
        data-testid="governance-summary-card"
        :loading="loading && !hasData"
        variant="highlight"
      >
        <div class="governance-summary-card__header">
          <div class="governance-summary-card__copy">
            <span class="governance-section-label">{{ t('governance.sections.summary') }}</span>
            <strong>{{ t('governance.sections.summary') }}</strong>
            <p>{{ t('governance.summary.description') }}</p>
          </div>
          <div class="governance-summary-card__actions">
            <a-button data-testid="governance-open-config" @click="router.push({ name: 'config' })">
              {{ t('governance.actions.openConfig') }}
            </a-button>
            <a-button data-testid="governance-open-commands" @click="router.push(buildCommandsLocation())">
              {{ t('governance.actions.openCommands') }}
            </a-button>
          </div>
        </div>

        <a-alert
          v-if="commandPolicyError"
          :message="t('errors.common.loadFailed')"
          type="warning"
          :description="commandPolicyError"
          show-icon
          class="section-gap"
        />

        <template v-if="hasData">
          <div class="governance-summary-grid">
            <article
              v-for="item in summaryItems"
              :key="item.key"
              :class="['governance-summary-tile', `governance-summary-tile--${item.tone}`]"
            >
              <span class="governance-summary-tile__label">{{ item.label }}</span>
              <strong class="governance-summary-tile__value">{{ item.value }}</strong>
              <small class="governance-summary-tile__meta">{{ item.meta }}</small>
            </article>
          </div>
        </template>

        <AppEmptyState
          v-else
          icon="command"
          :title="t('governance.empty.summaryTitle')"
          :description="t('governance.empty.summaryDescription')"
        />
      </AppCard>

      <AppCard
        v-motion="cardMotion(1)"
        borderless
        class="governance-panel-card governance-panel-card--whitelist"
        data-testid="governance-whitelist-card"
        :loading="whitelistLoading && !whitelist"
      >
        <div class="governance-panel-header">
          <div class="governance-panel-header__copy">
            <span class="governance-section-label">{{ t('governance.sections.whitelist') }}</span>
            <strong>{{ t('governance.cards.whitelistTitle') }}</strong>
            <p>{{ t('governance.cards.whitelistDescription') }}</p>
          </div>
          <div class="governance-panel-header__meta">
            <a-tag :color="whitelistEnabled ? 'warning' : 'default'">
              {{ whitelistEnabled ? t('governance.summary.whitelistEnabled') : t('governance.summary.whitelistDisabled') }}
            </a-tag>
            <a-tag>{{ totalWhitelistEntries }}</a-tag>
            <a-switch
              :checked="whitelistEnabled"
              :loading="whitelistMutating"
              data-testid="governance-whitelist-enabled"
              @change="handleWhitelistToggle"
            />
          </div>
        </div>

        <a-alert
          v-if="whitelistRegionError"
          :message="t('errors.common.actionFailed')"
          type="warning"
          :description="whitelistRegionError"
          show-icon
          class="section-gap"
        />

        <div v-if="showWhitelistEmptyWarning" class="governance-risk-banner">
          <div class="governance-risk-banner__header">
            <strong>{{ t('governance.whitelist.emptyWarningTitle') }}</strong>
            <a-tag color="warning">{{ t('governance.summary.whitelistEnabled') }}</a-tag>
          </div>
          <p>{{ t('governance.whitelist.emptyWarningDescription') }}</p>
        </div>

        <div class="governance-toolbar">
          <div class="governance-toolbar__row">
            <a-select v-model:value="whitelistScopeFilter" :options="scopeFilterOptions" class="governance-toolbar__filter" />
            <span class="governance-toolbar__count">{{ t('governance.table.total', { total: filteredWhitelistEntries.length }) }}</span>
          </div>
          <form class="governance-toolbar__form" :data-testid="'governance-whitelist-form'" @submit.prevent="addWhitelistEntry">
            <div class="governance-form-grid">
              <a-select v-model:value="whitelistDraftScope" :options="scopeOptions" class="governance-form-select" />
              <a-input
                v-model:value="getEntryDraft(whitelistDrafts, whitelistDraftScope).targetId"
                :placeholder="t('governance.entryForm.placeholderTargetId')"
                class="governance-form-input"
              />
              <a-input
                v-model:value="getEntryDraft(whitelistDrafts, whitelistDraftScope).reason"
                :placeholder="t('governance.entryForm.placeholderReason')"
                class="governance-form-input governance-form-input--wide"
              />
              <a-button
                type="primary"
                :loading="whitelistMutating"
                data-testid="governance-whitelist-add"
                class="governance-form-button"
                @click="addWhitelistEntry"
              >
                {{ t('governance.entryForm.add') }}
              </a-button>
            </div>
          </form>
        </div>

        <a-table
          class="governance-data-table app-data-table"
          :columns="tableColumns"
          :data-source="paginatedWhitelistEntries"
          :pagination="false"
          :row-key="(row: BlacklistEntry) => `${row.entry_type}-${row.target_id}`"
          :loading="whitelistLoading && !whitelist"
        >
          <template #emptyText>
            <div class="governance-empty-hint">
              <p>{{ t('governance.empty.whitelistTitle') }}</p>
              <span>{{ t('governance.empty.whitelistDescription') }}</span>
            </div>
          </template>

          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'type'">
              <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                {{ getEntryTypeLabel(record.entry_type) }}
              </a-tag>
            </template>

            <template v-else-if="column.key === 'targetId'">
              <span class="mono-text">{{ record.target_id }}</span>
            </template>

            <template v-else-if="column.key === 'createdAt'">
              <span>{{ formatDateTime(record.created_at) }}</span>
            </template>

            <template v-else-if="column.key === 'actions'">
              <a-button type="link" danger size="small" @click="removeWhitelistEntry(record)">
                {{ t('governance.entryForm.remove') }}
              </a-button>
            </template>
          </template>
        </a-table>

        <div v-if="filteredWhitelistEntries.length > 0" class="governance-pagination">
          <a-pagination
            v-model:current="whitelistPagination.current"
            v-model:pageSize="whitelistPagination.pageSize"
            :total="filteredWhitelistEntries.length"
            show-size-changer
            :page-size-options="['10', '20', '50']"
            :show-total="(total: number) => t('governance.table.total', { total })"
          />
        </div>
      </AppCard>

      <AppCard
        v-motion="cardMotion(2)"
        borderless
        class="governance-panel-card governance-panel-card--blacklist"
        data-testid="governance-blacklist-card"
        :loading="blacklistLoading && !blacklist"
      >
        <div class="governance-panel-header">
          <div class="governance-panel-header__copy">
            <span class="governance-section-label">{{ t('governance.sections.blacklist') }}</span>
            <strong>{{ t('governance.cards.blacklistTitle') }}</strong>
            <p>{{ t('governance.cards.blacklistDescription') }}</p>
          </div>
          <div class="governance-panel-header__meta">
            <a-tag color="warning">{{ totalBlacklistEntries }}</a-tag>
          </div>
        </div>

        <a-alert
          v-if="blacklistRegionError"
          :message="t('errors.common.actionFailed')"
          type="warning"
          :description="blacklistRegionError"
          show-icon
          class="section-gap"
        />

        <div class="governance-toolbar">
          <div class="governance-toolbar__row">
            <a-select v-model:value="blacklistScopeFilter" :options="scopeFilterOptions" class="governance-toolbar__filter" />
            <span class="governance-toolbar__count">{{ t('governance.table.total', { total: filteredBlacklistEntries.length }) }}</span>
          </div>
          <form class="governance-toolbar__form" :data-testid="'governance-blacklist-form'" @submit.prevent="addBlacklistEntry">
            <div class="governance-form-grid">
              <a-select v-model:value="blacklistDraftScope" :options="scopeOptions" class="governance-form-select" />
              <a-input
                v-model:value="getEntryDraft(blacklistDrafts, blacklistDraftScope).targetId"
                :placeholder="t('governance.entryForm.placeholderTargetId')"
                class="governance-form-input"
              />
              <a-input
                v-model:value="getEntryDraft(blacklistDrafts, blacklistDraftScope).reason"
                :placeholder="t('governance.entryForm.placeholderReason')"
                class="governance-form-input governance-form-input--wide"
              />
              <a-button
                type="primary"
                :loading="blacklistMutating"
                data-testid="governance-blacklist-add"
                class="governance-form-button"
                @click="addBlacklistEntry"
              >
                {{ t('governance.entryForm.add') }}
              </a-button>
            </div>
          </form>
        </div>

        <a-table
          class="governance-data-table app-data-table"
          :columns="tableColumns"
          :data-source="paginatedBlacklistEntries"
          :pagination="false"
          :row-key="(row: BlacklistEntry) => `${row.entry_type}-${row.target_id}`"
          :loading="blacklistLoading && !blacklist"
        >
          <template #emptyText>
            <div class="governance-empty-hint">
              <p>{{ t('governance.empty.blacklistTitle') }}</p>
              <span>{{ t('governance.empty.blacklistDescription') }}</span>
            </div>
          </template>

          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'type'">
              <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                {{ getEntryTypeLabel(record.entry_type) }}
              </a-tag>
            </template>

            <template v-else-if="column.key === 'targetId'">
              <span class="mono-text">{{ record.target_id }}</span>
            </template>

            <template v-else-if="column.key === 'createdAt'">
              <span>{{ formatDateTime(record.created_at) }}</span>
            </template>

            <template v-else-if="column.key === 'actions'">
              <a-button type="link" danger size="small" @click="removeBlacklistEntry(record)">
                {{ t('governance.entryForm.remove') }}
              </a-button>
            </template>
          </template>
        </a-table>

        <div v-if="filteredBlacklistEntries.length > 0" class="governance-pagination">
          <a-pagination
            v-model:current="blacklistPagination.current"
            v-model:pageSize="blacklistPagination.pageSize"
            :total="filteredBlacklistEntries.length"
            show-size-changer
            :page-size-options="['10', '20', '50']"
            :show-total="(total: number) => t('governance.table.total', { total })"
          />
        </div>
      </AppCard>
    </div>
  </AppPage>

  <a-modal
    v-model:open="whitelistConfirmVisible"
    :title="t('governance.whitelist.enableConfirmTitle')"
    :ok-text="t('governance.whitelist.enableConfirmAction')"
    :confirm-loading="whitelistMutating"
    @ok="confirmEmptyWhitelistEnable"
  >
    <p>{{ t('governance.whitelist.enableConfirmDescription') }}</p>
  </a-modal>
</template>

<style scoped lang="scss">
.governance-page__stack {
  display: grid;
  gap: 20px;
}

.governance-summary-card,
.governance-panel-card {
  border-radius: 16px;
}

.governance-summary-card {
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--accent-soft) 55%, var(--surface)) 0%, var(--surface) 100%);
}

.governance-summary-card__header,
.governance-panel-header,
.governance-risk-banner__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.governance-summary-card__copy,
.governance-panel-header__copy {
  display: grid;
  gap: 6px;
}

.governance-summary-card__copy strong,
.governance-panel-header__copy strong {
  font-size: 1.05rem;
  line-height: 1.3;
}

.governance-summary-card__copy p,
.governance-panel-header__copy p,
.governance-risk-banner p {
  margin: 0;
  color: var(--muted);
}

.governance-section-label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--accent);
}

.governance-summary-card__actions,
.governance-panel-header__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.governance-summary-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  margin-top: 18px;
}

.governance-summary-tile {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 20px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  border: 1px solid color-mix(in srgb, var(--border) 90%, transparent);
  box-shadow: var(--shadow-sm);
}

.governance-summary-tile__label {
  font-size: 0.8rem;
  color: var(--muted);
}

.governance-summary-tile__value {
  font-size: clamp(1.1rem, 2vw, 1.35rem);
  line-height: 1.2;
  color: var(--text);
  font-weight: 600;
}

.governance-summary-tile__meta {
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.governance-summary-tile--accent {
  background: color-mix(in srgb, var(--accent-soft) 50%, var(--surface));
}

.governance-summary-tile--success {
  background: color-mix(in srgb, var(--success) 10%, var(--surface));
}

.governance-summary-tile--warning {
  background: color-mix(in srgb, var(--warning) 16%, var(--surface));
}

.governance-panel-card {
  display: grid;
  gap: 16px;
}

.governance-panel-card--whitelist {
  background: color-mix(in srgb, var(--surface) 88%, var(--accent-soft));
}

.governance-panel-card--blacklist {
  background: color-mix(in srgb, var(--surface) 95%, color-mix(in srgb, var(--warning) 10%, transparent));
}

.governance-risk-banner {
  padding: 14px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--warning) 12%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--warning) 22%, var(--border));
  display: grid;
  gap: 8px;
}

.governance-toolbar {
  display: grid;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.governance-toolbar__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.governance-toolbar__filter {
  width: 120px;
}

.governance-toolbar__count {
  font-size: 0.82rem;
  color: var(--muted);
}

.governance-form-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: stretch;
}

.governance-form-select {
  width: 100px;
  flex-shrink: 0;
}

.governance-form-input {
  flex: 1 1 160px;
  min-width: 160px;
}

.governance-form-input--wide {
  flex: 2 1 200px;
  min-width: 200px;
}

.governance-form-button {
  flex-shrink: 0;
  align-self: center;
}

.governance-data-table {
  border-radius: var(--app-card-radius);
  overflow: hidden;
}

.governance-data-table :deep(.ant-table-tbody > tr:hover > td) {
  background: color-mix(in srgb, var(--accent) 4%, var(--surface-soft));
}

.governance-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
}

.governance-empty-hint {
  padding: var(--space-xl) var(--space-md);
  text-align: center;
}

.governance-empty-hint p {
  margin: 0 0 4px;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--muted);
}

.governance-empty-hint span {
  font-size: 0.82rem;
  color: var(--muted);
}

.mono-text {
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.88rem;
}

@media (max-width: 768px) {
  .governance-summary-card__header,
  .governance-panel-header {
    flex-direction: column;
  }

  .governance-summary-card__actions,
  .governance-panel-header__meta,
  .governance-toolbar__row {
    width: 100%;
  }

  .governance-toolbar__row {
    flex-direction: column;
    align-items: flex-start;
  }

  .governance-toolbar__count {
    text-align: right;
    width: 100%;
  }

  .governance-form-grid {
    flex-direction: column;
    align-items: stretch;
  }

  .governance-form-select,
  .governance-form-input,
  .governance-form-input--wide,
  .governance-form-button {
    width: 100%;
    min-width: auto;
    flex: 1 1 auto;
  }

  .governance-form-button :deep(.ant-btn) {
    width: 100%;
  }

  .governance-pagination {
    justify-content: center;
  }
}
</style>
