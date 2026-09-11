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
import type { AdapterProtocol, ConfigDocument, ConfigUpdateResponse } from '@/types/api'
import OneBotConnectionFields from './OneBotConnectionFields.vue'

const props = defineProps<{ adapterId?: string; open: boolean }>()
const emit = defineEmits<{ close: []; afterClose: []; saved: [response: ConfigUpdateResponse] }>()
const configStore = useConfigStore()
const adaptersStore = useAdaptersStore()
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
const protocolName = computed(() => draft.value?.type === 'qqofficial' ? t('protocols.qqTitle') : 'OneBot11')
const runtimeSnapshot = computed(() => descriptor.value?.onebot11 ?? null)
const intentOptions: { value: QQOfficialSettings['intents'][number]; label: string }[] = [
  { value: 'group_and_c2c', label: t('protocols.qqIntents.groupAndC2c') },
  { value: 'public_guild_messages', label: t('protocols.qqIntents.publicGuildMessages') },
  { value: 'guilds', label: t('protocols.qqIntents.guilds') },
  { value: 'guild_members', label: t('protocols.qqIntents.guildMembers') },
  { value: 'guild_messages', label: t('protocols.qqIntents.guildMessages') },
  { value: 'direct_message', label: t('protocols.qqIntents.directMessage') },
]
const sharedFields: { key: keyof ConfigDocument['adapter']; label: string; min: number; max?: number; step: number }[] = [
  { key: 'connect_timeout_seconds', label: t('protocols.connectionDialog.sharedFields.connectTimeoutSeconds'), min: 1, step: 1 },
  { key: 'reconnect_initial_seconds', label: t('protocols.connectionDialog.sharedFields.reconnectInitialSeconds'), min: 1, step: 1 },
  { key: 'reconnect_multiplier', label: t('protocols.connectionDialog.sharedFields.reconnectMultiplier'), min: 1, step: 0.1 },
  { key: 'reconnect_max_seconds', label: t('protocols.connectionDialog.sharedFields.reconnectMaxSeconds'), min: 1, step: 1 },
  { key: 'reconnect_jitter_ratio', label: t('protocols.connectionDialog.sharedFields.reconnectJitterRatio'), min: 0, max: 1, step: 0.05 },
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
        error.value = t('protocols.connectionDialog.missingConnection')
        return
      }
      baseline.value = copy(instance)
      draft.value = copy(instance)
      initialDraft.value = JSON.stringify(draft.value)
      void adaptersStore.refresh().catch(() => undefined)
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
      if (typeof value !== 'number' || !Number.isFinite(value) || value < min || (max !== undefined && value > max) || (step === 1 && !Number.isInteger(value))) fieldErrors.value[key] = t('protocols.connectionDialog.invalidNumber')
    }
  }
  if (Object.keys(fieldErrors.value).length) {
    error.value = t('protocols.connectionDialog.checkFields')
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
        if (JSON.stringify(configStore.document.adapter) !== initialShared.value) throw new Error(t('protocols.connectionDialog.sharedPolicyChanged'))
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
  <AppDialog :open="open" :title="draft ? t(isEditing ? 'protocols.connectionDialog.titleConfigure' : 'protocols.connectionDialog.titleAddProtocol', { protocol: protocolName }) : t('protocols.connectionDialog.titleAdd')" :width="640" :busy="saving" fallback-focus="[data-testid=adapter-add]" @close="requestClose" @after-close="$emit('afterClose')">
    <div v-if="loading" class="dialog-loading" role="status" :aria-label="t('protocols.connectionDialog.loading')"><Skeleton class="h-5 w-2/3" /><Skeleton class="h-10 w-full" /><Skeleton class="h-5 w-1/2" /><Skeleton class="h-10 w-full" /></div>
    <div v-else>
      <AppAlert v-if="error" tone="danger" :title="error" class="dialog-error" role="alert">
        <template v-if="!draft" #action><AppButton size="sm" :loading="loading" @click="load">{{ t('protocols.retry') }}</AppButton></template>
      </AppAlert>
      <div v-if="!draft && !loading && !error" class="protocol-picker">
        <p class="dialog-description">{{ t('protocols.connectionDialog.choose') }}</p>
        <button v-for="protocol in adaptersStore.availableProtocols" :key="protocol.protocol" type="button" class="protocol-choice" :data-testid="`adapter-select-${protocol.protocol}`" @click="selectProtocol(protocol.protocol)">
          <span><strong>{{ protocol.display_name }}</strong><small>{{ protocol.description }}</small></span>
          <ChevronRightIcon />
        </button>
        <p v-if="!adaptersStore.availableProtocols.length" role="status">{{ t('protocols.connectionDialog.noProtocols') }}</p>
      </div>
      <form v-if="draft" ref="formElement" novalidate @submit.prevent="save">
        <fieldset :disabled="saving || loading" class="dialog-fields">
          <p class="dialog-description">{{ isEditing ? t('protocols.connectionDialog.instanceId', { id: draft.id }) : t('protocols.connectionDialog.addDescription') }}</p>
          <template v-if="draft.qqofficial">
            <AppField floating label="AppID" for="adapter-app-id" :required="draft.enabled" :error="fieldErrors.app_id">
              <AppInput id="adapter-app-id" v-model="draft.qqofficial.app_id" :placeholder="t('protocols.connectionDialog.appIdPlaceholder')" inputmode="numeric" autocomplete="off" />
            </AppField>
            <AppField floating label="AppSecret" for="adapter-app-secret" :required="draft.enabled" :error="fieldErrors.app_secret">
              <AppInput type="password" id="adapter-app-secret" v-model="draft.qqofficial.app_secret" :placeholder="t('protocols.connectionDialog.appSecretPlaceholder')" autocomplete="new-password" />
              <p class="field-hint">{{ draft.qqofficial.app_secret === '********' ? t('protocols.connectionDialog.appSecretSaved') : t('protocols.connectionDialog.appSecretHint') }}</p>
            </AppField>
            <AppField :label="t('protocols.connectionDialog.intentsLabel')" for="adapter-intents">
              <AppSelect id="adapter-intents" v-model="draft.qqofficial.intents" :multiple="true" :options="intentOptions" :placeholder="t('protocols.connectionDialog.intentsPlaceholder')" />
              <p class="field-hint">{{ t('protocols.connectionDialog.intentsHint') }}</p>
            </AppField>
          </template>
          <OneBotConnectionFields v-if="draft.onebot11" v-model="draft.onebot11" :adapter-id="draft.id" :errors="fieldErrors" />
          <div class="enable-connection">
            <div><label for="adapter-enabled">{{ t('protocols.connectionDialog.enable') }}</label><p class="field-hint">{{ t('protocols.connectionDialog.enableHint') }}</p></div>
            <AppSwitch id="adapter-enabled" v-model="draft.enabled" :aria-label="t('protocols.connectionDialog.enable')" />
          </div>
          <details class="dialog-disclosure" :open="advancedOpen" @toggle="advancedOpen = ($event.target as HTMLDetailsElement).open">
            <summary>{{ t('protocols.connectionDialog.advanced') }}<span>{{ draft.qqofficial ? t('protocols.connectionDialog.advancedQQ') : t('protocols.connectionDialog.advancedOneBot') }}</span></summary>
            <div class="disclosure-content">
              <AppField floating :label="t('protocols.connectionDialog.instanceField')" for="adapter-id" :error="fieldErrors.id">
                <AppInput id="adapter-id" v-model="draft.id" :disabled="isEditing" :maxlength="64" />
                <p class="field-hint">{{ isEditing ? t('protocols.connectionDialog.instanceFixed') : t('protocols.connectionDialog.instanceGenerated') }}</p>
              </AppField>
              <AppCheckbox v-if="draft.qqofficial" v-model="draft.qqofficial.sandbox">{{ t('protocols.connectionDialog.sandbox') }}</AppCheckbox>
              <p v-if="draft.qqofficial" class="field-hint">{{ t('protocols.connectionDialog.sandboxHint') }}</p>
              <template v-if="draft.onebot11 && sharedDraft">
                <h3>{{ t('protocols.connectionDialog.sharedPolicyTitle') }}</h3>
                <p class="field-hint">{{ t('protocols.connectionDialog.sharedPolicyHint') }}</p>
                <div class="shared-fields">
                  <AppField floating v-for="field in sharedFields" :key="field.key" :label="field.label" :for="`adapter-${field.key}`" :error="fieldErrors[field.key]">
                    <AppNumberInput :id="`adapter-${field.key}`" v-model="sharedDraft[field.key]" :min="field.min" :max="field.max" :step="field.step" />
                  </AppField>
                </div>
              </template>
            </div>
          </details>
          <details v-if="isEditing" class="dialog-disclosure">
            <summary>{{ t('protocols.connectionDialog.runtimeTitle') }}</summary>
            <div class="disclosure-content">
              <p>{{ descriptor?.summary || t('protocols.connectionDialog.runtimeUnknown') }}</p>
              <p v-if="descriptor?.identity" class="field-hint">{{ t('protocols.connectionDialog.identity', { name: descriptor.identity.name || descriptor.identity.id }) }}</p>
              <AppAlert v-if="adaptersStore.error" tone="warning" :title="adaptersStore.error" />
              <ul v-if="runtimeSnapshot" class="runtime-list">
                <li v-for="transport in runtimeSnapshot.transport_status" :key="transport.transport">
                  <strong>{{ transport.transport }}</strong> · {{ transport.summary }}
                  <small>{{ [transport.app_name, transport.app_version, transport.nickname, transport.user_id].filter(Boolean).join(' · ') }}</small>
                </li>
                <li v-for="issue in runtimeSnapshot.recent_transport_issues" :key="issue.code">{{ issue.code }} · {{ issue.summary }}</li>
              </ul>
              <AppButton size="sm" @click="openLogs">{{ draft.type === 'onebot11' ? t('protocols.connectionDialog.protocolLogs') : t('protocols.connectionDialog.logs') }}</AppButton>
            </div>
          </details>
        </fieldset>
      </form>
    </div>
    <template #footer>
      <div class="dialog-footer">
        <AppButton v-if="draft && !isEditing" variant="ghost" :disabled="saving" @click="changeProtocol"><ArrowLeftIcon />{{ t('protocols.connectionDialog.changeProtocol') }}</AppButton>
        <span v-else class="footer-spacer" />
        <AppButton :disabled="saving" @click="requestClose">{{ t('protocols.connectionDialog.cancel') }}</AppButton>
        <AppButton v-if="draft" variant="default" :loading="saving" :disabled="loading || configStore.saving || (isEditing && !dirty)" data-testid="adapter-save" @click="save">{{ isEditing ? t('protocols.connectionDialog.saveChanges') : t('protocols.connectionDialog.saveConnection') }}</AppButton>
      </div>
      <p v-if="draft && !isEditing" class="save-hint">{{ t('protocols.connectionDialog.restartHint') }}</p>
    </template>
  </AppDialog>
  <AppConfirmDialog :open="confirmOpen" :title="t('protocols.connectionDialog.discardTitle')" :description="t('protocols.connectionDialog.discardDescription')" :confirm-text="t('protocols.connectionDialog.discardConfirm')" :cancel-text="t('protocols.connectionDialog.discardCancel')" danger @confirm="finishConfirmation(true)" @cancel="finishConfirmation(false)" />
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
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
@media (max-width: #{bp.$smallPhone}) {
  .dialog-disclosure summary span { display: none; }
  .shared-fields { grid-template-columns: 1fr; }
}
</style>
