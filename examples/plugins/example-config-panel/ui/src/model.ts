import type { BotAccount } from '@rayleabot/plugin-ui'

export interface ConfigPanelSettings { default_city: string; unit: 'celsius' | 'fahrenheit' }

export function normalizeSettings(value: Record<string, unknown>): ConfigPanelSettings {
  return {
    default_city: typeof value.default_city === 'string' ? value.default_city : '北京',
    unit: value.unit === 'fahrenheit' ? 'fahrenheit' : 'celsius',
  }
}

export function normalizeNotes(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((note): note is string => typeof note === 'string') : []
}

// The host's directory entry for the selected account; it changes while the page is open,
// so it is only shown, never used to decide where data belongs.
export function accountSummary(account: BotAccount): string {
  const name = account.nickname ? `${account.nickname}（${account.bot_id}）` : account.bot_id
  if (account.state === 'online') return `当前账号：${name}，经 ${account.adapter_display_name}`
  if (account.state === 'offline') return `当前账号：${name}，连接离线`
  return `当前账号：${name}，没有连接`
}
