import { t } from '@/i18n'
import type { AdapterProtocol, BotAccount, BotAccountRef } from '@/types/api'

const protocols: readonly AdapterProtocol[] = ['onebot11', 'qqofficial']

// A bot account travels in URLs as "<source_protocol>:<bot_id>". The account part
// is used verbatim, so only the first colon separates the two halves.
export function parseBotAccountKey(value: unknown): BotAccountRef | null {
  if (typeof value !== 'string') return null
  const separator = value.indexOf(':')
  const protocol = value.slice(0, separator) as AdapterProtocol
  const botID = value.slice(separator + 1)
  if (separator <= 0 || !botID || !protocols.includes(protocol)) return null
  return { source_protocol: protocol, bot_id: botID }
}

export function formatBotAccountKey(account: BotAccountRef): string {
  return `${account.source_protocol}:${account.bot_id}`
}

// The account's own name: nickname with the account in parentheses, or the account alone.
export function botAccountName(account: Pick<BotAccount, 'nickname' | 'bot_id'>): string {
  return account.nickname ? `${account.nickname}（${account.bot_id}）` : account.bot_id
}

// Connections lead the label because operators think of each connection as one bot; an
// account without a connection names itself and says so, keeping its data reachable.
export function botAccountLabel(account: BotAccount): string {
  const name = botAccountName(account)
  if (account.state === 'no_connection') return t('botAccounts.label.noConnection', { name })
  const label = `${account.adapter_display_name} · ${name}`
  return account.state === 'offline' ? t('botAccounts.label.offline', { label }) : label
}

export function botAccountIdLabel(account: BotAccountRef): string {
  return account.source_protocol === 'qqofficial'
    ? t('botAccounts.qqOfficialId', { id: account.bot_id })
    : t('botAccounts.qqNumber', { id: account.bot_id })
}
