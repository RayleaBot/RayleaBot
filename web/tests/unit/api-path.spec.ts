import { describe, expect, it } from 'vitest'
import ts from 'typescript'
import path from 'node:path'
import { apiPath } from '@/lib/api-path'

describe('contract-derived HTTP paths', () => {
  it('encodes each path parameter once and keeps query values out of the pathname', () => {
    const id = '插件 /?#%+&'
    const query = new URLSearchParams({ source_id: 'source /?#%+&' })
    const result = apiPath('/api/plugin-store/plugins/{plugin_id}', { plugin_id: id }, query)
    expect(result).toBe(`/api/plugin-store/plugins/${encodeURIComponent(id)}?${query}`)
    const url = new URL(result, 'https://management.test')
    expect(decodeURIComponent(url.pathname.split('/').at(-1)!)).toBe(id)
    expect(url.searchParams.get('source_id')).toBe('source /?#%+&')
    expect(url.hash).toBe('')
    expect(apiPath('/api/governance/blacklist/entries/{entry_type}/{target_id}', { entry_type: 'user', target_id: 'id/a' })).toBe('/api/governance/blacklist/entries/user/id%2Fa')
    expect(apiPath('/api/plugins/{plugin_id}', { plugin_id: 'a%2Fb' })).toBe('/api/plugins/a%252Fb')
  })
  it('rejects empty and standalone dot segments that browsers would normalize', () => {
    for (const id of ['', '.', '..']) {
      expect(() => apiPath('/api/plugins/{plugin_id}', { plugin_id: id })).toThrow(TypeError)
    }
  })
  it('accepts parameter and query routes while rejecting undeclared paths', () => {
    const config = ts.readConfigFile(path.resolve('tsconfig.app.json'), ts.sys.readFile)
    const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, process.cwd())
    const program = ts.createProgram([path.resolve('tests/types/api-path.ts')], parsed.options)
    const diagnostics = ts.getPreEmitDiagnostics(program)
    expect(diagnostics.map(item => ts.flattenDiagnosticMessageText(item.messageText, '\n'))).toEqual([])
  }, 30_000)
})
