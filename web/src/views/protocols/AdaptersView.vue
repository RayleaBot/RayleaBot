<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import {
  adapterEditorRoute,
  buildAdapterInstance,
  nextAdapterInstanceId,
  readAdapterInstances,
} from '@/lib/adapters'
import { cloneConfig } from '@/lib/config-form'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import type { AdapterDescriptor, AdapterProtocol, AdapterProtocolDescriptor } from '@/types/api'

const router = useRouter()
const adaptersStore = useAdaptersStore()
const configStore = useConfigStore()
const { document: configDocument, saving } = storeToRefs(configStore)
const pendingProtocol = ref<AdapterProtocol | null>(null)
const removingId = ref<string | null>(null)

onMounted(() => {
  void adaptersStore.refresh().catch(() => undefined)
  void configStore.fetchConfig().catch(() => undefined)
})

const adapters = computed(() => adaptersStore.adapters)
const availableProtocols = computed(() => adaptersStore.availableProtocols)

// A configured adapter that is switched off is not a failure, so it reads as
// its own state rather than borrowing the connection vocabulary.
function statusTone(adapter: AdapterDescriptor) {
  if (!adapter.enabled) {
    return 'default'
  }
  switch (adapter.state) {
    case 'connected':
    case 'listening':
      return 'success'
    case 'connecting':
    case 'reconnecting':
      return 'processing'
    case 'auth_failed':
      return 'error'
    default:
      return 'warning'
  }
}

function statusLabel(adapter: AdapterDescriptor) {
  if (!adapter.enabled) {
    return t('protocols.adapterDisabled')
  }
  return t(`protocols.adapterState.${adapter.state}`)
}

function openAdapter(adapter: AdapterDescriptor) {
  void router.push(adapterEditorRoute(adapter.protocol, adapter.id))
}

// Adding an instance writes it to the config and opens its settings: the
// instance exists from that moment, which is what gives it an identifier for
// its ingress URL and its secrets.
async function addAdapter(protocol: AdapterProtocolDescriptor) {
  if (!configDocument.value || pendingProtocol.value) {
    return
  }
  pendingProtocol.value = protocol.protocol
  try {
    const draft = cloneConfig(configDocument.value)
    const id = nextAdapterInstanceId(draft, protocol.protocol)
    const instances = readAdapterInstances(draft)
    ;(draft as Record<string, unknown>).adapters = [...instances, buildAdapterInstance(id, protocol.protocol)]

    const response = await configStore.saveConfig(draft)
    await adaptersStore.refresh().catch(() => undefined)
    notifySuccess(response.restart_required ? t('config.saveRestart') : t('protocols.addAdapterSuccess'))
    void router.push(adapterEditorRoute(protocol.protocol, id))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err, 'errors.common.saveFailed'))
  } finally {
    pendingProtocol.value = null
  }
}

async function removeAdapter(adapter: AdapterDescriptor) {
  if (!configDocument.value || removingId.value) {
    return
  }
  removingId.value = adapter.id
  try {
    const draft = cloneConfig(configDocument.value)
    ;(draft as Record<string, unknown>).adapters = readAdapterInstances(draft)
      .filter((instance) => instance.id !== adapter.id)

    const response = await configStore.saveConfig(draft)
    await adaptersStore.refresh().catch(() => undefined)
    notifySuccess(response.restart_required ? t('config.saveRestart') : t('protocols.removeAdapterSuccess'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err, 'errors.common.saveFailed'))
  } finally {
    removingId.value = null
  }
}
</script>

