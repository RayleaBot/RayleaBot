import { describe, expect, it } from 'vitest'

import { mergeCommandCenterRows } from '@/lib/plugin-commands'
import type { GovernanceCommandPolicyEntry, PluginSummary } from '@/types/api'

function createPlugin(overrides: Partial<PluginSummary> = {}): PluginSummary {
  return {
    id: 'raylea.fortune',
    name: '运势',
    role: 'builtin',
    registration_state: 'installed',
    desired_state: 'enabled',
    runtime_state: 'running',
    display_state: 'running',
    commands: [],
    command_conflicts: [],
    ...overrides,
  }
}

function createPolicy(overrides: Partial<GovernanceCommandPolicyEntry> = {}): GovernanceCommandPolicyEntry {
  return {
    plugin_id: 'raylea.fortune',
    plugin_name: '运势',
    command: '我的运势',
    aliases: [],
    command_source: 'dynamic',
    declaration_id: 'fortune',
    declared_permission: 'everyone',
    effective_permission: 'group_admin',
    permission_source: 'declared',
    ...overrides,
  }
}

describe('plugin command merging', () => {
  it('matches policies by declaration id before command name', () => {
    const rows = mergeCommandCenterRows([
      createPlugin({
        commands: [
          {
            name: '今日运势',
            command_source: 'dynamic',
            declaration_id: 'fortune',
          },
        ],
      }),
    ], [
      createPolicy({
        command: '我的运势',
        declaration_id: 'fortune',
      }),
    ])

    expect(rows).toHaveLength(1)
    expect(rows[0].command.name).toBe('今日运势')
    expect(rows[0].policy?.effective_permission).toBe('group_admin')
  })

  it('falls back to plugin id and command name when declaration id is absent', () => {
    const rows = mergeCommandCenterRows([
      createPlugin({
        commands: [
          {
            name: 'help',
            command_source: 'manifest',
          },
        ],
      }),
    ], [
      createPolicy({
        command: 'help',
        declaration_id: undefined,
        command_source: 'manifest',
      }),
    ])

    expect(rows).toHaveLength(1)
    expect(rows[0].policy?.command).toBe('help')
  })

  it('keeps declaration id mismatches as separate rows', () => {
    const rows = mergeCommandCenterRows([
      createPlugin({
        commands: [
          {
            name: '我的运势',
            command_source: 'dynamic',
            declaration_id: 'fortune-current',
          },
        ],
      }),
    ], [
      createPolicy({
        command: '我的运势',
        declaration_id: 'fortune-policy',
      }),
    ])

    expect(rows).toHaveLength(2)
    expect(rows[0].policy).toBeNull()
    expect(rows[1].policy?.declaration_id).toBe('fortune-policy')
  })

  it('keeps policy-only and plugin-only rows visible', () => {
    const rows = mergeCommandCenterRows([
      createPlugin({
        commands: [
          {
            name: 'echo',
            description: '复读收到的内容',
            command_source: 'manifest',
          },
        ],
      }),
    ], [
      createPolicy({
        plugin_id: 'ops.tools',
        plugin_name: 'Ops Tools',
        command: 'ops',
        declaration_id: undefined,
        command_source: 'manifest',
      }),
    ])

    expect(rows.map((row) => row.command.name)).toEqual(['echo', 'ops'])
    expect(rows[0].policy).toBeNull()
    expect(rows[1].pluginId).toBe('ops.tools')
    expect(rows[1].availability).toBe('not_ready')
  })
})
