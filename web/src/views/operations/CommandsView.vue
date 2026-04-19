<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatCommandUsage, getPrimaryCommandPrefix } from '@/lib/command-usage'
import { getBooleanLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import { flattenPluginCommands, type PluginCommandAvailability } from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import type { BlacklistEntry, CommandPermissionLevel, CommandPermissionSource, PluginCommandSummary, PluginSummary } from '@/types/api'

const pluginsStore = usePluginsStore()
const configStore = useConfigStore()
const governanceStore = useGovernanceStore()

const { error, items, loading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)
const {
  blacklist,
  blacklistError,
  commandPolicy,
  commandPolicyError,
  error: governanceError,
  hasData: governanceHasData,
  loading: governanceLoading,
} = storeToRefs(governanceStore)

const selectedPluginIds = ref<string[]>([])

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

function getBlacklistTitle(entryType: BlacklistEntry['entry_type']) {
  return entryType === 'group' ? t('commands.blacklist.groupTitle') : t('commands.blacklist.userTitle')
}

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
        <a-card :bordered="false" class="app-view-card commands-section-card">
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

        <a-card :bordered="false" class="app-view-card commands-section-card">
          <template #title>
            <div class="card-header">
              <span>{{ t('commands.sections.blacklist') }}</span>
              <a-tag>{{ totalBlacklistEntries }}</a-tag>
            </div>
          </template>

          <a-skeleton :loading="governanceLoading && !blacklist" active>
            <a-alert
              v-if="blacklistError"
              :message="t('errors.common.loadFailed')"
              type="warning"
              :description="blacklistError"
              show-icon
              class="section-gap"
            />

            <div v-if="blacklist" class="commands-blacklist-grid">
              <section class="blacklist-section">
                <div class="blacklist-section__header">
                  <strong>{{ getBlacklistTitle('user') }}</strong>
                  <a-tag>{{ userBlacklistEntries.length }}</a-tag>
                </div>

                <AppEmptyState
                  v-if="userBlacklistEntries.length === 0"
                  icon="command"
                  :title="t('commands.empty.blacklistTitle')"
                  :description="t('commands.empty.blacklistDescription')"
                  compact
                />

                <article v-for="entry in userBlacklistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                  <strong>{{ entry.target_id }}</strong>
                  <span>{{ entry.reason }}</span>
                  <small>{{ formatDateTime(entry.created_at) }}</small>
                </article>
              </section>

              <section class="blacklist-section">
                <div class="blacklist-section__header">
                  <strong>{{ getBlacklistTitle('group') }}</strong>
                  <a-tag>{{ groupBlacklistEntries.length }}</a-tag>
                </div>

                <AppEmptyState
                  v-if="groupBlacklistEntries.length === 0"
                  icon="command"
                  :title="t('commands.empty.blacklistTitle')"
                  :description="t('commands.empty.blacklistDescription')"
                  compact
                />

                <article v-for="entry in groupBlacklistEntries" :key="`${entry.entry_type}-${entry.target_id}`" class="blacklist-entry">
                  <strong>{{ entry.target_id }}</strong>
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
                <strong>{{ record.plugin_name }}</strong>
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
                <strong>{{ record.plugin.name }}</strong>
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
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

.blacklist-section {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.blacklist-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
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

.command-plugin-cell small {
  font-family: "Cascadia Mono", "Consolas", monospace;
}
</style>
