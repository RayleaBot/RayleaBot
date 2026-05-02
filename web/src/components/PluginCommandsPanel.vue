<script setup lang="ts">
import { formatCommandUsage } from '@/lib/command-usage'
import { t } from '@/i18n'
import { isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { PluginCommandSummary } from '@/types/api'

const props = withDefaults(defineProps<{
  commands: PluginCommandSummary[]
  commandConflicts?: string[]
  commandPrefix?: string
}>(), {
  commandConflicts: () => [],
  commandPrefix: '/',
})

function getText(value?: string) {
  return value?.trim() || t('display.empty')
}

function getAliasesText(command: PluginCommandSummary) {
  return command.aliases?.length ? command.aliases.join(', ') : t('display.empty')
}

function getPermissionText(command: PluginCommandSummary) {
  return command.permission?.trim() || t('plugins.commandPermissionDefault')
}

function getUsageText(command: PluginCommandSummary) {
  return formatCommandUsage(command, props.commandPrefix) || t('display.empty')
}

function isConflicted(command: PluginCommandSummary) {
  return isPluginCommandConflicted(command, props.commandConflicts)
}
</script>

<template>
  <a-empty v-if="commands.length === 0" :description="t('plugins.empty.commands')" />

  <div v-else class="plugin-command-panel" role="list">
    <article
      v-for="command in commands"
      :key="command.name"
      class="plugin-command-row"
      :class="{ 'is-conflicted': isConflicted(command) }"
      role="listitem"
    >
      <div class="plugin-command-row__command">
        <a-tag :color="isConflicted(command) ? 'warning' : 'success'">
          {{ command.name }}
        </a-tag>
        <a-tag v-if="isConflicted(command)" color="warning">
          {{ t('plugins.commandConflictBadge') }}
        </a-tag>
      </div>

      <dl class="plugin-command-row__meta">
        <div class="plugin-command-row__aliases">
          <dt>{{ t('plugins.commandAliases') }}</dt>
          <dd>{{ getAliasesText(command) }}</dd>
        </div>
        <div class="plugin-command-row__description">
          <dt>{{ t('plugins.commandDescription') }}</dt>
          <dd>{{ getText(command.description) }}</dd>
        </div>
        <div class="plugin-command-row__usage">
          <dt>{{ t('plugins.commandUsage') }}</dt>
          <dd>{{ getUsageText(command) }}</dd>
        </div>
        <div class="plugin-command-row__permission">
          <dt>{{ t('plugins.fields.permission') }}</dt>
          <dd>{{ getPermissionText(command) }}</dd>
        </div>
      </dl>
    </article>
  </div>
</template>

<style scoped lang="scss">
.plugin-command-panel {
  display: grid;
  gap: 8px;
  container-type: inline-size;
}

.plugin-command-row {
  display: grid;
  grid-template-columns: minmax(108px, 0.28fr) minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-soft) 72%, transparent);
}

.plugin-command-row:hover {
  background: color-mix(in srgb, var(--accent) 5%, var(--surface-soft));
}

.plugin-command-row__command {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.plugin-command-row__meta {
  display: grid;
  grid-template-columns: minmax(72px, 0.62fr) minmax(116px, 1.1fr) minmax(116px, 1.12fr) minmax(78px, 0.72fr);
  gap: 8px 12px;
  margin: 0;
}

.plugin-command-row__meta div {
  display: grid;
  min-width: 0;
  gap: 4px;
}

.plugin-command-row__meta dt {
  color: var(--muted);
  font-size: 0.74rem;
}

.plugin-command-row__meta dd {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--text);
  font-size: 0.86rem;
  line-height: 1.45;
}

.plugin-command-row__usage dd,
.plugin-command-row__permission dd {
  font-family: var(--font-mono);
}

@container (max-width: 520px) {
  .plugin-command-row,
  .plugin-command-row__meta {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .plugin-command-row {
    padding: 10px;
  }
}
</style>
