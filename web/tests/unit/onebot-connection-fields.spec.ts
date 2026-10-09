import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AppCheckbox from '@/components/AppCheckbox.vue'
import OneBotConnectionFields from '@/views/protocols/OneBotConnectionFields.vue'
import type { OneBotSettings } from '@/lib/adapters'

function settings(): OneBotSettings {
  const transport = () => ({ enabled: false, url: '', access_token: '', access_token_query_compat: false })
  return { reverse_ws: transport(), forward_ws: transport(), http_api: { enabled: false, url: '', access_token: '' }, webhook: transport() }
}

async function enable(value: OneBotSettings, label: string) {
  const wrapper = mount(OneBotConnectionFields, { props: { modelValue: value, adapterId: 'onebot11', errors: {} } })
  await wrapper.findAllComponents(AppCheckbox).find((box) => box.text() === label)!.vm.$emit('update:modelValue', true)
  return value
}

describe('OneBot connection fields', () => {
  it('gives newly enabled inbound transports a random token and keeps existing ones', async () => {
    const fresh = await enable(settings(), '反向 WebSocket')
    expect(fresh.reverse_ws.access_token).toMatch(/^[0-9a-f]{64}$/)

    const saved = settings()
    saved.webhook.access_token = '********'
    expect((await enable(saved, 'Webhook')).webhook.access_token).toBe('********')

    expect((await enable(settings(), 'HTTP API')).http_api.access_token).toBe('')
  })
})
