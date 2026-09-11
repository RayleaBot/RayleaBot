<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import { computed, ref } from 'vue'
import { CopyIcon } from '@lucide/vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import type { OneBotSettings, OneBotTransport } from '@/lib/adapters'
import { buildOneBot11ReverseWsUrl, buildOneBot11WebhookUrl } from '@/lib/protocols'

const settings = defineModel<OneBotSettings>({ required: true })
const props = defineProps<{ adapterId: string; errors: Record<string, string> }>()
const baseUrl = import.meta.env.VITE_WS_BASE_URL || window.location.origin
const transports: { key: OneBotTransport; label: string; hint: string; placeholder: string }[] = [
  { key: 'reverse_ws', label: t('protocols.transportFields.reverseWs.label'), hint: t('protocols.transportFields.reverseWs.hint'), placeholder: t('protocols.transportFields.reverseWs.placeholder') },
  { key: 'forward_ws', label: t('protocols.transportFields.forwardWs.label'), hint: t('protocols.transportFields.forwardWs.hint'), placeholder: 'ws://127.0.0.1:3001' },
  { key: 'http_api', label: 'HTTP API', hint: t('protocols.transportFields.httpApiHint'), placeholder: 'http://127.0.0.1:3000' },
  { key: 'webhook', label: 'Webhook', hint: t('protocols.transportFields.webhook.hint'), placeholder: t('protocols.transportFields.webhook.placeholder') },
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
  { value: 'reverse_ws', label: t('protocols.transportFields.modes.reverseWs') },
  { value: 'forward_ws', label: t('protocols.transportFields.modes.forwardWs') },
  { value: 'http', label: 'HTTP API + Webhook' },
  { value: 'custom', label: t('protocols.transportFields.modes.custom') },
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
    notifySuccess(t('protocols.transportFields.copied'))
  } catch { notifyError(t('protocols.transportFields.copyFailed')) }
}
</script>

<template>
  <AppField floating :label="t('protocols.transportFields.mode')" for="adapter-transport-mode" :error="errors.transports">
    <AppSelect id="adapter-transport-mode" :model-value="mode" :options="modeOptions" @update:model-value="selectMode" />
  </AppField>
  <div v-if="mode === 'custom'" class="transport-choices" role="group" :aria-label="t('protocols.transportFields.enabledGroup')">
    <AppCheckbox v-for="item in transports" :key="item.key" :model-value="settings[item.key].enabled" @update:model-value="(value: boolean) => enableTransport(item.key, value)">
      {{ item.label }}
    </AppCheckbox>
  </div>
  <section v-for="item in enabledTransports" :key="item.key" class="transport-fields" :aria-label="item.label">
    <h3 v-if="enabledTransports.length > 1 || !settings[item.key].enabled">{{ item.label }}{{ settings[item.key].enabled ? '' : t('protocols.transportFields.disabledSuffix') }}</h3>
    <p class="field-hint">{{ item.hint }}</p>
    <AppField floating
      :label="item.key === 'reverse_ws' ? t('protocols.reverseWsCallbackLabel') : item.key === 'webhook' ? t('protocols.transportFields.reportAddress') : t('protocols.transportFields.address', { label: item.label })"
      :for="`adapter-${item.key}-url`"
      :error="errors[`${item.key}.url`]"

    >
      <div class="address-field">
        <AppInput :id="`adapter-${item.key}-url`" :model-value="String(settings[item.key].url ?? '')" :placeholder="item.placeholder" @update:model-value="(value: string) => settings[item.key].url = value" />
        <AppButton v-if="item.key === 'reverse_ws' || item.key === 'webhook'" :aria-label="t('protocols.transportFields.copyAddress', { label: item.label })" @click="copyAddress(item.key)"><CopyIcon /></AppButton>
      </div>
    </AppField>
    <AppField floating :label="t('protocols.transportFields.accessToken')" :for="`adapter-${item.key}-token`">
      <AppInput type="password" :id="`adapter-${item.key}-token`" v-model="settings[item.key].access_token" autocomplete="new-password" :placeholder="t('protocols.transportFields.accessTokenPlaceholder')" />
      <p class="field-hint">{{ settings[item.key].access_token === '********' ? t('protocols.transportFields.accessTokenSaved') : t('protocols.transportFields.accessTokenHint') }}</p>
    </AppField>
    <details v-if="'access_token_query_compat' in settings[item.key]" class="transport-advanced">
      <summary>{{ t('protocols.transportFields.tokenCompat') }}</summary>
      <AppCheckbox :model-value="Boolean((settings[item.key] as OneBotSettings['reverse_ws']).access_token_query_compat)" @update:model-value="(value: boolean) => (settings[item.key] as OneBotSettings['reverse_ws']).access_token_query_compat = value">
        {{ t('protocols.transportFields.tokenQuery') }}
      </AppCheckbox>
      <p class="field-hint">{{ t('protocols.transportFields.tokenQueryHint') }}</p>
    </details>
  </section>
</template>

<style scoped lang="scss">
.field-hint { margin: 6px 0 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.transport-fields { margin: 0 0 24px; }
.transport-fields > .field-hint { margin: 0 0 16px; }
.transport-fields h3 { margin: 0 0 8px; font-size: 14px; }
.transport-choices { display: flex; flex-wrap: wrap; gap: 12px; margin: 0 0 20px; }
.address-field { display: flex; align-items: center; gap: 8px; }
.address-field .app-input-wrap { min-width: 0; }
.transport-advanced { color: var(--muted); font-size: 13px; }
.transport-advanced summary { cursor: pointer; width: fit-content; margin-bottom: 12px; }
</style>
