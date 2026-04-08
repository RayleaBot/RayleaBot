import type { PluginCommandSummary, PluginSummary } from '@/types/api'

export type PluginCommandAvailability = 'available' | 'starting' | 'switching' | 'not_ready' | 'disabled'

export interface CommandCenterRow {
  command: PluginCommandSummary
  plugin: PluginSummary
  availability: PluginCommandAvailability
  conflicted: boolean
}

function normalizeToken(value: string) {
  return value.trim().toLowerCase()
}

export function isPluginCommandConflicted(command: PluginCommandSummary, conflicts?: string[]) {
  const tokens = new Set((conflicts ?? []).map(normalizeToken).filter(Boolean))
  if (tokens.size === 0) {
    return false
  }

  if (tokens.has(normalizeToken(command.name))) {
    return true
  }

  return (command.aliases ?? []).some((alias) => tokens.has(normalizeToken(alias)))
}

export function getPluginCommandAvailability(plugin: PluginSummary): PluginCommandAvailability {
  if (plugin.registration_state !== 'installed') {
    return 'not_ready'
  }

  if (plugin.desired_state === 'disabled') {
    return 'disabled'
  }

  switch (plugin.runtime_state) {
    case 'running':
      return 'available'
    case 'starting':
      return 'starting'
    case 'stopping':
      return 'switching'
    case 'stopped':
    case 'backoff':
    case 'crashed':
    case 'dead_letter':
    default:
      return 'not_ready'
  }
}

export function flattenPluginCommands(plugins: PluginSummary[]): CommandCenterRow[] {
  return plugins.flatMap((plugin) => (
    (plugin.commands ?? []).map((command) => ({
      command,
      plugin,
      availability: getPluginCommandAvailability(plugin),
      conflicted: isPluginCommandConflicted(command, plugin.command_conflicts),
    }))
  ))
}
