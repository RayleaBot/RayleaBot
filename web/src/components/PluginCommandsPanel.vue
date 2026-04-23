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

  <div v-else class="plugin-command-panel">
    <article
      v-for="command in commands"
      :key="command.name"
      class="plugin-command-card"
    >
      <div class="plugin-command-card__header">
        <div class="plugin-command-card__title">
          <a-tag :color="isConflicted(command) ? 'warning' : 'success'">
            {{ command.name }}
          </a-tag>
          <a-tag v-if="isConflicted(command)" color="warning">
            {{ t('plugins.commandConflictBadge') }}
          </a-tag>
        </div>
        <small>{{ getPermissionText(command) }}</small>
      </div>

      <dl class="plugin-command-card__meta">
        <div>
          <dt>{{ t('plugins.commandAliases') }}</dt>
          <dd>{{ getAliasesText(command) }}</dd>
        </div>
        <div>
          <dt>{{ t('plugins.commandDescription') }}</dt>
          <dd>{{ getText(command.description) }}</dd>
        </div>
        <div>
          <dt>{{ t('plugins.commandUsage') }}</dt>
          <dd>{{ getUsageText(command) }}</dd>
        </div>
      </dl>
    </article>
  </div>
</template>

<style scoped lang="scss">
.plugin-command-panel {
  display: grid;
  gap: 12px;
}

.plugin-command-card {
  display: grid;
  gap: 12px;
  padding: 16px 18px;
  border-radius: var(--radius-lg);
  background: rgba(247, 250, 246, 0.88);
  border: 1px solid rgba(22, 33, 39, 0.08);
}

.plugin-command-card__header,
.plugin-command-card__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.plugin-command-card__header small {
  color: var(--muted);
}

.plugin-command-card__meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
  margin: 0;
}

.plugin-command-card__meta div {
  display: grid;
  gap: 6px;
}

.plugin-command-card__meta dt {
  color: var(--muted);
  font-size: 0.84rem;
}

.plugin-command-card__meta dd {
  margin: 0;
  word-break: break-word;
  line-height: 1.55;
}

@media (max-width: 768px) {
  .plugin-command-card {
    padding: 14px 16px;
  }
}
</style>
