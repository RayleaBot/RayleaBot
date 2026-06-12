<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppEmptyState from '@/components/AppEmptyState.vue'
import NativeTemplatePreviewFrame from '@/components/NativeTemplatePreviewFrame.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { useToastFeedback } from '@/adapter/feedback'
import TemplatePreviewFrame from '@/components/TemplatePreviewFrame.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import {
  buildRenderTemplatePreviewSample,
  buildRenderTemplateSchemaNodes,
  parseRenderTemplatePreviewData,
} from '@/lib/render-template-editor'
import { t } from '@/i18n'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import type { RenderTemplatePreviewHTMLResponse, RenderTemplateSummary } from '@/types/api'

const route = useRoute()
const router = useRouter()
const renderTemplatesStore = useRenderTemplatesStore()
const { detailById, error, items, loading, workspaceLoading } = storeToRefs(renderTemplatesStore)

interface PreviewDocumentState extends RenderTemplatePreviewHTMLResponse {
  cacheKey: string
  resourceKeys: string[]
}

interface PreviewResourceCacheEntry {
  blobUrl: string
  refCount: number
}

interface PreviewResourceTextCacheEntry {
  text: string
}

interface PreviewResourceRewriteContext {
  createdResourceKeys: string[]
  resourceKeys: string[]
  resolved: Map<string, Promise<string>>
  resolvedText: Map<string, Promise<string>>
  signal: AbortSignal
  templateId: string
}

const hasRequestedList = ref(false)
const pageActive = ref(true)
const previewDataByTemplate = ref<Record<string, string>>({})
const previewDocumentByTemplate = ref<Record<string, PreviewDocumentState>>({})
const previewErrorByTemplate = ref<Record<string, string>>({})
const previewErrorKeyByTemplate = ref<Record<string, string>>({})
const pendingPreviewKeyByTemplate = ref<Record<string, string>>({})
const lastPreviewKeyByTemplate = ref<Record<string, string>>({})

const previewControllers = new Map<string, AbortController>()
const previewDocumentCache = new Map<string, PreviewDocumentState>()
const previewResourceCache = new Map<string, PreviewResourceCacheEntry>()
const previewResourceTextCache = new Map<string, PreviewResourceTextCacheEntry>()
let autoPreviewHandle: number | null = null
let previewRunId = 0

const isTemplateRoute = computed(() => route.name === 'render-templates')
const isActiveTemplateRoute = computed(() => pageActive.value && isTemplateRoute.value)

const activeTemplateId = computed(() => (
  isTemplateRoute.value && typeof route.params.templateId === 'string' && route.params.templateId
    ? route.params.templateId
    : ''
))

const currentTemplate = computed(() => (
  activeTemplateId.value ? detailById.value[activeTemplateId.value] ?? null : null
))

const groupedTemplates = computed(() => {
  const systemTemplates: RenderTemplateSummary[] = []
  const pluginTemplates: RenderTemplateSummary[] = []

  for (const template of items.value) {
    if (template.source.type === 'plugin') {
      pluginTemplates.push(template)
    } else {
      systemTemplates.push(template)
    }
  }

  return [
    {
      key: 'system',
      title: t('renderTemplates.sources.system'),
      items: systemTemplates,
    },
    {
      key: 'plugin',
      title: t('renderTemplates.sources.plugin'),
      items: pluginTemplates,
    },
  ].filter((group) => group.items.length > 0)
})

const currentPreviewDataText = computed({
  get() {
    if (!activeTemplateId.value) {
      return '{}'
    }
    return previewDataByTemplate.value[activeTemplateId.value] ?? '{}'
  },
  set(value: string) {
    if (!activeTemplateId.value) {
      return
    }
    previewDataByTemplate.value = {
      ...previewDataByTemplate.value,
      [activeTemplateId.value]: value,
    }
  },
})

const previewParseResult = computed(() => parseRenderTemplatePreviewData(currentPreviewDataText.value))
const schemaNodes = computed(() => buildRenderTemplateSchemaNodes(currentTemplate.value?.input_schema_json ?? null))
const displaySchemaNodes = computed(() => schemaNodes.value.filter((node) => node.depth > 0))

const previewRequestKey = computed(() => {
  if (!activeTemplateId.value || !currentTemplate.value || !previewParseResult.value.data) {
    return ''
  }

  return JSON.stringify({
    template: activeTemplateId.value,
    updated_at: currentTemplate.value.updated_at,
    theme: 'default',
    data: previewParseResult.value.data,
  })
})

const currentPreviewDocument = computed(() => (
  activeTemplateId.value ? previewDocumentByTemplate.value[activeTemplateId.value] ?? null : null
))

