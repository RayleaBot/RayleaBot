<script setup lang="ts">
import AppSelect from '@/components/AppSelect.vue'
import AppDataTable from '@/components/AppDataTable.vue'
import AppTag from '@/components/AppTag.vue'
import AppField from '@/components/AppField.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
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
import { mergeCommandCenterRows, type PluginCommandAvailability, type UnifiedCommandRow } from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import type {
  CommandPermissionLevel,
  CommandPermissionSource,
  PluginCommandSummary,
  PluginSummary,
} from '@/types/api'

const route = useRoute()
const router = useRouter()
const navigate = useMotionNavigation()
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
  { label: t('commands.fields.command'), key: 'command', width: 180 },
  { label: t('commands.fields.aliases'), key: 'aliases', width: 180 },
  { label: t('commands.fields.source'), key: 'source', width: 120 },
  { label: t('commands.fields.description'), key: 'description' },
  { label: t('commands.fields.usage'), key: 'usage' },
  { label: t('commands.fields.permission'), key: 'permission', width: 180 },
  { label: t('commands.fields.plugin'), key: 'plugin', width: 220 },
  { label: t('commands.fields.status'), key: 'status', width: 120 },
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

function getAliasesText(command: PluginCommandSummary) {
  const aliases = command.effective_names.slice(1)
  return aliases.length ? aliases.join(', ') : t('display.empty')
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
      return 'neutral'
    case 'not_ready':
    default:
      return 'info'
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

function getCommandSourceLabel(source: PluginCommandSummary['trigger']['type']) {
  return t(`commands.commandSource.${source}`)
}

function getCommandSourceColor(source: PluginCommandSummary['trigger']['type']) {
  if (source === 'pattern') {
    return 'info'
  }
  return source === 'setting' ? 'info' : 'neutral'
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
  <AppPage :title="t('commands.title')" :show-header="false" width="detail">
    <template #toolbar>
      <div class="app-view-card commands-filter-toolbar">
        <div class="commands-filter-form">
          <AppField :label="t('commands.filters.plugins')">
            <AppSelect
              v-model="selectedPluginIds"
              multiple
              clearable
              :options="pluginOptions"
              :placeholder="t('commands.filters.allPlugins')"
            />
          </AppField>
        </div>
        <AppButton variant="default" data-testid="commands-open-permission-policy" :aria-label="t('commands.actions.openPermissionPolicy')" @click="navigate(buildPermissionPolicyLocation())">
          {{ t('commands.actions.openPermissionPolicy') }}
        </AppButton>
      </div>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('errors.common.loadFailed')"
      :description="pageErrorMessage ?? t('errors.common.loadFailed')"
      :loading="loading || commandPolicyLoading"
      @retry="loadCommands()"
    />

    <template v-else>
      <AppCard
        borderless
        class="app-view-card commands-section-card"
      >
        <template #title>
          <div class="card-header">
            <span>{{ t('commands.sections.commandList') }}</span>
            <AppTag tone="info">{{ commandRows.length }}</AppTag>
          </div>
        </template>

        <AppDataTable
          class="commands-data-table app-data-table"
          :columns="commandTableColumns"
          :rows="commandRows"

          :row-key="(row: UnifiedCommandRow) => row.key"
          :loading="(loading || commandPolicyLoading) && commandRows.length === 0"
          :min-width="1260" :label="t('commands.sections.commandList')"
        >
          <template #empty>
            <AppEmptyState
              icon="command"
              :title="t('commands.empty.title')"
              :description="t('commands.empty.description')"
            />
          </template>

          <template #cell="{ column, row: record }">
            <template v-if="column.key === 'command'">
              <AppTag :tone="record.conflicted ? 'warning' : 'info'" :aria-label="`指令：${record.command.name}`">
                {{ record.command.name }}
              </AppTag>
            </template>

            <template v-else-if="column.key === 'aliases'">
              <span>{{ getAliasesText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'source'">
              <AppTag :tone="getCommandSourceColor(record.command.trigger.type)">
                {{ getCommandSourceLabel(record.command.trigger.type) }}
              </AppTag>
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
                <MotionRouterLink class="command-plugin-link" :to="buildPluginDetailLocation(record.pluginId)">
                  {{ record.pluginName }}
                </MotionRouterLink>
                <small>{{ record.pluginId }}</small>
              </div>
            </template>

            <template v-else-if="column.key === 'status'">
              <AppTag :tone="getStatusColor(record.availability)" :aria-label="`可用性：${getStatusLabel(record.availability)}`">
                {{ getStatusLabel(record.availability) }}
              </AppTag>
            </template>
          </template>
        </AppDataTable>

        <div class="commands-mobile-list" aria-label="指令列表">
          <article v-for="record in commandRows" :key="record.key" class="commands-mobile-row">
            <div class="commands-mobile-row__heading">
              <strong>{{ record.command.name }}</strong>
              <AppTag :tone="getStatusColor(record.availability)">
                {{ getStatusLabel(record.availability) }}
              </AppTag>
            </div>
            <p>{{ record.command.description || t('display.empty') }}</p>
            <dl>
              <div>
                <dt>{{ t('commands.fields.plugin') }}</dt>
                <dd>
                  <MotionRouterLink :to="buildPluginDetailLocation(record.pluginId)">
                    {{ record.pluginName }}
                  </MotionRouterLink>
                </dd>
              </div>
              <div>
                <dt>{{ t('commands.fields.usage') }}</dt>
                <dd class="monospace">{{ getUsageText(record.command) }}</dd>
              </div>
              <div>
                <dt>{{ t('commands.fields.permission') }}</dt>
                <dd>{{ getEffectivePermissionText(record.policy) }}</dd>
              </div>
            </dl>
          </article>

          <AppEmptyState
            v-if="commandRows.length === 0"
            icon="command"
            :title="t('commands.empty.title')"
            :description="t('commands.empty.description')"
          />
        </div>
      </AppCard>
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
  box-shadow: none;
}

.commands-filter-toolbar {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 12px;
  padding: 16px;

  .commands-filter-form {
    flex: 1 1 260px;
    min-width: 0;
  }

  :deep(.app-field) {
    margin-bottom: 0;
  }
}

:global(.commands-plugin-select-popup) {
  z-index: 1060;
}

:deep(.app-data-table-row:hover > td) {
  background: var(--surface-accent) !important;
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

.commands-mobile-list {
  display: none;
}

.commands-mobile-row {
  display: grid;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
}

.commands-mobile-row:last-child {
  border-bottom: 0;
}

.commands-mobile-row__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.commands-mobile-row p {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.55;
}

.commands-mobile-row dl {
  display: grid;
  gap: 8px;
  margin: 0;
}

.commands-mobile-row dl > div {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 12px;
}

.commands-mobile-row dt,
.commands-mobile-row dd {
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.commands-mobile-row dt {
  color: var(--muted);
}

@media (max-width: 639px) {
  .commands-page__actions {
    justify-content: flex-end;
  }

  .commands-data-table {
    display: none;
  }

  .commands-mobile-list {
    display: block;
  }
}
</style>
