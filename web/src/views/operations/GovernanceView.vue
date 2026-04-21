<script setup lang="ts">
import {
  ClockCircleOutlined,
  MessageOutlined,
  SafetyCertificateOutlined,
  SafetyOutlined,
  StopOutlined,
} from '@ant-design/icons-vue'
import { MotionDirective as vMotion } from '@vueuse/motion'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import AppStatCard from '@/components/AppStatCard.vue'
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

const activeTab = ref<'whitelist' | 'blacklist'>('whitelist')

const addModalVisible = ref(false)
const addModalTarget = ref<'whitelist' | 'blacklist'>('blacklist')
const addModalError = ref<string | null>(null)
const addModalMutating = ref(false)
const addModalDraft = reactive({
  entry_type: 'user' as GovernanceEntryType,
  target_id: '',
  reason: '',
})

const blacklistScopeFilter = ref<'all' | 'user' | 'group'>('all')
const whitelistScopeFilter = ref<'all' | 'user' | 'group'>('all')

const blacklistPagination = reactive({ current: 1, pageSize: 10 })
const whitelistPagination = reactive({ current: 1, pageSize: 10 })

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

const summaryCards = computed(() => [
  {
    key: 'default-permission',
    icon: SafetyCertificateOutlined,
    label: t('governance.summary.defaultPermission'),
    tone: 'primary' as const,
    value: getCommandPermissionLabel(commandPolicy.value?.default_level),
    description: t('governance.summary.defaultPermissionMeta'),
  },
  {
    key: 'user-cooldown',
    icon: ClockCircleOutlined,
    label: t('governance.summary.userCooldown'),
    tone: 'default' as const,
    value: formatRateLimit(commandPolicy.value?.cooldown.user_command_rate_limit),
    description: t('governance.summary.userCooldownMeta', {
      value: commandPolicy.value?.cooldown.user_command_rate_limit ?? t('display.empty'),
    }),
  },
  {
    key: 'group-cooldown',
    icon: ClockCircleOutlined,
    label: t('governance.summary.groupCooldown'),
    tone: 'default' as const,
    value: formatRateLimit(commandPolicy.value?.cooldown.group_command_rate_limit),
    description: t('governance.summary.groupCooldownMeta', {
      value: commandPolicy.value?.cooldown.group_command_rate_limit ?? t('display.empty'),
    }),
  },
  {
    key: 'cooldown-reply',
    icon: MessageOutlined,
    label: t('governance.summary.cooldownReply'),
    tone: commandPolicy.value?.cooldown.cooldown_reply ? 'success' : 'default' as const,
    value: commandPolicy.value?.cooldown.cooldown_reply
      ? t('governance.summary.cooldownReplyEnabled')
      : t('governance.summary.cooldownReplyDisabled'),
    description: t('governance.summary.cooldownReplyDescription'),
  },
  {
    key: 'blacklist-count',
    icon: StopOutlined,
    label: t('governance.summary.blacklistCount'),
    tone: 'warning' as const,
    value: String(totalBlacklistEntries.value),
    description: t('governance.cards.blacklistTitle'),
  },
  {
    key: 'whitelist-status',
    icon: SafetyOutlined,
    label: t('governance.summary.whitelistStatus'),
    tone: whitelistEnabled.value
      ? (showWhitelistEmptyWarning.value ? 'warning' : 'success')
      : 'default' as const,
    value: whitelistEnabled.value
      ? t('governance.summary.whitelistEnabled')
      : t('governance.summary.whitelistDisabled'),
    description: t('governance.summary.whitelistCount', { count: totalWhitelistEntries.value }),
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
  { title: t('governance.table.columns.actions'), key: 'actions', width: 100, align: 'center' as const, fixed: 'right' as const },
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

async function loadGovernance() {
  try {
    await governanceStore.refresh()
  } catch {
    // store state drives the page
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

function openAddModal(target: 'whitelist' | 'blacklist') {
  addModalTarget.value = target
  addModalError.value = null
  const scopeFilter = target === 'blacklist' ? blacklistScopeFilter.value : whitelistScopeFilter.value
  addModalDraft.entry_type = scopeFilter === 'all' ? 'user' : scopeFilter
  addModalDraft.target_id = ''
  addModalDraft.reason = ''
  addModalVisible.value = true
}

function closeAddModal() {
  addModalVisible.value = false
}

function resetAddModalDraft() {
  addModalDraft.entry_type = 'user'
  addModalDraft.target_id = ''
  addModalDraft.reason = ''
}

async function submitAddModal() {
  const targetId = addModalDraft.target_id.trim()
  const reason = addModalDraft.reason.trim()

  if (!targetId || !reason) {
    addModalError.value = t('governance.validation.entryRequired')
    return
  }

  addModalMutating.value = true
  addModalError.value = null

  const payload = {
    entry_type: addModalDraft.entry_type,
    target_id: targetId,
    reason,
  }

  try {
    if (addModalTarget.value === 'blacklist') {
      await governanceStore.addBlacklistEntry(payload)
      blacklistActionError.value = null
      blacklistPagination.current = 1
      notifySuccess(t('governance.feedback.blacklistSaved'))
    } else {
      await governanceStore.addWhitelistEntry(payload)
      whitelistActionError.value = null
      whitelistPagination.current = 1
      notifySuccess(t('governance.feedback.whitelistSaved'))
    }
    closeAddModal()
    resetAddModalDraft()
  } catch (error) {
    addModalError.value = getDisplayErrorMessage(error)
  } finally {
    addModalMutating.value = false
  }
}

async function copyTargetId(targetId: string) {
  try {
    await navigator.clipboard.writeText(targetId)
    notifySuccess(t('governance.actions.copyTargetId'))
  } catch {
    // Silently fail if clipboard is unavailable
  }
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
          <div class="governance-summary-cards">
            <AppStatCard
              v-for="card in summaryCards"
              :key="card.key"
              :icon="card.icon"
              :label="card.label"
              :tone="card.tone"
              :value="card.value"
              :description="card.description"
            />
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
        class="governance-tabs-card"
        :loading="(whitelistLoading && !whitelist) || (blacklistLoading && !blacklist)"
      >
        <a-tabs v-model:activeKey="activeTab" class="governance-tabs">
          <a-tab-pane key="whitelist" :tab="`${t('governance.tabs.whitelist')} (${totalWhitelistEntries})`">
            <div data-testid="governance-whitelist-card" class="governance-tab-content">
              <div class="governance-tab-header">
                <div class="governance-tab-header__copy">
                  <strong>{{ t('governance.cards.whitelistTitle') }}</strong>
                  <p>{{ t('governance.cards.whitelistDescription') }}</p>
                </div>
                <div class="governance-tab-header__meta">
                  <a-tag :color="whitelistEnabled ? 'warning' : 'default'">
                    {{ whitelistEnabled ? t('governance.summary.whitelistEnabled') : t('governance.summary.whitelistDisabled') }}
                  </a-tag>
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
                  <div class="governance-toolbar__actions">
                    <span class="governance-toolbar__count">{{ t('governance.table.total', { total: filteredWhitelistEntries.length }) }}</span>
                    <a-button type="primary" data-testid="governance-whitelist-add-btn" @click="openAddModal('whitelist')">
                      {{ t('governance.actions.addEntry') }}
                    </a-button>
                  </div>
                </div>
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
                    <span class="mono-text copyable-text" @click="copyTargetId(record.target_id)">
                      {{ record.target_id }}
                    </span>
                  </template>

                  <template v-else-if="column.key === 'createdAt'">
                    <span>{{ formatDateTime(record.created_at) }}</span>
                  </template>

                  <template v-else-if="column.key === 'actions'">
                    <a-popconfirm
                      :title="t('governance.confirm.removeTitle')"
                      :description="t('governance.confirm.removeDescription')"
                      @confirm="removeWhitelistEntry(record)"
                    >
                      <a-button type="link" danger size="small">
                        {{ t('governance.entryForm.remove') }}
                      </a-button>
                    </a-popconfirm>
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
            </div>
          </a-tab-pane>

          <a-tab-pane key="blacklist" :tab="`${t('governance.tabs.blacklist')} (${totalBlacklistEntries})`">
            <div data-testid="governance-blacklist-card" class="governance-tab-content">
              <div class="governance-tab-header">
                <div class="governance-tab-header__copy">
                  <strong>{{ t('governance.cards.blacklistTitle') }}</strong>
                  <p>{{ t('governance.cards.blacklistDescription') }}</p>
                </div>
                <div class="governance-tab-header__meta">
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
                  <div class="governance-toolbar__actions">
                    <span class="governance-toolbar__count">{{ t('governance.table.total', { total: filteredBlacklistEntries.length }) }}</span>
                    <a-button type="primary" data-testid="governance-blacklist-add-btn" @click="openAddModal('blacklist')">
                      {{ t('governance.actions.addEntry') }}
                    </a-button>
                  </div>
                </div>
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
                    <span class="mono-text copyable-text" @click="copyTargetId(record.target_id)">
                      {{ record.target_id }}
                    </span>
                  </template>

                  <template v-else-if="column.key === 'createdAt'">
                    <span>{{ formatDateTime(record.created_at) }}</span>
                  </template>

                  <template v-else-if="column.key === 'actions'">
                    <a-popconfirm
                      :title="t('governance.confirm.removeTitle')"
                      :description="t('governance.confirm.removeDescription')"
                      @confirm="removeBlacklistEntry(record)"
                    >
                      <a-button type="link" danger size="small">
                        {{ t('governance.entryForm.remove') }}
                      </a-button>
                    </a-popconfirm>
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
            </div>
          </a-tab-pane>
        </a-tabs>
      </AppCard>
    </div>

    <a-modal
      v-model:open="addModalVisible"
      :title="t('governance.modal.addTitle', {
        target: addModalTarget === 'whitelist'
          ? t('governance.modal.addTargetWhitelist')
          : t('governance.modal.addTargetBlacklist'),
      })"
      :confirm-loading="addModalMutating"
      :ok-text="t('governance.modal.save')"
      :cancel-text="t('governance.modal.cancel')"
      @ok="submitAddModal"
      @cancel="closeAddModal"
    >
      <a-alert
        v-if="addModalError"
        :message="t('errors.common.actionFailed')"
        type="warning"
        :description="addModalError"
        show-icon
        class="section-gap"
      />

      <a-form layout="vertical" class="add-modal-form">
        <a-form-item :label="t('governance.entryForm.scope')">
          <a-select v-model:value="addModalDraft.entry_type" :options="scopeOptions" />
        </a-form-item>
        <a-form-item :label="t('governance.entryForm.targetId')">
          <a-input
            v-model:value="addModalDraft.target_id"
            :placeholder="t('governance.entryForm.placeholderTargetId')"
          />
        </a-form-item>
        <a-form-item :label="t('governance.entryForm.reason')">
          <a-input
            v-model:value="addModalDraft.reason"
            :placeholder="t('governance.entryForm.placeholderReason')"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="whitelistConfirmVisible"
      data-testid="governance-whitelist-confirm-modal"
      :title="t('governance.whitelist.enableConfirmTitle')"
      :ok-text="t('governance.whitelist.enableConfirmAction')"
      :confirm-loading="whitelistMutating"
      @ok="confirmEmptyWhitelistEnable"
    >
      <p>{{ t('governance.whitelist.enableConfirmDescription') }}</p>
    </a-modal>
  </AppPage>
</template>

<style scoped lang="scss">
.governance-page__stack {
  display: grid;
  gap: 20px;
}

.governance-summary-card {
  border-radius: 16px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--accent-soft) 55%, var(--surface)) 0%, var(--surface) 100%);
}

.governance-summary-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.governance-summary-card__copy {
  display: grid;
  gap: 6px;
}

.governance-summary-card__copy strong {
  font-size: 1.05rem;
  line-height: 1.3;
}

.governance-summary-card__copy p {
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

.governance-summary-card__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.governance-summary-cards {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  margin-top: 18px;
}

.governance-tabs-card {
  border-radius: 16px;
}

.governance-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 16px;
}

.governance-tab-content {
  display: grid;
  gap: 16px;
}

.governance-tab-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.governance-tab-header__copy {
  display: grid;
  gap: 6px;
}

.governance-tab-header__copy strong {
  font-size: 1.05rem;
  line-height: 1.3;
}

.governance-tab-header__copy p {
  margin: 0;
  color: var(--muted);
}

.governance-tab-header__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.governance-risk-banner {
  padding: 14px;
  border-radius: 14px;
  background: color-mix(in srgb, var(--warning) 12%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--warning) 22%, var(--border));
  display: grid;
  gap: 8px;
}

.governance-risk-banner__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.governance-risk-banner p {
  margin: 0;
  color: var(--muted);
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

.governance-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.governance-toolbar__count {
  font-size: 0.82rem;
  color: var(--muted);
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

.copyable-text {
  cursor: pointer;
  transition: color 0.15s ease;
}

.copyable-text:hover {
  color: var(--accent);
}

.add-modal-form {
  margin-top: 8px;
}

@media (max-width: 768px) {
  .governance-summary-card__header,
  .governance-tab-header {
    flex-direction: column;
  }

  .governance-summary-card__actions,
  .governance-tab-header__meta {
    width: 100%;
  }

  .governance-toolbar__row {
    flex-direction: column;
    align-items: flex-start;
  }

  .governance-toolbar__actions {
    width: 100%;
    justify-content: space-between;
  }

  .governance-toolbar__count {
    text-align: right;
  }

  .governance-pagination {
    justify-content: center;
  }
}
</style>