const currentPreviewError = computed(() => (
  activeTemplateId.value ? previewErrorByTemplate.value[activeTemplateId.value] ?? '' : ''
))

const currentPreviewPending = computed(() => (
  Boolean(activeTemplateId.value && previewRequestKey.value && pendingPreviewKeyByTemplate.value[activeTemplateId.value] === previewRequestKey.value)
))
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

const currentLocalHelpMenuData = computed(() => (
  activeTemplateId.value === 'help.menu' && previewParseResult.value.data
    ? previewParseResult.value.data
    : null
))

const showLocalHelpMenuPreview = computed(() => (
  activeTemplateId.value === 'help.menu'
    && !currentPreviewDocument.value
    && Boolean(currentLocalHelpMenuData.value)
))

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

  return template.source.plugin_id || t('display.empty')
}

function getTemplateLocalId(template: RenderTemplateSummary) {
  if (template.source.type !== 'plugin') {
    return ''
  }

  return template.source.local_id || ''
}

function buildDefaultPreviewData(templateId: string, schema: Record<string, unknown> | null = null, previewData: Record<string, unknown> | null = null) {
  if (previewData) {
    return JSON.stringify(previewData, null, 2)
  }

  if (templateId === 'help.menu') {
    return JSON.stringify({
      title: '帮助菜单',
      subtitle: '常用命令入口',
      user: {
        avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10001&s=100',
        nickname: '星野',
        title: '指令调度员',
        id: '10001',
      },
      group: {
        name: '测试群组',
      },
      permission: {
        level: 'admin',
      },
      items: [
        {
          name: 'weather',
          description: '查询天气',
          usage: '/weather <城市>',
        },
      ],
    }, null, 2)
  }

  if (templateId === 'status.panel') {
    return JSON.stringify({
      title: 'Runtime Status',
      status: 'ready',
      summary: '所有核心服务已就绪。',
      user: {
        avatar_url: 'https://q1.qlogo.cn/g?b=qq&nk=10086&s=100',
        nickname: '凌川',
        title: '系统观察员',
        id: '10086',
      },
      group: {
        name: 'RayleaBot 运维群',
      },
      permission: {
        level: 'super_admin',
      },
      metrics: [
        { label: 'Plugins', value: '8 loaded' },
        { label: 'Queue', value: 'idle' },
      ],
    }, null, 2)
  }

  if (schema) {
    return JSON.stringify(buildRenderTemplatePreviewSample(schema), null, 2)
  }

  return ''
}

function ensurePreviewDefaults(templateId: string) {
  if (!previewDataByTemplate.value[templateId]) {
    const detail = detailById.value[templateId]
    const previewData = buildDefaultPreviewData(templateId, detail?.input_schema_json ?? null, detail?.preview_data_json ?? null)
    if (!previewData) {
      return
    }

    previewDataByTemplate.value = {
      ...previewDataByTemplate.value,
      [templateId]: previewData,
    }
  }
}

function clearAutoPreviewTimer() {
  if (autoPreviewHandle === null) {
    return
  }

  window.clearTimeout(autoPreviewHandle)
  autoPreviewHandle = null
}

function revokePreviewDocument(templateId: string) {
  if (!(templateId in previewDocumentByTemplate.value)) {
    return
  }
  const next = { ...previewDocumentByTemplate.value }
  delete next[templateId]
  previewDocumentByTemplate.value = next
}

function retainPreviewDocumentResources(document: PreviewDocumentState) {
  for (const cacheKey of document.resourceKeys) {
    const entry = previewResourceCache.get(cacheKey)
    if (entry) {
      entry.refCount += 1
    }
  }
}

function releasePreviewDocumentResources(document: PreviewDocumentState) {
  releasePreviewResourceKeys(document.resourceKeys, { force: false })
}

function releasePreviewResourceKeys(cacheKeys: string[], options: { force: boolean }) {
  for (const cacheKey of new Set(cacheKeys)) {
    const entry = previewResourceCache.get(cacheKey)
    if (!entry) {
      continue
    }
    if (!options.force) {
      entry.refCount -= 1
    }
    if (options.force || entry.refCount <= 0) {
      previewResourceCache.delete(cacheKey)
      window.URL.revokeObjectURL(entry.blobUrl)
    }
  }
}

