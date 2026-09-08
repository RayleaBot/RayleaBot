import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { getConfigSections } from '@/lib/config-form'
import { getConfigWorkbenchGroups, getWorkbenchSections, rebaseConfigDraft } from '@/views/system/config-workbench'
import { createConfigDocumentFixture } from './config-document.fixture'

describe('configuration workbench', () => {
  it('keeps displayed defaults and restart hints aligned with the formal configuration schema', () => {
    interface SchemaNode { properties?: Record<string, SchemaNode>; $ref?: string; default?: unknown; $defs?: Record<string, SchemaNode>; 'x-apply-policy'?: string }
    const schema: SchemaNode = JSON.parse(readFileSync(resolve(process.cwd(), '../contracts/config.user.schema.json'), 'utf8'))
    for (const field of getConfigSections().flatMap(section => section.fields)) {
      let node = schema
      for (const part of field.path.split('.')) {
        if (node.$ref) node = { ...schema.$defs![node.$ref.split('/').at(-1)!], ...node }
        node = node.properties![part]!
      }
      expect(field.defaultValue, field.path).toEqual(node.default)
      expect(Boolean(field.restartRequired), field.path).toBe(node['x-apply-policy'] === 'restart_required')
    }
  })

  it('keeps every existing field in exactly one common or advanced category', () => {
    const expected = getConfigSections().flatMap(section => section.fields.map(field => field.path)).sort()
    const actual = getConfigWorkbenchGroups().flatMap(group => [false, true].flatMap(advanced => getWorkbenchSections(group, advanced).flatMap(section => section.fields.map(field => field.path)))).sort()
    expect(actual).toEqual(expected)
    expect(new Set(actual).size).toBe(actual.length)
  })

  it('searches advanced parameters by path without needing their disclosure open', () => {
    const group = getConfigWorkbenchGroups().find(group => group.key === 'render')!
    expect(getWorkbenchSections(group, true, 'browser_args').flatMap(section => section.fields.map(field => field.path))).toEqual(['render.browser_args'])
    expect(getWorkbenchSections(group, false, 'browser_args')).toEqual([])
  })

  it('merges unrelated incoming changes while preserving local edits', () => {
    const base = createConfigDocumentFixture()
    const draft = createConfigDocumentFixture()
    const incoming = createConfigDocumentFixture()
    draft.server.port = 22334
    incoming.log.level = 'debug'
    const fields = getConfigSections().flatMap(section => section.fields)
    const result = rebaseConfigDraft(base, draft, incoming, fields)
    expect(result.draft.server.port).toBe(22334)
    expect(result.draft.log.level).toBe('debug')
    expect(result.conflicts).toEqual([])
    expect(incoming.server.port).toBe(base.server.port)
  })
})
