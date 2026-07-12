<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { SyncOutlined } from '@ant-design/icons-vue'

import AppCard from '@/components/AppCard.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { UpdateStatusResponse } from '@/types/api'

const status = ref<UpdateStatusResponse | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const stateLabel = computed(() => {
  const state = status.value?.state
  return state ? t(`dashboard.update.states.${state}`) : t('dashboard.update.states.unknown')
})

const detail = computed(() => {
  const snapshot = status.value
  if (!snapshot) return t('dashboard.update.notChecked')
  if (snapshot.state === 'update_available' && snapshot.available_version) {
    return snapshot.automatic_install_supported
      ? t('dashboard.update.availableInLauncher', { version: snapshot.available_version })
      : t('dashboard.update.guidedAvailable', { version: snapshot.available_version })
  }
  if (snapshot.state === 'up_to_date') {
    return t('dashboard.update.upToDate', { version: snapshot.current_version })
  }
  if (snapshot.state === 'disabled') {
    return t('dashboard.update.trustBaselineRequired')
  }
  return t('dashboard.update.currentVersion', { version: snapshot.current_version })
})

async function fetchStatus() {
  error.value = null
  try {
    status.value = await apiRequest<UpdateStatusResponse>('/api/update/status')
  } catch (requestError) {
    error.value = getDisplayErrorMessage(requestError)
  }
}

async function check() {
  loading.value = true
  error.value = null
  try {
    status.value = await apiRequest<UpdateStatusResponse>('/api/update/check', { method: 'POST' })
  } catch (requestError) {
    const message = getDisplayErrorMessage(requestError)
    await fetchStatus()
    error.value = message
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void fetchStatus()
})
</script>

<template>
  <AppCard :title="t('dashboard.update.title')" borderless class="dashboard-update-card">
    <div class="dashboard-update-card__body" aria-live="polite">
      <div>
        <span class="dashboard-update-card__state">{{ stateLabel }}</span>
        <p>{{ detail }}</p>
      </div>
      <a-button :loading="loading" @click="check">
        <template #icon><SyncOutlined v-if="!loading" /></template>
        {{ t('dashboard.update.check') }}
      </a-button>
      <p v-if="error" class="dashboard-update-card__error" role="alert">{{ error }}</p>
    </div>
  </AppCard>
</template>

<style scoped lang="scss">
.dashboard-update-card {
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: none;
}

.dashboard-update-card__body {
  display: grid;
  gap: var(--space-md);
}

.dashboard-update-card__state {
  color: var(--text);
  font-weight: 700;
}

.dashboard-update-card__body p {
  margin: 4px 0 0;
  color: var(--muted);
  line-height: 1.5;
}

.dashboard-update-card__error {
  color: var(--text-danger) !important;
}
</style>
