<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import PluginBotAccountPicker from '@/components/plugins/PluginBotAccountPicker.vue'
import { t } from '@/i18n'
import { botAccountIdLabel, botAccountLabel, formatBotAccountKey } from '@/lib/bot-accounts'
import { buildProtocolsLocation } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { useAdaptersStore } from '@/stores/adapters'
import { useBotAccountsStore } from '@/stores/bot-accounts'
import type { BotAccount, BotAccountRef } from '@/types/api'

const props = defineProps<{ requested: { present: boolean; account: BotAccountRef | null } }>()
const emit = defineEmits<{
  select: [account: BotAccountRef, options: { replace?: boolean }]
  // The account the page may load for; null while none is valid, so no page shows another account's data.
  resolved: [account: BotAccount | null]
}>()

const quickLimit = 6
const navigate = useMotionNavigation()
const store = useBotAccountsStore()
const adaptersStore = useAdaptersStore()
const { accounts, total, loaded, loading, error, revision } = storeToRefs(store)
const current = ref<BotAccount | null>(null)
const problem = ref<'invalid' | 'missing' | 'failed' | null>(null)
const pickerOpen = ref(false)
let lookup = 0

const requestedKey = computed(() => props.requested.account ? formatBotAccountKey(props.requested.account) : '')
const currentKey = computed(() => current.value ? formatBotAccountKey(current.value) : '')

onMounted(() => {
  if (!loaded.value) void store.fetchAccounts().catch(() => undefined)
  if (!adaptersStore.loaded) void adaptersStore.refresh().catch(() => undefined)
})

function show(account: BotAccount | null, nextProblem: typeof problem.value) {
  current.value = account
  problem.value = nextProblem
  emit('resolved', account)
}

// The directory, not the address, decides whether the requested account exists. Only an absent
// account is filled in, with the first one in the directory order; a wrong one is reported instead.
async function resolveRequested() {
  const request = ++lookup
  const requested = props.requested
  if (!requested.present) {
    show(null, null)
    if (loaded.value && accounts.value[0]) emit('select', accounts.value[0], { replace: true })
    return
  }
  if (!requested.account) {
    show(null, 'invalid')
    return
  }
  const listed = accounts.value.find(account => formatBotAccountKey(account) === requestedKey.value)
  if (listed) {
    show(listed, null)
    return
  }
  if (currentKey.value !== requestedKey.value) show(null, null)
  try {
    const account = await store.fetchAccount(requested.account)
    if (request === lookup) show(account, account ? null : 'missing')
  } catch {
    if (request === lookup) show(null, 'failed')
  }
}

watch([requestedKey, () => props.requested.present, accounts, revision, loaded], () => { void resolveRequested() }, { immediate: true })

// The first accounts in directory order are one click away; the current one always stays visible.
const quickAccounts = computed(() => {
  const list = accounts.value.slice(0, quickLimit)
  const selected = current.value
  if (selected && !list.some(account => formatBotAccountKey(account) === currentKey.value)) {
    return [...list.slice(0, quickLimit - 1), selected]
  }
  return list
})
const options = computed(() => quickAccounts.value.map(account => ({
  value: formatBotAccountKey(account),
  label: botAccountLabel(account),
  title: `${botAccountLabel(account)} · ${botAccountIdLabel(account)}`,
})))
const selectedKey = computed({
  get: () => currentKey.value,
  set: (key: string) => {
    const account = quickAccounts.value.find(item => formatBotAccountKey(item) === key)
    if (account) emit('select', account, {})
  },
})
const hasMore = computed(() => total.value > options.value.length)
const single = computed(() => total.value === 1 && current.value !== null && problem.value === null)
const empty = computed(() => loaded.value && total.value === 0)
const noConnections = computed(() => adaptersStore.loaded && adaptersStore.adapters.length === 0)

function retry() {
  void store.fetchAccounts().catch(() => undefined)
  void resolveRequested()
}

function openProtocols() {
  void navigate(buildProtocolsLocation())
}
</script>

<template>
  <section class="plugin-bot-account-bar" :aria-label="t('botAccounts.bar.label')" data-testid="plugin-bot-account-bar">
    <AppSkeleton v-if="loading && !loaded" :rows="1" />

    <AppAlert v-else-if="error && !loaded" tone="danger" :title="t('botAccounts.bar.loadFailed')" :description="error">
      <template #action><AppButton @click="retry">{{ t('botAccounts.bar.retry') }}</AppButton></template>
    </AppAlert>

    <AppEmptyState
      v-else-if="empty"
      icon="generic"
      :title="t(noConnections ? 'botAccounts.bar.emptyNoConnectionTitle' : 'botAccounts.bar.emptyPendingTitle')"
      :description="t(noConnections ? 'botAccounts.bar.emptyNoConnectionDescription' : 'botAccounts.bar.emptyPendingDescription')"
      :action-label="t('botAccounts.bar.openProtocols')"
      data-testid="plugin-bot-account-empty"
      @action="openProtocols"
    />

    <template v-else>
      <p v-if="single" class="plugin-bot-account-bar__summary" data-testid="plugin-bot-account-summary">
        {{ t('botAccounts.bar.summary', { label: botAccountLabel(current!) }) }}
      </p>
      <div v-else class="plugin-bot-account-bar__switch">
        <span class="plugin-bot-account-bar__label">{{ t('botAccounts.bar.label') }}</span>
        <AppSegmented v-model="selectedKey" :options="options" fit="content" :label="t('botAccounts.bar.label')" data-testid="plugin-bot-account-switch" />
        <AppButton v-if="hasMore" data-testid="plugin-bot-account-all" @click="pickerOpen = true">{{ t('botAccounts.bar.all') }}</AppButton>
      </div>

      <AppAlert v-if="problem === 'invalid'" tone="warning" :title="t('botAccounts.bar.invalidTitle')" :description="t('botAccounts.bar.invalidDescription')" data-testid="plugin-bot-account-problem" />
      <AppAlert v-else-if="problem === 'missing'" tone="warning" :title="t('botAccounts.bar.missingTitle', { account: requested.account?.bot_id ?? '' })" :description="t('botAccounts.bar.missingDescription')" data-testid="plugin-bot-account-problem" />
      <AppAlert v-else-if="problem === 'failed'" tone="danger" :title="t('botAccounts.bar.lookupFailed')" data-testid="plugin-bot-account-problem">
        <template #action><AppButton @click="retry">{{ t('botAccounts.bar.retry') }}</AppButton></template>
      </AppAlert>
    </template>

    <PluginBotAccountPicker :open="pickerOpen" :current-key="currentKey" @close="pickerOpen = false" @select="emit('select', $event, {})" />
  </section>
</template>

<style scoped lang="scss">
.plugin-bot-account-bar { display: grid; flex: none; gap: 10px; margin-bottom: 12px; }
.plugin-bot-account-bar__switch { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; min-width: 0; }
.plugin-bot-account-bar__label { color: var(--muted); font-size: 13px; }
.plugin-bot-account-bar__summary { margin: 0; color: var(--muted); font-size: 13px; }
</style>
