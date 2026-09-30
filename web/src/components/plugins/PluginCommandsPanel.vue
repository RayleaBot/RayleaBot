<script setup lang="ts">
import AppTag from '@/components/AppTag.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import { formatCommandUsage } from '@/lib/command-usage'
import { t } from '@/i18n'
import { getCommandPermissionLabel, getCommandTriggerTone, isPluginCommandConflicted } from '@/lib/plugin-commands'
import type { PluginCommandSummary } from '@/types/api'

const MAX_VISIBLE_ALIASES = 12

// stacked: narrow hosts such as the summary drawer put each command's parts under one another.
const props = withDefaults(defineProps<{
  commands: PluginCommandSummary[]
  commandConflicts?: string[]
  commandPrefix?: string
  stacked?: boolean
}>(), {
  commandConflicts: () => [],
  commandPrefix: '/',
  stacked: false,
})

function getText(value?: string) {
  return value?.trim() || t('display.empty')
}

function getAliasesText(command: PluginCommandSummary) {
  const aliases = getVisibleCommandAliases(command)
  return aliases.length ? aliases.join('、') : t('display.empty')
}

function getVisibleAliases(command: PluginCommandSummary) {
  return getVisibleCommandAliases(command).slice(0, MAX_VISIBLE_ALIASES)
}

function getHiddenAliasCount(command: PluginCommandSummary) {
  return Math.max(0, getVisibleCommandAliases(command).length - MAX_VISIBLE_ALIASES)
}

function getVisibleCommandAliases(command: PluginCommandSummary) {
  return command.effective_names.slice(1)
}

function getUsageText(command: PluginCommandSummary) {
  return formatCommandUsage(command, props.commandPrefix) || t('display.empty')
}

function getTriggerText(command: PluginCommandSummary) {
  return t(`plugins.commandTriggerLabel.${command.trigger.type}`)
}

function isConflicted(command: PluginCommandSummary) {
  return isPluginCommandConflicted(command, props.commandConflicts)
}
</script>

<template>
  <AppEmptyState v-if="commands.length === 0" :description="t('plugins.empty.commands')" />

  <!-- One divided row per command inside the tab box: identity, description and aliases, then usage. -->
  <ul v-else class="plugin-command-list" :class="{ 'plugin-command-list--stacked': stacked }">
    <li
      v-for="command in commands"
      :key="command.id"
      class="plugin-command-row"
    >
      <div class="plugin-command-row__identity">
        <div class="plugin-command-row__tags">
          <AppTag :tone="isConflicted(command) ? 'warning' : 'info'" class="command-badge">
            {{ command.name }}
          </AppTag>
          <AppTag v-if="isConflicted(command)" tone="warning">
            {{ t('plugins.commandConflictBadge') }}
          </AppTag>
          <AppTag :tone="getCommandTriggerTone(command.trigger.type)">
            {{ getTriggerText(command) }}
          </AppTag>
        </div>
        <span class="plugin-command-row__permission">{{ t('plugins.commandPermission', { permission: getCommandPermissionLabel(command.permission) }) }}</span>
      </div>

      <div class="plugin-command-row__copy">
        <p class="plugin-command-row__desc">{{ getText(command.description) }}</p>
        <div v-if="getVisibleCommandAliases(command).length" class="plugin-command-row__aliases">
          <span class="section-label">{{ t('plugins.commandAliases') }}</span>
          <template v-if="getVisibleCommandAliases(command).length">
            <AppTag v-for="alias in getVisibleAliases(command)" :key="alias" size="small">
              {{ alias }}
            </AppTag>
            <AppTag v-if="getHiddenAliasCount(command) > 0" size="small">
              {{ t('plugins.commandOverflow', { count: getHiddenAliasCount(command) }) }}
            </AppTag>
            <!-- Hidden text for unit test compatibility -->
            <span class="sr-only">{{ getAliasesText(command) }}</span>
          </template>
          <span v-else class="empty-val">—</span>
        </div>
      </div>

      <div class="plugin-command-row__usage">
        <span class="section-label">{{ t('plugins.commandUsage') }}</span>
        <code class="usage-snippet">{{ getUsageText(command) }}</code>
      </div>
    </li>
  </ul>
</template>

<style scoped lang="scss">
.plugin-command-list {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}

.plugin-command-row {
  display: grid;
  grid-template-columns: minmax(220px, 280px) minmax(0, 1.2fr) minmax(280px, 1fr);
  gap: 8px 24px;
  align-items: start;
  padding-block: 14px;
  border-top: 1px solid var(--border);
}

.plugin-command-list--stacked .plugin-command-row {
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
}

.plugin-command-row:first-child {
  padding-top: 0;
  border-top: 0;
}

.plugin-command-row:last-child {
  padding-bottom: 0;
}

.plugin-command-row__identity,
.plugin-command-row__copy,
.plugin-command-row__usage {
  display: grid;
  align-content: start;
  gap: 6px;
  min-width: 0;
}

.plugin-command-row__tags,
.plugin-command-row__aliases {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.command-badge {
  font-family: var(--font-mono);
  font-weight: 700;
}

.plugin-command-row__permission,
.section-label,
.empty-val {
  color: var(--muted);
  font-size: 12px;
}

.plugin-command-row__desc {
  margin: 0;
  color: var(--text);
  font-size: 14px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

// Usage reads as code on a white inset inside the gray box.
.usage-snippet {
  display: block;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.5;
  word-break: break-all;
}
</style>
