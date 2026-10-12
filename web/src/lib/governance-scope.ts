import { t } from '@/i18n'
import { getLogProtocolLabel } from '@/lib/display'
import type { GovernanceScope, BlacklistEntry } from '@/types/governance'

export function oneBotGlobalScope(): GovernanceScope {
  return { kind: 'global', source_protocol: 'onebot11', bot_id: '' }
}

export function governanceEntryKey(entry: BlacklistEntry) {
  return JSON.stringify([entry.scope.source_protocol, entry.scope.bot_id, entry.entry_type, entry.target_id])
}

// The protocol reads as its product name; the bot account stays as entered, since it identifies the bot.
export function governanceScopeLabel(scope: GovernanceScope) {
  if (scope.kind === 'global') return t('accessLists.namespace.onebotGlobal')
  return `${getLogProtocolLabel(scope.source_protocol)} · ${scope.bot_id}`
}