function clearPreviewDocumentCaches() {
  const released = new Set<string>()
  for (const document of Object.values(previewDocumentByTemplate.value)) {
    if (released.has(document.cacheKey)) {
      continue
    }
    released.add(document.cacheKey)
    releasePreviewDocumentResources(document)
  }
  for (const [cacheKey, document] of previewDocumentCache) {
    if (released.has(cacheKey)) {
      continue
    }
    released.add(cacheKey)
    releasePreviewDocumentResources(document)
  }
  previewDocumentCache.clear()
  previewDocumentByTemplate.value = {}
  previewResourceTextCache.clear()
  for (const cacheKey of Array.from(previewResourceCache.keys())) {
    releasePreviewResourceKeys([cacheKey], { force: true })
  }
}

function setPreviewError(templateId: string, requestKey: string, message: string) {
  previewErrorByTemplate.value = {
    ...previewErrorByTemplate.value,
    [templateId]: message,
  }
  previewErrorKeyByTemplate.value = {
    ...previewErrorKeyByTemplate.value,
    [templateId]: requestKey,
  }
}

function clearPreviewError(templateId: string) {
  previewErrorByTemplate.value = {
    ...previewErrorByTemplate.value,
    [templateId]: '',
  }
  previewErrorKeyByTemplate.value = {
    ...previewErrorKeyByTemplate.value,
    [templateId]: '',
  }
}

async function loadTemplateList() {
  hasRequestedList.value = true
  try {
    await renderTemplatesStore.fetchTemplates()
  } catch {
    // store error state drives the page
  }
}

async function loadTemplateWorkspace(templateId: string, options: { force?: boolean } = {}) {
  if (!options.force && detailById.value[templateId]) {
    renderTemplatesStore.clearError()
    return
  }

  try {
    await renderTemplatesStore.fetchTemplateWorkspace(templateId)
  } catch {
    // store error state drives the page
  }
}

async function reloadCurrentTemplate() {
  if (!activeTemplateId.value) {
    return
  }

  lastPreviewKeyByTemplate.value = {
    ...lastPreviewKeyByTemplate.value,
    [activeTemplateId.value]: '',
  }
  previewDocumentCache.delete(previewRequestKey.value)
  await loadTemplateWorkspace(activeTemplateId.value, { force: true })
  scheduleAutoPreview({ immediate: true })
}

async function syncRouteTemplate() {
  if (!isActiveTemplateRoute.value || items.value.length === 0) {
    return
  }

  if (!activeTemplateId.value) {
    await router.replace({
      name: 'render-templates',
      params: {
        templateId: items.value[0].id,
      },
    })
    return
  }

  await loadTemplateWorkspace(activeTemplateId.value)
  ensurePreviewDefaults(activeTemplateId.value)
}

async function selectTemplate(templateId: string) {
  if (templateId === activeTemplateId.value) {
    return
  }

  await router.replace({
    name: 'render-templates',
    params: {
      templateId,
    },
  })
}

async function submitPreview(templateId: string, requestKey: string) {
  if (!isActiveTemplateRoute.value || activeTemplateId.value !== templateId || !previewParseResult.value.data) {
    return
  }

  const currentPendingKey = pendingPreviewKeyByTemplate.value[templateId]
  if (currentPendingKey === requestKey) {
    return
  }

  const cached = previewDocumentCache.get(requestKey)
  if (cached) {
    revokePreviewDocument(templateId)
    previewDocumentByTemplate.value = {
      ...previewDocumentByTemplate.value,
      [templateId]: cached,
    }
  }

  previewControllers.get(templateId)?.abort()
  const controller = new AbortController()
  previewControllers.set(templateId, controller)
  const runId = ++previewRunId

  pendingPreviewKeyByTemplate.value = {
    ...pendingPreviewKeyByTemplate.value,
    [templateId]: requestKey,
  }
  clearPreviewError(templateId)

  try {
    const response = await renderTemplatesStore.previewTemplateHTML(templateId, {
      theme: 'default',
      data: previewParseResult.value.data,
    }, controller.signal)
    const rewritten = await rewritePreviewDocumentResources(templateId, response.html, response.revision_id, controller.signal)
    if (controller.signal.aborted || runId !== previewRunId || activeTemplateId.value !== templateId || previewRequestKey.value !== requestKey) {
      releasePreviewResourceKeys(rewritten.createdResourceKeys, { force: true })
      return
    }

    revokePreviewDocument(templateId)
    const document = {
      ...response,
      cacheKey: requestKey,
      html: rewritten.html,
      resourceKeys: rewritten.resourceKeys,
    }
    retainPreviewDocumentResources(document)
    const previousCached = previewDocumentCache.get(requestKey)
    if (previousCached) {
      previewDocumentCache.delete(requestKey)
      releasePreviewDocumentResources(previousCached)
    }
    previewDocumentCache.set(requestKey, document)
    previewDocumentByTemplate.value = {
      ...previewDocumentByTemplate.value,
      [templateId]: document,
    }
    lastPreviewKeyByTemplate.value = {
      ...lastPreviewKeyByTemplate.value,
      [templateId]: requestKey,
    }
  } catch (err) {
    if (controller.signal.aborted || runId !== previewRunId || activeTemplateId.value !== templateId || previewRequestKey.value !== requestKey) {
      return
    }
    setPreviewError(templateId, requestKey, getDisplayErrorMessage(err))
  } finally {
    if (previewControllers.get(templateId) === controller) {
      previewControllers.delete(templateId)
    }
    if (pendingPreviewKeyByTemplate.value[templateId] === requestKey) {
      pendingPreviewKeyByTemplate.value = {
        ...pendingPreviewKeyByTemplate.value,
        [templateId]: '',
      }
    }
  }
}

