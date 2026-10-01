<script setup lang="ts">
import { ArrowUpRightIcon } from '@lucide/vue'
import PluginPicker from '@/components/plugins/PluginPicker.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
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
import { formatCommandUsage, getPluginCommandPrefixes } from '@/lib/command-usage'
import {
  areLocationQueriesEqual,
  buildCommandsLocation,
  buildPermissionPolicyLocation,
  buildPluginDetailLocation,
  readCommandsPluginIds,
} from '@/lib/management-links'
import { t } from '@/i18n'
import {
  getCommandPermissionLabel,
  mergeCommandCenterRows,
  type PluginCommandAvailability,
  type UnifiedCommandRow,
} from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import type { PluginCommandSummary } from '@/types/api'

const route = useRoute()
const router = useRouter()
const navigate = useMotionNavigation()
const pluginsStore = usePluginsStore()
const configStore = useConfigStore()
const governanceStore = useGovernanceStore()

const pluginCollection = usePluginCollection()
const { error, items, loading, total, nextCursor, loadingMore, loaded } = pluginCollection
const { document: configDocument } = storeToRefs(configStore)
const { commandPolicy, commandPolicyError, commandPolicyLoading } = storeToRefs(governanceStore)

const selectedPluginIds = ref<string[]>([])
const selectedLoading = computed(() => selectedPluginIds.value.some(id => pluginsStore.detailLoadingByPluginId[id]))
const selectedPlugins = computed(() => {
  const known = new Map([...items.value, ...pluginsStore.knownItems].map(plugin => [plugin.id, plugin]))
  return selectedPluginIds.value.flatMap(id => known.has(id) ? [known.get(id)!] : [])
})

const pluginsWithCommands = computed(() => (
  [...(selectedPluginIds.value.length ? selectedPlugins.value : items.value)]
    .filter((plugin) => (plugin.commands?.length ?? 0) > 0)
    .sort((left, right) => compareByLabel(left.name, right.name) || compareByLabel(left.id, right.id))
))
// Each row's usage starts with its own plugin's prefix; rows known only from the policy use the global ones.
const commandPrefixesByPlugin = computed(() => new Map(pluginsWithCommands.value.map(plugin => [
  plugin.id,
  getPluginCommandPrefixes(plugin, configDocument.value?.command?.prefixes),
])))
const commandRows = computed(() => {
  const selectedIds = new Set(selectedPluginIds.value)
  const visibleIds = new Set(pluginsWithCommands.value.map(plugin => plugin.id))
  const policies = (commandPolicy.value?.commands ?? []).filter(entry => selectedIds.size
    ? selectedIds.has(entry.plugin_id)
    : visibleIds.has(entry.plugin_id) || (loaded.value && !nextCursor.value))
  return mergeCommandCenterRows(pluginsWithCommands.value, policies)
    .filter((row) => selectedIds.size === 0 || selectedIds.has(row.pluginId))
    .sort((left, right) => compareByLabel(left.command.name, right.command.name) || compareByLabel(left.pluginId, right.pluginId))
})

