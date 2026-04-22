import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  RenderTemplateDetail,
  RenderTemplateDetailResponse,
  RenderTemplateListResponse,
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

  return {
    clearError,
    detailById,
    error,
    fetchTemplateWorkspace,
    fetchTemplates,
    items,
    loading,
    templateMap,
    workspaceLoading,
  }
})
