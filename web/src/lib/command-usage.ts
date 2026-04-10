import type { PluginCommandSummary } from '@/types/api'

export function getPrimaryCommandPrefix(prefixes?: string[] | null) {
  for (const prefix of prefixes ?? []) {
    const trimmed = prefix.trim()
    if (trimmed) {
      return trimmed
    }
  }
  return '/'
}

export function formatCommandUsage(command: PluginCommandSummary, prefix: string) {
  const commandName = command.name.trim()
  if (!commandName) {
    return ''
  }

  const usage = command.usage?.trim()
  const trigger = `${prefix}${commandName}`
  if (!usage) {
    return trigger
  }

  const [head, ...rest] = usage.split(/\s+/)
  const normalizedHead = head.replace(/^[^0-9A-Za-z\u4e00-\u9fa5_-]+/u, '')
  if (normalizedHead === commandName || command.aliases?.includes(normalizedHead)) {
    const tail = rest.join(' ').trim()
    return tail ? `${prefix}${normalizedHead} ${tail}` : `${prefix}${normalizedHead}`
  }

  return usage
}
