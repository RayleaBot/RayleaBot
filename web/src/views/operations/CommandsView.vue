<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatCommandUsage, getPrimaryCommandPrefix } from '@/lib/command-usage'
import { t } from '@/i18n'
import { flattenPluginCommands, type PluginCommandAvailability } from '@/lib/plugin-commands'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginCommandSummary, PluginSummary } from '@/types/api'

const pluginsStore = usePluginsStore()
const configStore = useConfigStore()
const { error, items, loading } = storeToRefs(pluginsStore)
const { document: configDocument } = storeToRefs(configStore)

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

const tableColumns = computed(() => [
  { title: t('commands.fields.command'), key: 'command', dataIndex: 'command', width: 180 },
  { title: t('commands.fields.aliases'), key: 'aliases', dataIndex: 'aliases', width: 180 },
  { title: t('commands.fields.description'), key: 'description', dataIndex: 'description' },
  { title: t('commands.fields.usage'), key: 'usage', dataIndex: 'usage' },
  { title: t('commands.fields.permission'), key: 'permission', dataIndex: 'permission', width: 180 },
  { title: t('commands.fields.plugin'), key: 'plugin', dataIndex: 'plugin', width: 220 },
  { title: t('commands.fields.status'), key: 'status', dataIndex: 'status', width: 120 },
])

async function loadCommands() {
  try {
    await Promise.all([
      pluginsStore.fetchList(),
      configStore.fetchConfig().catch(() => undefined),
    ])
  } catch {
    // store error state drives the page
  }
}

function compareByLabel(left: string, right: string) {
  return left.localeCompare(right, 'zh-CN')
}

function getPluginLabel(plugin: PluginSummary) {
  return `${plugin.name}（${plugin.id}）`
}

function getAliasesText(command: PluginCommandSummary) {
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

function getSelectPopupContainer(triggerNode: HTMLElement) {
  return triggerNode.parentElement ?? triggerNode
}

onMounted(() => {
  void loadCommands()
})
</script>

<template>
  <AppPage :title="t('commands.title')" :description="t('commands.subtitle')" full-height>
    <template #extra>
      <a-button :loading="loading" @click="loadCommands()">
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
      v-if="error && commandRows.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadCommands()"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <a-table
      v-else
      class="commands-data-table app-data-table"
      :columns="tableColumns"
      :data-source="commandRows"
      :pagination="false"
      :row-key="(row) => `${row.plugin.id}-${row.command.name}`"
      :scroll="{ x: 1180 }"
    >
      <template #emptyText>
        {{ t('commands.empty') }}
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
  </AppPage>
</template>

<style scoped lang="scss">
.commands-filter-toolbar,
.commands-data-table {
  border-radius: 10px;
}

.command-plugin-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.command-plugin-cell small {
  color: var(--muted);
  font-family: "Cascadia Mono", "Consolas", monospace;
}
</style>
