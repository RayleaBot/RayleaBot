<script setup lang="ts">
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { matchedRouteKey, onBeforeRouteLeave } from 'vue-router'
import { storeToRefs } from 'pinia'
import { CollapsibleContent, CollapsibleRoot, CollapsibleTrigger, TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
import { ChevronRightIcon, CircleAlertIcon, CpuIcon, DatabaseIcon, ImageIcon, RotateCcwIcon, SaveIcon, ScrollTextIcon, SearchIcon, ShieldCheckIcon, UsersRoundIcon } from '@lucide/vue'
import AppPage from '@/components/page/AppPage.vue'
import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import ConfigFieldRow from '@/components/config/ConfigFieldRow.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { notifySuccess } from '@/adapter/feedback'
import { cloneConfig, setValueByPath, type ConfigFieldDefinition } from '@/lib/config-form'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import { finishActiveViewTransition } from '@/motion/runtime'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'
import { changedConfigPaths, getConfigWorkbenchGroups, getWorkbenchSections, matchesConfigField, readConfigField, rebaseConfigDraft, type ConfigWorkbenchGroup } from './config-workbench'

const configStore = useConfigStore()
const { document: configDocument, error, loading, restartRequired, saving } = storeToRefs(configStore)
const groups = getConfigWorkbenchGroups()
const categoryIcons = { access: ShieldCheckIcon, render: ImageIcon, accounts: UsersRoundIcon, runtime: CpuIcon, data: DatabaseIcon, logs: ScrollTextIcon }
const fields = groups.flatMap(group => group.sections.flatMap(section => section.fields))
const initialSection = typeof window === 'undefined' ? '' : window.location.hash.replace('#config-section-', '')
const initialGroup = groups.find(group => group.sections.some(section => section.key === initialSection))
const selectedKey = ref(initialGroup?.key ?? 'access')
const expanded = ref<Record<string, boolean>>(initialGroup ? { [initialGroup.key]: true } : {})
const query = ref('')
const draft = ref<ConfigDocument | null>(null)
const baseline = ref<ConfigDocument | null>(null)
const conflictPaths = ref<string[]>([])
const saveError = ref('')
const loadError = ref('')
const savePending = ref(false)
const editorTitle = ref<HTMLElement | null>(null)
const saveControl = ref<InstanceType<typeof AppButton> | null>(null)
const isSaving = computed(() => saving.value || savePending.value)
const changedPaths = computed(() => draft.value && baseline.value ? changedConfigPaths(baseline.value, draft.value, fields) : [])
const isDirty = computed(() => changedPaths.value.length > 0)
const restartPending = computed(() => fields.some(field => field.restartRequired && changedPaths.value.includes(field.path)))
const conflictMessage = computed(() => {
  const names = conflictPaths.value.filter(path => changedPaths.value.includes(path)).map(path => fields.find(field => field.path === path)?.label ?? path)
  return names.length ? t('config.workbench.conflict', { fields: names.join('、') }) : ''
})

watch(configDocument, incoming => {
  if (!incoming) return
  if (baseline.value && draft.value) {
    const rebased = rebaseConfigDraft(baseline.value, draft.value, incoming, fields)
    draft.value = rebased.draft
    conflictPaths.value = [...new Set([...conflictPaths.value, ...rebased.conflicts])]
  } else {
    draft.value = cloneConfig(incoming)
  }
  baseline.value = cloneConfig(incoming)
}, { immediate: true })

function matchCount(group: ConfigWorkbenchGroup) {
  return group.sections.reduce((total, section) => total + section.fields.filter(field => matchesConfigField(group, section, field, query.value)).length, 0)
}
function groupChanged(group: ConfigWorkbenchGroup) {
  return group.sections.some(section => section.fields.some(field => changedPaths.value.includes(field.path)))
}
const visibleGroups = computed(() => groups.filter(group => matchCount(group) > 0))
const activeGroup = computed(() => visibleGroups.value.find(group => group.key === selectedKey.value) ?? visibleGroups.value[0])
const commonSections = computed(() => activeGroup.value ? getWorkbenchSections(activeGroup.value, false, query.value) : [])
const advancedSections = computed(() => activeGroup.value ? getWorkbenchSections(activeGroup.value, true, query.value) : [])
const advancedOpen = computed(() => Boolean(query.value.trim()) || Boolean(activeGroup.value && expanded.value[activeGroup.value.key]))
const categoryOptions = computed(() => visibleGroups.value.map(group => ({ value: group.key, label: group.title })))
const resultCount = computed(() => visibleGroups.value.reduce((total, group) => total + matchCount(group), 0))
const statusLabel = computed(() => {
  if (isSaving.value) return t('config.workbench.saving')
  if (saveError.value && isDirty.value) return t('config.workbench.retrySave')
  if (isDirty.value) return t('config.workbench.changed', { count: changedPaths.value.length })
  if (restartRequired.value !== null) return t(restartRequired.value ? 'config.restartNeeded' : 'config.hotApplied')
  return t('config.saveIdle')
})
const saveState = computed(() => isSaving.value ? 'saving' : isDirty.value ? saveError.value ? 'error' : 'dirty' : 'idle')
const saveDescription = computed(() => [statusLabel.value, saveError.value && isDirty.value ? t('config.workbench.changed', { count: changedPaths.value.length }) : '', isDirty.value && restartPending.value ? t('config.workbench.requiresRestart') : ''].filter(Boolean).join('；'))

async function loadConfig() {
  loadError.value = ''
  try { await configStore.fetchConfig() } catch (failure) {
    loadError.value = error.value || getDisplayErrorMessage(failure, 'errors.common.loadFailed')
  }
}
async function changeGroup(value: string | number) {
  selectedKey.value = String(value)
  await nextTick()
  editorTitle.value?.scrollIntoView?.({ block: 'nearest' })
}
function setAdvancedOpen(open: boolean) {
  if (!query.value.trim() && activeGroup.value) expanded.value[activeGroup.value.key] = open
}
function readField(path: string, type: ConfigFieldDefinition['type']) {
  return draft.value ? readConfigField(draft.value, path) : type === 'boolean' ? false : type === 'number' ? null : ''
}
function writeField(path: string, value: unknown) {
  if (!draft.value || isSaving.value) return
  setValueByPath(draft.value as unknown as Record<string, unknown>, path, value)
  saveError.value = ''
}
function discard() {
  if (!baseline.value || isSaving.value) return
  draft.value = cloneConfig(baseline.value)
  conflictPaths.value = []
  saveError.value = ''
}
async function discardFromFloatingButton() {
  discard()
  await nextTick()
  saveControl.value?.$el?.focus()
}
async function save() {
  if (!draft.value || !isDirty.value || isSaving.value) return
  savePending.value = true
  saveError.value = ''
  try {
    const response = await configStore.saveConfig(cloneConfig(draft.value))
    baseline.value = cloneConfig(response.config)
    draft.value = cloneConfig(response.config)
    conflictPaths.value = []
    notifySuccess(t(response.restart_required ? 'config.saveRestart' : 'config.saveSuccess'))
  } catch (failure) {
    saveError.value = getDisplayErrorMessage(failure)
  } finally {
    savePending.value = false
  }
}

const leaveConfirmation = ref(false)
let resolveLeave: ((allowed: boolean) => void) | null = null
let pendingLeave: Promise<boolean> | null = null
function finishLeave(allowed: boolean) {
  if (allowed) discard()
  leaveConfirmation.value = false
  resolveLeave?.(allowed)
  resolveLeave = null
}
function canLeave() {
  if (isSaving.value) return false
  if (!isDirty.value) return true
  // A navigation snapshot must not freeze the page while a guard awaits input.
  finishActiveViewTransition()
  if (!pendingLeave) pendingLeave = new Promise<boolean>(resolve => {
    resolveLeave = resolve
    leaveConfirmation.value = true
  }).finally(() => { pendingLeave = null })
  return pendingLeave
}
if (inject(matchedRouteKey, undefined)) {
  onBeforeRouteLeave(to => to.name === 'login' || to.name === 'setup' ? true : canLeave())
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (isDirty.value || isSaving.value) { event.preventDefault(); event.returnValue = '' }
}
onMounted(() => {
  void loadConfig()
  window.addEventListener('beforeunload', beforeUnload)
})
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', beforeUnload)
  finishLeave(false)
})
</script>

