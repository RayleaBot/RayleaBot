<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppAlert from '@/components/AppAlert.vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Skeleton } from '@/components/ui/skeleton'
import { PlusIcon, RefreshCwIcon } from '@lucide/vue'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import { t } from '@/i18n'
import { readAdapterInstances, type AdapterInstanceDocument } from '@/lib/adapters'
import { cloneConfig } from '@/lib/config-form'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildProtocolsLocation, buildProtocolCompatibilityLocation } from '@/lib/management-links'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import type { AdapterDescriptor, ConfigUpdateResponse } from '@/types/api'
import AdapterConfigDialog from './AdapterConfigDialog.vue'
import AdapterConnectionCard from './AdapterConnectionCard.vue'
import ProtocolCompatibilityPanel from './ProtocolCompatibilityPanel.vue'

const router = useRouter()
const route = useRoute()
const adaptersStore = useAdaptersStore()
const configStore = useConfigStore()
const loading = ref(true)
const removingId = ref<string | null>(null)
const restartPending = ref(false)
const pageError = computed(() => configStore.error || adaptersStore.error)
const rows = computed(() => readAdapterInstances(configStore.document).map((config) => ({
  config,
  runtime: adaptersStore.adapters.find((item) => item.id === config.id),
})))
const editorId = computed(() => typeof route.query.adapter === 'string' ? route.query.adapter : undefined)
const editorOpen = computed(() => route.path === '/protocols' && (Boolean(editorId.value) || route.query.view === 'add'))
const compatibilityOpen = computed(() => route.path === '/protocols' && route.query.view === 'compatibility' && !editorId.value)
const editorSession = ref<{ id?: string } | null>(null)
const removeTarget = ref<AdapterInstanceDocument | null>(null)
watch([editorOpen, editorId], ([open, id]) => {
  if (open) editorSession.value = { id }
}, { immediate: true })
function editorClosed() { if (!editorOpen.value) editorSession.value = null }
function badgeTone(color: string): 'neutral' | 'info' | 'success' | 'warning' | 'danger' {
  if (color === 'error') return 'danger'
  if (color === 'processing') return 'info'
  if (color === 'success' || color === 'warning') return color
  return 'neutral'
}

async function refresh() {
  loading.value = true
  await Promise.allSettled([configStore.fetchConfig(), adaptersStore.refresh()])
  loading.value = false
}
onMounted(() => { void refresh() })

function status(runtime: AdapterDescriptor | undefined, config: AdapterInstanceDocument) {
  if (!runtime) return { label: '等待服务加载', color: 'default' }
  if (runtime.enabled !== config.enabled) return { label: '配置待应用', color: 'default' }
  if (!runtime.enabled) return { label: t('protocols.adapterDisabled'), color: 'default' }
  const color = ['connected', 'listening'].includes(runtime.state) ? 'success'
    : ['connecting', 'reconnecting'].includes(runtime.state) ? 'processing'
      : runtime.state === 'auth_failed' ? 'error' : 'warning'
  return { label: t(`protocols.adapterState.${runtime.state}`), color }
}
function closeDialog() { void router.replace(buildProtocolsLocation()) }
async function saved(response: ConfigUpdateResponse) {
  restartPending.value ||= response.restart_required
  closeDialog()
  notifySuccess(response.restart_required ? '连接配置已保存，重启服务后生效。' : '连接配置已保存。')
  await adaptersStore.refresh().catch(() => undefined)
}
async function removeAdapter(instance: AdapterInstanceDocument) {
  if (removingId.value || configStore.saving) return
  removingId.value = instance.id
  try {
    await configStore.fetchConfig()
    if (!configStore.document) return
    const current = configStore.document.adapters.find((entry) => entry.id === instance.id)
    if (JSON.stringify(current) !== JSON.stringify(instance)) {
      notifyError('此连接已更改，请刷新后重新确认删除。')
      return
    }
    const next = cloneConfig(configStore.document)
    next.adapters = next.adapters.filter((entry) => entry.id !== instance.id)
    const response = await configStore.saveConfig(next)
    removeTarget.value = null
    restartPending.value ||= response.restart_required
    notifySuccess(response.restart_required ? '连接已移除，重启服务后停止加载。' : '连接已移除。')
    await adaptersStore.refresh().catch(() => undefined)
  } catch (err) {
    notifyError(getDisplayErrorMessage(err, 'errors.common.saveFailed'))
  } finally { removingId.value = null }
}
</script>

