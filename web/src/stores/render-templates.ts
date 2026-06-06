import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import type {
  RenderTemplateDetail,
  RenderTemplateDetailResponse,
  RenderTemplateListResponse,
  RenderTemplatePreviewHTMLRequest,
  RenderTemplatePreviewHTMLResponse,
  RenderTemplateSummary,
} from '@/types/api'

function sortTemplateSummaries(items: RenderTemplateSummary[]) {
  return [...items].sort((left, right) => right.updated_at.localeCompare(left.updated_at))
}

export const useRenderTemplatesStore = defineStore('render-templates', () => {
  const items = ref<RenderTemplateSummary[]>([])
  const detailById = ref<Record<string, RenderTemplateDetail>>({})
  const loading = ref(false)
  const workspaceLoading = ref(false)
  const error = ref<string | null>(null)

  const templateMap = computed(() => Object.fromEntries(items.value.map((item) => [item.id, item])))

  function upsertTemplateSummary(summary: RenderTemplateSummary) {
    const next = items.value.filter((item) => item.id !== summary.id)
    items.value = sortTemplateSummaries([summary, ...next])
  }

  function clearError() {
    error.value = null
  }

  async function fetchTemplates() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateListResponse>('/api/system/render/templates')
      items.value = sortTemplateSummaries(response.items)
      return response
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchTemplateWorkspace(templateId: string) {
    workspaceLoading.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateDetailResponse>(`/api/system/render/templates/${encodeURIComponent(templateId)}`)
      detailById.value = {
        ...detailById.value,
        [templateId]: response.template,
      }
      upsertTemplateSummary(response.template)
      return response.template
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      workspaceLoading.value = false
    }
  }

  async function previewTemplateHTML(templateId: string, payload: RenderTemplatePreviewHTMLRequest, signal?: AbortSignal) {
    return apiRequest<RenderTemplatePreviewHTMLResponse>(
      `/api/system/render/templates/${encodeURIComponent(templateId)}/preview-html`,
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
      `/api/system/render/templates/${encodeURIComponent(templateId)}/asset?${params.toString()}`,
      { signal },
    )
  }

  return {
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
