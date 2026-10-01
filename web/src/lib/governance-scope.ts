import { t } from '@/i18n'
import { getLogProtocolLabel } from '@/lib/display'
import type { GovernanceScope, BlacklistEntry } from '@/types/governance'

export function oneBotGlobalScope(): GovernanceScope {
  return { kind: 'global', source_protocol: 'onebot11', source_adapter: '', bot_id: '' }
}

export function governanceEntryKey(entry: BlacklistEntry) {
  return JSON.stringify([entry.scope.source_protocol, entry.scope.source_adapter, entry.scope.bot_id, entry.entry_type, entry.target_id])
}

// The protocol reads as its product name; the connection and bot IDs stay as entered, since they identify the bot.
export function governanceScopeLabel(scope: GovernanceScope) {
  if (!scope.source_adapter) return t('accessLists.namespace.onebotGlobal')
  return `${getLogProtocolLabel(scope.source_protocol)} · ${scope.source_adapter} · ${scope.bot_id}`
}