<template>
  <AppPage title="协议中心" description="管理机器人连接，查看运行状态并调整接入配置。" width="detail">
    <template #extra>
      <AppButton @click="router.push(buildProtocolCompatibilityLocation())">兼容矩阵</AppButton>
      <AppButton variant="default" :disabled="loading || !configStore.document || Boolean(removingId)" data-testid="adapter-add" @click="router.push(buildProtocolsLocation({ view: 'add' }))"><PlusIcon />添加连接</AppButton>
    </template>
    <div class="connections-workspace">
      <AppAlert v-if="restartPending" tone="info" title="配置已保存，部分变更需要重启服务" description="新增或移除的连接将在服务重启后生效。下方状态来自当前运行中的服务。" data-testid="adapter-restart-notice" />
      <AppAlert v-if="pageError" tone="danger" :title="pageError" role="alert">
        <template #action><AppButton size="sm" :loading="loading" @click="refresh">重试</AppButton></template>
      </AppAlert>
      <section class="connections-surface" aria-labelledby="connections-title">
        <header class="connections-toolbar">
          <div><h2 id="connections-title">已配置连接</h2><span v-if="!loading && configStore.document">{{ rows.length }} 个连接</span></div>
          <AppButton variant="ghost" :loading="loading" :disabled="Boolean(removingId) || configStore.saving" aria-label="刷新连接状态" @click="refresh"><RefreshCwIcon /></AppButton>
        </header>
        <div v-if="loading && !configStore.document" class="connections-loading"><Skeleton class="h-36 w-full" /></div>
        <div v-else-if="!pageError && !rows.length" class="connections-empty">
          <h3>还没有机器人连接</h3>
          <p>添加一个连接，填写配置后即可接入机器人。</p>
        </div>
        <ul v-else class="connections-grid">
          <AdapterConnectionCard v-for="{ config, runtime } in rows" :key="config.id"
            :config="config" :runtime="runtime" :status-label="status(runtime, config).label" :status-tone="badgeTone(status(runtime, config).color)"
            :busy="Boolean(removingId) || configStore.saving" :removing="removingId === config.id"
            @configure="router.push(buildProtocolsLocation({ adapterId: config.id }))" @remove="removeTarget = config" />
        </ul>
      </section>
    </div>
    <AdapterConfigDialog v-if="editorSession" :key="editorSession.id || 'add'" :open="editorOpen" :adapter-id="editorSession.id" @close="closeDialog" @after-close="editorClosed" @saved="saved" />
    <AppDialog :open="compatibilityOpen" title="协议兼容矩阵" :width="1040" @close="closeDialog">
      <ProtocolCompatibilityPanel />
    </AppDialog>
    <AppConfirmDialog :open="Boolean(removeTarget)" title="删除连接？" :description="t('protocols.removeAdapterConfirm', { name: removeTarget?.id || '' })" confirm-text="删除" danger :busy="Boolean(removingId)" @cancel="removeTarget = null" @confirm="removeTarget && removeAdapter(removeTarget)" />
  </AppPage>
</template>

<style scoped lang="scss">
.connections-workspace { display: grid; gap: 20px; }
.connections-surface { display: grid; gap: 16px; }
.connections-toolbar { display: flex; align-items: center; justify-content: space-between; min-height: 36px; }
.connections-toolbar > div { display: flex; align-items: baseline; gap: 12px; }
.connections-toolbar h2 { margin: 0; font-size: 14px; font-weight: 600; }
.connections-toolbar span { color: var(--muted); font-size: 12px; }
.connections-loading { padding: 24px; }
.connections-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 320px), 1fr)); gap: 20px; margin: 0; padding: 0; list-style: none; }
.connections-empty { padding: 56px 24px; border: 1px solid var(--border); border-radius: var(--app-card-radius); background: var(--surface-strong); text-align: center; }
.connections-empty p { margin: 12px 0 0; color: var(--muted); font-size: 13px; }
@media (max-width: 639px) {
  .connections-grid { gap: 16px; }
}
</style>
