<script setup lang="ts">
import AppDialog from '@/components/AppDialog.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppButton from '@/components/AppButton.vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppAlert from '@/components/AppAlert.vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRouter } from 'vue-router'
import { Skeleton } from '@/components/ui/skeleton'
import { ArrowLeftIcon, ChevronRightIcon } from '@lucide/vue'

import { t } from '@/i18n'
import { buildAdapterInstance, findAdapterInstance, nextAdapterInstanceId, type AdapterInstanceDocument, type QQOfficialSettings } from '@/lib/adapters'
import { mergeAdapterDraft, validateAdapterDraft } from '@/lib/adapter-form'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildProtocolRealtimeLogsLocation } from '@/lib/management-links'
import { buildOneBot11ReverseWsUrl, buildOneBot11WebhookUrl } from '@/lib/protocols'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { AdapterProtocol, ConfigDocument, ConfigUpdateResponse } from '@/types/api'
import OneBotConnectionFields from './OneBotConnectionFields.vue'

const props = defineProps<{ adapterId?: string; open: boolean }>()
const emit = defineEmits<{ close: []; afterClose: []; saved: [response: ConfigUpdateResponse] }>()
const configStore = useConfigStore()
const adaptersStore = useAdaptersStore()
const protocolsStore = useProtocolsStore()
const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const fieldErrors = ref<Record<string, string>>({})
const draft = ref<AdapterInstanceDocument | null>(null)
const baseline = ref<AdapterInstanceDocument | null>(null)
const initialDraft = ref('')
const sharedDraft = ref<ConfigDocument['adapter'] | null>(null)
const initialShared = ref('')
const advancedOpen = ref(false)
const allowedClose = ref(false)
const formElement = ref<HTMLFormElement | null>(null)
const isEditing = computed(() => Boolean(props.adapterId))
const dirty = computed(() => Boolean(draft.value) && (JSON.stringify(draft.value) !== initialDraft.value || JSON.stringify(sharedDraft.value) !== initialShared.value))
const descriptor = computed(() => adaptersStore.adapters.find((item) => item.id === props.adapterId))
const protocolName = computed(() => draft.value?.type === 'qqofficial' ? 'QQ 官方机器人' : 'OneBot11')
const firstOneBot = computed(() => configStore.document?.adapters.find((item) => item.type === 'onebot11')?.id)
const runtimeSnapshot = computed(() => props.adapterId === firstOneBot.value ? protocolsStore.snapshot : null)
const intentOptions: { value: QQOfficialSettings['intents'][number]; label: string }[] = [
  { value: 'group_and_c2c', label: t('protocols.qqIntents.groupAndC2c') },
  { value: 'public_guild_messages', label: t('protocols.qqIntents.publicGuildMessages') },
  { value: 'guilds', label: t('protocols.qqIntents.guilds') },
  { value: 'guild_members', label: t('protocols.qqIntents.guildMembers') },
  { value: 'guild_messages', label: t('protocols.qqIntents.guildMessages') },
  { value: 'direct_message', label: t('protocols.qqIntents.directMessage') },
]
const sharedFields: { key: keyof ConfigDocument['adapter']; label: string; min: number; max?: number; step: number }[] = [
  { key: 'connect_timeout_seconds', label: '连接超时（秒）', min: 1, step: 1 },
  { key: 'reconnect_initial_seconds', label: '初始重连时间（秒）', min: 1, step: 1 },
  { key: 'reconnect_multiplier', label: '重连倍率', min: 1, step: 0.1 },
  { key: 'reconnect_max_seconds', label: '最大重连时间（秒）', min: 1, step: 1 },
  { key: 'reconnect_jitter_ratio', label: '重连抖动比例', min: 0, max: 1, step: 0.05 },
]
const baseUrl = import.meta.env.VITE_WS_BASE_URL || window.location.origin
function copy<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T }

