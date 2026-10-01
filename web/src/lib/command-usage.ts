import type { PluginCommandSummary, PluginSummary } from '@/types/api'

// Usages written by plugin authors often start with one of these, whatever prefixes are configured.
const commonExamplePrefixes = ['/', '#', '*', '＊']

export function getPrimaryCommandPrefix(prefixes?: string[] | null) {
  for (const prefix of prefixes ?? []) {
    const trimmed = prefix.trim()
    if (trimmed) {
      return trimmed
    }
  }
  return '/'
}

// A plugin can be addressed only through its own prefixes (Star Rail answers to * but not /), so usages start with
// the first prefix the server reports for that plugin. Live events do not carry prefixes; until the plugin's summary
// is read its list is empty and the global prefixes stand in.
export function getPluginCommandPrefixes(plugin: Pick<PluginSummary, 'command_prefixes'> | null | undefined, globalPrefixes?: string[] | null) {
  const own = (plugin?.command_prefixes ?? []).map(prefix => prefix.trim()).filter(Boolean)
  if (own.length > 0) return own
  const global = (globalPrefixes ?? []).map(prefix => prefix.trim()).filter(Boolean)
  return global.length > 0 ? global : [getPrimaryCommandPrefix(globalPrefixes)]
}

function prefixesLongestFirst(prefixes: readonly string[]) {
  return [...new Set([...prefixes, ...commonExamplePrefixes])].filter(Boolean).sort((left, right) => right.length - left.length)
}

// The first word of a usage names the command, possibly behind a prefix the author typed; a name that itself starts
// with a prefix word (三角洲密码 under the prefix 三角洲) is kept whole.
function matchCommandHead(head: string, names: readonly string[], prefixes: readonly string[]) {
  const candidates = [head, ...prefixesLongestFirst(prefixes).filter(prefix => head.startsWith(prefix)).map(prefix => head.slice(prefix.length))]
  return candidates.map(candidate => candidate.replace(/^[^0-9A-Za-z一-龥_-]+/u, '')).find(candidate => names.includes(candidate))
}

// As in the menu the bot sends, a usage starts with the plugin's first prefix, replacing any prefix the author wrote.
export function formatCommandUsage(command: PluginCommandSummary, prefixes: readonly string[]) {
  const prefix = prefixes[0] ?? ''
  if (command.trigger.type === 'pattern') {
    const usage = command.usage.trim()
    if (!usage) return ''
    const written = prefixesLongestFirst(prefixes).find(candidate => usage.startsWith(candidate))
    return `${prefix}${written ? usage.slice(written.length).trimStart() : usage}`
  }
  const commandName = command.effective_names[0]?.trim() || command.name.trim()
  if (!commandName) {
    return ''
  }

  const usage = command.usage?.trim()
  const trigger = `${prefix}${commandName}`
  if (!usage) {
    return trigger
  }

  const [head, ...rest] = usage.split(/\s+/)
  const matchedName = matchCommandHead(head, [commandName, ...command.effective_names], prefixes)
  if (matchedName) {
    const tail = rest.join(' ').trim()
    return tail ? `${prefix}${matchedName} ${tail}` : `${prefix}${matchedName}`
  }

  return usage
}
