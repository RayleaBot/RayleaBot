import { computed, onDeactivated, onScopeDispose, ref, watch, type Ref } from 'vue'
import { storeToRefs } from 'pinia'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildRenderTemplatePreviewSample, parseRenderTemplatePreviewData } from '@/lib/render-template-editor'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { useRenderPreviewResources } from './useRenderPreviewResources'

export function useTemplatePreview(activeTemplateId: Ref<string>, isActiveTemplateRoute: Ref<boolean>) {
  const renderTemplatesStore = useRenderTemplatesStore()
  const { detailById } = storeToRefs(renderTemplatesStore)
  const previewDataByTemplate = ref<Record<string, string>>({})

  const previewErrorByTemplate = ref<Record<string, string>>({})

  const previewErrorKeyByTemplate = ref<Record<string, string>>({})

  const pendingPreviewKeyByTemplate = ref<Record<string, string>>({})

  const lastPreviewKeyByTemplate = ref<Record<string, string>>({})

  const previewControllers = new Map<string, AbortController>()

  const {
    clearPreviewDocumentCaches,
    previewDocumentByTemplate,
    previewDocumentCache,
    releasePreviewDocumentResources,
    releasePreviewResourceKeys,
    retainPreviewDocumentResources,
    revokePreviewDocument,
    rewritePreviewDocumentResources,
  } = useRenderPreviewResources(renderTemplatesStore)

  let autoPreviewHandle: number | null = null

  let previewRunId = 0

  const currentTemplate = computed(() => (
    activeTemplateId.value ? detailById.value[activeTemplateId.value] ?? null : null
  ))

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

  function buildDefaultPreviewData(schema: Record<string, unknown> | null = null, previewData: Record<string, unknown> | null = null) {
    if (previewData) {
      return JSON.stringify(previewData, null, 2)
    }

    if (schema) {
      return JSON.stringify(buildRenderTemplatePreviewSample(schema), null, 2)
    }

    return ''
  }

  function ensurePreviewDefaults(templateId: string) {
    if (!previewDataByTemplate.value[templateId]) {
      const detail = detailById.value[templateId]
      const previewData = buildDefaultPreviewData(detail?.input_schema_json ?? null, detail?.preview_data_json ?? null)
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

  function resetPreviewData() {
    if (!currentTemplate.value) return
    currentPreviewDataText.value = buildDefaultPreviewData(currentTemplate.value.input_schema_json, currentTemplate.value.preview_data_json)
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
      const rewritten = await rewritePreviewDocumentResources(templateId, response.html, response.source_digest, controller.signal)
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
  ], (next, previous) => {
    const immediate = !previous
      || next[0] !== previous[0]
      || next[1] !== previous[1]
      || next[3] !== previous[3]
    scheduleAutoPreview({ immediate })
  }, { immediate: true })

  function cancelPendingPreview() {
    clearAutoPreviewTimer()
    previewRunId += 1
    for (const controller of previewControllers.values()) controller.abort()
    previewControllers.clear()
    pendingPreviewKeyByTemplate.value = {}
  }

  function resetPreviewCaches() {
    cancelPendingPreview()
    lastPreviewKeyByTemplate.value = {}
    clearPreviewDocumentCaches()
  }

  function retainPreviewDrafts(ids: Set<string>) {
    previewDataByTemplate.value = Object.fromEntries(Object.entries(previewDataByTemplate.value).filter(([id]) => ids.has(id)))
    previewErrorByTemplate.value = {}
    previewErrorKeyByTemplate.value = {}
  }

  function invalidateCurrentPreview() {
    lastPreviewKeyByTemplate.value = { ...lastPreviewKeyByTemplate.value, [activeTemplateId.value]: '' }
    const cached = previewDocumentCache.get(previewRequestKey.value)
    if (cached) {
      previewDocumentCache.delete(previewRequestKey.value)
      releasePreviewDocumentResources(cached)
    }
    revokePreviewDocument(activeTemplateId.value)
  }

  onDeactivated(cancelPendingPreview)
  onScopeDispose(resetPreviewCaches)
  return { currentTemplate, currentPreviewDataText, previewParseResult, currentPreviewDocument,
    currentPreviewError, currentPreviewPending, ensurePreviewDefaults, resetPreviewData,
    scheduleAutoPreview, resetPreviewCaches, retainPreviewDrafts, invalidateCurrentPreview }
}
