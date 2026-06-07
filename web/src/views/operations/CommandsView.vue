<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { useToastFeedback } from '@/adapter/feedback'
import { formatCommandUsage, getPrimaryCommandPrefix } from '@/lib/command-usage'
import {
  areLocationQueriesEqual,
  buildCommandsLocation,
  buildPermissionPolicyLocation,
  buildPluginDetailLocation,
  readCommandsPluginIds,
} from '@/lib/management-links'
import { t } from '@/i18n'
import { mergeCommandCenterRows, type PluginCommandAvailability } from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import type {
  CommandPermissionLevel,
  CommandPermissionSource,
  PluginCommandSource,
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
const { commandPolicy, commandPolicyError, commandPolicyLoading } = storeToRefs(governanceStore)

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
  return mergeCommandCenterRows(pluginsWithCommands.value, commandPolicy.value?.commands ?? [])
    .filter((row) => selectedIds.size === 0 || selectedIds.has(row.pluginId))
    .sort((left, right) => compareByLabel(left.command.name, right.command.name) || compareByLabel(left.pluginId, right.pluginId))
})

const pageErrorMessage = computed(() => error.value ?? commandPolicyError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && commandRows.value.length === 0)
const feedbackToast = computed(() => {
  if (error.value && commandRows.value.length > 0) {
    return {
      key: `commands-error:${error.value}`,
      level: 'error' as const,
      message: error.value,
    }
  }

  if (commandPolicyError.value && commandRows.value.length > 0) {
    return {
      key: `commands-policy-error:${commandPolicyError.value}`,
      level: 'warning' as const,
      message: commandPolicyError.value,
    }
  }

  return null
})

useToastFeedback(feedbackToast)

const commandTableColumns = computed(() => [
  { title: t('commands.fields.command'), key: 'command', dataIndex: 'command', width: 180 },
  { title: t('commands.fields.aliases'), key: 'aliases', dataIndex: 'aliases', width: 180 },
  { title: t('commands.fields.source'), key: 'source', dataIndex: 'source', width: 120 },
  { title: t('commands.fields.description'), key: 'description', dataIndex: 'description' },
  { title: t('commands.fields.usage'), key: 'usage', dataIndex: 'usage' },
  { title: t('commands.fields.permission'), key: 'permission', dataIndex: 'permission', width: 180 },
  { title: t('commands.fields.plugin'), key: 'plugin', dataIndex: 'plugin', width: 220 },
  { title: t('commands.fields.status'), key: 'status', dataIndex: 'status', width: 120 },
])

function samePluginIds(left: string[], right: string[]) {
  return left.length === right.length && left.every((item, index) => item === right[index])
}

async function loadCommands() {
  await Promise.allSettled([
    pluginsStore.fetchList(),
    configStore.fetchConfig(),
    governanceStore.fetchCommandPolicy(),
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

function getEffectivePermissionText(policy: { effective_permission?: CommandPermissionLevel } | null) {
  if (!policy?.effective_permission) {
    return t('display.empty')
  }
  return getCommandPermissionLabel(policy.effective_permission)
}

function getDeclaredPermissionText(command: PluginCommandSummary, policy: { declared_permission?: CommandPermissionLevel | null } | null) {
  if (policy) {
    return getCommandPermissionLabel(policy.declared_permission)
  }
  return getPermissionText(command)
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

function getCommandSourceLabel(source: PluginCommandSource) {
  return t(`commands.commandSource.${source}`)
}

function getCommandSourceColor(source: PluginCommandSource) {
  return source === 'dynamic' ? 'purple' : 'default'
}

function getSelectPopupContainer() {
  return document.body
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
      <div class="commands-page__actions">
        <a-button data-testid="commands-open-permission-policy" :aria-label="t('commands.actions.openPermissionPolicy')" @click="router.push(buildPermissionPolicyLocation())">
          {{ t('commands.actions.openPermissionPolicy') }}
        </a-button>
        <a-button :loading="loading || commandPolicyLoading" type="primary" :aria-label="t('commands.refresh')" @click="loadCommands()">
          {{ t('commands.refresh') }}
        </a-button>
      </div>
    </template>

    <template #toolbar>
      <a-card
        :bordered="false"
        class="app-view-card commands-filter-toolbar"
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 0 } } }"
      >
        <a-form layout="vertical">
          <a-form-item :label="t('commands.filters.plugins')">
            <a-select
              v-model:value="selectedPluginIds"
              mode="multiple"
              allow-clear
              :get-popup-container="getSelectPopupContainer"
              popup-class-name="commands-plugin-select-popup"
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
      :loading="loading || commandPolicyLoading"
      @retry="loadCommands()"
    />

    <template v-else>
      <a-card
        :bordered="false"
        class="app-view-card commands-section-card"
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 100 } } }"
      >
        <template #title>
          <div class="card-header">
            <span>{{ t('commands.sections.commandList') }}</span>
            <a-tag color="blue">{{ commandRows.length }}</a-tag>
          </div>
        </template>

        <a-table
          class="commands-data-table app-data-table"
          :columns="commandTableColumns"
          :data-source="commandRows"
          :pagination="false"
          :row-key="(row) => row.key"
          :loading="(loading || commandPolicyLoading) && commandRows.length === 0"
          :scroll="{ x: 1260 }"
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
              <a-tag :color="record.conflicted ? 'warning' : 'blue'" :aria-label="`指令：${record.command.name}`">
                {{ record.command.name }}
              </a-tag>
            </template>

            <template v-else-if="column.key === 'aliases'">
              <span>{{ getAliasesText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'source'">
              <a-tag :color="getCommandSourceColor(record.command.command_source)">
                {{ getCommandSourceLabel(record.command.command_source) }}
              </a-tag>
            </template>

            <template v-else-if="column.key === 'description'">
              <span>{{ record.command.description || t('display.empty') }}</span>
            </template>

            <template v-else-if="column.key === 'usage'">
              <span>{{ getUsageText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'permission'">
              <div class="command-permission-cell">
                <span>{{ getEffectivePermissionText(record.policy) }}</span>
                <small>
                  {{ t('commands.fields.declaredPermission') }}：{{ getDeclaredPermissionText(record.command, record.policy) }}
                </small>
                <small v-if="record.policy">
                  {{ t('commands.fields.permissionSource') }}：{{ getPermissionSourceLabel(record.policy.permission_source) }}
                </small>
              </div>
            </template>

            <template v-else-if="column.key === 'plugin'">
              <div class="command-plugin-cell">
                <RouterLink class="command-plugin-link" :to="buildPluginDetailLocation(record.pluginId)">
                  {{ record.pluginName }}
                </RouterLink>
                <small>{{ record.pluginId }}</small>
              </div>
            </template>

            <template v-else-if="column.key === 'status'">
              <a-tag :color="getStatusColor(record.availability)" :aria-label="`可用性：${getStatusLabel(record.availability)}`">
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
  border-radius: var(--radius-md);
}

.commands-filter-toolbar,
.commands-section-card {
  box-shadow: var(--shadow-xs);
}

:global(.commands-plugin-select-popup) {
  z-index: 1060;
}

:deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

.commands-page__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-plugin-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.command-permission-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.command-plugin-link {
  color: var(--accent);
  font-weight: 600;
}

.command-plugin-cell small,
.command-permission-cell small {
  color: var(--muted);
}

.command-plugin-cell small {
  font-family: var(--font-mono);
}

@media (max-width: 768px) {
  .commands-page__actions {
    justify-content: flex-end;
  }
}
</style>
