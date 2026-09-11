<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Settings2Icon, Trash2Icon, UserRoundIcon } from '@lucide/vue'
import AppAvatar from '@/components/AppAvatar.vue'
import AppBadge from '@/components/AppBadge.vue'
import AppButton from '@/components/AppButton.vue'
import { t } from '@/i18n'
import type { AdapterInstanceDocument } from '@/lib/adapters'
import type { StatusTone } from '@/lib/status-tone'
import type { AdapterDescriptor } from '@/types/api'

const props = defineProps<{
  config: AdapterInstanceDocument
  runtime?: AdapterDescriptor
  statusLabel: string
  statusTone: StatusTone
  busy: boolean
  removing: boolean
}>()
defineEmits<{ configure: []; remove: [] }>()

const identity = computed(() => props.runtime?.identity)
const protocolName = computed(() => props.config.type === 'qqofficial' ? t('protocols.qqTitle') : 'OneBot11')
const accountLabel = computed(() => props.config.type === 'qqofficial' ? t('protocols.connectionCard.botId') : 'QQ')
const name = computed(() => identity.value?.name || (identity.value?.id ? t('protocols.connectionCard.nameUnavailable') : t('protocols.connectionCard.accountPending')))
const avatarFailed = ref(false)
watch(() => [identity.value?.id, identity.value?.avatar_url], () => { avatarFailed.value = false })
const showSummary = computed(() => !identity.value?.id || props.runtime?.state !== 'connected')
</script>

<template>
  <li class="connection-card">
    <header class="connection-heading">
      <span class="connection-protocol">{{ protocolName }}</span>
      <AppBadge :tone="statusTone">{{ statusLabel }}</AppBadge>
    </header>
    <div class="connection-account">
      <AppAvatar :size="56" class="connection-avatar">
        <img v-if="identity?.avatar_url && !avatarFailed" :key="identity.avatar_url" :src="identity.avatar_url" alt="" width="56" height="56" loading="lazy" referrerpolicy="no-referrer" @error="avatarFailed = true" />
        <UserRoundIcon v-else aria-hidden="true" />
      </AppAvatar>
      <div class="connection-identity">
        <h3 :class="{ 'connection-name-pending': !identity?.name }">{{ name }}</h3>
        <p v-if="identity?.id" class="connection-number"><span>{{ accountLabel }}</span><span>{{ identity.id }}</span></p>
        <p v-else class="connection-pending">{{ t('protocols.connectionCard.accountAfterConnect') }}</p>
      </div>
    </div>
    <p v-if="showSummary" class="connection-summary">{{ runtime?.summary || t('protocols.connectionCard.savedWaiting') }}</p>
    <footer class="connection-footer">
      <div class="connection-instance"><span>{{ t('protocols.connectionDialog.instanceField') }}</span><code>{{ config.id }}</code></div>
      <div class="connection-actions">
        <AppButton size="sm" :data-testid="`adapter-${config.id}`" :disabled="busy" @click="$emit('configure')"><Settings2Icon />{{ t('protocols.connectionCard.configure') }}</AppButton>
        <AppButton size="sm" variant="ghost" class="connection-remove" :loading="removing" :disabled="busy" :aria-label="t('protocols.connectionCard.removeNamed', { id: config.id })" :title="t('protocols.connectionCard.remove')" @click="$emit('remove')"><Trash2Icon /></AppButton>
      </div>
    </footer>
  </li>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.connection-card { display: flex; flex-direction: column; min-width: 0; min-height: 240px; padding: 20px; border: 1px solid var(--border); border-radius: var(--app-card-radius); background: var(--surface-strong); }
.connection-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; }
.connection-protocol { color: var(--muted); font-size: 12px; font-weight: 500; }
.connection-account { display: flex; align-items: center; gap: 14px; margin: 24px 0; }
.connection-avatar { box-shadow: 0 0 0 1px var(--border); }
.connection-avatar :deep(svg) { width: 24px; height: 24px; }
.connection-identity { flex: 1; min-width: 0; }
.connection-identity h3 { margin: 0; font-size: 18px; font-weight: 600; line-height: 1.5; overflow-wrap: anywhere; }
.connection-identity .connection-name-pending { color: var(--muted); font-size: 16px; font-weight: 500; }
.connection-number { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 8px; margin: 5px 0 0; font-size: 12px; line-height: 1.6; }
.connection-number > :first-child { color: var(--muted); }
.connection-number > :last-child { min-width: 0; font-family: var(--font-mono); font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.connection-pending { margin: 5px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.connection-summary { margin: -8px 0 20px; color: var(--muted); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.connection-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: auto; padding-top: 16px; border-top: 1px solid var(--border); }
.connection-instance { display: grid; gap: 4px; min-width: 0; color: var(--muted); font-size: 11px; line-height: 1.5; }
.connection-instance code { font-size: 12px; overflow-wrap: anywhere; }
.connection-actions { display: flex; flex: none; align-items: center; gap: 4px; }
.connection-remove { color: var(--muted); }
.connection-remove:hover { color: var(--text-danger); background: var(--surface-danger); }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) {
  .connection-actions :deep(button) { min-width: 44px; min-height: 44px; }
}
</style>
