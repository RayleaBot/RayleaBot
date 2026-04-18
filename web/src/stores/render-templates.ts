import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError, apiRequest } from '@/lib/http'
import {
  cloneRenderTemplateDraft,
  formatRenderTemplateDraft,
  renderTemplateDraftEquals,
} from '@/lib/render-template-editor'
import type {
  RenderTemplateDetail,
  RenderTemplateDetailResponse,
  RenderTemplateListResponse,
  RenderTemplateRollbackRequest,
  RenderTemplateSource,
  RenderTemplateSourceResponse,
  RenderTemplateSourceUpdateRequest,
  RenderTemplateSummary,
  RenderTemplateTextDraft,
  RenderTemplateValidateRequest,
  RenderTemplateValidateResponse,
  RenderTemplateVersion,
  RenderTemplateVersionListResponse,
} from '@/types/api'

type SourceRevisionMeta = {
  revision_id: string
  template_id: string
}

function sortTemplateSummaries(items: RenderTemplateSummary[]) {
  return [...items].sort((left, right) => right.updated_at.localeCompare(left.updated_at))
}

export const useRenderTemplatesStore = defineStore('render-templates', () => {
  const items = ref<RenderTemplateSummary[]>([])
  const detailById = ref<Record<string, RenderTemplateDetail>>({})
  const sourceMetaById = ref<Record<string, SourceRevisionMeta>>({})
  const baseDraftById = ref<Record<string, RenderTemplateTextDraft>>({})
  const draftById = ref<Record<string, RenderTemplateTextDraft>>({})
  const versionsById = ref<Record<string, RenderTemplateVersion[]>>({})
  const validationById = ref<Record<string, RenderTemplateValidateResponse | null>>({})
  const conflictById = ref<Record<string, boolean>>({})
  const loading = ref(false)
  const workspaceLoading = ref(false)
  const savePending = ref(false)
  const validatePending = ref(false)
  const rollbackPending = ref(false)
  const error = ref<string | null>(null)

  const templateMap = computed(() => Object.fromEntries(items.value.map((item) => [item.id, item])))

  function upsertTemplateSummary(summary: RenderTemplateSummary) {
    const next = items.value.filter((item) => item.id !== summary.id)
    items.value = sortTemplateSummaries([summary, ...next])
  }

  function setWorkspaceState(
    templateId: string,
    detail: RenderTemplateDetail,
    sourceResponse: RenderTemplateSourceResponse,
    versions: RenderTemplateVersion[],
    options: { resetDraft: boolean },
  ) {
    detailById.value = {
      ...detailById.value,
      [templateId]: detail,
    }
    sourceMetaById.value = {
      ...sourceMetaById.value,
      [templateId]: {
        revision_id: sourceResponse.revision_id,
        template_id: sourceResponse.template_id,
      },
    }
    versionsById.value = {
      ...versionsById.value,
      [templateId]: versions,
    }

    const formattedDraft = formatRenderTemplateDraft(sourceResponse.source)
    baseDraftById.value = {
      ...baseDraftById.value,
      [templateId]: formattedDraft,
    }

    if (options.resetDraft || !draftById.value[templateId]) {
      draftById.value = {
        ...draftById.value,
        [templateId]: cloneRenderTemplateDraft(formattedDraft),
      }
    }

    conflictById.value = {
      ...conflictById.value,
      [templateId]: false,
    }

    upsertTemplateSummary(detail)
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

  async function fetchTemplateWorkspace(templateId: string, options: { resetDraft?: boolean } = {}) {
    workspaceLoading.value = true
    error.value = null
    try {
      const [detailResponse, sourceResponse, versionsResponse] = await Promise.all([
        apiRequest<RenderTemplateDetailResponse>(`/api/system/render/templates/${encodeURIComponent(templateId)}`),
        apiRequest<RenderTemplateSourceResponse>(`/api/system/render/templates/${encodeURIComponent(templateId)}/source`),
        apiRequest<RenderTemplateVersionListResponse>(`/api/system/render/templates/${encodeURIComponent(templateId)}/versions`),
      ])

      setWorkspaceState(
        templateId,
        detailResponse.template,
        sourceResponse,
        versionsResponse.items,
        { resetDraft: Boolean(options.resetDraft) },
      )

      return {
        detail: detailResponse.template,
        source: sourceResponse.source,
        versions: versionsResponse.items,
      }
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      workspaceLoading.value = false
    }
  }

  function updateDraftField(templateId: string, field: keyof RenderTemplateTextDraft, value: string) {
    const current = draftById.value[templateId]
    if (!current) {
      return
    }

    draftById.value = {
      ...draftById.value,
      [templateId]: {
        ...current,
        [field]: value,
      },
    }

    validationById.value = {
      ...validationById.value,
      [templateId]: null,
    }
  }

  function clearError() {
    error.value = null
  }

  function replaceDraft(templateId: string, draft: RenderTemplateTextDraft) {
    draftById.value = {
      ...draftById.value,
      [templateId]: cloneRenderTemplateDraft(draft),
    }
    validationById.value = {
      ...validationById.value,
      [templateId]: null,
    }
  }

  function resetDraft(templateId: string) {
    const baseDraft = baseDraftById.value[templateId]
    if (!baseDraft) {
      return
    }

    draftById.value = {
      ...draftById.value,
      [templateId]: cloneRenderTemplateDraft(baseDraft),
    }
    validationById.value = {
      ...validationById.value,
      [templateId]: null,
    }
    conflictById.value = {
      ...conflictById.value,
      [templateId]: false,
    }
  }

  async function validateTemplate(templateId: string, request?: RenderTemplateValidateRequest) {
    validatePending.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateValidateResponse>(
        `/api/system/render/templates/${encodeURIComponent(templateId)}/validate`,
        {
          method: 'POST',
          body: request,
        },
      )

      validationById.value = {
        ...validationById.value,
        [templateId]: response,
      }
      return response
    } catch (err) {
      error.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      validatePending.value = false
    }
  }

  async function saveTemplate(templateId: string, request: RenderTemplateSourceUpdateRequest) {
    savePending.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateDetailResponse>(
        `/api/system/render/templates/${encodeURIComponent(templateId)}/source`,
        {
          method: 'PUT',
          body: request,
        },
      )

      await fetchTemplateWorkspace(templateId, { resetDraft: true })
      validationById.value = {
        ...validationById.value,
        [templateId]: response.template.last_validation.valid
          ? {
            valid: response.template.last_validation.valid,
            issues: [],
            normalized_manifest: request.source.manifest_json,
          }
          : validationById.value[templateId] ?? null,
      }
      conflictById.value = {
        ...conflictById.value,
        [templateId]: false,
      }
      return response
    } catch (err) {
      if (err instanceof ApiError && err.code === 'platform.template_revision_conflict') {
        conflictById.value = {
          ...conflictById.value,
          [templateId]: true,
        }
      }
      error.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      savePending.value = false
    }
  }

  async function rollbackTemplate(templateId: string, request: RenderTemplateRollbackRequest) {
    rollbackPending.value = true
    error.value = null
    try {
      const response = await apiRequest<RenderTemplateDetailResponse>(
        `/api/system/render/templates/${encodeURIComponent(templateId)}/rollback`,
        {
          method: 'POST',
          body: request,
        },
      )

      await fetchTemplateWorkspace(templateId, { resetDraft: true })
      conflictById.value = {
        ...conflictById.value,
        [templateId]: false,
      }
      return response
    } catch (err) {
      if (err instanceof ApiError && err.code === 'platform.template_revision_conflict') {
        conflictById.value = {
          ...conflictById.value,
          [templateId]: true,
        }
      }
      error.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      rollbackPending.value = false
    }
  }

  function getDraft(templateId: string) {
    return draftById.value[templateId] ?? null
  }

  function getBaseRevisionId(templateId: string) {
    return sourceMetaById.value[templateId]?.revision_id ?? detailById.value[templateId]?.current_revision_id ?? null
  }

  function isDraftDirty(templateId: string) {
    return !renderTemplateDraftEquals(draftById.value[templateId] ?? null, baseDraftById.value[templateId] ?? null)
  }

  return {
    baseDraftById,
    conflictById,
    detailById,
    draftById,
    error,
    clearError,
    getBaseRevisionId,
    getDraft,
    isDraftDirty,
    items,
    loading,
    rollbackPending,
    savePending,
    sourceMetaById,
    templateMap,
    validationById,
    validatePending,
    versionsById,
    workspaceLoading,
    fetchTemplates,
    fetchTemplateWorkspace,
    replaceDraft,
    resetDraft,
    rollbackTemplate,
    saveTemplate,
    updateDraftField,
    validateTemplate,
  }
})
