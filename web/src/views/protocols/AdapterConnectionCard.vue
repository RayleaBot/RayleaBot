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
// A stopped connection is already named by its badge, so the server's summary would only repeat it.
const showSummary = computed(() => props.config.enabled && (!identity.value?.id || props.runtime?.state !== 'connected'))
// Every connection is a light gray box; problems get a ring in their tone.
const attentionTone = computed(() => props.statusTone === 'danger' || props.statusTone === 'warning' ? props.statusTone : undefined)
</script>

<template>
  <li class="connection-card app-box" :data-attention="attentionTone">
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
        <p v-else class="connection-pending">{{ t(config.enabled ? 'protocols.connectionCard.accountAfterConnect' : 'protocols.connectionCard.accountDisabled') }}</p>
      </div>
    </div>
    <!-- A raw transport error can run long; the card keeps three lines and the configure dialog shows it all. -->
    <p v-if="showSummary" class="connection-summary" :title="runtime?.summary">{{ runtime?.summary || t('protocols.connectionCard.savedWaiting') }}</p>
    <footer class="connection-footer">
      <div class="connection-instance"><span>{{ t('protocols.connectionDialog.instanceField') }}</span><code>{{ config.id }}</code></div>
      <div class="connection-actions">
        <AppButton size="sm" :data-testid="`adapter-${config.id}`" :disabled="busy" @click="$emit('configure')"><template #icon><Settings2Icon /></template>{{ t('protocols.connectionCard.configure') }}</AppButton>
        <AppButton size="sm" variant="ghost" class="connection-remove" :loading="removing" :disabled="busy" :aria-label="t('protocols.connectionCard.removeNamed', { id: config.id })" :title="t('protocols.connectionCard.remove')" @click="$emit('remove')"><Trash2Icon /></AppButton>
      </div>
    </footer>
  </li>
</template>

<style scoped lang="scss">
.connection-card { display: flex; flex-direction: column; min-width: 0; min-height: 240px; padding: 20px; color: var(--text); }
.connection-card[data-attention=warning] { box-shadow: inset 0 0 0 2px var(--warning), var(--shadow-card); }
.connection-card[data-attention=danger] { box-shadow: inset 0 0 0 2px var(--danger), var(--shadow-card); }
.connection-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; }
.connection-protocol { color: var(--muted); font-size: 12px; font-weight: 500; }
.connection-account { display: flex; align-items: center; gap: 14px; margin: 24px 0; }
.connection-avatar { box-shadow: 0 0 0 1px var(--border); }
.connection-avatar :deep(svg) { width: 24px; height: 24px; }
.connection-identity { flex: 1; min-width: 0; }
.connection-identity h3 { margin: 0; font-size: 18px; font-weight: 700; line-height: 1.5; overflow-wrap: anywhere; }
.connection-identity .connection-name-pending { color: var(--muted); font-size: 16px; font-weight: 500; }
.connection-number { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 8px; margin: 5px 0 0; font-size: 12px; line-height: 1.6; }
.connection-number > :first-child { color: var(--muted); }
.connection-number > :last-child { min-width: 0; font-family: var(--font-mono); font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.connection-pending { margin: 5px 0 0; color: var(--muted); font-size: 12px; line-height: 1.6; }
.connection-summary { display: -webkit-box; margin: -8px 0 20px; overflow: hidden; color: var(--muted); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; -webkit-box-orient: vertical; -webkit-line-clamp: 3; line-clamp: 3; }
.connection-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: auto; padding-top: 16px; border-top: 1px solid var(--border); }
.connection-instance { display: grid; gap: 4px; min-width: 0; color: var(--muted); font-size: 12px; line-height: 1.5; }
.connection-instance code { font-size: 12px; overflow-wrap: anywhere; }
.connection-actions { display: flex; flex: none; align-items: center; gap: 4px; }
.connection-remove { color: var(--muted); }
.connection-remove:hover { color: var(--text-danger); background: var(--surface-danger); }
@media (pointer: coarse) {
  .connection-actions :deep(button) { min-width: 44px; min-height: 44px; }
}
@media (forced-colors: active) {
  .connection-card[data-attention] { outline: 2px solid Highlight; outline-offset: -4px; }
}
</style>