<template>
  <section class="adapters">
    <header class="adapters__header">
      <h1>{{ t('protocols.title') }}</h1>
      <p>{{ t('protocols.adaptersSubtitle') }}</p>
    </header>

    <a-alert
      v-if="adaptersStore.error"
      type="error"
      show-icon
      :message="adaptersStore.error"
      class="adapters__error"
    />

    <section aria-labelledby="adapters-added-title" class="adapters__group">
      <h2 id="adapters-added-title">{{ t('protocols.addedTitle') }}</h2>
      <p class="adapters__hint">{{ t('protocols.addedHint') }}</p>

      <a-empty v-if="!adaptersStore.loading && adapters.length === 0" :description="t('protocols.addedEmpty')" />

      <ul v-else class="adapters__list">
        <li v-for="adapter in adapters" :key="adapter.id" class="adapters__item">
          <div class="adapters__card">
            <button
              type="button"
              class="adapters__open"
              :data-testid="`adapter-${adapter.id}`"
              @click="openAdapter(adapter)"
            >
              <span class="adapters__card-head">
                <span class="adapters__name">{{ adapter.display_name }}</span>
                <a-tag :color="statusTone(adapter)">{{ statusLabel(adapter) }}</a-tag>
              </span>
              <span class="adapters__summary">{{ adapter.summary }}</span>
              <span v-if="adapter.identity" class="adapters__identity">
                {{ t('protocols.adapterIdentity') }}：{{ adapter.identity.name || adapter.identity.id }}
              </span>
            </button>
            <a-popconfirm
              :title="t('protocols.removeAdapterConfirm', { name: adapter.display_name })"
              :ok-text="t('protocols.removeAdapterAction')"
              :cancel-text="t('protocols.removeAdapterCancel')"
              @confirm="removeAdapter(adapter)"
            >
              <a-button
                danger
                type="text"
                size="small"
                :loading="removingId === adapter.id"
                :disabled="saving"
                :data-testid="`adapter-remove-${adapter.id}`"
              >
                {{ t('protocols.removeAdapter') }}
              </a-button>
            </a-popconfirm>
          </div>
        </li>
      </ul>
    </section>

    <section aria-labelledby="adapters-available-title" class="adapters__group">
      <h2 id="adapters-available-title">{{ t('protocols.availableTitle') }}</h2>
      <p class="adapters__hint">{{ t('protocols.availableHint') }}</p>

      <ul class="adapters__list">
        <li v-for="protocol in availableProtocols" :key="protocol.protocol" class="adapters__item">
          <div class="adapters__card adapters__card--available">
            <span class="adapters__card-head">
              <span class="adapters__name">{{ protocol.display_name }}</span>
            </span>
            <span class="adapters__summary">{{ protocol.description }}</span>
            <a-button
              type="primary"
              :loading="pendingProtocol === protocol.protocol"
              :disabled="saving || !configDocument"
              :data-testid="`adapter-add-${protocol.protocol}`"
              @click="addAdapter(protocol)"
            >
              {{ t('protocols.addAdapter') }}
            </a-button>
          </div>
        </li>
      </ul>
    </section>
  </section>
</template>

<style scoped lang="scss">
.adapters { display: grid; gap: 24px; }
.adapters__header h1 { margin: 0; font-size: 20px; font-weight: 600; }
.adapters__header p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 13px; }
.adapters__group { display: grid; gap: 8px; }
.adapters__group h2 { margin: 0; font-size: 16px; font-weight: 600; }
.adapters__hint { margin: 0 0 8px; color: var(--color-text-muted); font-size: 13px; }
.adapters__list { display: grid; gap: 12px; margin: 0; padding: 0; list-style: none; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); }
.adapters__card {
  display: grid;
  gap: 8px;
  justify-items: start;
  width: 100%;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
}
.adapters__open {
  display: grid;
  gap: 8px;
  width: 100%;
  padding: 0;
  text-align: left;
  color: inherit;
  border: 0;
  background: none;
  cursor: pointer;
  font: inherit;
  &:hover .adapters__name { color: var(--color-brand-foreground); }
  &:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
}
.adapters__card--available { cursor: default; }
.adapters__card-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.adapters__name { font-size: 15px; font-weight: 600; }
.adapters__summary { color: var(--color-text-muted); font-size: 13px; line-height: 1.6; }
.adapters__identity { color: var(--color-text-muted); font-size: 12px; }
</style>
