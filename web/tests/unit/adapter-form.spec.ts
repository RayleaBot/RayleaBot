import { describe, expect, it } from 'vitest'
import { buildAdapterInstance } from '@/lib/adapters'
import { mergeAdapterDraft, validateAdapterDraft } from '@/lib/adapter-form'
import { buildOneBot11ReverseWsUrl, buildOneBot11WebhookUrl } from '@/lib/protocols'
import { createConfigDocumentFixture } from './config-document.fixture'

describe('adapter form boundaries', () => {
  it.each(['https://bot.example', 'wss://bot.example'])('keeps secure callback schemes for %s', (origin) => {
    expect(buildOneBot11ReverseWsUrl(origin, 'second')).toBe('wss://bot.example/api/adapters/second/reverse-ws')
    expect(buildOneBot11WebhookUrl(origin, 'second')).toBe('https://bot.example/api/adapters/second/webhook')
  })
  it('requires valid enabled transport URLs and permits an intentionally disabled draft', () => {
    const instance = buildAdapterInstance('bot', 'onebot11')
    expect(validateAdapterDraft(instance)).toEqual({})
    instance.enabled = true
    expect(validateAdapterDraft(instance)).toHaveProperty('transports')
    instance.onebot11!.forward_ws.enabled = true
    instance.onebot11!.forward_ws.url = 'https://client.example'
    expect(validateAdapterDraft(instance)).toHaveProperty('forward_ws.url')
    instance.onebot11!.forward_ws.url = 'wss://client.example'
    expect(validateAdapterDraft(instance)).toEqual({})
  })
  it('rejects duplicate identifiers discovered just before creation', () => {
    const doc = createConfigDocumentFixture()
    expect(() => mergeAdapterDraft(doc, null, buildAdapterInstance('onebot11', 'qqofficial'))).toThrow('已被使用')
    expect(doc.adapters).toHaveLength(1)
  })
  it('does not resurrect an instance deleted while editing', () => {
    const doc = createConfigDocumentFixture()
    const baseline = doc.adapters[0]
    doc.adapters = []
    expect(() => mergeAdapterDraft(doc, baseline, baseline)).toThrow('更改或删除')
  })
})
