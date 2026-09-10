<script setup lang="ts">
import AppCollectionPagination from '@/components/AppCollectionPagination.vue'
import { ApiError } from '@/lib/http'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppTag from '@/components/AppTag.vue'
import AppButton from '@/components/AppButton.vue'
import AppInput from '@/components/AppInput.vue'
import AppTabs from '@/components/AppTabs.vue'
import { ChevronDownIcon, FileImageIcon, RefreshCwIcon, SearchIcon } from '@lucide/vue'
import { computed, onActivated, onDeactivated, onMounted, ref, useId, watch } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { useToastFeedback } from '@/adapter/feedback'
import TemplatePreviewFrame from '@/components/TemplatePreviewFrame.vue'
import { formatDateTime } from '@/lib/format'
import {
  buildRenderTemplateSchemaNodes,
} from '@/lib/render-template-editor'
import { t } from '@/i18n'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { usePluginsStore } from '@/stores/plugins'
import { getRenderTemplateTypeLabel } from '@/lib/render-template-display'
import type { RenderTemplateSummary } from '@/types/api'
import { useTemplatePreview } from './useTemplatePreview'

const route = useRoute()
const router = useRouter()
const renderTemplatesStore = useRenderTemplatesStore()
const pluginsStore = usePluginsStore()
const search = ref('')
const compact = useMediaQuery('(max-width: 767px)')
const catalogOpen = ref(false)
const catalogId = useId()
const workspaceTab = ref('preview')
const removedTemplateNotice = ref(false)
const workspaceTabs = computed(() => [
  { value: 'preview', label: t('renderTemplates.previewTab') },
  { value: 'data', label: t('renderTemplates.sampleTab') },
  { value: 'info', label: t('renderTemplates.infoTab') },
])
const { detailById, error, items, loading, workspaceLoading, total, nextCursor, loadingMore } = storeToRefs(renderTemplatesStore)

const hasRequestedList = ref(false)
const pageActive = ref(true)

const isTemplateRoute = computed(() => route.name === 'render-templates')
const isActiveTemplateRoute = computed(() => pageActive.value && isTemplateRoute.value)

const activeTemplateId = computed(() => (
  isTemplateRoute.value && typeof route.params.templateId === 'string' && route.params.templateId
    ? route.params.templateId
    : ''
))

const { currentTemplate, currentPreviewDataText, previewParseResult, currentPreviewDocument,
  currentPreviewError, currentPreviewPending, ensurePreviewDefaults, resetPreviewData,
  scheduleAutoPreview, resetPreviewCaches, invalidateCurrentPreview } = useTemplatePreview(activeTemplateId, isActiveTemplateRoute)

const groupedTemplates = computed(() => {
  const groups = new Map<string, { key: string; title: string; items: RenderTemplateSummary[] }>()
  for (const template of items.value) {
    const title = getTemplateSourceLabel(template)
    const key = template.source.type === 'system' ? 'system' : template.source.plugin_id || 'plugin'
    if (!groups.has(key)) groups.set(key, { key, title, items: [] })
    groups.get(key)!.items.push(template)
  }
  return [...groups.values()].sort((a, b) => a.key === 'system' ? -1 : b.key === 'system' ? 1 : a.title.localeCompare(b.title, 'zh-CN'))
    .map(group => ({ ...group, items: [...group.items].sort((a, b) => a.name.localeCompare(b.name, 'zh-CN')) }))
})

const schemaNodes = computed(() => buildRenderTemplateSchemaNodes(currentTemplate.value?.input_schema_json ?? null))
const displaySchemaNodes = computed(() => schemaNodes.value.filter((node) => node.depth > 0))