async function load() {
  loading.value = true
  error.value = ''
  try {
    await configStore.fetchConfig()
    if (!configStore.document) throw new Error('missing config')
    sharedDraft.value = copy(configStore.document.adapter)
    initialShared.value = JSON.stringify(sharedDraft.value)
    if (props.adapterId) {
      const instance = findAdapterInstance(configStore.document, props.adapterId)
      if (!instance) {
        error.value = '此连接已被删除或尚未配置。请关闭弹窗并刷新列表。'
        return
      }
      baseline.value = copy(instance)
      draft.value = copy(instance)
      initialDraft.value = JSON.stringify(draft.value)
      if (instance.type === 'onebot11' && instance.id === firstOneBot.value) {
        void protocolsStore.refresh().catch(() => undefined)
      }
    }
  } catch (err) {
    error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
  } finally { loading.value = false }
}
onMounted(() => { void load() })

async function selectProtocol(protocol: AdapterProtocol) {
  if (!configStore.document) return
  const instance = buildAdapterInstance(nextAdapterInstanceId(configStore.document, protocol), protocol)
  instance.enabled = true
  if (instance.onebot11) {
    instance.onebot11.reverse_ws.enabled = true
    instance.onebot11.reverse_ws.url = buildOneBot11ReverseWsUrl(baseUrl, instance.id)
  }
  if (instance.qqofficial) instance.qqofficial.intents = ['group_and_c2c']
  draft.value = instance
  initialDraft.value = JSON.stringify(instance)
  fieldErrors.value = {}
  error.value = ''
  await nextTick()
  formElement.value?.querySelector<HTMLElement>('input:not([type=hidden]), [role=combobox]')?.focus()
}

watch(() => draft.value?.id, (id, previous) => {
  if (!id || !previous || isEditing.value || !draft.value?.onebot11) return
  for (const [key, buildUrl] of [['reverse_ws', buildOneBot11ReverseWsUrl], ['webhook', buildOneBot11WebhookUrl]] as const) {
    if (draft.value.onebot11[key].url === buildUrl(baseUrl, previous)) draft.value.onebot11[key].url = buildUrl(baseUrl, id)
  }
})

const confirmOpen = ref(false)
let confirmResolve: ((value: boolean) => void) | null = null
let closeConfirmation: Promise<boolean> | null = null
function finishConfirmation(value: boolean) {
  confirmOpen.value = false
  confirmResolve?.(value)
  confirmResolve = null
}
onBeforeUnmount(() => finishConfirmation(false))
function canClose(): Promise<boolean> {
  if (allowedClose.value) return Promise.resolve(true)
  if (saving.value || configStore.saving) return Promise.resolve(false)
  if (!dirty.value) return Promise.resolve(true)
  if (closeConfirmation) return closeConfirmation
  closeConfirmation = new Promise<boolean>((resolve) => {
    confirmResolve = resolve
    confirmOpen.value = true
  }).finally(() => { closeConfirmation = null })
  return closeConfirmation
}
async function requestClose() {
  if (await canClose()) { allowedClose.value = true; emit('close') }
}
async function changeProtocol() {
  if (!await canClose()) return
  draft.value = null
  sharedDraft.value = JSON.parse(initialShared.value)
  advancedOpen.value = false
  error.value = ''
  fieldErrors.value = {}
}
onBeforeRouteLeave(canClose)
onBeforeRouteUpdate((to, from) => (
  to.query.adapter !== from.query.adapter || to.query.view !== from.query.view ? canClose() : true
))
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value || saving.value) { event.preventDefault(); event.returnValue = '' }
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))

