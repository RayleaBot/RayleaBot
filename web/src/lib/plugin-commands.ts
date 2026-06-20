import type {
  GovernanceCommandPolicyEntry,
  PluginCommandSource,
  PluginCommandSummary,
  PluginSummary,
} from '@/types/api'

export type PluginCommandAvailability = 'available' | 'starting' | 'switching' | 'not_ready' | 'disabled'

export interface CommandCenterRow {
  command: PluginCommandSummary
  plugin: PluginSummary
  availability: PluginCommandAvailability
  conflicted: boolean
}

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
  if (tokens.size === 0) {
    return false
  }

  if (tokens.has(normalizeToken(command.name))) {
    return true
  }

  return (command.aliases ?? []).some((alias) => tokens.has(normalizeToken(alias)))
}

export function getPluginCommandAvailability(plugin: PluginSummary): PluginCommandAvailability {
  switch (plugin.state) {
    case 'running':
      return 'available'
    case 'starting':
      return 'starting'
    case 'stopping':
      return 'switching'
    case 'enabled':
      return 'starting'
    case 'disabled':
      return 'disabled'
    case 'failed':
    case 'invalid':
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

export function mergeCommandCenterRows(
  plugins: PluginSummary[],
  policyCommands: GovernanceCommandPolicyEntry[],
): UnifiedCommandRow[] {
  const policyIndex = createPolicyCommandIndex(policyCommands)
  const matchedPolicyKeys = new Set<string>()
  const rows: UnifiedCommandRow[] = []

  for (const plugin of plugins) {
    for (const command of plugin.commands ?? []) {
      const policyMatch = findPolicyCommand(policyIndex, plugin.id, command)
      if (policyMatch) {
        matchedPolicyKeys.add(policyMatch.key)
      }
      rows.push({
        key: commandRowKey(plugin.id, command),
        pluginId: plugin.id,
        pluginName: plugin.name,
        command,
        policy: policyMatch?.entry ?? null,
        availability: getPluginCommandAvailability(plugin),
        conflicted: isPluginCommandConflicted(command, plugin.command_conflicts),
      })
    }
  }

  for (const entry of policyCommands) {
    const key = policyEntryKey(entry)
    if (matchedPolicyKeys.has(key)) {
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

function createPolicyCommandIndex(entries: GovernanceCommandPolicyEntry[]) {
  const byDeclaration = new Map<string, { key: string, entry: GovernanceCommandPolicyEntry }>()
  const byCommand = new Map<string, { key: string, entry: GovernanceCommandPolicyEntry }>()

  for (const entry of entries) {
    const indexed = { key: policyEntryKey(entry), entry }
    const declarationID = entry.declaration_id?.trim()
    if (declarationID) {
      byDeclaration.set(`${entry.plugin_id}:${normalizeToken(declarationID)}`, indexed)
    }
    byCommand.set(`${entry.plugin_id}:${normalizeToken(entry.command)}`, indexed)
  }

  return { byDeclaration, byCommand }
}

function findPolicyCommand(
  index: ReturnType<typeof createPolicyCommandIndex>,
  pluginID: string,
  command: PluginCommandSummary,
) {
  const declarationID = command.declaration_id?.trim()
  if (declarationID) {
    const byDeclaration = index.byDeclaration.get(`${pluginID}:${normalizeToken(declarationID)}`)
    if (byDeclaration) {
      return byDeclaration
    }
    return undefined
  }

  return index.byCommand.get(`${pluginID}:${normalizeToken(command.name)}`)
}

function policyEntryToCommand(entry: GovernanceCommandPolicyEntry): PluginCommandSummary {
  return {
    name: entry.command,
    aliases: [...entry.aliases],
    command_source: entry.command_source as PluginCommandSource,
    declaration_id: entry.declaration_id,
  }
}

function commandRowKey(pluginID: string, command: PluginCommandSummary) {
  const declarationID = command.declaration_id?.trim()
  if (declarationID) {
    return `plugin:${pluginID}:declaration:${normalizeToken(declarationID)}`
  }
  return `plugin:${pluginID}:command:${normalizeToken(command.name)}`
}

function policyEntryKey(entry: GovernanceCommandPolicyEntry) {
  const declarationID = entry.declaration_id?.trim()
  if (declarationID) {
    return `${entry.plugin_id}:declaration:${normalizeToken(declarationID)}`
  }
  return `${entry.plugin_id}:command:${normalizeToken(entry.command)}`
}
