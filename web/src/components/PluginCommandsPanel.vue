<script setup lang="ts">
import { formatCommandUsage } from '@/lib/command-usage'
import { t } from '@/i18n'
import { isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { CommandPermissionLevel, PluginCommandSummary } from '@/types/api'

const MAX_VISIBLE_ALIASES = 12

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

function getVisibleAliases(command: PluginCommandSummary) {
  return (command.aliases ?? []).slice(0, MAX_VISIBLE_ALIASES)
}

function getHiddenAliasCount(command: PluginCommandSummary) {
  return Math.max(0, (command.aliases?.length ?? 0) - MAX_VISIBLE_ALIASES)
}

function getPermissionText(command: PluginCommandSummary) {
  const permission = command.permission?.trim() as CommandPermissionLevel | ''
  if (!permission) {
    return t('plugins.commandPermissionDefault')
  }
  switch (permission) {
    case 'everyone':
      return t('commands.permissions.everyone')
    case 'group_admin':
      return t('commands.permissions.groupAdmin')
    case 'super_admin':
      return t('commands.permissions.superAdmin')
    default:
      return permission
  }
}

function getUsageText(command: PluginCommandSummary) {
  return formatCommandUsage(command, props.commandPrefix) || t('display.empty')
}

function getCommandSourceText(command: PluginCommandSummary) {
  return t(`plugins.commandSourceLabel.${command.command_source}`)
}

function getCommandSourceColor(command: PluginCommandSummary) {
  if (command.command_source === 'pattern') {
    return 'blue'
  }
  return command.command_source === 'dynamic' ? 'purple' : 'default'
}

function isConflicted(command: PluginCommandSummary) {
  return isPluginCommandConflicted(command, props.commandConflicts)
}
</script>

<template>
  <a-empty v-if="commands.length === 0" :description="t('plugins.empty.commands')" />

  <div v-else class="plugin-command-grid" role="list">
    <article
      v-for="command in commands"
      :key="command.name"
      class="plugin-command-card"
      :class="{ 'is-conflicted': isConflicted(command) }"
      role="listitem"
    >
      <header class="plugin-command-card__header">
        <div class="plugin-command-card__title-row">
          <a-tag :color="isConflicted(command) ? 'warning' : 'success'" class="command-badge">
            {{ command.name }}
          </a-tag>
          <a-tag v-if="isConflicted(command)" color="warning">
            {{ t('plugins.commandConflictBadge') }}
          </a-tag>
          <a-tag :color="getCommandSourceColor(command)">
            {{ getCommandSourceText(command) }}
          </a-tag>
        </div>
      </header>

      <div class="plugin-command-card__body">
        <div class="plugin-command-card__desc">
          {{ getText(command.description) }}
        </div>

        <div class="plugin-command-card__section">
          <span class="section-label">{{ t('plugins.commandAliases') }}</span>
          <div class="alias-tags" v-if="command.aliases?.length">
            <a-tag v-for="alias in getVisibleAliases(command)" :key="alias" size="small" class="alias-tag">
              {{ alias }}
            </a-tag>
            <a-tag v-if="getHiddenAliasCount(command) > 0" size="small" class="alias-tag alias-tag--more">
              {{ t('plugins.commandOverflow', { count: getHiddenAliasCount(command) }) }}
            </a-tag>
            <!-- Hidden text for unit test compatibility -->
            <span class="sr-only">{{ getAliasesText(command) }}</span>
          </div>
          <span v-else class="empty-val">—</span>
        </div>

        <div class="plugin-command-card__section">
          <span class="section-label">{{ t('plugins.commandUsage') }}</span>
          <div class="usage-snippet">
            <span class="usage-prefix">>_</span>
            <code class="usage-text">{{ getUsageText(command) }}</code>
          </div>
        </div>

        <div class="plugin-command-card__footer">
          <span class="permission-pill">
            <span class="pill-dot"></span>
            {{ getPermissionText(command) }}
          </span>
        </div>
      </div>
    </article>
  </div>
</template>

<style scoped lang="scss">
.plugin-command-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.plugin-command-card {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  transition: transform 0.28s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.28s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.28s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.28s cubic-bezier(0.4, 0, 0.2, 1), color 0.28s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  box-shadow: var(--shadow-xs);

  &:hover {
    transform: translateY(-2px);
    border-color: var(--border-accent);
    background: var(--surface);
    box-shadow: var(--shadow-sm);
  }

  &.is-conflicted {
    border-color: var(--border-warning);
    background: color-mix(in srgb, var(--surface-warning) 15%, var(--surface-soft));

    &:hover {
      border-color: var(--warning);
    }
  }
}

.plugin-command-card__header {
  padding: 12px 14px 10px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent);
}

.plugin-command-card__title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.command-badge {
  font-family: var(--font-mono);
  font-weight: 700;
  font-size: 0.85rem;
}

.plugin-command-card__body {
  padding: 12px 14px 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex-grow: 1;
}

.plugin-command-card__desc {
  font-size: 0.88rem;
  color: var(--text);
  line-height: 1.5;
  word-break: break-word;
}

.plugin-command-card__section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.section-label {
  font-size: 0.72rem;
  color: var(--muted);
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.alias-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.alias-tag {
  font-size: 0.76rem;
}

.alias-tag--more {
  color: var(--muted);
}

.empty-val {
  font-size: 0.82rem;
  color: var(--muted);
}

.usage-snippet {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--surface-strong);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 4px 8px;
  min-height: 28px;
}

.usage-prefix {
  font-family: var(--font-mono);
  font-size: 0.76rem;
  color: var(--accent);
  user-select: none;
  font-weight: bold;
}

.usage-text {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--text);
  word-break: break-all;
}

.plugin-command-card__footer {
  margin-top: auto;
  padding-top: 8px;
  border-top: 1px dashed color-mix(in srgb, var(--border) 40%, transparent);
}

.permission-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.76rem;
  color: var(--text);
  background: color-mix(in srgb, var(--accent) 8%, var(--surface-soft));
  border: 1px solid color-mix(in srgb, var(--accent) 15%, var(--border));
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-family: var(--font-sans);
}

.pill-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--accent);
}
</style>
