<script setup lang="ts">
import { ref } from 'vue'
import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import { apiRequest } from '@/lib/http'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useUpdateStatus } from '@/views/dashboard/useUpdateStatus'
import type { components } from '@/types/generated'
import { t } from '@/i18n'

defineProps<{ dirty: boolean; disabled: boolean }>()
const emit = defineEmits<{ releases: [items: components['schemas']['UpdateRelease'][]] }>()
const { status, checking, error, check } = useUpdateStatus()
const loading = ref(false)
const listError = ref('')
const count = ref<number | null>(null)
async function refresh() {
  loading.value = true
  listError.value = ''
  try {
    const result = await apiRequest<components['schemas']['UpdateReleasesResponse']>('/api/update/releases', { timeoutMs: 25000 })
    emit('releases', result.releases)
    count.value = result.releases.length
  } catch (failure) { listError.value = getDisplayErrorMessage(failure) }
  finally { loading.value = false }
}
</script>

<template>
  <div class="update-status">
    <p>{{ t('config.update.trust') }}</p>
    <div class="update-status__actions">
      <AppButton :disabled="dirty || disabled || checking" :loading="loading" @click="refresh">{{ t('config.update.refresh') }}</AppButton>
      <AppButton :disabled="dirty || disabled || loading" :loading="checking" @click="check">{{ t('config.update.check') }}</AppButton>
      <span v-if="dirty" role="status">{{ t('config.update.saveFirst') }}</span>
      <span v-else-if="count !== null" role="status">{{ t('config.update.listLoaded', { count }) }}</span>
    </div>
    <AppAlert v-if="error || listError" tone="danger" :title="error || listError" />
    <p v-if="status" role="status">
      {{ t('config.update.current', { version: status.current_version }) }}
      <strong v-if="status.state === 'update_available'"> · {{ t('config.update.found', { version: status.available_version ?? '' }) }}</strong>
      <span v-else-if="status.state === 'up_to_date'"> · {{ t('config.update.upToDate') }}</span>
    </p>
    <template v-if="status?.routes?.length">
      <table class="update-routes">
        <caption>{{ t('config.update.routeHelp') }}</caption>
        <thead><tr><th scope="col">{{ t('config.update.route') }}</th><th scope="col">{{ t('config.update.latency') }}</th></tr></thead>
        <tbody><tr v-for="route in status.routes" :key="route.url">
          <td><span class="update-routes__url">{{ route.url }}</span><strong v-if="route.selected">{{ t('config.update.selected') }}</strong></td>
          <td>{{ route.available ? `${route.latency_ms} ms` : t('config.update.unavailable') }}</td>
        </tr></tbody>
      </table>
    </template>
    <p>{{ t('config.update.guidance') }}</p>
  </div>
</template>

<style scoped lang="scss">
.update-status { display: grid; gap: 14px; margin-bottom: 28px; color: var(--muted); font-size: 0.85rem; line-height: 1.6; }
.update-status__actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
.update-status strong { color: var(--text); font-weight: 600; }
.update-routes { width: 100%; border-collapse: collapse; text-align: left; }
.update-routes caption { text-align: left; margin-bottom: 8px; }
.update-routes th, .update-routes td { padding: 8px 0; border-bottom: 1px solid var(--border); vertical-align: top; }
.update-routes th { font-weight: 600; color: var(--text); }
.update-routes td:last-child { white-space: nowrap; padding-left: 16px; font-variant-numeric: tabular-nums; }
.update-routes__url { display: block; overflow-wrap: anywhere; }
</style>