const pluginLoadError = computed(() => error.value ?? selectedPluginIds.value.map(id => pluginsStore.detailErrorsByPluginId[id]).find(Boolean) ?? null)
const pageErrorMessage = computed(() => pluginLoadError.value ?? commandPolicyError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && commandRows.value.length === 0)
const feedbackToast = computed(() => {
  if (pluginLoadError.value && commandRows.value.length > 0) {
    return {
      key: `commands-error:${pluginLoadError.value}`,
      level: 'error' as const,
      message: pluginLoadError.value,
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
    pluginCollection.load({}),
    configStore.fetchConfig(),
    governanceStore.fetchCommandPolicy(),
  ])
}

function compareByLabel(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function getAliasesText(command: PluginCommandSummary) {
  const aliases = command.effective_names.slice(1)
  return aliases.length ? aliases.join('、') : t('display.empty')
}

// One line says who may use the command: the effective level from the policy, or the command's own declaration
// when no policy entry is loaded (its plugin is not running, or the policy did not load).
function getPermissionText(record: UnifiedCommandRow) {
  return getCommandPermissionLabel(record.policy?.effective_permission ?? record.command.permission)
}

function getUsageText(record: UnifiedCommandRow) {
  const prefixes = commandPrefixesByPlugin.value.get(record.pluginId) ?? getPluginCommandPrefixes(null, configDocument.value?.command?.prefixes)
  return formatCommandUsage(record.command, prefixes) || t('display.empty')
}

function getStatusLabel(status: PluginCommandAvailability) {
  return t(`commands.status.${status}`)
}

function getStatusColor(status: PluginCommandAvailability) {
  switch (status) {
    case 'available':
      return 'success'
    case 'starting':
    case 'not_running':
    case 'switching':
      return 'warning'
    case 'disabled':
      return 'neutral'
    case 'not_ready':
    default:
      return 'info'
  }
}

function getCommandSourceLabel(source: PluginCommandSummary['trigger']['type']) {
  return t(`commands.commandSource.${source}`)
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
    for (const id of nextPluginIds) {
      if (!pluginsStore.knownItems.some(plugin => plugin.id === id)) void pluginsStore.ensureDetail(id).catch(() => undefined)
    }
    if (route.name !== 'commands') {
      return
    }

    const target = buildCommandsLocation(nextPluginIds)
    if (areLocationQueriesEqual(route.query, target.query ?? {})) {
      return
    }

    await router.replace(target)
  },
  { deep: true, immediate: true },
)

onMounted(() => {
  void loadCommands()
})
</script>

<template>
  <AppPage :title="t('commands.title')" width="detail">
    <template #toolbar>
      <!-- The plugin filter sits on the page like the plugin list's filters; permission policy is another page. -->
      <div class="commands-filter-toolbar">
        <div class="commands-filter-form">
          <AppField :label="t('commands.filters.plugins')">
            <PluginPicker
              v-model="selectedPluginIds"
              multiple
              :placeholder="t('commands.filters.allPlugins')"
            />
          </AppField>
        </div>
        <AppButton data-testid="commands-open-permission-policy" :aria-label="t('commands.actions.openPermissionPolicy')" @click="navigate(buildPermissionPolicyLocation())">
          {{ t('commands.actions.openPermissionPolicy') }}
          <ArrowUpRightIcon class="commands-filter-toolbar__arrow" aria-hidden="true" />
        </AppButton>
      </div>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('errors.common.loadFailed')"
      :description="pageErrorMessage ?? t('errors.common.loadFailed')"
      :loading="loading || selectedLoading || commandPolicyLoading"
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
            <AppTag>{{ commandRows.length }}</AppTag>
          </div>
        </template>

        <AppCollectionPagination
          v-if="selectedPluginIds.length === 0"
          :loaded="items.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore || loading"
          @more="pluginCollection.loadMore().catch(() => undefined)"
        />

        <AppDataTable
          class="commands-data-table app-data-table"
          :columns="commandTableColumns"
          :rows="commandRows"

          :row-key="(row: UnifiedCommandRow) => row.key"
          :loading="(loading || selectedLoading || commandPolicyLoading) && commandRows.length === 0"
          :min-width="1260" :label="t('commands.sections.commandList')"
        >
          <template #empty>
            <AppEmptyState
              icon="command"
              :title="nextCursor && !selectedPluginIds.length ? t('commands.empty.partialTitle') : t('commands.empty.title')"
              :description="nextCursor && !selectedPluginIds.length ? t('commands.empty.partialDescription') : t('commands.empty.description')"
            />
          </template>

          <template #cell="{ column, row: record }">
            <template v-if="column.key === 'command'">
              <AppTag :tone="record.conflicted ? 'warning' : 'neutral'" class="command-name-tag" :aria-label="t('commands.aria.command', { name: record.command.name })">
                {{ record.command.name }}
              </AppTag>
              <AppTag v-if="record.conflicted" tone="warning" class="command-conflict-tag">{{ t('plugins.commandConflictBadge') }}</AppTag>
            </template>

            <template v-else-if="column.key === 'aliases'">
              <span>{{ getAliasesText(record.command) }}</span>
            </template>

            <template v-else-if="column.key === 'source'">
              <AppTag>
                {{ getCommandSourceLabel(record.command.trigger.type) }}
              </AppTag>
            </template>

            <template v-else-if="column.key === 'description'">
              <span>{{ record.command.description || t('display.empty') }}</span>
            </template>

            <template v-else-if="column.key === 'usage'">
              <span>{{ getUsageText(record) }}</span>
            </template>

            <template v-else-if="column.key === 'permission'">
              <div class="command-permission-cell">
                <span>{{ getPermissionText(record) }}</span>
                <small v-if="record.policy?.permission_source === 'default_level'">{{ t('commands.permissionDefault') }}</small>
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
              <AppTag :tone="getStatusColor(record.availability)" :aria-label="t('commands.aria.availability', { status: getStatusLabel(record.availability) })">
                {{ getStatusLabel(record.availability) }}
              </AppTag>
            </template>
          </template>
        </AppDataTable>
      </AppCard>
    </template>
  </AppPage>
</template>

<style scoped lang="scss">
.commands-filter-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;

  .commands-filter-form {
    flex: 0 1 420px;
    min-width: 0;
  }

  :deep(.app-field) {
    margin-bottom: 0;
  }
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
  width: fit-content;
  border-radius: var(--radius-xs);
  color: var(--brand-foreground);
  font-weight: 600;
  text-underline-offset: 3px;
}

.command-plugin-link:hover {
  text-decoration: underline;
}

.command-plugin-link:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}

.command-name-tag {
  font-family: var(--font-mono);
  font-weight: 600;
}

.commands-filter-toolbar__arrow {
  width: 14px;
  height: 14px;
  color: var(--muted);
}

.command-plugin-cell small {
  color: var(--muted);
}

.command-permission-cell small {
  color: var(--muted);
}

.command-plugin-cell small {
  font-family: var(--font-mono);
}
</style>
