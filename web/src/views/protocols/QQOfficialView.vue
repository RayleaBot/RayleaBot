<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import { cloneConfig, getValueByPath, setValueByPath } from '@/lib/config-form'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const adaptersStore = useAdaptersStore()
const { document: configDocument, saving } = storeToRefs(configStore)
const draft = ref<ConfigDocument | null>(null)

// The gateway subscribes event families by bitmask; the contract names them so
// an operator never writes a raw number.
const intentOptions = [
  { value: 'group_and_c2c', label: t('protocols.qqIntents.groupAndC2c') },
  { value: 'public_guild_messages', label: t('protocols.qqIntents.publicGuildMessages') },
  { value: 'guilds', label: t('protocols.qqIntents.guilds') },
  { value: 'guild_members', label: t('protocols.qqIntents.guildMembers') },
  { value: 'guild_messages', label: t('protocols.qqIntents.guildMessages') },
  { value: 'direct_message', label: t('protocols.qqIntents.directMessage') },
]

watch(configDocument, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

onMounted(() => {
  void configStore.fetchConfig().catch(() => undefined)
  void adaptersStore.refresh().catch(() => undefined)
})

const descriptor = computed(() => adaptersStore.adapters.find((item) => item.protocol === 'qqofficial') ?? null)

function read<T>(path: string, fallback: T): T {
  if (!draft.value) {
    return fallback
  }
  const value = getValueByPath(draft.value as unknown as Record<string, unknown>, `qq_official.${path}`)
  return (value ?? fallback) as T
}

function write(path: string, value: unknown) {
  if (!draft.value) {
    return
  }
  setValueByPath(draft.value as unknown as Record<string, unknown>, `qq_official.${path}`, value)
}

const enabled = computed({ get: () => read('enabled', false), set: (value) => write('enabled', value) })
const appId = computed({ get: () => read('app_id', ''), set: (value) => write('app_id', value) })
const appSecret = computed({ get: () => read('app_secret', ''), set: (value) => write('app_secret', value) })
const sandbox = computed({ get: () => read('sandbox', false), set: (value) => write('sandbox', value) })
const intents = computed<string[]>({
  get: () => read<string[]>('intents', []),
  set: (value) => write('intents', value),
})

const isDirty = computed(() => Boolean(draft.value && configDocument.value)
  && JSON.stringify(draft.value) !== JSON.stringify(configDocument.value))
const canSave = computed(() => isDirty.value && !saving.value)

async function save() {
  if (!draft.value) {
    return
  }
  try {
    const response = await configStore.saveConfig(draft.value)
    notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
    await adaptersStore.refresh().catch(() => undefined)
  } catch (err) {
    notifyError(getDisplayErrorMessage(err, 'errors.common.saveFailed'))
  }
}
</script>

<template>
  <section class="qq-adapter">
    <header class="qq-adapter__header">
      <h1>{{ t('protocols.qqTitle') }}</h1>
      <p>{{ t('protocols.qqSubtitle') }}</p>
    </header>

    <a-alert
      v-if="descriptor"
      :type="descriptor.state === 'auth_failed' ? 'error' : 'info'"
      show-icon
      :message="descriptor.summary"
      data-testid="qq-adapter-status"
    />

    <a-form layout="vertical" class="qq-adapter__form" @submit.prevent="save">
      <a-form-item :label="t('protocols.qqEnabled')">
        <a-switch v-model:checked="enabled" data-testid="qq-enabled" />
      </a-form-item>

      <a-form-item :label="t('protocols.qqAppId')" :help="t('protocols.qqAppIdHint')">
        <a-input v-model:value="appId" data-testid="qq-app-id" autocomplete="off" />
      </a-form-item>

      <a-form-item :label="t('protocols.qqAppSecret')" :help="t('protocols.qqAppSecretHint')">
        <a-input-password v-model:value="appSecret" data-testid="qq-app-secret" autocomplete="new-password" />
      </a-form-item>

      <a-form-item :label="t('protocols.qqIntentsLabel')" :help="t('protocols.qqIntentsHint')">
        <a-select
          v-model:value="intents"
          mode="multiple"
          :options="intentOptions"
          data-testid="qq-intents"
        />
      </a-form-item>

      <a-form-item :label="t('protocols.qqSandbox')" :help="t('protocols.qqSandboxHint')">
        <a-switch v-model:checked="sandbox" data-testid="qq-sandbox" />
      </a-form-item>

      <a-button type="primary" :disabled="!canSave" :loading="saving" data-testid="qq-save" @click="save">
        {{ t('config.save') }}
      </a-button>
    </a-form>
  </section>
</template>

<style scoped lang="scss">
.qq-adapter { display: grid; gap: 16px; max-width: 640px; }
.qq-adapter__header h1 { margin: 0; font-size: 20px; font-weight: 600; }
.qq-adapter__header p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 13px; }
.qq-adapter__form { margin-top: 8px; }
</style>