const pageErrorToast = computed(() => (
  error.value && items.value.length > 0
    ? {
        key: `render-templates-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))
const previewParseIssueToast = computed(() => (
  previewParseResult.value.issue
    ? {
        key: `render-templates-preview-parse:${previewParseResult.value.issue.message}`,
        level: 'warning' as const,
        message: previewParseResult.value.issue.message,
      }
    : null
))
const previewErrorToast = computed(() => (
  currentPreviewError.value
    ? {
        key: `render-templates-preview-error:${activeTemplateId.value}:${currentPreviewError.value}`,
        level: 'error' as const,
        message: currentPreviewError.value,
      }
    : null
))

useToastFeedback(pageErrorToast)
useToastFeedback(previewParseIssueToast)
useToastFeedback(previewErrorToast)

const previewEmptyDescription = computed(() => {
  if (previewParseResult.value.issue) {
    return previewParseResult.value.issue.message
  }
  if (currentPreviewError.value) {
    return currentPreviewError.value
  }
  if (currentPreviewPending.value) {
    return t('renderTemplates.previewPending')
  }
  return t('renderTemplates.previewEmpty')
})

function formatTemplateSize(width?: number, height?: number) {
  if (!width || !height) {
    return t('display.empty')
  }

  return `宽度 ${width}px · 高度自适应（初始 ${height}px）`
}

function getTemplateSourceLabel(template: RenderTemplateSummary) {
  if (template.source.type !== 'plugin') {
    return t('renderTemplates.sources.system')
  }

  const id = template.source.plugin_id
  if (!id) return t('renderTemplates.sources.plugin')
  return pluginsStore.getPluginDisplayName(id)
}

function getTemplateLocalId(template: RenderTemplateSummary) {
  if (template.source.type !== 'plugin') {
    return ''
  }

  return template.source.local_id || ''
}

async function loadTemplateList() {
  const refreshing = hasRequestedList.value
  if (refreshing) resetPreviewCaches()
  hasRequestedList.value = true
  try {
    await Promise.all([renderTemplatesStore.fetchTemplates({ query: search.value }), pluginsStore.ensureList().catch(() => undefined)])
    if (refreshing && activeTemplateId.value && !items.value.some(item => item.id === activeTemplateId.value)) await loadTemplateWorkspace(activeTemplateId.value, { force: true })
  } catch {
    // store error state drives the page
  }
}

async function loadTemplateWorkspace(templateId: string, options: { force?: boolean } = {}) {
  if (!options.force && detailById.value[templateId]) {
    renderTemplatesStore.clearError()
    return true
  }

  try {
    await renderTemplatesStore.fetchTemplateWorkspace(templateId)
    return true
  } catch (cause) {
    return cause instanceof ApiError && cause.status === 404 ? false : null
  }
}

async function reloadCurrentTemplate() {
  if (!activeTemplateId.value) {
    return
  }

  invalidateCurrentPreview()
  await loadTemplateWorkspace(activeTemplateId.value, { force: true })
  scheduleAutoPreview({ immediate: true })
}

async function syncRouteTemplate() {
  if (!isActiveTemplateRoute.value) return
  if (activeTemplateId.value) {
    const exists = await loadTemplateWorkspace(activeTemplateId.value)
    if (exists !== false) { ensurePreviewDefaults(activeTemplateId.value); return }
    removedTemplateNotice.value = true
  }
  const fallback = items.value.find(item => item.id === 'help.menu')?.id || items.value[0]?.id
  if (fallback && fallback !== activeTemplateId.value) await router.replace({ name: 'render-templates', params: { templateId: fallback } })
}

watch(search, () => { void renderTemplatesStore.fetchTemplates({ query: search.value }).catch(() => undefined) })

watch([items, currentTemplate], () => {
  const visible = [...items.value, ...(currentTemplate.value ? [currentTemplate.value] : [])]
  const owners = new Set(visible.map(item => item.source.plugin_id).filter((id): id is string => Boolean(id)))
  for (const id of owners) {
    if (pluginsStore.getPluginDisplayName(id) === id) void pluginsStore.ensureDetail(id).catch(() => undefined)
  }
})

async function selectTemplate(templateId: string) {
  if (compact.value) {
    catalogOpen.value = false
    search.value = ''
  }
  if (templateId === activeTemplateId.value) {
    return
  }
  removedTemplateNotice.value = false

  await router.replace({
    name: 'render-templates',
    params: {
      templateId,
    },
  })
}

watch([items, isActiveTemplateRoute, () => route.params.templateId], () => {
  void syncRouteTemplate()
}, { immediate: true })

onMounted(() => {
  void loadTemplateList()
})

onActivated(() => {
  pageActive.value = true
})

onDeactivated(() => {
  pageActive.value = false
})

</script>

<template>
  <AppPage :title="t('renderTemplates.title')" :description="t('renderTemplates.subtitle')" full-height>
    <template #extra>
      <AppButton :disabled="loading" @click="loadTemplateList"><RefreshCwIcon :size="16" />{{ t('renderTemplates.refreshList') }}</AppButton>
    </template>
    <RetryPanel v-if="error && items.length === 0" :title="t('renderTemplates.title')" :description="error" :loading="loading" @retry="loadTemplateList" />
    <AppEmptyState v-else-if="!loading && hasRequestedList && items.length === 0 && !search && !activeTemplateId" icon="box" :title="t('renderTemplates.noTemplates')" :description="t('renderTemplates.catalogHint')" />
    <div v-else class="render-templates-shell">
      <aside class="template-catalog" :aria-label="t('renderTemplates.templateList')">
        <AppButton v-if="compact" class="template-catalog__toggle" :aria-label="t('renderTemplates.chooseTemplate')" :aria-expanded="catalogOpen" :aria-controls="catalogId" @click="catalogOpen = !catalogOpen">
          <span>{{ currentTemplate ? currentTemplate.name : t('renderTemplates.chooseTemplate') }}</span><ChevronDownIcon :size="16" :class="{ 'is-open': catalogOpen }" />
        </AppButton>
        <div :id="catalogId" v-show="!compact || catalogOpen" class="template-catalog__content">
          <div class="template-catalog__search"><SearchIcon :size="16" aria-hidden="true" /><AppInput v-model="search" :maxlength="200" type="search" :aria-label="t('renderTemplates.search')" :placeholder="t('renderTemplates.search')" /></div>
          <p class="template-catalog__hint">{{ t('renderTemplates.catalogHint') }}<span>{{ items.length }}</span></p>
          <div class="template-catalog__list">
            <AppSkeleton v-if="loading && !items.length" :rows="6" />
            <section v-for="group in groupedTemplates" :key="group.key" class="template-nav-group">
              <h2 class="template-nav-group__title">{{ group.title }}<span>{{ group.items.length }}</span></h2>
              <button v-for="template in group.items" :key="template.id" type="button" class="template-nav-item" :class="{ 'is-active': template.id === activeTemplateId }" :aria-current="template.id === activeTemplateId ? 'page' : undefined" :title="template.id" @click="selectTemplate(template.id)">
                <FileImageIcon :size="17" aria-hidden="true" />
                <span><strong>{{ template.name }}</strong><small v-if="template.description">{{ template.description }}</small></span>
              </button>
            </section>
            <div v-if="!loading && !groupedTemplates.length" class="template-catalog__empty"><p>{{ t('renderTemplates.noMatches') }}</p><AppButton variant="ghost" @click="search = ''">{{ t('renderTemplates.clearSearch') }}</AppButton></div>
            <AppCollectionPagination :loaded="items.length" :total="total" :next-cursor="nextCursor" :loading="loadingMore || loading" @more="renderTemplatesStore.loadMore().catch(() => undefined)" />
          </div>
        </div>
      </aside>
      <section class="template-workspace" :aria-label="t('renderTemplates.previewTitle')">
        <p v-if="removedTemplateNotice" class="template-workspace__notice" role="status">{{ t('renderTemplates.removedTemplate') }}</p>
        <header v-if="currentTemplate" class="template-workspace__heading">
          <div><h2>{{ currentTemplate.name }}</h2><p>{{ getTemplateSourceLabel(currentTemplate) }}<span aria-hidden="true"> · </span>{{ currentTemplate.width }} px</p></div>
          <AppTag v-if="currentPreviewPending" tone="info" role="status">{{ t('renderTemplates.previewPending') }}</AppTag>
        </header>
        <AppSkeleton v-else-if="workspaceLoading || loading" :rows="3" />
        <AppTabs v-model="workspaceTab" :items="workspaceTabs" :label="t('renderTemplates.title')" keep-alive class="template-workspace__tabs">
          <template #extra><AppButton variant="ghost" :disabled="!activeTemplateId || workspaceLoading" @click="reloadCurrentTemplate"><RefreshCwIcon :size="16" />{{ t('renderTemplates.reloadAction') }}</AppButton></template>
          <template #preview>
            <div class="render-template-preview-area" data-testid="render-template-preview-result">
              <p class="template-preview-hint">{{ t('renderTemplates.previewHint') }}</p>
              <TemplatePreviewFrame v-if="currentPreviewDocument" :frame-title="currentTemplate ? currentTemplate.name : t('renderTemplates.previewTitle')" :frame-width="currentPreviewDocument.width" :srcdoc="currentPreviewDocument.html" :template-id="currentPreviewDocument.template_id" :payload="currentPreviewDataText" test-id-prefix="render-template-preview" />
              <div v-else class="render-template-preview-empty"><AppEmptyState :description="previewEmptyDescription" /></div>
            </div>
          </template>
          <template #data>
            <div class="template-data-workspace">
              <div class="template-data-workspace__intro"><p>{{ t('renderTemplates.sampleHint') }}</p><AppButton :disabled="!currentTemplate" @click="resetPreviewData">{{ t('renderTemplates.resetSample') }}</AppButton></div>
              <AppTextarea v-model="currentPreviewDataText" class="render-templates-json-input" :aria-label="t('renderTemplates.previewData')" :aria-invalid="previewParseResult.issue ? true : undefined" :placeholder="t('renderTemplates.previewDataPlaceholder')" :rows="18" spellcheck="false" />
              <p class="template-preview-hint">{{ t('renderTemplates.autoPreview') }}</p>
            </div>
          </template>
          <template #info>
            <div v-if="currentTemplate" class="template-information">
              <p v-if="currentTemplate.description" class="template-information__description">{{ currentTemplate.description }}</p>
              <h3>{{ t('renderTemplates.schemaPreviewTitle') }}</h3>
              <p v-if="!displaySchemaNodes.length" class="template-preview-hint">{{ t('renderTemplates.schemaPreviewEmpty') }}</p>
              <div v-else class="schema-tree">
                <div v-for="node in displaySchemaNodes" :key="node.key" class="schema-tree-row" :style="{ '--schema-depth': node.depth }">
                  <div><strong>{{ node.description || node.label }}</strong><code v-if="node.description">{{ node.label }}</code></div>
                  <span>{{ getRenderTemplateTypeLabel(node.type) }}</span><small v-if="node.required">{{ t('renderTemplates.fields.required') }}</small>
                </div>
              </div>
              <details class="template-technical"><summary>{{ t('renderTemplates.technicalDetails') }}</summary>
                <dl class="template-info-list">
                  <div><dt>{{ t('renderTemplates.fields.id') }}</dt><dd>{{ currentTemplate.id }}</dd></div>
                  <div v-if="currentTemplate.source.plugin_id"><dt>{{ t('renderTemplates.fields.source') }}</dt><dd>{{ currentTemplate.source.plugin_id }}</dd></div>
                  <div v-if="getTemplateLocalId(currentTemplate)"><dt>{{ t('renderTemplates.fields.localId') }}</dt><dd>{{ getTemplateLocalId(currentTemplate) }}</dd></div>
                  <div><dt>{{ t('renderTemplates.fields.version') }}</dt><dd>{{ currentTemplate.version }}</dd></div>
                  <div><dt>{{ t('renderTemplates.fields.size') }}</dt><dd>{{ formatTemplateSize(currentTemplate.width, currentTemplate.height) }}</dd></div>
                  <div><dt>{{ t('renderTemplates.fields.updatedAt') }}</dt><dd>{{ formatDateTime(currentTemplate.updated_at) }}</dd></div>
                </dl>
              </details>
            </div>
          </template>
        </AppTabs>
      </section>
    </div>
  </AppPage>
</template>

<style scoped>
.render-templates-shell { display: grid; grid-template-columns: 252px minmax(0, 1fr); gap: 28px; flex: 1; min-height: 0; }
.template-catalog { display: flex; flex-direction: column; min-height: 0; padding-right: 20px; border-right: 1px solid var(--border); }
.template-catalog__content { display: flex; flex: 1; flex-direction: column; min-height: 0; }
.template-catalog__toggle { width: 100%; justify-content: space-between; text-align: left; }
.template-catalog__toggle svg { flex: none; transition: transform 160ms ease; }
.template-catalog__toggle svg.is-open { transform: rotate(180deg); }
.template-catalog__search { position: relative; }
.template-catalog__search > svg { position: absolute; z-index: 1; top: 14px; left: 12px; color: var(--muted); pointer-events: none; }
.template-catalog__search :deep(input) { padding-left: 36px; }
.template-catalog__hint { display: flex; justify-content: space-between; gap: 10px; margin: 12px 2px 18px; color: var(--muted); font-size: 12px; }
.template-catalog__list { min-height: 0; overflow: auto; scrollbar-width: thin; scrollbar-color: var(--border-strong) transparent; }
.template-nav-group + .template-nav-group { margin-top: 24px; }
.template-nav-group__title { display: flex; justify-content: space-between; gap: 12px; margin: 0 10px 8px; color: var(--muted); font-size: 12px; font-weight: 600; line-height: 1.5; }
.template-nav-group__title span { font-weight: 400; font-variant-numeric: tabular-nums; }
.template-nav-item { display: flex; align-items: flex-start; gap: 10px; width: 100%; min-height: 44px; padding: 12px 10px; border: 0; border-radius: 8px; background: transparent; color: var(--text); text-align: left; cursor: pointer; transition: background-color 160ms ease; }
.template-nav-item > svg { flex: none; margin-top: 2px; color: var(--muted); }
.template-nav-item > span { min-width: 0; }
.template-nav-item strong { display: block; font-size: 14px; font-weight: 500; line-height: 1.5; overflow-wrap: anywhere; }
.template-nav-item small { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; line-height: 1.5; }
.template-nav-item:hover { background: var(--surface-soft); }
.template-nav-item.is-active { background: var(--brand-soft); color: var(--brand-foreground); }
.template-nav-item.is-active svg, .template-nav-item.is-active small { color: var(--brand-foreground); }
.template-nav-item:focus-visible, .template-technical summary:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.template-catalog__empty { padding: 24px 8px; color: var(--muted); font-size: 13px; text-align: center; }
.template-workspace { display: flex; flex-direction: column; min-height: 0; min-width: 0; }
.template-workspace__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin: 0 0 18px; }
.template-workspace__heading h2 { margin: 0; font-size: 22px; font-weight: 600; line-height: 1.4; }
.template-workspace__heading p { margin: 8px 0 0; color: var(--muted); font-size: 13px; line-height: 1.5; }
.template-workspace__notice { margin: 0 0 16px; color: var(--muted); font-size: 13px; }
.template-workspace__tabs { display: flex; flex-direction: column; flex: 1; min-height: 0; }
.template-workspace__tabs :deep(.app-tabs__header) { flex: none; margin-bottom: 16px; }
.template-workspace__tabs :deep(.app-tabs__content[data-state=active]) { display: flex; flex: 1; flex-direction: column; min-height: 0; overflow: auto; }
.render-template-preview-area { display: flex; flex-direction: column; min-height: 0; flex: 1; }
.template-preview-hint { margin: 0 0 14px; color: var(--muted); font-size: 13px; line-height: 1.6; }
.render-template-preview-empty { display: grid; place-items: center; min-height: 320px; }
.template-data-workspace, .template-information { width: 100%; max-width: 960px; }
.template-data-workspace__intro { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 16px; }
.template-data-workspace__intro p { max-width: 60ch; margin: 0; color: var(--muted); font-size: 14px; line-height: 1.7; }
.template-data-workspace__intro button { flex: none; }
.render-templates-json-input { font-family: var(--font-mono); font-size: 13px; line-height: 1.7; }
.template-information__description { margin: 0 0 24px; font-size: 14px; line-height: 1.7; }
.template-information h3 { margin: 0 0 12px; font-size: 14px; font-weight: 600; }
.schema-tree-row { display: flex; gap: 12px; align-items: baseline; padding: 12px 0 12px calc((var(--schema-depth) - 1) * 14px); border-bottom: 1px solid var(--border); }
.schema-tree-row > div { flex: 1; min-width: 0; }
.schema-tree-row strong { display: block; font-size: 13px; font-weight: 500; line-height: 1.6; overflow-wrap: anywhere; }
.schema-tree-row code { display: block; margin-top: 3px; color: var(--muted); font-size: 12px; overflow-wrap: anywhere; }
.schema-tree-row > span, .schema-tree-row small { color: var(--muted); font-size: 12px; white-space: nowrap; }
.template-technical { margin-top: 28px; }
.template-technical summary { padding: 12px 0; color: var(--muted); font-size: 13px; cursor: pointer; }
.template-info-list { display: grid; gap: 12px; margin: 8px 0 0; }
.template-info-list div { display: grid; grid-template-columns: 80px minmax(0, 1fr); gap: 16px; font-size: 13px; line-height: 1.6; }
.template-info-list dt { color: var(--muted); }
.template-info-list dd { margin: 0; overflow-wrap: anywhere; }
@media (max-width: 1100px) { .render-templates-shell { grid-template-columns: 220px minmax(0, 1fr); gap: 20px; } .template-catalog { padding-right: 16px; } }
@media (max-width: 767px) {
  .render-templates-shell { display: flex; flex-direction: column; gap: 24px; }
  .template-catalog { flex: none; padding: 0 0 16px; border-right: 0; border-bottom: 1px solid var(--border); }
  .template-catalog__content { margin-top: 12px; }
  .template-catalog__hint { margin-bottom: 10px; }
  .template-catalog__list { max-height: 190px; }
  .template-workspace { min-height: 440px; flex: 1; }
  .template-workspace__heading h2 { font-size: 20px; }
  .template-workspace__tabs :deep(.app-tabs__header) { flex-wrap: wrap; gap: 8px; }
  .template-workspace__tabs :deep(.app-tabs__list) { gap: 18px; }
  .template-data-workspace__intro { align-items: flex-start; }
}
@media (prefers-reduced-motion: reduce) { .template-nav-item, .template-catalog__toggle svg { transition: none; } }
</style>
