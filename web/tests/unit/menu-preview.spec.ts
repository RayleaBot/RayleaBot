import { describe, expect, it } from 'vitest'

import { buildPluginMenuGroups, buildRootMenuItems, isMenuPreviewPlugin } from '@/lib/menu-preview'
import type { PluginCommandSummary, PluginSummary } from '@/types/api'

function command(id: string): PluginCommandSummary {
  return { id, name: id, effective_names: [id], description: `${id} 说明`, usage: id, permission: 'everyone', trigger: { type: 'exact', names: [id] } }
}

function plugin(overrides: Partial<PluginSummary> = {}): PluginSummary {
  return { id: 'weather', name: 'Weather', role: 'community', state: 'running', commands: [command('weather')], command_groups: [], help: {}, ...overrides }
}

// The preview stands in for the menu the bot sends, so it follows the server's rules rather than its own.
describe('menu preview', () => {
  it('lists every enabled plugin with a readable manifest and something to show, running or not', () => {
    expect(isMenuPreviewPlugin(plugin({ state: 'failed' }))).toBe(true)
    expect(isMenuPreviewPlugin(plugin({ state: 'enabled' }))).toBe(true)
    expect(isMenuPreviewPlugin(plugin({ state: 'disabled' }))).toBe(false)
    expect(isMenuPreviewPlugin(plugin({ state: 'invalid' }))).toBe(false)
    expect(isMenuPreviewPlugin(plugin({ commands: [] }))).toBe(false)
    expect(isMenuPreviewPlugin(plugin({ commands: [], help: { summary: '天气菜单' } }))).toBe(true)
  })

  it('describes each plugin by its description, then its help summary, then the generic line', () => {
    expect(buildRootMenuItems([
      plugin({ description: '查询天气', help: { summary: '天气菜单' } }),
      plugin({ help: { summary: '天气菜单' } }),
      plugin(),
    ]).map(item => item.description)).toEqual(['查询天气', '天气菜单', '可用插件菜单'])
  })

  it('puts commands outside every group first under their own title', () => {
    const groups = buildPluginMenuGroups(
      plugin({ commands: [command('weather'), command('forecast')], command_groups: [{ id: 'query', title: '查询', commands: ['forecast'] }] }),
      { prefixes: ['/'] },
    )
    expect(groups.map(group => group.title)).toEqual(['命令', '查询'])
    expect(groups[0]!.items.map(item => item.name)).toEqual(['weather'])
  })
})