async function save() {
  if (!draft.value || saving.value || configStore.saving) return
  fieldErrors.value = validateAdapterDraft(draft.value)
  if (sharedDraft.value && draft.value.type === 'onebot11') {
    for (const { key, min, max, step } of sharedFields) {
      const value = sharedDraft.value[key]
      if (typeof value !== 'number' || !Number.isFinite(value) || value < min || (max !== undefined && value > max) || (step === 1 && !Number.isInteger(value))) fieldErrors.value[key] = '请填写有效范围内的数值。'
    }
  }
  if (Object.keys(fieldErrors.value).length) {
    error.value = '请检查标出的配置项。'
    advancedOpen.value = Boolean(fieldErrors.value.id || sharedFields.some(({ key }) => fieldErrors.value[key]))
    await nextTick()
    formElement.value?.querySelector<HTMLElement>('[data-invalid=true] input, [data-invalid=true] [role=combobox]')?.focus()
    return
  }
  saving.value = true
  error.value = ''
  try {
    await configStore.fetchConfig()
    if (!configStore.document) throw new Error('missing config')
    let nextDocument: ConfigDocument
    try {
      nextDocument = mergeAdapterDraft(configStore.document, baseline.value, draft.value)
      if (JSON.stringify(sharedDraft.value) !== initialShared.value && sharedDraft.value) {
        if (JSON.stringify(configStore.document.adapter) !== initialShared.value) throw new Error('共用重连策略已在其他位置更改，请重新打开后再编辑。')
        nextDocument.adapter = copy(sharedDraft.value)
      }
    } catch (err) {
      error.value = (err as Error).message
      advancedOpen.value = !isEditing.value
      return
    }
    const response = await configStore.saveConfig(nextDocument)
    allowedClose.value = true
    emit('saved', response)
  } catch (err) {
    error.value = getDisplayErrorMessage(err, 'errors.common.saveFailed')
  } finally { saving.value = false }
}
function openLogs() {
  if (draft.value) void router.push(draft.value.type === 'onebot11' ? buildProtocolRealtimeLogsLocation() : { path: '/logs' })
}
</script>

