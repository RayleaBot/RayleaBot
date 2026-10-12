<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import AppAlert from '@/components/AppAlert.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppInput from '@/components/AppInput.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import { t } from '@/i18n'
import { botAccountIdLabel, botAccountName, formatBotAccountKey } from '@/lib/bot-accounts'
import { collectionURL, createCollectionPager, mergeCollectionItems } from '@/lib/collection-pager'
import { apiRequest } from '@/lib/http'
import type { BotAccount, BotAccountListResponse } from '@/types/api'

const props = defineProps<{ open: boolean; currentKey: string }>()
const emit = defineEmits<{ close: []; select: [account: BotAccount] }>()

// The picker pages its own search, so searching never replaces the switcher's accounts.
const items = ref<BotAccount[]>([])
const query = ref('')
const pager = createCollectionPager<BotAccountListResponse>({
  request: (search, cursor, signal) => apiRequest(collectionURL('/api/bot-accounts', search, cursor), { signal }),
  apply: (response, append) => {
    items.value = append ? mergeCollectionItems(items.value, response.items, formatBotAccountKey) : response.items
  },
})
const { total, nextCursor, loading, loadingMore, loaded, error } = pager

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(() => props.open, (open) => {
  if (open) void pager.load({ query: query.value }).catch(() => undefined)
}, { immediate: true })
watch(query, (value) => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { void pager.load({ query: value }).catch(() => undefined) }, 250)
})

function connectionText(account: BotAccount) {
  if (account.state === 'no_connection') {
    return t('botAccounts.state.lastConnection', { connection: account.last_adapter_display_name ?? '' })
  }
  const state = account.state === 'online' ? t('botAccounts.state.online') : t('botAccounts.state.offline')
  return `${account.adapter_display_name} · ${state}`
}

const rows = computed(() => items.value.map(account => ({
  account,
  key: formatBotAccountKey(account),
  name: botAccountName(account),
  detail: `${botAccountIdLabel(account)} · ${connectionText(account)}`,
})))

function choose(account: BotAccount) {
  emit('select', account)
  emit('close')
}
</script>

<template>
  <AppDialog :open="open" :title="t('botAccounts.picker.title')" :description="t('botAccounts.picker.description')" :width="560" @close="emit('close')">
    <div class="bot-account-picker">
      <AppInput v-model="query" type="search" allow-clear :placeholder="t('botAccounts.picker.search')" :aria-label="t('botAccounts.picker.search')" />
      <AppSkeleton v-if="loading && !loaded" :rows="3" />
      <AppAlert v-else-if="error" tone="danger" :title="error" />
      <p v-else-if="rows.length === 0" class="bot-account-picker__empty">{{ t('botAccounts.picker.empty') }}</p>
      <ul v-else class="bot-account-picker__list" :aria-label="t('botAccounts.picker.title')">
        <li v-for="row in rows" :key="row.key">
          <button
            type="button"
            class="bot-account-picker__option"
            :aria-current="row.key === currentKey ? 'true' : undefined"
            :data-state="row.account.state"
            data-testid="bot-account-picker-option"
            @click="choose(row.account)"
          >
            <span class="bot-account-picker__name">{{ row.name }}</span>
            <span class="bot-account-picker__detail">{{ row.detail }}</span>
            <span v-if="row.key === currentKey" class="bot-account-picker__current">{{ t('botAccounts.picker.current') }}</span>
          </button>
        </li>
      </ul>
      <AppCollectionPagination :loaded="items.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore" @more="pager.loadMore()" />
    </div>
  </AppDialog>
</template>

<style scoped lang="scss">
.bot-account-picker { display: grid; gap: 12px; }
.bot-account-picker__empty { margin: 0; padding: 12px 0; color: var(--muted); }
.bot-account-picker__list { display: grid; gap: 4px; margin: 0; padding: 0; list-style: none; }
// Each account is one row button: its name, then the account number, connection and state.
.bot-account-picker__option { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 2px 12px; width: 100%; padding: 10px 14px; border: 1px solid transparent; border-radius: var(--radius-md); background: transparent; color: var(--text); text-align: left; cursor: pointer; transition: background-color var(--motion-fast); }
.bot-account-picker__option:hover { background: var(--surface-soft); }
.bot-account-picker__option[aria-current='true'] { background: var(--surface-raised); box-shadow: var(--shadow-xs); }
.bot-account-picker__option:focus-visible { outline: 2px solid var(--focus); outline-offset: var(--focus-outline-offset); }
.bot-account-picker__name { overflow: hidden; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.bot-account-picker__detail { grid-column: 1; overflow: hidden; color: var(--muted); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.bot-account-picker__current { grid-column: 2; grid-row: 1 / span 2; align-self: center; color: var(--muted); font-size: 13px; }
@media (pointer: coarse) { .bot-account-picker__option { min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .bot-account-picker__option { transition: none; } }
@media (forced-colors: active) { .bot-account-picker__option[aria-current='true'] { outline: 2px solid Highlight; outline-offset: -2px; } }
</style>
