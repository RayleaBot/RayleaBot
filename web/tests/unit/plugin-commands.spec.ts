import { describe, expect, it } from 'vitest'

import { mergeCommandCenterRows } from '@/lib/plugin-commands'
import type { GovernanceCommandPolicyEntry, PluginCommandSummary, PluginSummary } from '@/types/api'

function command(id: string, name = id): PluginCommandSummary {
  return {
    id,
    name,
    effective_names: [name],
    description: `${name} description`,
    usage: `/${name}`,
    permission: 'everyone',
    trigger: { type: 'exact', names: [name] },
  }
}

function plugin(commands: PluginCommandSummary[]): PluginSummary {
  return {
    id: 'raylea.fortune', name: '运势', role: 'official', state: 'running',
    commands, command_groups: [], help: {}, command_conflicts: [],
  }
}

function policy(commandID: string, name = commandID): GovernanceCommandPolicyEntry {
  return {
    plugin_id: 'raylea.fortune', plugin_name: '运势', command_id: commandID, command: name,
    aliases: [], trigger: { type: 'exact', names: [name] },
    declared_permission: 'everyone', effective_permission: 'group_admin', permission_source: 'declared',
  }
}

describe('plugin command merging', () => {
  it('matches policy rows by stable command id', () => {
    const rows = mergeCommandCenterRows([plugin([command('fortune', '今日运势')])], [policy('fortune', '我的运势')])
    expect(rows).toHaveLength(1)
    expect(rows[0].command.name).toBe('今日运势')
    expect(rows[0].policy?.effective_permission).toBe('group_admin')
  })

  it('keeps different stable ids as separate rows even when names match', () => {
    const rows = mergeCommandCenterRows([plugin([command('fortune-current', '我的运势')])], [policy('fortune-policy', '我的运势')])
    expect(rows).toHaveLength(2)
    expect(rows[0].policy).toBeNull()
    expect(rows[1].policy?.command_id).toBe('fortune-policy')
  })

  it('keeps policy-only and plugin-only rows visible', () => {
    const rows = mergeCommandCenterRows([plugin([command('echo')])], [
      { ...policy('ops'), plugin_id: 'ops.tools', plugin_name: 'Ops Tools' },
    ])
    expect(rows.map((row) => row.command.name)).toEqual(['echo', 'ops'])
    expect(rows[1].availability).toBe('not_ready')
  })
})