<template>
  <AppDialog :open="open" :title="draft ? (isEditing ? '配置 ' : '添加 ') + protocolName : '添加连接'" :width="640" :busy="saving" fallback-focus="[data-testid=adapter-add]" @close="requestClose" @after-close="$emit('afterClose')">
    <div v-if="loading" class="dialog-loading" role="status" aria-label="正在加载连接配置"><Skeleton class="h-5 w-2/3" /><Skeleton class="h-10 w-full" /><Skeleton class="h-5 w-1/2" /><Skeleton class="h-10 w-full" /></div>
    <div v-else>
      <AppAlert v-if="error" tone="danger" :title="error" class="dialog-error" role="alert">
        <template v-if="!draft" #action><AppButton size="sm" :loading="loading" @click="load">重试</AppButton></template>
      </AppAlert>
      <div v-if="!draft && !loading && !error" class="protocol-picker">
        <p class="dialog-description">选择接入方式，接下来填写连接配置。</p>
        <button v-for="protocol in adaptersStore.availableProtocols" :key="protocol.protocol" type="button" class="protocol-choice" :data-testid="`adapter-select-${protocol.protocol}`" @click="selectProtocol(protocol.protocol)">
          <span><strong>{{ protocol.display_name }}</strong><small>{{ protocol.description }}</small></span>
          <ChevronRightIcon />
        </button>
        <p v-if="!adaptersStore.availableProtocols.length" role="status">暂时没有可用协议，请刷新列表后重试。</p>
      </div>
      <form v-if="draft" ref="formElement" novalidate @submit.prevent="save">
        <fieldset :disabled="saving || loading" class="dialog-fields">
          <p class="dialog-description">{{ isEditing ? `连接标识：${draft.id}` : '完成配置后保存，即可在协议中心管理此连接。' }}</p>
          <template v-if="draft.qqofficial">
            <AppField label="AppID" for="adapter-app-id" :required="draft.enabled" :error="fieldErrors.app_id">
              <AppInput id="adapter-app-id" v-model="draft.qqofficial.app_id" placeholder="QQ 开放平台的机器人 AppID" inputmode="numeric" autocomplete="off" />
            </AppField>
            <AppField label="AppSecret" for="adapter-app-secret" :required="draft.enabled" :error="fieldErrors.app_secret">
              <AppInput type="password" id="adapter-app-secret" v-model="draft.qqofficial.app_secret" placeholder="填写机器人密钥" autocomplete="new-password" />
              <p class="field-hint">{{ draft.qqofficial.app_secret === '********' ? '已保存密钥。保持现值可沿用，重新输入可替换。' : '在 QQ 开放平台的机器人管理中获取。' }}</p>
            </AppField>
            <AppField label="接收消息" for="adapter-intents">
              <AppSelect id="adapter-intents" v-model="draft.qqofficial.intents" :multiple="true" :options="intentOptions" placeholder="选择要接收的事件" />
              <p class="field-hint">按机器人已获授权的能力选择；不选择任何事件时不会收到消息。</p>
            </AppField>
          </template>
          <OneBotConnectionFields v-if="draft.onebot11" v-model="draft.onebot11" :adapter-id="draft.id" :errors="fieldErrors" />
          <div class="enable-connection">
            <div><label for="adapter-enabled">启用此连接</label><p class="field-hint">关闭后保留配置，暂停连接和消息收发。</p></div>
            <AppSwitch id="adapter-enabled" v-model="draft.enabled" aria-label="启用此连接" />
          </div>
          <details class="dialog-disclosure" :open="advancedOpen" @toggle="advancedOpen = ($event.target as HTMLDetailsElement).open">
            <summary>高级设置<span>连接标识{{ draft.qqofficial ? '、沙箱环境' : '、共用重连策略' }}</span></summary>
            <div class="disclosure-content">
              <AppField label="连接标识" for="adapter-id" :error="fieldErrors.id">
                <AppInput id="adapter-id" v-model="draft.id" :disabled="isEditing" :maxlength="64" />
                <p class="field-hint">{{ isEditing ? '创建后固定，用于识别此连接。' : '已自动生成。标识用于区分连接，也会出现在回连地址中。' }}</p>
              </AppField>
              <AppCheckbox v-if="draft.qqofficial" v-model="draft.qqofficial.sandbox">使用 QQ 沙箱环境</AppCheckbox>
              <p v-if="draft.qqofficial" class="field-hint">仅对沙箱名单内的账号和群生效。</p>
              <template v-if="draft.onebot11 && sharedDraft">
                <h3>所有连接共用的重连策略</h3>
                <p class="field-hint">这些参数同时影响所有适配器连接。</p>
                <div class="shared-fields">
                  <AppField v-for="field in sharedFields" :key="field.key" :label="field.label" :for="`adapter-${field.key}`" :error="fieldErrors[field.key]">
                    <AppNumberInput :id="`adapter-${field.key}`" v-model="sharedDraft[field.key]" :min="field.min" :max="field.max" :step="field.step" />
                  </AppField>
                </div>
              </template>
            </div>
          </details>
          <details v-if="isEditing" class="dialog-disclosure">
            <summary>运行状态与诊断</summary>
            <div class="disclosure-content">
              <p>{{ descriptor?.summary || '尚未读取到此连接的运行状态。' }}</p>
              <p v-if="descriptor?.identity" class="field-hint">登录身份：{{ descriptor.identity.name || descriptor.identity.id }}</p>
              <AppAlert v-if="adapterId === firstOneBot && protocolsStore.error" tone="warning" :title="protocolsStore.error" />
              <ul v-if="runtimeSnapshot" class="runtime-list">
                <li v-for="transport in runtimeSnapshot.transport_status" :key="transport.transport">
                  <strong>{{ transport.transport }}</strong> · {{ transport.summary }}
                  <small>{{ [transport.app_name, transport.app_version, transport.nickname, transport.user_id].filter(Boolean).join(' · ') }}</small>
                </li>
                <li v-for="issue in runtimeSnapshot.recent_transport_issues" :key="issue.code">{{ issue.code }} · {{ issue.summary }}</li>
              </ul>
              <AppButton size="sm" @click="openLogs">{{ draft.type === 'onebot11' ? '查看此协议的实时日志' : '查看实时日志' }}</AppButton>
            </div>
          </details>
        </fieldset>
      </form>
    </div>
    <template #footer>
      <div class="dialog-footer">
        <AppButton v-if="draft && !isEditing" variant="ghost" :disabled="saving" @click="changeProtocol"><ArrowLeftIcon />更换协议</AppButton>
        <span v-else class="footer-spacer" />
        <AppButton :disabled="saving" @click="requestClose">取消</AppButton>
        <AppButton v-if="draft" variant="default" :loading="saving" :disabled="loading || configStore.saving || (isEditing && !dirty)" data-testid="adapter-save" @click="save">{{ isEditing ? '保存修改' : '保存连接' }}</AppButton>
      </div>
      <p v-if="draft && !isEditing" class="save-hint">新增连接将在服务重启后加载。</p>
    </template>
  </AppDialog>
  <AppConfirmDialog :open="confirmOpen" title="放弃未保存的修改？" description="填写的内容还未保存，离开后将丢失。" confirm-text="放弃修改" cancel-text="继续编辑" danger @confirm="finishConfirmation(true)" @cancel="finishConfirmation(false)" />
