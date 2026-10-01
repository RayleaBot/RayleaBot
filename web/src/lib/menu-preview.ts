import { buildVersionLabel } from '@/lib/build-info'
import { getPrimaryCommandPrefix } from '@/lib/command-usage'
import type { CommandPermissionLevel, PluginCommandSummary, PluginSummary } from '@/types/api'

// The menu center previews the data the server builds for the help.menu template. Titles, the
// fallback group name and the version placeholder follow the server's chat output, not the UI language.
export const defaultMenuCommands = ['help', '帮助']
const defaultRenderFooterTemplate = 'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}'
const developmentVersion = '开发版本'
const ungroupedCommandsTitle = '命令'
const rootItemFallbackDescription = '可用插件菜单'
const systemMenuPluginName = 'RayleaBot'
const commonCommandPrefixes = ['/', '#', '*', '＊']

type CommandUsagePartKind = 'literal' | 'required' | 'optional'

interface CommandUsagePart {
  kind: CommandUsagePartKind
  text: string
}

export interface MenuPreviewContext {
  // Effective menu prefixes: the menu's own prefixes, or the global command prefixes.
  prefixes: string[]
  defaultPermission?: string | null
}

type MenuPreviewPlugin = Pick<PluginSummary, 'id' | 'name' | 'version' | 'description' | 'help' | 'commands' | 'command_groups'>

// The menu lists every enabled plugin with a valid manifest that has commands or help to show, running or not.
export function isMenuPreviewPlugin(plugin: Pick<PluginSummary, 'state' | 'help' | 'commands'>) {
  if (plugin.state === 'disabled' || plugin.state === 'invalid') return false
  return plugin.commands.length > 0 || Boolean(plugin.help?.title?.trim() || plugin.help?.summary?.trim())
}

export function normalizeMenuTokens(values?: readonly string[] | null, fallback: string[] = []) {
  const seen = new Set<string>()
  const items: string[] = []
  for (const value of values ?? fallback) {
    const trimmed = String(value).trim()
    if (!trimmed || seen.has(trimmed)) {
      continue
    }
    seen.add(trimmed)
    items.push(trimmed)
  }
  return items
}

export function buildMenuTriggerExamples(target: string | null, prefixes: string[], commands: readonly string[]) {
  if (!target) {
    return []
  }
  const menuCommands = normalizeMenuTokens(commands, defaultMenuCommands)
  const primaryPrefix = getPrimaryCommandPrefix(prefixes)
  const secondaryPrefix = prefixes[1] || primaryPrefix
  return [
    `${primaryPrefix}${menuCommands[0] || defaultMenuCommands[0]} ${target}`,
    `${secondaryPrefix}${target}${menuCommands[1] || menuCommands[0] || defaultMenuCommands[1]}`,
  ]
}

export function renderMenuPreviewFooter(template?: string, plugin?: Pick<PluginSummary, 'id' | 'name' | 'version'> | null) {
  const source = template?.trim() || defaultRenderFooterTemplate
  const pluginName = plugin ? plugin.name || plugin.id : systemMenuPluginName
  const version = String(plugin?.version ?? '').trim()
  return source
    .replaceAll('{{rayleabot_version}}', buildVersionLabel)
    .replaceAll('{{plugin_name}}', pluginName)
    .replaceAll('{{plugin_version}}', version && version !== '0.0.0-dev' ? version : developmentVersion)
}

export function buildRootMenuItems(plugins: readonly MenuPreviewPlugin[]) {
  return plugins.map((plugin) => ({
    name: plugin.name || plugin.id,
    description: plugin.description?.trim() || plugin.help?.summary?.trim() || rootItemFallbackDescription,
  }))
}