<template>
  <AppPage :title="t('config.title')" :description="t('config.workbench.description')" class="config-workbench-page">
    <RetryPanel v-if="error && !draft" :title="t('config.title')" :description="error" :loading="loading" @retry="loadConfig" />
    <AppSkeletonCard v-else-if="loading && !draft" show-header :rows="7" />
    <div v-else-if="draft" class="config-page">
      <TabsRoot class="config-workbench" :model-value="activeGroup?.key ?? selectedKey" orientation="vertical" @update:model-value="changeGroup(String($event))">
        <aside class="config-navigation">
          <AppInput v-model="query" type="search" allow-clear :placeholder="t('config.workbench.search')" :aria-label="t('config.workbench.search')" class="config-search">
            <template #prefix><SearchIcon :size="17" /></template>
          </AppInput>
          <TabsList class="config-categories" :aria-label="t('config.workbench.categories')">
            <TabsTrigger v-for="group in visibleGroups" :key="group.key" :value="group.key" class="config-category" :data-category="group.key">
              <span class="config-category__label">
                <component :is="categoryIcons[group.key as keyof typeof categoryIcons]" :size="18" aria-hidden="true" class="config-category__icon" />
                <span>{{ group.title }}</span>
              </span>
              <span v-if="groupChanged(group)" class="config-category__status">{{ t('config.workbench.edited') }}</span>
              <span v-else-if="query.trim()" class="config-category__status">{{ matchCount(group) }}</span>
            </TabsTrigger>
          </TabsList>
          <div v-if="activeGroup" class="config-category-select">
            <AppSelect :model-value="activeGroup.key" :options="categoryOptions" :aria-label="t('config.workbench.categories')" @update:model-value="changeGroup" />
          </div>
          <p v-if="query.trim()" class="config-search-status" role="status">{{ t('config.workbench.results', { count: resultCount }) }}</p>
        </aside>

        <div class="config-editor">
          <div v-if="saveError || conflictMessage || loadError" class="config-editor__feedback">
            <AppAlert v-if="saveError" tone="danger" :title="saveError" />
            <AppAlert v-if="conflictMessage" tone="warning" :title="conflictMessage" />
            <AppAlert v-if="loadError && !saveError" tone="warning" :title="loadError">
              <template #action><AppButton size="sm" :loading="loading" @click="loadConfig">{{ t('config.workbench.reload') }}</AppButton></template>
            </AppAlert>
          </div>
          <TabsContent v-if="activeGroup" :key="activeGroup.key" :value="activeGroup.key" class="config-editor__body" :aria-labelledby="`config-group-${activeGroup.key}`">
            <header class="config-editor__header">
              <h2 :id="`config-group-${activeGroup.key}`" ref="editorTitle">{{ activeGroup.title }}</h2>
              <p>{{ activeGroup.description }}</p>
            </header>

            <div v-if="commonSections.length" class="config-common">
              <h3 v-if="activeGroup.sections.length === 1" class="config-section-heading">{{ t('config.workbench.common') }}</h3>
              <section v-for="section in commonSections" :id="`config-section-${section.key}`" :key="section.key" class="config-section" :data-section-key="section.key">
                <h3 v-if="activeGroup.sections.length > 1" class="config-section-heading">{{ section.title }}</h3>
                <fieldset class="config-fields" :disabled="isSaving" :aria-label="section.title">
                  <ConfigFieldRow v-for="field in section.fields" :key="field.path" layout="row" :field="field" :value="readField(field.path, field.type)" :disabled="isSaving" @update:value="value => writeField(field.path, value)" />
                </fieldset>
              </section>
            </div>

            <CollapsibleRoot v-if="advancedSections.length" class="config-advanced" :open="advancedOpen" @update:open="setAdvancedOpen">
              <CollapsibleTrigger v-if="!query.trim()" class="config-advanced__trigger">
                <ChevronRightIcon :size="18" class="config-advanced__chevron" aria-hidden="true" />
                <span><strong>{{ activeGroup.advancedTitle }}</strong><span class="config-advanced__description">{{ activeGroup.advancedDescription }}</span></span>
              </CollapsibleTrigger>
              <h3 v-else class="config-section-heading">{{ t('config.workbench.advanced') }}</h3>
              <CollapsibleContent class="config-advanced__content">
                <section v-for="section in advancedSections" :id="`config-section-${section.key}${commonSections.some(common => common.key === section.key) ? '-advanced' : ''}`" :key="section.key" class="config-section" :data-section-key="section.key">
                  <h3 v-if="activeGroup.sections.length > 1" class="config-section-heading">{{ section.title }}</h3>
                  <fieldset class="config-fields" :disabled="isSaving" :aria-label="section.title">
                    <ConfigFieldRow v-for="field in section.fields" :key="field.path" layout="row" :field="field" :value="readField(field.path, field.type)" :disabled="isSaving" @update:value="value => writeField(field.path, value)" />
                  </fieldset>
                </section>
              </CollapsibleContent>
            </CollapsibleRoot>
          </TabsContent>
          <div v-else class="config-editor__empty">
            <AppEmptyState :title="t('config.workbench.noResults')" :description="t('config.workbench.searchHint')" />
            <AppButton @click="query = ''">{{ t('config.workbench.clearSearch') }}</AppButton>
          </div>

        </div>
      </TabsRoot>
      <div class="config-save-anchor">
        <span id="config-save-status" class="sr-only" role="status">{{ saveDescription }}</span>
        <AppTooltip v-if="isDirty" :title="t('config.workbench.discardAll')">
          <AppButton class="config-discard-fab" variant="ghost" size="icon" data-testid="config-discard" :aria-label="t('config.workbench.discard')" :disabled="isSaving" @click="discardFromFloatingButton">
            <template #icon><RotateCcwIcon :size="20" aria-hidden="true" /></template>
          </AppButton>
        </AppTooltip>
        <AppTooltip :title="saveDescription">
          <AppButton ref="saveControl" class="config-save-fab" variant="ghost" size="icon" :data-state="saveState" data-testid="config-save" :aria-label="t('config.save')" aria-describedby="config-save-status" :aria-disabled="!isDirty || isSaving" :loading="isSaving" @click="save">
            <template #icon>
              <CircleAlertIcon v-if="saveState === 'error'" :size="22" aria-hidden="true" />
              <SaveIcon v-else :size="22" aria-hidden="true" />
            </template>
            <span v-if="isDirty && !isSaving" class="config-save-fab__count" aria-hidden="true">{{ changedPaths.length }}</span>
          </AppButton>
        </AppTooltip>
      </div>
    </div>
  </AppPage>
  <AppConfirmDialog :open="leaveConfirmation" :title="t('config.workbench.leaveTitle')" :description="t('config.workbench.leaveDescription')" :confirm-text="t('config.workbench.leaveConfirm')" :cancel-text="t('config.workbench.leaveCancel')" danger @confirm="finishLeave(true)" @cancel="finishLeave(false)" />
