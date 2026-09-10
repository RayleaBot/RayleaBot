import { t } from '@/i18n'
import type { GovernanceScope, BlacklistEntry } from '@/types/governance'

export function oneBotGlobalScope(): GovernanceScope {
  return { kind: 'global', source_protocol: 'onebot11', source_adapter: '', bot_id: '' }
}

export function governanceEntryKey(entry: BlacklistEntry) {
  return JSON.stringify([entry.scope.source_protocol, entry.scope.source_adapter, entry.scope.bot_id, entry.entry_type, entry.target_id])
}

export function governanceScopeLabel(scope: GovernanceScope) {
  if (!scope.source_adapter) return t('accessLists.namespace.onebotGlobal')
  return `${scope.source_protocol} · ${scope.source_adapter} · ${scope.bot_id}`
}
