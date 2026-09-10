import type {
  GovernanceCommandPolicyEntry,
  PluginCommandSummary,
  PluginSummary,
} from '@/types/api'

export type PluginCommandAvailability = 'available' | 'starting' | 'switching' | 'not_ready' | 'disabled'

export interface UnifiedCommandRow {
  key: string
  pluginId: string
  pluginName: string
  command: PluginCommandSummary
  policy: GovernanceCommandPolicyEntry | null
  availability: PluginCommandAvailability
  conflicted: boolean
}

function normalizeToken(value: string) {
  return value.trim().toLowerCase()
}

export function isPluginCommandConflicted(command: PluginCommandSummary, conflicts?: string[]) {
  const tokens = new Set((conflicts ?? []).map(normalizeToken).filter(Boolean))
  return command.effective_names.some((name) => tokens.has(normalizeToken(name)))
}

export function getPluginCommandAvailability(plugin: PluginSummary): PluginCommandAvailability {
  switch (plugin.state) {
    case 'running':
      return 'available'
    case 'starting':
    case 'enabled':
      return 'starting'
    case 'stopping':
      return 'switching'
    case 'disabled':
      return 'disabled'
    default:
      return 'not_ready'
  }
}

export function mergeCommandCenterRows(
  plugins: PluginSummary[],
  policyCommands: GovernanceCommandPolicyEntry[],
): UnifiedCommandRow[] {
  const policyIndex = new Map(policyCommands.map((entry) => [policyEntryKey(entry), entry]))
  const matched = new Set<string>()
  const rows: UnifiedCommandRow[] = []

  for (const plugin of plugins) {
    for (const command of plugin.commands) {
      const key = commandIdentity(plugin.id, command.id)
      const policy = policyIndex.get(key) ?? null
      if (policy) {
        matched.add(key)
      }
      rows.push({
        key,
        pluginId: plugin.id,
        pluginName: plugin.name,
        command,
        policy,
        availability: getPluginCommandAvailability(plugin),
        conflicted: isPluginCommandConflicted(command, plugin.command_conflicts),
      })
    }
  }

  for (const entry of policyCommands) {
    const key = policyEntryKey(entry)
    if (matched.has(key)) {
      continue
    }
    rows.push({
      key: `policy:${key}`,
      pluginId: entry.plugin_id,
      pluginName: entry.plugin_name,
      command: policyEntryToCommand(entry),
      policy: entry,
      availability: 'not_ready',
      conflicted: false,
    })
  }

  return rows
}

function policyEntryToCommand(entry: GovernanceCommandPolicyEntry): PluginCommandSummary {
  return {
    id: entry.command_id,
    name: entry.command,
    effective_names: [entry.command, ...entry.aliases],
    description: '',
    usage: '',
    permission: entry.declared_permission ?? entry.effective_permission,
    trigger: entry.trigger,
  }
}

function commandIdentity(pluginID: string, commandID: string) {
  return `${pluginID}:${normalizeToken(commandID)}`
}

function policyEntryKey(entry: GovernanceCommandPolicyEntry) {
  return commandIdentity(entry.plugin_id, entry.command_id)
}
