import { describe, expect, it } from 'vitest'

import { formatCommandUsage, getPluginCommandPrefixes } from '@/lib/command-usage'
import type { PluginCommandSummary } from '@/types/api'

function command(name: string, usage: string, type: 'exact' | 'pattern' = 'exact'): PluginCommandSummary {
  return { id: name, name, effective_names: [name], description: '', usage, permission: 'everyone', trigger: type === 'pattern' ? { type, pattern: '.*' } : { type, names: [name] } }
}

// Usages follow the menu the bot sends: the plugin's first prefix, replacing whatever prefix the author wrote.
describe('command usage', () => {
  const starRail = ['*', '星铁', 'sr']

  it('starts with the plugin prefix in place of the one the author wrote', () => {
    expect(formatCommandUsage(command('体力', '#体力'), starRail)).toBe('*体力')
    expect(formatCommandUsage(command('体力', '星铁体力 [UID]'), starRail)).toBe('*体力 [UID]')
    expect(formatCommandUsage(command('卡片', '#卡片 [UID]', 'pattern'), starRail)).toBe('*卡片 [UID]')
  })

  it('keeps a command name that itself starts with a prefix word', () => {
    expect(formatCommandUsage(command('三角洲密码', '三角洲密码'), ['三角洲'])).toBe('三角洲三角洲密码')
  })

  it('falls back to the global prefixes until the plugin reports its own', () => {
    expect(getPluginCommandPrefixes({ command_prefixes: [] }, ['!'])).toEqual(['!'])
    expect(getPluginCommandPrefixes({ command_prefixes: ['*'] }, ['!'])).toEqual(['*'])
    expect(getPluginCommandPrefixes(null, [])).toEqual(['/'])
  })
})
