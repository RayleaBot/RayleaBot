import { describe, expect, it } from 'vitest'

import { PluginUIBridgeClient } from '../src/client'

describe('PluginUIBridgeClient', () => {
  it('uses the contracted config field for settings updates', async () => {
    const client = new PluginUIBridgeClient()
    const sent: Array<Record<string, unknown>> = []
    Object.defineProperty(client, 'port', {
      value: {
        postMessage: (message: Record<string, unknown>) => sent.push(message),
        close: () => undefined,
      },
      configurable: true,
    })

    const request = client.saveSettings({ default_city: '广州' })
    expect(sent).toHaveLength(1)
    expect(sent[0]).toMatchObject({
      type: 'settings.save',
      payload: { config: { default_city: '广州' } },
    })

    client.close()
    await expect(request).rejects.toMatchObject({ code: 'plugin.bridge_closed' })
  })

  it('clamps reported height to the bridge contract', () => {
    const client = new PluginUIBridgeClient()
    const sent: unknown[] = []
    Object.defineProperty(client, 'port', {
      value: { postMessage: (message: unknown) => sent.push(message) },
      configurable: true,
    })
    client.reportHeight(12)
    client.reportHeight(9000)
    expect(sent).toMatchObject([
      { type: 'ui.resize', payload: { height: 320 } },
      { type: 'ui.resize', payload: { height: 1600 } },
    ])
  })
})