</template>

<style scoped lang="scss">
.dialog-fields { border: 0; padding: 0; margin: 0; min-width: 0; }
.dialog-loading { display: grid; gap: 20px; min-height: 300px; align-content: start; }
.dialog-description { margin: 0 0 24px; color: var(--muted); line-height: 1.6; }
.dialog-error { margin-bottom: 20px; }
.protocol-picker { display: grid; gap: 12px; padding: 4px 0 16px; }
.protocol-picker .dialog-description { margin-bottom: 8px; }
.protocol-choice { display: flex; align-items: center; justify-content: space-between; gap: 20px; width: 100%; padding: 20px; background: var(--surface-strong); border: 1px solid var(--border); border-radius: var(--app-card-radius); color: var(--text); text-align: left; cursor: pointer; font: inherit; }
.protocol-choice:hover { background: var(--surface-soft); border-color: var(--app-primary); }
.protocol-choice:focus-visible { outline: 2px solid var(--app-primary); outline-offset: var(--focus-outline-offset); }
.protocol-choice strong { display: block; margin-bottom: 6px; font-size: 15px; }
.protocol-choice small, .field-hint { color: var(--muted); font-size: 13px; line-height: 1.6; }
.field-hint { margin: 6px 0 0; }
.enable-connection { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 16px 0 24px; }
.enable-connection label { font-weight: 500; }
.dialog-disclosure { border-top: 1px solid var(--border); }
.dialog-disclosure summary { padding: 18px 0; color: var(--text); font-weight: 500; cursor: pointer; }
.dialog-disclosure summary span { font-size: 12px; font-weight: 400; color: var(--muted); margin-left: 12px; }
.disclosure-content { padding-bottom: 20px; }
.disclosure-content h3 { margin: 24px 0 0; font-size: 14px; }
.shared-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 16px; margin-top: 16px; }
.runtime-list { padding-left: 18px; font-size: 13px; }
.runtime-list li { margin-bottom: 12px; overflow-wrap: anywhere; }
.runtime-list small { display: block; color: var(--muted); }
.dialog-footer { display: flex; align-items: center; gap: 8px; }
.dialog-footer > :first-child { margin-right: auto; }
.save-hint { margin: 10px 0 0; color: var(--muted); font-size: 12px; text-align: right; }
@media (max-width: 480px) {
  .dialog-disclosure summary span { display: none; }
  .shared-fields { grid-template-columns: 1fr; }
}
</style>
