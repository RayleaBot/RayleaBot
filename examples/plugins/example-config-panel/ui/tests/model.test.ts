import { describe, expect, it } from 'vitest'
import { accountSummary, normalizeNotes, normalizeSettings } from '../src/model'

describe('config panel model', () => {
  it('normalizes unsupported units', () => {
    expect(normalizeSettings({ default_city: '上海', unit: 'kelvin' })).toEqual({ default_city: '上海', unit: 'celsius' })
  })

  it('keeps only text notes and describes the account by its connection state', () => {
    expect(normalizeNotes(['a', 1, null, 'b'])).toEqual(['a', 'b'])
    expect(accountSummary({ source_protocol: 'onebot11', bot_id: '10001', nickname: '小雷', state: 'online', adapter_id: 'napcat-home', adapter_display_name: 'OneBot11（napcat-home）', last_seen_at: '2026-10-12T08:00:00Z' })).toBe('当前账号：小雷（10001），经 OneBot11（napcat-home）')
    expect(accountSummary({ source_protocol: 'onebot11', bot_id: '10003', state: 'no_connection', last_adapter_id: 'napcat-old', last_adapter_display_name: 'OneBot11（napcat-old）', last_seen_at: '2026-10-10T08:00:00Z' })).toBe('当前账号：10003，没有连接')
  })
})