function scheduleAutoPreview(options: { immediate?: boolean } = {}) {
  clearAutoPreviewTimer()

  if (!isActiveTemplateRoute.value || !activeTemplateId.value || !currentTemplate.value) {
    return
  }

  if (previewParseResult.value.data === null) {
    return
  }

  const requestKey = previewRequestKey.value
  const templateId = activeTemplateId.value
  if (previewErrorKeyByTemplate.value[templateId] && previewErrorKeyByTemplate.value[templateId] !== requestKey) {
    clearPreviewError(templateId)
  }

  if (!requestKey || pendingPreviewKeyByTemplate.value[templateId] === requestKey) {
    return
  }

  if (!options.immediate && lastPreviewKeyByTemplate.value[templateId] === requestKey) {
    return
  }

  const cached = previewDocumentCache.get(requestKey)
  if (cached) {
    revokePreviewDocument(templateId)
    previewDocumentByTemplate.value = {
      ...previewDocumentByTemplate.value,
      [templateId]: cached,
    }
  }

  if (options.immediate) {
    void submitPreview(templateId, requestKey)
    return
  }

  autoPreviewHandle = window.setTimeout(() => {
    autoPreviewHandle = null
    if (!isActiveTemplateRoute.value || activeTemplateId.value !== templateId || previewRequestKey.value !== requestKey) {
      return
    }

    void submitPreview(templateId, requestKey)
  }, 350)
}

async function rewritePreviewDocumentResources(templateId: string, html: string, revisionId: string, signal: AbortSignal) {
  const createdResourceKeys: string[] = []
  const resourceKeys: string[] = []
  const context: PreviewResourceRewriteContext = {
    createdResourceKeys,
    resourceKeys,
    resolved: new Map(),
    resolvedText: new Map(),
    signal,
    templateId,
  }
  const document = new DOMParser().parseFromString(html, 'text/html')

  try {
    await Promise.all([
      ...Array.from(document.querySelectorAll('style')).map(async (style) => {
        style.textContent = await rewriteCSSResources(style.textContent ?? '', '', revisionId, context)
      }),
      ...Array.from(document.querySelectorAll<HTMLElement>('[style]')).map(async (element) => {
        const style = element.getAttribute('style') ?? ''
        element.setAttribute('style', await rewriteCSSResources(style, '', revisionId, context))
      }),
      ...Array.from(document.querySelectorAll<HTMLElement>('[src]')).map((element) => (
        rewriteElementResourceAttribute(element, 'src', '', revisionId, context)
      )),
      ...Array.from(document.querySelectorAll<HTMLLinkElement>('link[href]')).map((link) => (
        rewriteLinkResource(link, document, revisionId, context)
      )),
    ])
  } catch (err) {
    releasePreviewResourceKeys(createdResourceKeys, { force: true })
    throw err
  }

  return {
    createdResourceKeys,
    html: `<!doctype html>\n${document.documentElement.outerHTML}`,
    resourceKeys,
  }
}

async function rewriteElementResourceAttribute(
  element: HTMLElement,
  attribute: string,
  basePath: string,
  revisionId: string,
  context: PreviewResourceRewriteContext,
) {
  const raw = element.getAttribute(attribute) ?? ''
  const resourcePath = resolvePreviewResourcePath(basePath, raw)
  if (!resourcePath) {
    return
  }

  const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
  element.setAttribute(attribute, blobUrl)
}

async function rewriteLinkResource(
  link: HTMLLinkElement,
  document: Document,
  revisionId: string,
  context: PreviewResourceRewriteContext,
) {
  const rel = (link.getAttribute('rel') ?? '').toLowerCase()
  const href = link.getAttribute('href') ?? ''
  const resourcePath = resolvePreviewResourcePath('', href)
  if (!resourcePath) {
    return
  }

  if (rel.includes('stylesheet')) {
    const css = await downloadTemplateAssetText(resourcePath, revisionId, context)
    const style = document.createElement('style')
    style.textContent = await rewriteCSSResources(css, dirname(resourcePath), revisionId, context)
    link.replaceWith(style)
    return
  }

  const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
  link.setAttribute('href', blobUrl)
}