</template>

<style scoped lang="scss">
.config-workbench-page { width: 100%; max-width: 1120px; margin-inline: auto; }
.config-page { container-type: inline-size; container-name: config-workbench; }
.config-workbench { display: grid; grid-template-columns: 200px minmax(0, 1fr); align-items: start; gap: 24px; }
.config-navigation { position: sticky; top: 12px; display: grid; gap: 18px; min-width: 0; }
.config-categories { display: grid; gap: 4px; }
.config-category { display: flex; align-items: center; justify-content: space-between; gap: 8px; width: 100%; min-height: 40px; padding: 10px 12px; border-radius: 8px; color: var(--muted); font-size: 14px; line-height: 1.5; text-align: left; cursor: pointer; }
.config-category:hover { color: var(--text); background: var(--surface-soft); }
.config-category[data-state=active] { color: var(--brand-foreground); background: var(--surface-soft); font-weight: 600; }
.config-category__label { display: flex; align-items: center; gap: 10px; min-width: 0; }
.config-category__icon { flex: none; stroke-width: 1.75; }
.config-category__status { flex: none; color: var(--brand-foreground); font-size: 11px; font-weight: 500; }
.config-category-select { display: none; }
.config-search-status { margin: 0; color: var(--muted); font-size: 12px; line-height: 1.5; }
.config-editor { display: flex; flex-direction: column; min-width: 0; min-height: 520px; border-radius: 12px; background: var(--surface-strong); container-type: inline-size; container-name: config-editor; }
.config-editor__body { flex: 1; padding: 24px 28px; min-width: 0; animation: config-section-enter 160ms ease-out; }
.config-editor__header { margin-bottom: 26px; }
.config-editor__header h2 { margin: 0; color: var(--text); font-size: 20px; font-weight: 600; line-height: 1.4; }
.config-editor__header p { margin: 8px 0 0; color: var(--muted); font-size: 13px; line-height: 1.65; }
.config-section + .config-section { margin-top: 24px; }
.config-section-heading { margin: 0 0 2px; font-size: 14px; font-weight: 600; line-height: 1.6; color: var(--text); }
.config-fields { min-width: 0; padding: 0; margin: 0; border: 0; }
.config-editor__feedback { display: grid; gap: 12px; padding: 20px 20px 0; }
.config-advanced { margin-top: 20px; border-top: 1px solid var(--border); }
.config-advanced__trigger { display: flex; align-items: center; gap: 14px; width: 100%; padding: 20px 0; border-radius: 6px; text-align: left; cursor: pointer; color: var(--text); }
.config-advanced__trigger:hover { color: var(--brand-foreground); }
.config-advanced__trigger strong { display: block; font-size: 14px; font-weight: 500; }
.config-advanced__description { display: block; margin-top: 4px; color: var(--muted); font-size: 13px; line-height: 1.6; }
.config-advanced__chevron { flex: none; transition: transform 160ms ease; }
.config-advanced__trigger[data-state=open] .config-advanced__chevron { transform: rotate(90deg); }
.config-advanced__content { padding-bottom: 4px; }
.config-advanced > h3 { margin-top: 20px; }
.config-editor__empty { display: grid; flex: 1; place-content: center; justify-items: center; gap: 16px; padding: 32px 24px; }
.config-save-anchor { position: sticky; bottom: max(20px, env(safe-area-inset-bottom)); z-index: 10; display: flex; align-items: center; justify-content: flex-end; gap: 12px; margin-top: 20px; padding: 0 4px 4px; pointer-events: none; }
.config-discard-fab { width: 48px; height: 48px; border: 0; border-radius: 16px; background: var(--surface-strong); color: var(--muted); box-shadow: var(--shadow-sm); pointer-events: auto; }
.config-discard-fab:hover { background: var(--surface-soft); color: var(--text); }
.config-discard-fab :deep(svg) { width: 20px; height: 20px; }
.config-save-fab { position: relative; width: 56px; height: 56px; min-height: 56px; border: 0; border-radius: 18px; background: var(--surface-strong); color: var(--muted); box-shadow: var(--shadow-floating); pointer-events: auto; transition: background-color 180ms ease, color 180ms ease, transform 180ms cubic-bezier(.16, 1, .3, 1); }
.config-save-fab :deep(svg) { width: 22px; height: 22px; }
.config-save-fab[data-state=idle] { cursor: default; }
.config-save-fab[data-state=idle]:hover { background: var(--surface-strong); color: var(--text); }
.config-save-fab[data-state=dirty], .config-save-fab[data-state=saving] { background: var(--attention); color: var(--surface-strong); --focus: var(--surface-strong); }
.config-save-fab[data-state=dirty]:hover { background: var(--attention); color: var(--surface-strong); transform: translateY(-2px); }
.config-save-fab[data-state=error] { background: var(--danger); color: var(--surface-strong); --focus: var(--surface-strong); }
.config-save-fab:disabled { opacity: 1; }
.config-save-fab__count { position: absolute; top: -5px; right: -5px; display: grid; place-items: center; min-width: 22px; height: 22px; padding-inline: 5px; border-radius: 11px; background: var(--surface-strong); color: var(--text-attention); box-shadow: var(--shadow-sm); font-size: 11px; font-weight: 600; font-variant-numeric: tabular-nums; }
:global([data-density=compact]) .config-navigation { gap: 12px; }
:global([data-density=compact]) .config-category { min-height: 36px; padding-block: 8px; }
:global([data-density=compact]) .config-editor { min-height: 460px; }
:global([data-density=compact]) .config-editor__body { padding: 20px 24px; }
:global([data-density=compact]) .config-editor__header { margin-bottom: 20px; }
:global([data-density=compact]) .config-advanced__trigger { padding-block: 16px; }
@container config-workbench (max-width: 760px) {
  .config-workbench { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .config-navigation { position: static; gap: 10px; }
  .config-categories { display: none; }
  .config-category-select { display: block; }
}
@container config-editor (max-width: 560px) {
  .config-editor__body { padding: 20px 16px; }
}
@media (max-width: 639px), (pointer: coarse) { .config-category { min-height: 44px; } }
@keyframes config-section-enter { from { opacity: .88; } to { opacity: 1; } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) {
  .config-editor__body { animation: none; }
  .config-advanced__chevron { transition: none; }
  .config-save-fab { transition: none; }
  .config-save-fab[data-state=dirty]:hover { transform: none; }
}
@media (forced-colors: active) { .config-editor { border: 1px solid CanvasText; } .config-category[data-state=active] { outline: 1px solid Highlight; outline-offset: -1px; } }
@media (forced-colors: active) { .config-save-fab { border: 1px solid ButtonText; } .config-save-fab[data-state=dirty], .config-save-fab[data-state=error] { border: 2px solid Highlight; } .config-save-fab__count { border: 1px solid CanvasText; } }
@media (forced-colors: active) { .config-discard-fab { border: 1px solid ButtonText; } }
</style>
