<script setup lang="ts">
import { computed, ref } from 'vue'
import { CopyOutlined } from '@ant-design/icons-vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import type { OneBotSettings, OneBotTransport } from '@/lib/adapters'
import { buildOneBot11ReverseWsUrl, buildOneBot11WebhookUrl } from '@/lib/protocols'

const settings = defineModel<OneBotSettings>({ required: true })
const props = defineProps<{ adapterId: string; errors: Record<string, string> }>()
const baseUrl = import.meta.env.VITE_WS_BASE_URL || window.location.origin
const transports: { key: OneBotTransport; label: string; hint: string; placeholder: string }[] = [
  { key: 'reverse_ws', label: '反向 WebSocket', hint: '将回连地址填入 OneBot 客户端，由客户端连接 RayleaBot。', placeholder: 'ws://或 wss:// 回连地址' },
  { key: 'forward_ws', label: '正向 WebSocket', hint: '由 RayleaBot 主动连接 OneBot 客户端提供的 WebSocket 服务。', placeholder: 'ws://127.0.0.1:3001' },
  { key: 'http_api', label: 'HTTP API', hint: '填写 OneBot 客户端的 API 地址，用于发送消息和调用接口。', placeholder: 'http://127.0.0.1:3000' },
  { key: 'webhook', label: 'Webhook', hint: '将上报地址填入 OneBot 客户端，用于接收事件；发送消息需同时配置 HTTP API。', placeholder: 'http://或 https:// 上报地址' },
]
const enabledTransports = computed(() => transports.filter(({ key }) => settings.value[key].enabled || props.errors[`${key}.url`]))
function detectMode() {
  const keys = transports.filter(({ key }) => settings.value[key].enabled).map(({ key }) => key).join(',')
  if (keys === 'reverse_ws' || keys === 'forward_ws') return keys
  if (keys === 'http_api,webhook') return 'http'
  return 'custom'
}
const mode = ref(detectMode())
const modeOptions = [
  { value: 'reverse_ws', label: '反向 WebSocket · 常用' },
  { value: 'forward_ws', label: '正向 WebSocket' },
  { value: 'http', label: 'HTTP API + Webhook' },
  { value: 'custom', label: '自定义组合' },
]
function enableTransport(key: OneBotTransport, enabled: boolean) {
  const entry = settings.value[key]
  entry.enabled = enabled
  if (enabled && !entry.url) {
    if (key === 'reverse_ws') entry.url = buildOneBot11ReverseWsUrl(baseUrl, props.adapterId)
    if (key === 'webhook') entry.url = buildOneBot11WebhookUrl(baseUrl, props.adapterId)
  }
}
function selectMode(value: string) {
  mode.value = value
  if (value === 'custom') return
  for (const { key } of transports) {
    enableTransport(key, value === 'http' ? key === 'http_api' || key === 'webhook' : key === value)
  }
}
async function copyAddress(key: OneBotTransport) {
  try {
    await navigator.clipboard.writeText(String(settings.value[key].url))
    notifySuccess('地址已复制')
  } catch { notifyError('无法复制，请手动选择地址复制。') }
}
</script>

<template>
  <a-form-item label="连接方式" html-for="adapter-transport-mode" :help="errors.transports" :validate-status="errors.transports ? 'error' : undefined">
    <a-select id="adapter-transport-mode" :value="mode" :options="modeOptions" @update:value="selectMode" />
  </a-form-item>
  <div v-if="mode === 'custom'" class="transport-choices" role="group" aria-label="启用的传输方式">
    <a-checkbox v-for="item in transports" :key="item.key" :checked="settings[item.key].enabled" @update:checked="(value: boolean) => enableTransport(item.key, value)">
      {{ item.label }}
    </a-checkbox>
  </div>
  <section v-for="item in enabledTransports" :key="item.key" class="transport-fields" :aria-label="item.label">
    <h3 v-if="enabledTransports.length > 1 || !settings[item.key].enabled">{{ item.label }}{{ settings[item.key].enabled ? '' : '（未启用）' }}</h3>
    <p class="field-hint">{{ item.hint }}</p>
    <a-form-item
      :label="item.key === 'reverse_ws' ? '协议端回连地址' : item.key === 'webhook' ? '事件上报地址' : `${item.label} 地址`"
      :html-for="`adapter-${item.key}-url`"
      :help="errors[`${item.key}.url`]"
      :validate-status="errors[`${item.key}.url`] ? 'error' : undefined"
    >
      <div class="address-field">
        <a-input :id="`adapter-${item.key}-url`" :value="String(settings[item.key].url ?? '')" :placeholder="item.placeholder" @update:value="(value: string) => settings[item.key].url = value" />
        <a-button v-if="item.key === 'reverse_ws' || item.key === 'webhook'" :aria-label="`复制${item.label}地址`" @click="copyAddress(item.key)"><CopyOutlined /></a-button>
      </div>
    </a-form-item>
    <a-form-item label="访问令牌" :html-for="`adapter-${item.key}-token`">
      <a-input-password :id="`adapter-${item.key}-token`" v-model:value="settings[item.key].access_token" autocomplete="new-password" placeholder="与 OneBot 客户端保持一致，可留空" />
      <p class="field-hint">{{ settings[item.key].access_token === '********' ? '已保存令牌。保持现值可沿用，清空后保存将移除。' : '仅用于此传输方式，请与客户端设置一致。' }}</p>
    </a-form-item>
    <details v-if="'access_token_query_compat' in settings[item.key]" class="transport-advanced">
      <summary>令牌兼容选项</summary>
      <a-checkbox :checked="Boolean((settings[item.key] as OneBotSettings['reverse_ws']).access_token_query_compat)" @update:checked="(value: boolean) => (settings[item.key] as OneBotSettings['reverse_ws']).access_token_query_compat = value">
        允许通过 URL 查询参数传递令牌
      </a-checkbox>
      <p class="field-hint">默认使用 Authorization 请求头。仅在客户端要求时开启。</p>
    </details>
  </section>
</template>

<style scoped lang="scss">
.field-hint { margin: 6px 0 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.transport-fields { margin: 0 0 24px; }
.transport-fields > .field-hint { margin: 0 0 16px; }
.transport-fields h3 { margin: 0 0 8px; font-size: 14px; }
.transport-choices { display: flex; flex-wrap: wrap; gap: 12px; margin: 0 0 20px; }
.address-field { display: flex; gap: 8px; }
.address-field .ant-input { min-width: 0; }
.transport-advanced { color: var(--muted); font-size: 13px; }
.transport-advanced summary { cursor: pointer; width: fit-content; margin-bottom: 12px; }
</style>