async function rewriteCSSResources(css: string, basePath: string, revisionId: string, context: PreviewResourceRewriteContext): Promise<string> {
  let rewritten = css

  rewritten = await replaceAsync(rewritten, /@import\s+(?:url\()?["']?([^"')\s;]+)["']?\)?[^;]*;?/gi, async (match, rawUrl: string) => {
    const resourcePath = resolvePreviewResourcePath(basePath, rawUrl)
    if (!resourcePath) {
      return match
    }
    const importedCSS = await downloadTemplateAssetText(resourcePath, revisionId, context)
    return rewriteCSSResources(importedCSS, dirname(resourcePath), revisionId, context)
  })

  rewritten = await replaceAsync(rewritten, /url\(\s*(["']?)([^"')]+)\1\s*\)/gi, async (match, quote: string, rawUrl: string) => {
    const resourcePath = resolvePreviewResourcePath(basePath, rawUrl)
    if (!resourcePath) {
      return match
    }
    const blobUrl = await downloadTemplateAssetObjectURL(resourcePath, revisionId, context)
    return `url(${quote}${blobUrl}${quote})`
  })

  return rewritten
}

async function downloadTemplateAssetText(path: string, revisionId: string, context: PreviewResourceRewriteContext) {
  const cacheKey = `${context.templateId}:${revisionId}:${path}`
  const existing = previewResourceTextCache.get(cacheKey)
  if (existing) {
    return existing.text
  }
  const pending = context.resolvedText.get(cacheKey)
  if (pending) {
    return pending
  }

  const promise = renderTemplatesStore.downloadTemplateAsset(context.templateId, path, context.signal)
    .then(async ({ blob }) => {
      const text = await readPreviewResourceText(blob)
      previewResourceTextCache.set(cacheKey, { text })
      return text
    })
  context.resolvedText.set(cacheKey, promise)
  return promise
}

async function readPreviewResourceText(blob: Blob) {
  if (typeof blob.text === 'function') {
    return blob.text()
  }
  return new Response(blob).text()
}

async function downloadTemplateAssetObjectURL(path: string, revisionId: string, context: PreviewResourceRewriteContext) {
  const cacheKey = `${context.templateId}:${revisionId}:${path}`
  const existing = previewResourceCache.get(cacheKey)
  if (existing) {
    addPreviewResourceKey(context, cacheKey)
    return existing.blobUrl
  }
  const pending = context.resolved.get(cacheKey)
  if (pending) {
    const blobUrl = await pending
    addPreviewResourceKey(context, cacheKey)
    return blobUrl
  }

  const promise = renderTemplatesStore.downloadTemplateAsset(context.templateId, path, context.signal)
    .then(({ blob }) => {
      const blobUrl = window.URL.createObjectURL(blob)
      previewResourceCache.set(cacheKey, { blobUrl, refCount: 0 })
      addCreatedPreviewResourceKey(context, cacheKey)
      return blobUrl
    })
  context.resolved.set(cacheKey, promise)
  const blobUrl = await promise
  addPreviewResourceKey(context, cacheKey)
  return blobUrl
}

function addPreviewResourceKey(context: PreviewResourceRewriteContext, cacheKey: string) {
  if (!context.resourceKeys.includes(cacheKey)) {
    context.resourceKeys.push(cacheKey)
  }
}

function addCreatedPreviewResourceKey(context: PreviewResourceRewriteContext, cacheKey: string) {
  if (!context.createdResourceKeys.includes(cacheKey)) {
    context.createdResourceKeys.push(cacheKey)
  }
}

function resolvePreviewResourcePath(basePath: string, rawUrl: string) {
  const url = stripResourceURL(rawUrl)
  if (!url || isExternalPreviewResource(url)) {
    return ''
  }
  return normalizePreviewResourcePath(basePath ? `${basePath}/${url}` : url)
}

function stripResourceURL(rawUrl: string) {
  return String(rawUrl ?? '').trim().replace(/^["']|["']$/g, '').split(/[?#]/)[0]
}

function isExternalPreviewResource(url: string) {
  return url.startsWith('#')
    || url.startsWith('/')
    || /^[a-z][a-z0-9+.-]*:/i.test(url)
}

function dirname(path: string) {
  const normalized = normalizePreviewResourcePath(path)
  const index = normalized.lastIndexOf('/')
  return index > 0 ? normalized.slice(0, index) : ''
}

function normalizePreviewResourcePath(path: string) {
  const segments: string[] = []
  for (const segment of path.replace(/\\/g, '/').split('/')) {
    if (!segment || segment === '.') {
      continue
    }
    if (segment === '..') {
      if (segments.length > 0 && segments[segments.length - 1] !== '..') {
        segments.pop()
      } else {
        segments.push(segment)
      }
      continue
    }
    segments.push(segment)
  }
  return segments.join('/')
}

async function replaceAsync(source: string, pattern: RegExp, replacer: (...args: any[]) => Promise<string>) {
  const matches = Array.from(source.matchAll(pattern))
  if (matches.length === 0) {
    return source
  }

  const replacements = await Promise.all(matches.map((match) => replacer(...match)))
  let result = ''
  let lastIndex = 0
  matches.forEach((match, index) => {
    result += source.slice(lastIndex, match.index)
    result += replacements[index]
    lastIndex = (match.index ?? 0) + match[0].length
  })
  result += source.slice(lastIndex)
  return result
}

watch([items, isActiveTemplateRoute, () => route.params.templateId], () => {
  void syncRouteTemplate()
}, { immediate: true })

watch(activeTemplateId, (templateId) => {
  if (!templateId) {
    return
  }

  ensurePreviewDefaults(templateId)
  if (!(templateId in previewErrorByTemplate.value)) {
    previewErrorByTemplate.value = {
      ...previewErrorByTemplate.value,
      [templateId]: '',
    }
  }
}, { immediate: true })

watch(() => [
  activeTemplateId.value,
  currentTemplate.value?.updated_at ?? '',
  currentPreviewDataText.value,
  isActiveTemplateRoute.value,
  pageActive.value,
], (next, previous) => {
  const immediate = !previous
    || next[0] !== previous[0]
    || next[1] !== previous[1]
    || next[3] !== previous[3]
    || next[4] !== previous[4]
  scheduleAutoPreview({ immediate })
}, { immediate: true })

onMounted(() => {
  void loadTemplateList()
})

onActivated(() => {
  pageActive.value = true
})

onDeactivated(() => {
  pageActive.value = false
  clearAutoPreviewTimer()
})

onBeforeUnmount(() => {
  clearAutoPreviewTimer()
  previewRunId += 1
  for (const controller of previewControllers.values()) {
    controller.abort()
  }
  previewControllers.clear()
  clearPreviewDocumentCaches()
})
</script>

<template>
  <AppPage :title="t('renderTemplates.title')" :description="t('renderTemplates.subtitle')" full-height>
    <template #extra>
      <div class="render-templates-actions">
        <a-button :disabled="!activeTemplateId" @click="reloadCurrentTemplate">
          {{ t('renderTemplates.reloadAction') }}
        </a-button>
      </div>
    </template>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('renderTemplates.title')"
      :description="error"
      :loading="loading"
      @retry="loadTemplateList"
    />

    <AppEmptyState
      v-else-if="!loading && hasRequestedList && items.length === 0"
      icon="box"
      :title="t('renderTemplates.noTemplates')"
      :description="t('renderTemplates.templateListHint')"
    />

    <div v-else class="render-templates-shell">
      <aside class="render-templates-float-panel">
        <div class="render-templates-float-panel__header">
          <span class="render-templates-float-panel__title">{{ t('renderTemplates.title') }}</span>
          <a-tag v-if="currentPreviewPending" color="blue" class="render-templates-live-tag">
            {{ t('renderTemplates.previewPending') }}
          </a-tag>
        </div>

        <div class="render-templates-float-panel__body">
          <section class="render-templates-panel-section">
            <div class="render-templates-panel-section__header">
              <span>{{ t('renderTemplates.templateList') }}</span>
              <small>{{ items.length }}</small>
            </div>
            <div class="template-nav-list">
              <section
                v-for="group in groupedTemplates"
                :key="group.key"
                class="template-nav-group"
              >
                <div class="template-nav-group__title">
                  <span>{{ group.title }}</span>
                  <small>{{ group.items.length }}</small>
                </div>
                <button
                  v-for="template in group.items"
                  :key="template.id"
                  type="button"
                  class="template-nav-item"
                  :class="{ 'is-active': template.id === activeTemplateId }"
                  @click="selectTemplate(template.id)"
                >
                  <span class="template-nav-item__id">{{ template.id }}</span>
                  <span class="template-nav-item__meta">
                    {{ getTemplateSourceLabel(template) }}
                    <span v-if="getTemplateLocalId(template)"> · {{ getTemplateLocalId(template) }}</span>
                  </span>
                </button>
              </section>
            </div>
          </section>

          <section v-if="currentTemplate" class="render-templates-panel-section">
            <div class="render-templates-panel-section__header">
              <span>{{ t('renderTemplates.summaryTitle') }}</span>
            </div>
            <dl class="template-info-list">
              <div>
                <dt>{{ t('renderTemplates.fields.id') }}</dt>
                <dd>{{ currentTemplate.id }}</dd>
              </div>
              <div>
                <dt>{{ t('renderTemplates.fields.source') }}</dt>
                <dd>{{ getTemplateSourceLabel(currentTemplate) }}</dd>
              </div>
              <div v-if="getTemplateLocalId(currentTemplate)">
                <dt>{{ t('renderTemplates.fields.localId') }}</dt>
                <dd>{{ getTemplateLocalId(currentTemplate) }}</dd>
              </div>
              <div>
                <dt>{{ t('renderTemplates.fields.version') }}</dt>
                <dd>{{ currentTemplate.version }}</dd>
              </div>
              <div>
                <dt>{{ t('renderTemplates.fields.size') }}</dt>
                <dd>{{ formatTemplateSize(currentTemplate.width, currentTemplate.height) }}</dd>
              </div>
              <div>
                <dt>{{ t('renderTemplates.fields.updatedAt') }}</dt>
                <dd>{{ formatDateTime(currentTemplate.updated_at) }}</dd>
              </div>
            </dl>
          </section>

          <section class="render-templates-panel-section">
            <div class="render-templates-panel-section__header">
              <span>{{ t('renderTemplates.previewData') }}</span>
            </div>
            <a-textarea
              v-model:value="currentPreviewDataText"
              :rows="12"
              :aria-label="t('renderTemplates.previewData')"
              :placeholder="t('renderTemplates.previewDataPlaceholder')"
              class="render-templates-json-input"
            />
          </section>

          <section class="render-templates-panel-section">
            <div class="render-templates-panel-section__header">
              <span>{{ t('renderTemplates.schemaPreviewTitle') }}</span>
              <small>{{ t('renderTemplates.schemaPreviewHint') }}</small>
            </div>
            <a-skeleton :loading="workspaceLoading && !currentTemplate" active :paragraph="{ rows: 5 }">
              <div v-if="displaySchemaNodes.length > 0" class="schema-tree">
                <div
                  v-for="node in displaySchemaNodes"
                  :key="node.key"
                  class="schema-tree-row"
                  :style="{ '--schema-depth': String(node.depth) }"
                >
                  <div class="schema-tree-row__content">
                    <div class="schema-tree-row__header">
                      <span class="schema-tree-row__name">{{ node.label }}</span>
                      <span class="schema-tree-row__type">{{ node.type }}</span>
                      <span v-if="node.required" class="schema-tree-row__required">{{ t('renderTemplates.required.yes') }}</span>
                    </div>
                    <div v-if="node.description" class="schema-tree-row__desc">{{ node.description }}</div>
                  </div>
                </div>
              </div>
              <a-empty v-else :description="t('renderTemplates.schemaPreviewEmpty')" />
            </a-skeleton>
          </section>
        </div>
      </aside>

      <main class="render-template-preview-area">
        <div class="render-template-preview-area__header">
          <div>
            <span>{{ t('renderTemplates.previewTitle') }}</span>
            <small>{{ t('renderTemplates.previewHint') }}</small>
          </div>
          <a-tag v-if="currentPreviewDocument" color="green">
            {{ currentPreviewDocument.revision_id }}
          </a-tag>
        </div>

        <div class="render-template-preview-surface" data-testid="render-template-preview-result">
          <TemplatePreviewFrame
            v-if="currentPreviewDocument"
            :frame-title="currentPreviewDocument.template_id"
            :frame-width="currentPreviewDocument.width"
            :srcdoc="currentPreviewDocument.html"
            :template-id="currentPreviewDocument.template_id"
            :payload="currentPreviewDataText"
            test-id-prefix="render-template-preview"
          />
          <NativeTemplatePreviewFrame
            v-else-if="showLocalHelpMenuPreview && currentLocalHelpMenuData"
            template-id="help.menu"
            :data="currentLocalHelpMenuData"
            data-testid="render-template-preview-local-frame"
          />
          <div v-else class="render-template-preview-empty">
            <a-empty :description="previewEmptyDescription" />
          </div>
        </div>
      </main>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.render-templates-actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.render-templates-shell {
  --render-template-panel-width: 360px;
  --render-template-panel-inset: var(--space-md);
  --render-template-preview-max-width: 1120px;

  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  padding: var(--space-lg);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
  box-shadow: var(--shadow-card);
}

.render-templates-float-panel {
  position: absolute;
  top: var(--render-template-panel-inset);
  left: var(--render-template-panel-inset);
  z-index: 10;
  display: flex;
  flex-direction: column;
  width: var(--render-template-panel-width);
  max-height: calc(100% - var(--render-template-panel-inset) * 2);
  min-height: 0;
  padding: var(--space-md);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-elevated);
}

.render-templates-float-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
  padding-bottom: var(--space-sm);
  border-bottom: 1px solid var(--border);
}

.render-templates-float-panel__title {
  color: var(--text);
  font-size: 0.92rem;
  font-weight: 600;
}

.render-templates-live-tag {
  margin: 0;
  font-size: 0.75rem;
}

.render-templates-float-panel__body {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: var(--space-md);
  min-height: 0;
  overflow: auto;
  padding-top: var(--space-md);
}

.render-templates-panel-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.render-templates-panel-section__header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-sm);
  color: var(--text);
  font-size: 0.84rem;
  font-weight: 600;

  small {
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 500;
    text-align: right;
  }
}

