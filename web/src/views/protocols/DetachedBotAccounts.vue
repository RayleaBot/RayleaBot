<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppButton from '@/components/AppButton.vue'
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import { t } from '@/i18n'
import { botAccountIdLabel, botAccountName, formatBotAccountKey } from '@/lib/bot-accounts'
import { collectionURL, createCollectionPager, mergeCollectionItems } from '@/lib/collection-pager'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { apiRequest } from '@/lib/http'
import { useBotAccountsStore } from '@/stores/bot-accounts'
import type { BotAccount, BotAccountListResponse } from '@/types/api'

const botAccountsStore = useBotAccountsStore()
const { revision } = storeToRefs(botAccountsStore)
const accounts = ref<BotAccount[]>([])
// Accounts without a connection are listed apart from the cards, so they get their own filtered pages.
const pager = createCollectionPager<BotAccountListResponse>({
  request: (query, cursor, signal) => apiRequest(collectionURL('/api/bot-accounts', query, cursor), { signal }),
  apply: (response, append) => {
    accounts.value = append ? mergeCollectionItems(accounts.value, response.items, formatBotAccountKey) : response.items
  },
})
const { total, nextCursor, loadingMore } = pager
const removeTarget = ref<BotAccount | null>(null)
const removing = ref(false)

onMounted(() => { void pager.load({ state: 'no_connection' }).catch(() => undefined) })
watch(revision, () => { void pager.load().catch(() => undefined) })

const rows = computed(() => accounts.value.map(account => ({
  account,
  key: formatBotAccountKey(account),
  name: botAccountName(account),
  detail: [
    botAccountIdLabel(account),
    t('protocols.detachedAccounts.lastConnection', { connection: account.last_adapter_display_name ?? '' }),
    t('protocols.detachedAccounts.lastSeen', { time: formatDateTime(account.last_seen_at) }),
  ].join(' · '),
})))

async function remove() {
  const target = removeTarget.value
  if (!target || removing.value) return
  removing.value = true
  try {
    await botAccountsStore.removeAccount(target)
    removeTarget.value = null
    notifySuccess(t('protocols.detachedAccounts.removed'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err, 'errors.common.saveFailed'))
  } finally {
    removing.value = false
  }
}
</script>

<template>
  <section v-if="rows.length" class="detached-accounts" aria-labelledby="detached-accounts-title" data-testid="detached-bot-accounts">
    <header class="detached-accounts__header">
      <h2 id="detached-accounts-title">{{ t('protocols.detachedAccounts.title') }}</h2>
      <span>{{ t('protocols.connectionsCount', { count: total }) }}</span>
    </header>
    <p class="detached-accounts__description">{{ t('protocols.detachedAccounts.description') }}</p>
    <ul class="detached-accounts__list app-box">
      <li v-for="row in rows" :key="row.key" class="detached-accounts__row">
        <div class="detached-accounts__identity">
          <strong>{{ row.name }}</strong>
          <span>{{ row.detail }}</span>
        </div>
        <div class="detached-accounts__actions">
          <AppButton size="sm" variant="ghost" class="detached-accounts__remove" :aria-label="t('protocols.detachedAccounts.removeNamed', { name: row.name })" :disabled="removing" data-testid="detached-bot-account-remove" @click="removeTarget = row.account">{{ t('protocols.detachedAccounts.remove') }}</AppButton>
        </div>
      </li>
    </ul>
    <AppCollectionPagination :loaded="accounts.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore" @more="pager.loadMore()" />
    <AppConfirmDialog
      :open="Boolean(removeTarget)"
      :title="t('protocols.detachedAccounts.removeTitle')"
      :description="removeTarget ? t('protocols.detachedAccounts.removeDescription', { name: botAccountName(removeTarget) }) : ''"
      :confirm-text="t('protocols.detachedAccounts.remove')"
      danger
      :busy="removing"
      @cancel="removeTarget = null"
      @confirm="remove"
    />
  </section>
</template>

<style scoped lang="scss">
.detached-accounts { display: grid; gap: 12px; }
.detached-accounts__header { display: flex; align-items: baseline; gap: 12px; min-height: 36px; }
.detached-accounts__header h2 { margin: 0; font-size: 14px; font-weight: 700; }
.detached-accounts__header span { color: var(--muted); font-size: 12px; }
.detached-accounts__description { margin: 0; color: var(--muted); font-size: 13px; }
.detached-accounts__list { display: grid; margin: 0; padding: 0 20px; list-style: none; }
.detached-accounts__row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 0; }
.detached-accounts__row + .detached-accounts__row { border-top: 1px solid var(--border); }
.detached-accounts__identity { display: grid; gap: 4px; min-width: 0; }
.detached-accounts__identity strong { overflow-wrap: anywhere; }
.detached-accounts__identity span { color: var(--muted); font-size: 12px; overflow-wrap: anywhere; }
.detached-accounts__actions { display: flex; flex: none; align-items: center; gap: 4px; }
.detached-accounts__remove { color: var(--muted); }
.detached-accounts__remove:hover { color: var(--text-danger); background: var(--surface-danger); }
@media (pointer: coarse) {
  .detached-accounts__actions :deep(button) { min-height: 44px; }
}
</style>
