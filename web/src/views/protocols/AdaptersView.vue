<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'

import { t } from '@/i18n'
import { useAdaptersStore } from '@/stores/adapters'
import type { AdapterDescriptor } from '@/types/api'

const router = useRouter()
const adaptersStore = useAdaptersStore()

onMounted(() => {
  void adaptersStore.refresh().catch(() => undefined)
})

const added = computed(() => adaptersStore.added)
const available = computed(() => adaptersStore.available)

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
  void router.push(`/protocols/${adapter.protocol}`)
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

      <a-empty v-if="!adaptersStore.loading && added.length === 0" :description="t('protocols.addedEmpty')" />

      <ul v-else class="adapters__list">
        <li v-for="adapter in added" :key="adapter.protocol" class="adapters__item">
          <button type="button" class="adapters__card" :data-testid="`adapter-${adapter.protocol}`" @click="openAdapter(adapter)">
            <span class="adapters__card-head">
              <span class="adapters__name">{{ adapter.display_name }}</span>
              <a-tag :color="statusTone(adapter)">{{ statusLabel(adapter) }}</a-tag>
            </span>
            <span class="adapters__summary">{{ adapter.summary }}</span>
            <span v-if="adapter.identity" class="adapters__identity">
              {{ t('protocols.adapterIdentity') }}：{{ adapter.identity.name || adapter.identity.id }}
            </span>
          </button>
        </li>
      </ul>
    </section>

    <section aria-labelledby="adapters-available-title" class="adapters__group">
      <h2 id="adapters-available-title">{{ t('protocols.availableTitle') }}</h2>
      <p class="adapters__hint">{{ t('protocols.availableHint') }}</p>

      <a-empty v-if="!adaptersStore.loading && available.length === 0" :description="t('protocols.availableEmpty')" />

      <ul v-else class="adapters__list">
        <li v-for="adapter in available" :key="adapter.protocol" class="adapters__item">
          <div class="adapters__card adapters__card--available">
            <span class="adapters__card-head">
              <span class="adapters__name">{{ adapter.display_name }}</span>
            </span>
            <span class="adapters__summary">{{ adapter.summary }}</span>
            <a-button type="primary" :data-testid="`adapter-add-${adapter.protocol}`" @click="openAdapter(adapter)">
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
  width: 100%;
  padding: 16px;
  text-align: left;
  color: inherit;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
  cursor: pointer;
  font: inherit;
  &:hover { border-color: var(--color-brand-foreground); }
  &:focus-visible { outline: 2px solid var(--color-focus); outline-offset: 2px; }
}
.adapters__card--available { cursor: default; justify-items: start; &:hover { border-color: var(--color-border); } }
.adapters__card-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.adapters__name { font-size: 15px; font-weight: 600; }
.adapters__summary { color: var(--color-text-muted); font-size: 13px; line-height: 1.6; }
.adapters__identity { color: var(--color-text-muted); font-size: 12px; }
</style>