// Groups follow the plugin's declared command groups; as in the menu the bot sends, commands outside every group
// come first under their own title.
export function buildPluginMenuGroups(plugin: MenuPreviewPlugin, context: MenuPreviewContext) {
  const commandByID = new Map(plugin.commands.map((command) => [command.id, command]))
  const covered = new Set<string>()
  const groups: Array<{ title: string, items: Array<Record<string, unknown>> }> = []
  for (const group of plugin.command_groups) {
    const items = group.commands.flatMap((commandID) => {
      const command = commandByID.get(commandID)
      if (!command) {
        return []
      }
      covered.add(command.id)
      return [buildCommandPreviewItem(command, context)]
    })
    if (items.length > 0) {
      groups.push({ title: group.title, items })
    }
  }

  const ungrouped = plugin.commands
    .filter((command) => !covered.has(command.id))
    .map((command) => buildCommandPreviewItem(command, context))
  if (ungrouped.length > 0) {
    groups.unshift({ title: ungroupedCommandsTitle, items: ungrouped })
  }

  return groups
}

function buildCommandPreviewItem(command: PluginCommandSummary, context: MenuPreviewContext) {
  return {
    name: command.effective_names[0] || command.name,
    ...commandUsageFields(command, context.prefixes),
    description: command.description || command.name,
    permission: normalizeCommandPermission(String(command.permission ?? '').trim() || context.defaultPermission),
  }
}

function commandUsageFields(command: PluginCommandSummary, prefixes: string[]) {
  if (command.trigger.type === 'pattern') {
    const usage = stripCommandExamplePrefix(String(command.usage ?? '').trim(), prefixes)
    return {
      trigger_type: command.trigger.type,
      command_prefixes: prefixes,
      usage,
      usage_parts: commandUsageParts(usage, 'literal'),
    }
  }
  const usageArgs = commandUsageArgs(command.effective_names[0] || command.name, command.usage, prefixes)
  return {
    trigger_type: command.trigger.type,
    command_prefixes: prefixes,
    ...(usageArgs
      ? {
          usage_args: usageArgs,
          usage_parts: commandUsageParts(usageArgs, 'required'),
        }
      : {}),
  }
}

function normalizeCommandPermission(value: unknown): CommandPermissionLevel {
  const permission = String(value ?? '').trim()
  return permission === 'super_admin' || permission === 'group_admin' ? permission : 'everyone'
}

// Usage examples may already carry a command prefix; the preview adds the configured ones itself.
function stripCommandExamplePrefix(value: string, prefixes: string[]) {
  const examplePrefixes = [...new Set([...prefixes, ...commonCommandPrefixes])]
    .map((prefix) => prefix.trim())
    .filter(Boolean)
    .sort((left, right) => right.length - left.length)
  const matchedPrefix = examplePrefixes.find((prefix) => value.startsWith(prefix))
  return matchedPrefix ? value.slice(matchedPrefix.length).trimStart() : value
}

function commandUsageArgs(commandName: string, usage: string | null | undefined, prefixes: string[]) {
  const command = commandName.trim()
  let value = String(usage ?? '').trim()
  if (!command || !value) {
    return ''
  }
  value = stripCommandExamplePrefix(value, prefixes)
  if (value === command) {
    return ''
  }
  if (value.startsWith(command)) {
    return value.slice(command.length).trim()
  }
  return ''
}

// [optional] and <required> placeholders become separate parts around the plain text.
function commandUsageParts(usage: string, plainKind: 'literal' | 'required'): CommandUsagePart[] {
  const source = usage.trim()
  if (!source) {
    return []
  }

  const parts: CommandUsagePart[] = []
  const append = (kind: CommandUsagePartKind, text: string) => {
    const normalized = text.trim()
    if (normalized) {
      parts.push({ kind, text: normalized })
    }
  }
  const pattern = /\[([^\]]+)\]|<([^>]+)>/g
  let cursor = 0
  for (const match of source.matchAll(pattern)) {
    const index = match.index ?? cursor
    append(plainKind, source.slice(cursor, index))
    append(match[1] === undefined ? 'required' : 'optional', match[1] ?? match[2] ?? '')
    cursor = index + match[0].length
  }
  append(plainKind, source.slice(cursor))
  return parts
}
