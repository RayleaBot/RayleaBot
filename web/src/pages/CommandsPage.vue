<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

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
const commandPrefix = computed(() => {
  return getPrimaryCommandPrefix(configDocument.value?.command?.prefixes)
})

const pluginsWithCommands = computed(() => (
  [...items.value]
    .filter((plugin) => (plugin.commands?.length ?? 0) > 0)
    .sort((left, right) => compareByLabel(left.name, right.name) || compareByLabel(left.id, right.id))
))

const commandRows = computed(() => {
  const selectedIds = new Set(selectedPluginIds.value)
  return flattenPluginCommands(pluginsWithCommands.value)
    .filter((row) => selectedIds.size === 0 || selectedIds.has(row.plugin.id))
    .sort((left, right) => (
      compareByLabel(left.command.name, right.command.name)
      || compareByLabel(left.plugin.id, right.plugin.id)
    ))
})

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

function getStatusType(status: PluginCommandAvailability) {
  switch (status) {
    case 'available':
      return 'success'
    case 'starting':
    case 'switching':
      return 'warning'
    case 'disabled':
      return 'info'
    case 'not_ready':
    default:
      return ''
  }
}

onMounted(() => {
  void loadCommands()
})
</script>

<template>
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('commands.title') }}</h1>
        <p>{{ t('commands.subtitle') }}</p>
      </div>

      <el-button :loading="loading" @click="loadCommands()">
        {{ t('commands.refresh') }}
      </el-button>
    </section>

    <el-card class="commands-filter-toolbar">
      <el-form label-position="top">
        <el-form-item :label="t('commands.filters.plugins')">
          <el-select
            v-model="selectedPluginIds"
            multiple
            clearable
            collapse-tags
            collapse-tags-tooltip
            :placeholder="t('commands.filters.allPlugins')"
          >
            <el-option
              v-for="plugin in pluginsWithCommands"
              :key="plugin.id"
              :label="getPluginLabel(plugin)"
              :value="plugin.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
    </el-card>

    <RetryPanel
      v-if="error && commandRows.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadCommands()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <el-table
      v-else
      :data="commandRows"
      style="width: 100%;"
      class="commands-data-table"
      :empty-text="t('commands.empty')"
    >
      <el-table-column :label="t('commands.fields.command')" min-width="160">
        <template #default="{ row }">
          <el-tag size="small" effect="plain" :type="row.conflicted ? 'warning' : 'info'">
            {{ row.command.name }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.aliases')" min-width="180">
        <template #default="{ row }">
          <span>{{ getAliasesText(row.command) }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.description')" min-width="220">
        <template #default="{ row }">
          <span>{{ row.command.description || t('display.empty') }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.usage')" min-width="220">
        <template #default="{ row }">
          <span>{{ getUsageText(row.command) }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.permission')" min-width="160">
        <template #default="{ row }">
          <span>{{ getPermissionText(row.command) }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.plugin')" min-width="220">
        <template #default="{ row }">
          <div class="command-plugin-cell">
            <strong>{{ row.plugin.name }}</strong>
            <small>{{ row.plugin.id }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('commands.fields.status')" width="120">
        <template #default="{ row }">
          <el-tag size="small" effect="plain" :type="getStatusType(row.availability)">
            {{ getStatusLabel(row.availability) }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped lang="scss">
.commands-filter-toolbar,
.commands-data-table {
  border-radius: 22px;
}

.commands-data-table {
  overflow: hidden;
  box-shadow: 0 14px 32px rgba(18, 32, 38, 0.06);
  border: 1px solid rgba(22, 33, 39, 0.08);

  :deep(.el-table__inner-wrapper) {
    background: rgba(247, 250, 246, 0.88);
  }

  :deep(.el-table__header-wrapper th) {
    background-color: transparent !important;
    border-bottom: 1px solid rgba(22, 33, 39, 0.08);
    color: var(--muted);
    font-size: 0.85rem;
    font-weight: 600;
    padding: 16px 8px;
  }

  :deep(.el-table__row) {
    background-color: transparent;
    transition: background-color 150ms ease;

    td {
      border-bottom: 1px solid rgba(22, 33, 39, 0.04);
      padding: 12px 8px;
    }

    &:hover {
      background-color: rgba(255, 255, 255, 0.6);

      td {
        background-color: transparent !important;
      }
    }
  }
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
