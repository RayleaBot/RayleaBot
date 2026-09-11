import { apiPath } from '@/lib/api-path'
import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import type {
  RenderTemplateDetail,
  RenderTemplateDetailResponse,
  RenderTemplateListResponse,
  RenderTemplatePreviewHTMLRequest,
  RenderTemplatePreviewHTMLResponse,
  RenderTemplateSummary,
} from '@/types/api'

function sortTemplateSummaries(items: RenderTemplateSummary[]) {
  return [...items].sort((left, right) => left.id.localeCompare(right.id))
}

export const useRenderTemplatesStore = defineStore('render-templates', () => {
  const items = ref<RenderTemplateSummary[]>([])
  const detailById = ref<Record<string, RenderTemplateDetail>>({})
  const workspaceLoading = ref(false)
  let catalogVersion = 0
  let workspaceRequest = 0
  const pendingWorkspaces = new Map<string, number>()

  const templateMap = computed(() => Object.fromEntries(items.value.map((item) => [item.id, item])))

  function upsertTemplateSummary(summary: RenderTemplateSummary) {
    items.value = items.value.map(item => item.id === summary.id ? summary : item)
  }

  function clearError() {
    error.value = null
  }

  const pager = createCollectionPager<RenderTemplateListResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/system/render/templates', query, cursor), { signal }),
    apply: (response, append, query) => {
      items.value = sortTemplateSummaries(append ? mergeCollectionItems(items.value, response.items, item => item.id) : response.items)
      if (!append) {
        catalogVersion += 1
        const complete = !query.query && !response.next_cursor && response.total === response.items.length
        const details = complete ? {} : { ...detailById.value }
        for (const item of response.items) delete details[item.id]
        detailById.value = details
        pendingWorkspaces.clear()
        workspaceLoading.value = false
      }
    },
  })
  const { total, nextCursor, loading, loadingMore, error } = pager
  function fetchTemplates(query?: CollectionQuery) { return pager.load(query) }
  function loadMore() { return pager.loadMore() }

  async function fetchTemplateWorkspace(templateId: string) {
    const requestCatalog = catalogVersion
    const request = ++workspaceRequest
    pendingWorkspaces.set(templateId, request)
    const isCurrentRequest = () => requestCatalog === catalogVersion && pendingWorkspaces.get(templateId) === request
    workspaceLoading.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateDetailResponse>(apiPath('/api/system/render/templates/{template_id}', { template_id: templateId }))
      // Only the latest request in the current catalog may update its workspace.
      if (!isCurrentRequest()) return response.template
      detailById.value = {
        ...detailById.value,
        [templateId]: response.template,
      }
      upsertTemplateSummary(response.template)
      return response.template
    } catch (err) {
      if (isCurrentRequest()) error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      if (pendingWorkspaces.get(templateId) === request) pendingWorkspaces.delete(templateId)
      workspaceLoading.value = pendingWorkspaces.size > 0
    }
  }

  async function previewTemplateHTML(templateId: string, payload: RenderTemplatePreviewHTMLRequest, signal?: AbortSignal) {
    return apiRequest<RenderTemplatePreviewHTMLResponse>(
      apiPath('/api/system/render/templates/{template_id}/preview-html', { template_id: templateId }),
      {
        body: payload,
        method: 'POST',
        signal,
      },
    )
  }

  async function downloadTemplateAsset(templateId: string, path: string, signal?: AbortSignal) {
    const params = new URLSearchParams({ path })
    return apiDownload(
      apiPath('/api/system/render/templates/{template_id}/asset', { template_id: templateId }, params),
      { signal },
    )
  }

  return {
    total, nextCursor, loadingMore, loadMore,
    clearError,
    detailById,
    downloadTemplateAsset,
    error,
    fetchTemplateWorkspace,
    fetchTemplates,
    items,
    loading,
    previewTemplateHTML,
    templateMap,
    workspaceLoading,
  }
})