.template-nav-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.template-nav-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.template-nav-group__title {
  display: flex;
  justify-content: space-between;
  gap: var(--space-sm);
  color: var(--muted);
  font-size: 0.76rem;
}

.template-nav-item {
  display: grid;
  gap: 4px;
  width: 100%;
  padding: 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  appearance: none;
  background: var(--surface);
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
}

.template-nav-item:hover {
  border-color: var(--border-accent);
  background: var(--surface-soft);
  transform: translateY(-1px);
}

.template-nav-item.is-active {
  border-color: var(--border-accent);
  background: var(--surface-accent);
}

.template-nav-item__id {
  font-family: var(--font-mono);
  font-size: 0.82rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.template-nav-item__meta {
  color: var(--muted);
  font-size: 0.76rem;
  overflow-wrap: anywhere;
}

.template-info-list {
  display: grid;
  gap: 7px;
  margin: 0;

  div {
    display: grid;
    grid-template-columns: 72px minmax(0, 1fr);
    gap: var(--space-sm);
  }

  dt {
    color: var(--muted);
    font-size: 0.76rem;
  }

  dd {
    min-width: 0;
    margin: 0;
    color: var(--text);
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }
}

.render-templates-json-input {
  font-family: var(--font-mono);
  font-size: 0.8rem;
}

.schema-tree {
  display: flex;
  flex-direction: column;
}

.schema-tree-row {
  display: flex;
  align-items: flex-start;
  gap: var(--space-xs);
  padding: 7px 0;
  padding-left: calc((var(--schema-depth) - 1) * 14px);
  border-bottom: 1px solid color-mix(in srgb, var(--border) 45%, transparent);
}

.schema-tree-row:last-child {
  border-bottom: none;
}

.schema-tree-row__content {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.schema-tree-row__header {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.schema-tree-row__name {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 600;
}

.schema-tree-row__type {
  padding: 1px 6px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--muted);
  font-size: 0.72rem;
}

.schema-tree-row__required {
  color: #ff4d4f;
  font-size: 0.72rem;
  font-weight: 600;
}

.schema-tree-row__desc {
  color: var(--muted);
  font-size: 0.76rem;
  line-height: 1.45;
}

.render-template-preview-area {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: var(--space-md);
  align-items: center;
  min-width: 0;
  min-height: 0;
  padding-left: calc(var(--render-template-panel-width) + var(--space-xl));
}

.render-template-preview-area__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  width: min(100%, var(--render-template-preview-max-width));
  color: var(--text);

  div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  span {
    font-size: 0.95rem;
    font-weight: 600;
  }

  small {
    color: var(--muted);
    font-size: 0.78rem;
  }
}

.render-template-preview-surface {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  width: min(100%, var(--render-template-preview-max-width));
  min-height: 0;
}

.render-template-preview-empty {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  justify-content: center;
  min-height: 320px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);
}

@media (max-width: 1180px) {
  .render-templates-shell {
    flex-direction: column;
    gap: var(--space-md);
  }

  .render-templates-float-panel {
    position: static;
    width: 100%;
    max-height: none;
    background: var(--surface);
  }

  .render-template-preview-area {
    padding-left: 0;
  }
}

@media (max-width: 720px) {
  .render-templates-shell {
    padding: var(--space-sm);
  }

  .render-templates-float-panel {
    padding: var(--space-sm);
  }

  .template-info-list div {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
</style>
