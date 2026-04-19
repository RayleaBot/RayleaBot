<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { notifySuccess } from '@/adapter/feedback'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatCommandUsage, getPrimaryCommandPrefix } from '@/lib/command-usage'
import { getBooleanLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import {
  areLocationQueriesEqual,
  buildCommandsLocation,
  buildPluginDetailLocation,
  readCommandsPluginIds,
} from '@/lib/management-links'
import { t } from '@/i18n'
import { flattenPluginCommands, type PluginCommandAvailability } from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import type {
  BlacklistEntry,
  CommandPermissionLevel,
  CommandPermissionSource,
  GovernanceEntryType,
  PluginCommandSummary,
  PluginSummary,
} from '@/types/api'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const configStore = useConfigStore()
const governanceStore = useGovernanceStore()

const { error, items, loading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)
const {
  blacklist,
  blacklistError,
  blacklistLoading,
  whitelist,
  whitelistError,
  whitelistLoading,
  commandPolicy,
  commandPolicyError,
  commandPolicyLoading,
  error: governanceError,
  hasData: governanceHasData,
  loading: governanceLoading,
} = storeToRefs(governanceStore)

const selectedPluginIds = ref<string[]>([])
const blacklistActionError = ref<string | null>(null)
const whitelistActionError = ref<string | null>(null)
const blacklistMutating = ref(false)
const whitelistMutating = ref(false)
const whitelistConfirmVisible = ref(false)

const blacklistDrafts = reactive<Record<GovernanceEntryType, { reason: string; targetId: string }>>({
  user: { targetId: '', reason: '' },
  group: { targetId: '', reason: '' },
})

const whitelistDrafts = reactive<Record<GovernanceEntryType, { reason: string; targetId: string }>>({
  user: { targetId: '', reason: '' },
  group: { targetId: '', reason: '' },
})

const commandPrefix = computed(() => getPrimaryCommandPrefix(configDocument.value?.command?.prefixes))
const pluginsWithCommands = computed(() => (
  [...items.value]
    .filter((plugin) => (plugin.commands?.length ?? 0) > 0)
    .sort((left, right) => compareByLabel(left.name, right.name) || compareByLabel(left.id, right.id))
))
const pluginOptions = computed(() => pluginsWithCommands.value.map((plugin) => ({
  label: getPluginLabel(plugin),
  value: plugin.id,
})))

const commandRows = computed(() => {
  const selectedIds = new Set(selectedPluginIds.value)
  return flattenPluginCommands(pluginsWithCommands.value)
    .filter((row) => selectedIds.size === 0 || selectedIds.has(row.plugin.id))
    .sort((left, right) => compareByLabel(left.command.name, right.command.name) || compareByLabel(left.plugin.id, right.plugin.id))
})

const governanceCommandRows = computed(() => {
  const selectedIds = new Set(selectedPluginIds.value)
  return [...(commandPolicy.value?.commands ?? [])]
    .filter((row) => selectedIds.size === 0 || selectedIds.has(row.plugin_id))
    .sort((left, right) => compareByLabel(left.command, right.command) || compareByLabel(left.plugin_id, right.plugin_id))
})

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

const pageErrorMessage = computed(() => error.value ?? governanceError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && commandRows.value.length === 0 && !governanceHasData.value)

const commandTableColumns = computed(() => [
  { title: t('commands.fields.command'), key: 'command', dataIndex: 'command', width: 180 },
  { title: t('commands.fields.aliases'), key: 'aliases', dataIndex: 'aliases', width: 180 },
  { title: t('commands.fields.description'), key: 'description', dataIndex: 'description' },
  { title: t('commands.fields.usage'), key: 'usage', dataIndex: 'usage' },
  { title: t('commands.fields.permission'), key: 'permission', dataIndex: 'permission', width: 180 },
  { title: t('commands.fields.plugin'), key: 'plugin', dataIndex: 'plugin', width: 220 },
  { title: t('commands.fields.status'), key: 'status', dataIndex: 'status', width: 120 },
])

const policyTableColumns = computed(() => [
  { title: t('commands.fields.command'), key: 'command', dataIndex: 'command', width: 180 },
  { title: t('commands.fields.aliases'), key: 'aliases', dataIndex: 'aliases', width: 180 },
  { title: t('commands.fields.declaredPermission'), key: 'declared_permission', dataIndex: 'declared_permission', width: 180 },
  { title: t('commands.fields.effectivePermission'), key: 'effective_permission', dataIndex: 'effective_permission', width: 180 },
  { title: t('commands.fields.permissionSource'), key: 'permission_source', dataIndex: 'permission_source', width: 160 },
  { title: t('commands.fields.plugin'), key: 'plugin', dataIndex: 'plugin', width: 220 },
])

function samePluginIds(left: string[], right: string[]) {
  return left.length === right.length && left.every((item, index) => item === right[index])
}

async function loadCommands() {
  await Promise.allSettled([
    pluginsStore.fetchList(),
    configStore.fetchConfig(),
    governanceStore.refresh(),
  ])
}

function compareByLabel(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function getPluginLabel(plugin: PluginSummary) {
  return `${plugin.name}（${plugin.id}）`
}

function getAliasesText(command: PluginCommandSummary | { aliases?: string[] }) {
  return command.aliases?.length ? command.aliases.join(', ') : t('display.empty')
}

function getPermissionText(command: PluginCommandSummary) {
  return command.permission?.trim() || t('plugins.commandPermissionDefault')
}

function getUsageText(command: PluginCommandSummary) {
  return formatCommandUsage(command, commandPrefix.value) || t('display.empty')
}

function getStatusLabel(status: PluginCommandAvailability) {
  return t(`commands.status.${status}`)
}

function getStatusColor(status: PluginCommandAvailability) {
  switch (status) {
    case 'available':
      return 'success'
    case 'starting':
    case 'switching':
      return 'warning'
    case 'disabled':
      return 'default'
    case 'not_ready':
    default:
      return 'processing'
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
      return t('commands.permissionDefault')
  }
}

function getPermissionSourceLabel(source: CommandPermissionSource) {
  return t(`commands.permissionSource.${source}`)
}

function getSelectPopupContainer(triggerNode: HTMLElement) {
  return triggerNode.parentElement ?? triggerNode
}

function getBlacklistTitle(entryType: GovernanceEntryType) {
  return entryType === 'group' ? t('commands.blacklist.groupTitle') : t('commands.blacklist.userTitle')
}

function getWhitelistTitle(entryType: GovernanceEntryType) {
  return entryType === 'group' ? t('commands.whitelist.groupTitle') : t('commands.whitelist.userTitle')
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

async function addBlacklistEntry(entryType: GovernanceEntryType) {
  const payload = validateEntryDraft(blacklistDrafts, entryType)
  if (!payload) {
    blacklistActionError.value = t('commands.validation.entryRequired')
    return
  }

  blacklistMutating.value = true
  blacklistActionError.value = null
  try {
    await governanceStore.addBlacklistEntry(payload)
    resetEntryDraft(blacklistDrafts, entryType)
    notifySuccess(t('commands.feedback.blacklistSaved'))
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
    notifySuccess(t('commands.feedback.blacklistRemoved'))
  } catch (error) {
    blacklistActionError.value = getDisplayErrorMessage(error)
  } finally {
    blacklistMutating.value = false
  }
}

async function applyWhitelistEnabled(enabled: boolean) {
  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.setWhitelistEnabled(enabled)
    notifySuccess(t(enabled ? 'commands.feedback.whitelistEnabled' : 'commands.feedback.whitelistDisabled'))
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

async function addWhitelistEntry(entryType: GovernanceEntryType) {
  const payload = validateEntryDraft(whitelistDrafts, entryType)
  if (!payload) {
    whitelistActionError.value = t('commands.validation.entryRequired')
    return
  }

  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.addWhitelistEntry(payload)
    resetEntryDraft(whitelistDrafts, entryType)
    notifySuccess(t('commands.feedback.whitelistSaved'))
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
    notifySuccess(t('commands.feedback.whitelistRemoved'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

watch(
  () => route.query,
  (query) => {
    if (route.name !== 'commands') {
      return
    }

    const nextPluginIds = readCommandsPluginIds(query)
    if (!samePluginIds(selectedPluginIds.value, nextPluginIds)) {
      selectedPluginIds.value = nextPluginIds
    }
  },
  { immediate: true },
)

watch(
  selectedPluginIds,
  async (nextPluginIds) => {
    if (route.name !== 'commands') {
      return
    }

    const target = buildCommandsLocation(nextPluginIds)
    if (areLocationQueriesEqual(route.query, target.query ?? {})) {
      return
    }

    await router.replace(target)
  },
  { deep: true },
)

onMounted(() => {
  void loadCommands()
})
</script>

<template>
  <AppPage :title="t('commands.title')" :description="t('commands.subtitle')" full-height>
    <template #extra>
      <a-button :loading="loading || governanceLoading" @click="loadCommands()">
        {{ t('commands.refresh') }}
      </a-button>
    </template>

    <template #toolbar>
      <a-card :bordered="false" class="app-view-card commands-filter-toolbar">
        <a-form layout="vertical">
          <a-form-item :label="t('commands.filters.plugins')">
            <a-select
              v-model:value="selectedPluginIds"
              mode="multiple"
              allow-clear
              :get-popup-container="getSelectPopupContainer"
              :options="pluginOptions"
              :placeholder="t('commands.filters.allPlugins')"
            />
          </a-form-item>
        </a-form>
      </a-card>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('errors.common.loadFailed')"
      :description="pageErrorMessage ?? t('errors.common.loadFailed')"
      :loading="loading || governanceLoading"
      @retry="loadCommands()"
    />

    <template v-else>
      <a-alert
        v-if="error && commandRows.length > 0"
        :message="t('errors.common.loadFailed')"
        type="error"
        :description="error"
        show-icon
        class="section-gap"
      />

      <div class="commands-governance-grid">
        <a-card :bordered="false" class="app-view-card commands-section-card" data-testid="commands-summary-card">
          <template #title>
            <div class="card-header">
              <span>{{ t('commands.sections.summary') }}</span>
            </div>
          </template>

          <a-skeleton :loading="governanceLoading && !commandPolicy" active>
            <a-alert
              v-if="commandPolicyError"
              :message="t('errors.common.loadFailed')"
              type="warning"
              :description="commandPolicyError"
              show-icon
              class="section-gap"
            />

            <a-descriptions v-if="commandPolicy" :column="1" bordered size="small">
              <a-descriptions-item :label="t('commands.summary.defaultPermission')">
                {{ getCommandPermissionLabel(commandPolicy.default_level) }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('commands.summary.userCooldown')">
                {{ commandPolicy.cooldown.user_command_rate_limit }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('commands.summary.groupCooldown')">
                {{ commandPolicy.cooldown.group_command_rate_limit }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('commands.summary.cooldownReply')">
                {{ getBooleanLabel(commandPolicy.cooldown.cooldown_reply) }}
              </a-descriptions-item>
              <a-descriptions-item :label="t('commands.summary.blacklistCount')">
                {{ totalBlacklistEntries }}
              </a-descriptions-item>
            </a-descriptions>

            <AppEmptyState
              v-else
              icon="command"
              :title="t('commands.empty.governanceTitle')"
              :description="t('commands.empty.governanceDescription')"
            />
          </a-skeleton>
        </a-card>

        <a-card :bordered="false" class="app-view-card commands-section-card" data-testid="commands-blacklist-card">
          <template #title>
            <div class="card-header">
              <span>{{ t('commands.sections.blacklist') }}</span>
              <a-tag>{{ totalBlacklistEntries }}</a-tag>
            </div>
          </template>

          <a-skeleton :loading="blacklistLoading && !blacklist" active>
            <a-alert
              v-if="blacklistRegionError"
              :message="t('errors.common.actionFailed')"
              type="warning"
              :description="blacklistRegionError"
              show-icon
              class="section-gap"
            />

            <div v-if="blacklist" class="commands-blacklist-grid">
              <section class="blacklist-section">
                <div class="blacklist-section__header">
                  <strong>{{ getBlacklistTitle('user') }}</strong>
                  <a-tag>{{ userBlacklistEntries.length }}</a-tag>
                </div>

                <a-form
                  layout="vertical"
                  class="entry-form"
                  data-testid="commands-blacklist-user-form"
                  @submit.prevent="addBlacklistEntry('user')"
                >
                  <a-form-item :label="t('commands.entryForm.targetId')">
                    <a-input
                      v-model:value="getEntryDraft(blacklistDrafts, 'user').targetId"
                      :placeholder="t('commands.entryForm.placeholderTargetId')"
                    />
                  </a-form-item>
                  <a-form-item :label="t('commands.entryForm.reason')">
                    <a-input
                      v-model:value="getEntryDraft(blacklistDrafts, 'user').reason"
                      :placeholder="t('commands.entryForm.placeholderReason')"
                    />
                  </a-form-item>
                  <a-button
                    type="primary"
                    :loading="blacklistMutating"
                    data-testid="commands-blacklist-add-user"
                    @click="addBlacklistEntry('user')"
                  >
                    {{ t('commands.entryForm.add') }}
                  </a-button>
                </a-form>

                <AppEmptyState
                  v-if="userBlacklistEntries.length === 0"
                  icon="command"
                  :title="t('commands.empty.blacklistTitle')"
                  :description="t('commands.empty.blacklistDescription')"
                  compact
                />

                <article v-for="entry in userBlacklistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                  <div class="entry-card__header">
                    <strong>{{ entry.target_id }}</strong>
                    <a-button
                      type="link"
                      danger
                      size="small"
                      data-testid="commands-blacklist-remove-user"
                      @click="removeBlacklistEntry(entry)"
                    >
                      {{ t('commands.entryForm.remove') }}
                    </a-button>
                  </div>
                  <span>{{ entry.reason }}</span>
                  <small>{{ formatDateTime(entry.created_at) }}</small>
                </article>
              </section>

              <section class="blacklist-section">
                <div class="blacklist-section__header">
                  <strong>{{ getBlacklistTitle('group') }}</strong>
                  <a-tag>{{ groupBlacklistEntries.length }}</a-tag>
                </div>

                <a-form
                  layout="vertical"
                  class="entry-form"
                  data-testid="commands-blacklist-group-form"
                  @submit.prevent="addBlacklistEntry('group')"
                >
                  <a-form-item :label="t('commands.entryForm.targetId')">
                    <a-input
                      v-model:value="getEntryDraft(blacklistDrafts, 'group').targetId"
                      :placeholder="t('commands.entryForm.placeholderTargetId')"
                    />
                  </a-form-item>
                  <a-form-item :label="t('commands.entryForm.reason')">
                    <a-input
                      v-model:value="getEntryDraft(blacklistDrafts, 'group').reason"
                      :placeholder="t('commands.entryForm.placeholderReason')"
                    />
                  </a-form-item>
                  <a-button
                    type="primary"
                    :loading="blacklistMutating"
                    data-testid="commands-blacklist-add-group"
                    @click="addBlacklistEntry('group')"
                  >
                    {{ t('commands.entryForm.add') }}
                  </a-button>
                </a-form>

                <AppEmptyState
                  v-if="groupBlacklistEntries.length === 0"
                  icon="command"
                  :title="t('commands.empty.blacklistTitle')"
                  :description="t('commands.empty.blacklistDescription')"
                  compact
                />

                <article v-for="entry in groupBlacklistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                  <div class="entry-card__header">
                    <strong>{{ entry.target_id }}</strong>
                    <a-button
                      type="link"
                      danger
                      size="small"
                      data-testid="commands-blacklist-remove-group"
                      @click="removeBlacklistEntry(entry)"
                    >
                      {{ t('commands.entryForm.remove') }}
                    </a-button>
                  </div>
                  <span>{{ entry.reason }}</span>
                  <small>{{ formatDateTime(entry.created_at) }}</small>
                </article>
              </section>
            </div>

            <AppEmptyState
              v-else
              icon="command"
              :title="t('commands.empty.blacklistTitle')"
              :description="t('commands.empty.blacklistDescription')"
            />
          </a-skeleton>
        </a-card>

        <a-card :bordered="false" class="app-view-card commands-section-card" data-testid="commands-whitelist-card">
          <template #title>
            <div class="card-header">
              <span>{{ t('commands.sections.whitelist') }}</span>
              <a-tag>{{ totalWhitelistEntries }}</a-tag>
            </div>
          </template>

          <a-skeleton :loading="whitelistLoading && !whitelist" active>
            <a-alert
              v-if="whitelistRegionError"
              :message="t('errors.common.actionFailed')"
              type="warning"
              :description="whitelistRegionError"
              show-icon
              class="section-gap"
            />

            <template v-if="whitelist">
              <div class="whitelist-header-row">
                <div class="whitelist-header-row__copy">
                  <strong>{{ t('commands.whitelist.enabled') }}</strong>
                  <p>{{ t('commands.whitelist.enabledHint') }}</p>
                </div>
                <a-switch
                  :checked="whitelistEnabled"
                  :loading="whitelistMutating"
                  data-testid="commands-whitelist-enabled"
                  @change="handleWhitelistToggle"
                />
              </div>

              <a-alert
                v-if="showWhitelistEmptyWarning"
                :message="t('commands.whitelist.emptyWarningTitle')"
                :description="t('commands.whitelist.emptyWarningDescription')"
                type="warning"
                show-icon
                class="section-gap"
              />

              <div class="commands-blacklist-grid">
                <section class="blacklist-section">
                  <div class="blacklist-section__header">
                    <strong>{{ getWhitelistTitle('user') }}</strong>
                    <a-tag>{{ userWhitelistEntries.length }}</a-tag>
                  </div>

                  <a-form
                    layout="vertical"
                    class="entry-form"
                    data-testid="commands-whitelist-user-form"
                    @submit.prevent="addWhitelistEntry('user')"
                  >
                    <a-form-item :label="t('commands.entryForm.targetId')">
                      <a-input
                        v-model:value="getEntryDraft(whitelistDrafts, 'user').targetId"
                        :placeholder="t('commands.entryForm.placeholderTargetId')"
                      />
                    </a-form-item>
                    <a-form-item :label="t('commands.entryForm.reason')">
                      <a-input
                        v-model:value="getEntryDraft(whitelistDrafts, 'user').reason"
                        :placeholder="t('commands.entryForm.placeholderReason')"
                      />
                    </a-form-item>
                    <a-button
                      type="primary"
                      :loading="whitelistMutating"
                      data-testid="commands-whitelist-add-user"
                      @click="addWhitelistEntry('user')"
                    >
                      {{ t('commands.entryForm.add') }}
                    </a-button>
                  </a-form>

                  <AppEmptyState
                    v-if="userWhitelistEntries.length === 0"
                    icon="command"
                    :title="t('commands.empty.whitelistTitle')"
                    :description="t('commands.empty.whitelistDescription')"
                    compact
                  />

                  <article v-for="entry in userWhitelistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                    <div class="entry-card__header">
                      <strong>{{ entry.target_id }}</strong>
                      <a-button
                        type="link"
                        danger
                        size="small"
                        data-testid="commands-whitelist-remove-user"
                        @click="removeWhitelistEntry(entry)"
                      >
                        {{ t('commands.entryForm.remove') }}
                      </a-button>
                    </div>
                    <span>{{ entry.reason }}</span>
                    <small>{{ formatDateTime(entry.created_at) }}</small>
                  </article>
                </section>

                <section class="blacklist-section">
                  <div class="blacklist-section__header">
                    <strong>{{ getWhitelistTitle('group') }}</strong>
                    <a-tag>{{ groupWhitelistEntries.length }}</a-tag>
                  </div>

                  <a-form
                    layout="vertical"
                    class="entry-form"
                    data-testid="commands-whitelist-group-form"
                    @submit.prevent="addWhitelistEntry('group')"
                  >
                    <a-form-item :label="t('commands.entryForm.targetId')">
                      <a-input
                        v-model:value="getEntryDraft(whitelistDrafts, 'group').targetId"
                        :placeholder="t('commands.entryForm.placeholderTargetId')"
                      />
                    </a-form-item>
                    <a-form-item :label="t('commands.entryForm.reason')">
                      <a-input
                        v-model:value="getEntryDraft(whitelistDrafts, 'group').reason"
                        :placeholder="t('commands.entryForm.placeholderReason')"
                      />
                    </a-form-item>
                    <a-button
                      type="primary"
                      :loading="whitelistMutating"
                      data-testid="commands-whitelist-add-group"
                      @click="addWhitelistEntry('group')"
                    >
                      {{ t('commands.entryForm.add') }}
                    </a-button>
                  </a-form>

                  <AppEmptyState
                    v-if="groupWhitelistEntries.length === 0"
                    icon="command"
                    :title="t('commands.empty.whitelistTitle')"
                    :description="t('commands.empty.whitelistDescription')"
                    compact
                  />

                  <article v-for="entry in groupWhitelistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                    <div class="entry-card__header">
                      <strong>{{ entry.target_id }}</strong>
                      <a-button
                        type="link"
                        danger
                        size="small"
                        data-testid="commands-whitelist-remove-group"
                        @click="removeWhitelistEntry(entry)"
                      >
                        {{ t('commands.entryForm.remove') }}
                      </a-button>
                    </div>
                    <span>{{ entry.reason }}</span>
                    <small>{{ formatDateTime(entry.created_at) }}</small>
                  </article>
                </section>
              </div>
            </template>

            <AppEmptyState
              v-else
              icon="command"
              :title="t('commands.empty.whitelistTitle')"
              :description="t('commands.empty.whitelistDescription')"
            />
          </a-skeleton>
        </a-card>
      </div>

      <a-card :bordered="false" class="app-view-card commands-section-card">
        <template #title>
          <div class="card-header">
            <span>{{ t('commands.sections.effectivePolicies') }}</span>
            <a-tag>{{ governanceCommandRows.length }}</a-tag>
          </div>
        </template>

        <a-alert
          v-if="commandPolicyError && governanceCommandRows.length > 0"
          :message="t('errors.common.loadFailed')"
          type="warning"
          :description="commandPolicyError"
          show-icon
          class="section-gap"
        />

        <a-table
          class="commands-data-table app-data-table"
          :columns="policyTableColumns"
          :data-source="governanceCommandRows"
          :pagination="false"
          :row-key="(row) => `${row.plugin_id}-${row.command}`"
          :loading="commandPolicyLoading && !commandPolicy"
          :scroll="{ x: 1100 }"
        >
          <template #emptyText>
            <AppEmptyState
              icon="command"
              :title="t('commands.empty.effectiveTitle')"
              :description="t('commands.empty.effectiveDescription')"
            />
          </template>

          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'command'">
              <a-tag color="blue">{{ record.command }}</a-tag>
            </template>

            <template v-else-if="column.key === 'aliases'">
              <span>{{ getAliasesText(record) }}</span>
            </template>

            <template v-else-if="column.key === 'declared_permission'">
              <span>{{ getCommandPermissionLabel(record.declared_permission) }}</span>
            </template>

            <template v-else-if="column.key === 'effective_permission'">
              <span>{{ getCommandPermissionLabel(record.effective_permission) }}</span>
            </template>

            <template v-else-if="column.key === 'permission_source'">
              <span>{{ getPermissionSourceLabel(record.permission_source) }}</span>
            </template>

            <template v-else-if="column.key === 'plugin'">
              <div class="command-plugin-cell">
                <RouterLink class="command-plugin-link" :to="buildPluginDetailLocation(record.plugin_id)">
                  {{ record.plugin_name }}
                </RouterLink>
                <small>{{ record.plugin_id }}</small>
              </div>
            </template>
          </template>
        </a-table>
      </a-card>

      <a-card :bordered="false" class="app-view-card commands-section-card">
        <template #title>
          <div class="card-header">
            <span>{{ t('commands.sections.declaredCommands') }}</span>
            <a-tag>{{ commandRows.length }}</a-tag>
          </div>
        </template>

        <a-table
          class="commands-data-table app-data-table"
          :columns="commandTableColumns"
          :data-source="commandRows"
          :pagination="false"
          :row-key="(row) => `${row.plugin.id}-${row.command.name}`"
          :scroll="{ x: 1180 }"
        >
          <template #emptyText>
            <AppEmptyState
              icon="command"
              :title="t('commands.empty.title')"
              :description="t('commands.empty.description')"
            />
          </template>

          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'command'">
              <a-tag :color="record.conflicted ? 'warning' : 'blue'">
                {{ record.command.name }}
              </a-tag>
            </template>

            <template v-else-if="column.key === 'aliases'">
              <span>{{ getAliasesText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'description'">
              <span>{{ record.command.description || t('display.empty') }}</span>
            </template>

            <template v-else-if="column.key === 'usage'">
              <span>{{ getUsageText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'permission'">
              <span>{{ getPermissionText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'plugin'">
              <div class="command-plugin-cell">
                <RouterLink class="command-plugin-link" :to="buildPluginDetailLocation(record.plugin.id)">
                  {{ record.plugin.name }}
                </RouterLink>
                <small>{{ record.plugin.id }}</small>
              </div>
            </template>

            <template v-else-if="column.key === 'status'">
              <a-tag :color="getStatusColor(record.availability)">
                {{ getStatusLabel(record.availability) }}
              </a-tag>
            </template>
          </template>
        </a-table>
      </a-card>
    </template>
  </AppPage>

  <a-modal
    v-model:open="whitelistConfirmVisible"
    :title="t('commands.whitelist.enableConfirmTitle')"
    :ok-text="t('commands.whitelist.enableConfirmAction')"
    :confirm-loading="whitelistMutating"
    @ok="confirmEmptyWhitelistEnable"
  >
    <p>{{ t('commands.whitelist.enableConfirmDescription') }}</p>
  </a-modal>
</template>

<style scoped lang="scss">
.commands-filter-toolbar,
.commands-section-card,
.commands-data-table {
  border-radius: 10px;
}

.commands-governance-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
}

.commands-blacklist-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
}

.blacklist-section {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.blacklist-section__header,
.card-header,
.entry-card__header,
.whitelist-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.whitelist-header-row {
  align-items: flex-start;
}

.whitelist-header-row__copy {
  display: grid;
  gap: 6px;
}

.whitelist-header-row__copy p {
  margin: 0;
  color: var(--muted);
}

.entry-form {
  padding: 14px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
}

.blacklist-entry {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--surface-soft);
  border: 1px solid var(--border);
}

.blacklist-entry span,
.blacklist-entry small,
.command-plugin-cell small {
  color: var(--muted);
}

.command-plugin-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.command-plugin-link {
  color: var(--app-primary);
  font-weight: 600;
}

.command-plugin-cell small {
  font-family: "Cascadia Mono", "Consolas", monospace;
}

@media (max-width: 768px) {
  .whitelist-header-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
