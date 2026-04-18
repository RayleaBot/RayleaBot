<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { apiDownload } from '@/lib/http'
import {
  buildRenderTemplateSchemaNodes,
  parseRenderTemplateDraft,
  parseRenderTemplatePreviewData,
} from '@/lib/render-template-editor'
import { t } from '@/i18n'
import { getTaskStatusLabel } from '@/lib/display'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'
import type {
  RenderPreviewRequest,
  RenderTemplateTextFieldKey,
  RenderTemplateVersion,
} from '@/types/api'

const route = useRoute()
const router = useRouter()
const renderTemplatesStore = useRenderTemplatesStore()
const systemStore = useSystemStore()
const tasksStore = useTasksStore()

const {
  conflictById,
  detailById,
  draftById,
  error,
  items,
  loading,
  rollbackPending,
  savePending,
  validationById,
  validatePending,
  versionsById,
  workspaceLoading,
} = storeToRefs(renderTemplatesStore)
const { previewPending } = storeToRefs(systemStore)

const activeFile = ref<RenderTemplateTextFieldKey>('manifest_json')
const hasRequestedList = ref(false)
const previewDataByTemplate = ref<Record<string, string>>({})
const previewThemeByTemplate = ref<Record<string, string>>({})
const previewOutputByTemplate = ref<Record<string, 'png' | 'jpeg'>>({})
const previewTaskIdByTemplate = ref<Record<string, string>>({})
const saveMessageByTemplate = ref<Record<string, string>>({})
const rollbackDialogOpen = ref(false)
const rollbackTarget = ref<RenderTemplateVersion | null>(null)
const rollbackMessage = ref('恢复到已验证版本')
const previewImageSrc = ref('')
let previewImageLoadVersion = 0
let previewWatcherActive = true

const fileTitleMap: Record<RenderTemplateTextFieldKey, string> = {
  manifest_json: t('renderTemplates.inputFileManifest'),
  html: t('renderTemplates.inputFileHtml'),
  stylesheet: t('renderTemplates.inputFileStylesheet'),
  input_schema_json: t('renderTemplates.inputFileInputSchema'),
}

const activeTemplateId = computed(() => (
  typeof route.params.templateId === 'string' && route.params.templateId
    ? route.params.templateId
    : ''
))

const currentTemplate = computed(() => (
  activeTemplateId.value ? detailById.value[activeTemplateId.value] ?? null : null
))

const currentDraft = computed(() => (
  activeTemplateId.value ? draftById.value[activeTemplateId.value] ?? null : null
))

const currentVersions = computed(() => (
  activeTemplateId.value ? versionsById.value[activeTemplateId.value] ?? [] : []
))

const currentValidation = computed(() => (
  activeTemplateId.value ? validationById.value[activeTemplateId.value] ?? null : null
))

const currentHasConflict = computed(() => (
  activeTemplateId.value ? Boolean(conflictById.value[activeTemplateId.value]) : false
))

const currentBaseRevisionId = computed(() => (
  activeTemplateId.value ? renderTemplatesStore.getBaseRevisionId(activeTemplateId.value) : null
))

const currentDraftDirty = computed(() => (
  activeTemplateId.value ? renderTemplatesStore.isDraftDirty(activeTemplateId.value) : false
))

const currentSaveMessage = computed({
  get() {
    if (!activeTemplateId.value) {
      return ''
    }
    return saveMessageByTemplate.value[activeTemplateId.value] ?? ''
  },
  set(value: string) {
    if (!activeTemplateId.value) {
      return
    }
    saveMessageByTemplate.value = {
      ...saveMessageByTemplate.value,
      [activeTemplateId.value]: value,
    }
  },
})

const currentPreviewTheme = computed({
  get() {
    if (!activeTemplateId.value) {
      return 'default'
    }
    return previewThemeByTemplate.value[activeTemplateId.value] ?? 'default'
  },
  set(value: string) {
    if (!activeTemplateId.value) {
      return
    }
    previewThemeByTemplate.value = {
      ...previewThemeByTemplate.value,
      [activeTemplateId.value]: value,
    }
  },
})

const currentPreviewOutput = computed({
  get() {
    if (!activeTemplateId.value) {
      return 'png'
    }
    return previewOutputByTemplate.value[activeTemplateId.value] ?? 'png'
  },
  set(value: 'png' | 'jpeg') {
    if (!activeTemplateId.value) {
      return
    }
    previewOutputByTemplate.value = {
      ...previewOutputByTemplate.value,
      [activeTemplateId.value]: value,
    }
  },
})

const currentPreviewDataText = computed({
  get() {
    if (!activeTemplateId.value) {
      return '{}'
    }
    return previewDataByTemplate.value[activeTemplateId.value] ?? '{\n  "title": "帮助菜单"\n}'
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

const currentPreviewTaskId = computed(() => (
  activeTemplateId.value ? previewTaskIdByTemplate.value[activeTemplateId.value] ?? '' : ''
))

const currentPreviewTask = computed(() => (
  currentPreviewTaskId.value
    ? tasksStore.items.find((item) => item.task_id === currentPreviewTaskId.value) ?? null
    : null
))

const parsedDraftResult = computed(() => (
  currentDraft.value ? parseRenderTemplateDraft(currentDraft.value) : { issues: [], source: null }
))

const localPreviewParseResult = computed(() => parseRenderTemplatePreviewData(currentPreviewDataText.value))

const localIssues = computed(() => {
  const issues = [...parsedDraftResult.value.issues]
  if (localPreviewParseResult.value.issue) {
    issues.push(localPreviewParseResult.value.issue)
  }
  return issues
})

const schemaNodes = computed(() => buildRenderTemplateSchemaNodes(parsedDraftResult.value.source?.input_schema_json ?? null))
const schemaPreviewDescription = computed(() => {
  if (localIssues.value.some((issue) => issue.field === 'input_schema_json')) {
    return t('renderTemplates.schemaPreviewInvalid')
  }

  return t('renderTemplates.schemaPreviewEmpty')
})

const previewImageUrl = computed(() => {
  const imageUrl = currentPreviewTask.value?.result?.details?.image_url
  return typeof imageUrl === 'string' ? imageUrl : ''
})

const canValidate = computed(() => Boolean(activeTemplateId.value) && Boolean(currentDraft.value) && !validatePending.value)
const canPreview = computed(() => Boolean(activeTemplateId.value) && Boolean(currentDraft.value) && !previewPending.value)
const canSave = computed(() => (
  Boolean(activeTemplateId.value)
  && Boolean(currentDraft.value)
  && Boolean(currentBaseRevisionId.value)
  && Boolean(currentSaveMessage.value.trim())
  && !savePending.value
))
const canRollback = computed(() => Boolean(activeTemplateId.value) && Boolean(currentBaseRevisionId.value) && !rollbackPending.value)

const templateLocalIssueGroups = computed(() => {
  const groups = new Map<string, string[]>()
  for (const issue of localIssues.value) {
    const existing = groups.get(issue.field) ?? []
    existing.push(issue.message)
    groups.set(issue.field, existing)
  }
  return Array.from(groups.entries())
})

async function loadTemplateList() {
  hasRequestedList.value = true
  try {
    await renderTemplatesStore.fetchTemplates()
  } catch {
    // store error state drives the page
  }
}

async function loadTemplateWorkspace(templateId: string, options: { force?: boolean; resetDraft?: boolean } = {}) {
  if (!options.force && detailById.value[templateId] && draftById.value[templateId] && versionsById.value[templateId]) {
    return
  }

  try {
    await renderTemplatesStore.fetchTemplateWorkspace(templateId, { resetDraft: options.resetDraft })
  } catch {
    // store error state drives the page
  }
}

function ensurePreviewDefaults(templateId: string) {
  if (!previewDataByTemplate.value[templateId]) {
    previewDataByTemplate.value = {
      ...previewDataByTemplate.value,
      [templateId]: '{\n  "title": "帮助菜单"\n}',
    }
  }

  if (!previewThemeByTemplate.value[templateId]) {
    previewThemeByTemplate.value = {
      ...previewThemeByTemplate.value,
      [templateId]: 'default',
    }
  }

  if (!previewOutputByTemplate.value[templateId]) {
    previewOutputByTemplate.value = {
      ...previewOutputByTemplate.value,
      [templateId]: 'png',
    }
  }
}

async function syncRouteTemplate() {
  if (items.value.length === 0) {
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

  ensurePreviewDefaults(activeTemplateId.value)
  await loadTemplateWorkspace(activeTemplateId.value)
}

async function selectTemplate(templateId: string) {
  if (templateId === activeTemplateId.value) {
    return
  }

  await router.push({
    name: 'render-templates',
    params: {
      templateId,
    },
  })
}

function updateDraft(field: RenderTemplateTextFieldKey, value: string) {
  if (!activeTemplateId.value) {
    return
  }

  renderTemplatesStore.updateDraftField(activeTemplateId.value, field, value)
}

function readDraftField(field: RenderTemplateTextFieldKey) {
  if (!currentDraft.value) {
    return ''
  }

  return currentDraft.value[field]
}

function resetPreviewImage() {
  if (!previewImageSrc.value) {
    return
  }

  window.URL.revokeObjectURL(previewImageSrc.value)
  previewImageSrc.value = ''
}

function localValidationBlocked() {
  if (localIssues.value.length === 0) {
    return false
  }

  notifyError(localIssues.value[0]?.message ?? t('renderTemplates.localIssuesTitle'))
  return true
}

async function validateTemplate() {
  if (!activeTemplateId.value || !currentDraft.value || localValidationBlocked()) {
    return
  }

  try {
    await renderTemplatesStore.validateTemplate(activeTemplateId.value, {
      source: parsedDraftResult.value.source ?? undefined,
    })
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

async function previewTemplate() {
  if (!activeTemplateId.value || !currentDraft.value || localValidationBlocked()) {
    return
  }

  if (!localPreviewParseResult.value.data) {
    return
  }

  const payload: RenderPreviewRequest = {
    template: activeTemplateId.value,
    theme: currentPreviewTheme.value.trim() || undefined,
    output: currentPreviewOutput.value,
    data: localPreviewParseResult.value.data,
    ...(currentDraftDirty.value && parsedDraftResult.value.source
      ? {
        draft: {
          source: parsedDraftResult.value.source,
        },
      }
      : {}),
  }

  try {
    const response = await systemStore.previewRender(payload)
    previewTaskIdByTemplate.value = {
      ...previewTaskIdByTemplate.value,
      [activeTemplateId.value]: response.task_id,
    }
    await tasksStore.fetchTask(response.task_id, { makeCurrent: false })
    notifySuccess(t('renderTemplates.previewAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

async function saveTemplate() {
  if (!activeTemplateId.value || !currentBaseRevisionId.value || !parsedDraftResult.value.source || localValidationBlocked()) {
    return
  }

  try {
    await renderTemplatesStore.saveTemplate(activeTemplateId.value, {
      base_revision_id: currentBaseRevisionId.value,
      message: currentSaveMessage.value.trim(),
      source: parsedDraftResult.value.source,
    })
    currentSaveMessage.value = ''
    notifySuccess(t('renderTemplates.saveAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

async function reloadLatest() {
  if (!activeTemplateId.value) {
    return
  }

  await loadTemplateWorkspace(activeTemplateId.value, { force: true, resetDraft: true })
}

function resetDraftToCurrentVersion() {
  if (!activeTemplateId.value) {
    return
  }

  renderTemplatesStore.resetDraft(activeTemplateId.value)
}

function openRollbackDialog(version: RenderTemplateVersion) {
  rollbackTarget.value = version
  rollbackMessage.value = version.message ?? '恢复到已验证版本'
  rollbackDialogOpen.value = true
}

async function confirmRollback() {
  if (!activeTemplateId.value || !rollbackTarget.value || !currentBaseRevisionId.value || !rollbackMessage.value.trim()) {
    return
  }

  try {
    await renderTemplatesStore.rollbackTemplate(activeTemplateId.value, {
      target_revision_id: rollbackTarget.value.revision_id,
      base_revision_id: currentBaseRevisionId.value,
      message: rollbackMessage.value.trim(),
    })
    rollbackDialogOpen.value = false
    rollbackTarget.value = null
    notifySuccess(t('renderTemplates.rollbackAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

function formatTemplateSize(width?: number, height?: number) {
  if (!width || !height) {
    return t('display.empty')
  }

  return `${width} × ${height}`
}

function formatVersionKind(kind?: string) {
  if (kind === 'rollback') {
    return t('renderTemplates.versionKind.rollback')
  }
  return t('renderTemplates.versionKind.save')
}

watch([items, () => route.params.templateId], () => {
  void syncRouteTemplate()
}, { immediate: true })

watch(activeTemplateId, (templateId) => {
  if (!templateId) {
    return
  }

  ensurePreviewDefaults(templateId)
  if (!saveMessageByTemplate.value[templateId]) {
    saveMessageByTemplate.value = {
      ...saveMessageByTemplate.value,
      [templateId]: '',
    }
  }
}, { immediate: true })

watch(previewImageUrl, async (imageUrl, _, onCleanup) => {
  const requestVersion = ++previewImageLoadVersion
  let cancelled = false
  const controller = new AbortController()

  onCleanup(() => {
    cancelled = true
    controller.abort()
  })

  if (!imageUrl) {
    resetPreviewImage()
    return
  }

  try {
    const { blob } = await apiDownload(imageUrl, { signal: controller.signal })
    if (cancelled || !previewWatcherActive || requestVersion !== previewImageLoadVersion) {
      return
    }

    const nextPreviewUrl = window.URL.createObjectURL(blob)
    if (cancelled || !previewWatcherActive || requestVersion !== previewImageLoadVersion) {
      window.URL.revokeObjectURL(nextPreviewUrl)
      return
    }

    resetPreviewImage()
    previewImageSrc.value = nextPreviewUrl
  } catch {
    if (cancelled || requestVersion !== previewImageLoadVersion) {
      return
    }
    resetPreviewImage()
  }
}, { immediate: true })

onMounted(() => {
  void loadTemplateList()
})

onBeforeUnmount(() => {
  previewWatcherActive = false
  previewImageLoadVersion += 1
  resetPreviewImage()
})
</script>

<template>
  <AppPage :title="t('renderTemplates.title')" :description="t('renderTemplates.subtitle')" full-height>
    <template #extra>
      <div class="table-actions">
        <a-button :loading="loading || workspaceLoading" @click="loadTemplateList">
          {{ t('dashboard.refresh') }}
        </a-button>
        <a-button :disabled="!activeTemplateId" @click="reloadLatest">
          {{ t('renderTemplates.reloadAction') }}
        </a-button>
      </div>
    </template>

    <div v-if="error || currentHasConflict || templateLocalIssueGroups.length > 0" class="render-templates__alerts">
      <a-alert
        v-if="error"
        type="error"
        show-icon
        :message="t('errors.common.actionFailed')"
        :description="error"
      />
      <a-alert
        v-if="currentHasConflict"
        type="warning"
        show-icon
        :message="t('renderTemplates.versionChangedTitle')"
        :description="t('renderTemplates.versionChangedDescription')"
      />
      <a-alert
        v-if="templateLocalIssueGroups.length > 0"
        type="error"
        show-icon
        :message="t('renderTemplates.localIssuesTitle')"
      >
        <template #description>
          <ul class="render-templates__issue-list">
            <li v-for="[field, messages] in templateLocalIssueGroups" :key="field">
              <strong>{{ fileTitleMap[field as RenderTemplateTextFieldKey] ?? field }}</strong>
              <span>{{ messages.join('；') }}</span>
            </li>
          </ul>
        </template>
      </a-alert>
    </div>

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

    <div v-else class="render-templates-layout">
      <div class="render-templates-layout__sidebar">
        <AppCard :title="t('renderTemplates.templateList')" borderless class="render-templates-card render-templates-card--nav">
          <template #extra>
            <span class="render-templates-card__meta">{{ items.length }}</span>
          </template>
          <div class="template-nav-list">
            <button
              v-for="template in items"
              :key="template.id"
              type="button"
              class="template-nav-item"
              :class="{ 'is-active': template.id === activeTemplateId }"
              @click="selectTemplate(template.id)"
            >
              <div class="template-nav-item__header">
                <strong>{{ template.id }}</strong>
                <a-tag size="small">{{ template.version }}</a-tag>
              </div>
              <div class="template-nav-item__meta">
                <span>{{ formatTemplateSize(template.width, template.height) }}</span>
                <span>{{ formatDateTime(template.updated_at) }}</span>
              </div>
            </button>
          </div>
        </AppCard>

        <AppCard :title="t('renderTemplates.currentVersion')" borderless class="render-templates-card">
          <a-skeleton :loading="workspaceLoading && !currentTemplate" active :paragraph="{ rows: 4 }">
            <div v-if="currentTemplate" class="summary-grid">
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.id') }}</span>
                <strong>{{ currentTemplate.id }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.size') }}</span>
                <strong>{{ formatTemplateSize(currentTemplate.width, currentTemplate.height) }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.version') }}</span>
                <strong>{{ currentTemplate.version }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.updatedAt') }}</span>
                <strong>{{ formatDateTime(currentTemplate.updated_at) }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.currentRevision') }}</span>
                <strong>{{ currentTemplate.current_revision.revision_id }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.message') }}</span>
                <strong>{{ currentTemplate.current_revision.message || t('display.empty') }}</strong>
              </div>
            </div>
          </a-skeleton>
        </AppCard>

        <AppCard :title="t('renderTemplates.validationStatus')" borderless class="render-templates-card">
          <a-skeleton :loading="workspaceLoading && !currentTemplate" active :paragraph="{ rows: 3 }">
            <div v-if="currentTemplate" class="summary-grid">
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.checkedAt') }}</span>
                <strong>{{ formatDateTime(currentTemplate.last_validation.checked_at) }}</strong>
              </div>
              <div class="summary-item">
                <span>{{ t('renderTemplates.fields.issueCount') }}</span>
                <strong>{{ currentTemplate.last_validation.issue_count }}</strong>
              </div>
              <div class="summary-item summary-item--full">
                <span>{{ currentTemplate.last_validation.valid ? t('renderTemplates.validationValid') : t('renderTemplates.validationInvalid') }}</span>
                <a-progress
                  :percent="currentTemplate.last_validation.valid ? 100 : Math.max(15, 100 - currentTemplate.last_validation.issue_count * 20)"
                  :status="currentTemplate.last_validation.valid ? 'success' : 'exception'"
                  size="small"
                />
              </div>
            </div>
          </a-skeleton>
        </AppCard>

        <AppCard :title="t('renderTemplates.versionHistory')" borderless class="render-templates-card render-templates-card--history">
          <template #extra>
            <span class="render-templates-card__meta">{{ currentVersions.length }}</span>
          </template>
          <div v-if="currentVersions.length" class="version-list">
            <section
              v-for="version in currentVersions"
              :key="version.revision_id"
              class="version-item"
            >
              <div class="version-item__header">
                <div>
                  <strong>{{ formatVersionKind(version.kind) }}</strong>
                  <small>{{ formatDateTime(version.saved_at) }}</small>
                </div>
                <a-tag size="small">{{ version.template_version }}</a-tag>
              </div>
              <code>{{ version.revision_id }}</code>
              <p>{{ version.message || t('display.empty') }}</p>
              <a-button
                size="small"
                type="link"
                class="version-item__action"
                :disabled="!canRollback || version.revision_id === currentBaseRevisionId"
                @click="openRollbackDialog(version)"
              >
                {{ t('renderTemplates.rollbackAction') }}
              </a-button>
            </section>
          </div>
          <a-empty v-else :description="t('renderTemplates.noVersions')" />
        </AppCard>
      </div>

      <div class="render-templates-layout__main">
        <AppCard borderless class="render-templates-card render-templates-card--editor">
          <template #title>
            <div class="editor-card__title">
              <div>
                <strong>{{ t('renderTemplates.editorTitle') }}</strong>
                <p>{{ t('renderTemplates.editorSubtitle') }}</p>
              </div>
              <div class="editor-card__badges">
                <a-tag :color="currentDraftDirty ? 'warning' : 'success'">
                  {{ currentDraftDirty ? t('renderTemplates.draftChanged') : t('renderTemplates.draftSynced') }}
                </a-tag>
                <a-tag>{{ currentTemplate?.files.manifest || 'template.json' }}</a-tag>
              </div>
            </div>
          </template>

          <div v-if="currentDraft" class="editor-workspace">
            <a-tabs v-model:activeKey="activeFile" class="editor-tabs">
              <a-tab-pane
                v-for="(label, key) in fileTitleMap"
                :key="key"
                :tab="label"
              />
            </a-tabs>

            <div class="editor-surface">
              <a-textarea
                id="render-template-editor-input"
                :value="readDraftField(activeFile)"
                :rows="24"
                class="render-template-textarea"
                :auto-size="false"
                @update:value="(value) => updateDraft(activeFile, value)"
              />
            </div>

            <div class="editor-actions-grid">
              <a-form layout="vertical" class="editor-actions-form">
                <a-form-item :label="t('renderTemplates.messageLabel')">
                  <a-input
                    v-model:value="currentSaveMessage"
                    :aria-label="t('renderTemplates.messageLabel')"
                    :placeholder="t('renderTemplates.messagePlaceholder')"
                  />
                </a-form-item>
              </a-form>

              <div class="editor-actions">
                <a-button data-testid="render-template-validate-button" :loading="validatePending" :disabled="!canValidate" @click="validateTemplate">
                  {{ t('renderTemplates.validateAction') }}
                </a-button>
                <a-button data-testid="render-template-preview-button" :loading="previewPending" :disabled="!canPreview" @click="previewTemplate">
                  {{ t('renderTemplates.previewAction') }}
                </a-button>
                <a-button :disabled="!activeTemplateId" @click="resetDraftToCurrentVersion">
                  {{ t('renderTemplates.resetDraftAction') }}
                </a-button>
                <a-button data-testid="render-template-save-button" type="primary" :loading="savePending" :disabled="!canSave" @click="saveTemplate">
                  {{ t('renderTemplates.saveAction') }}
                </a-button>
              </div>
            </div>
          </div>
        </AppCard>

        <div class="render-templates-bottom-grid">
          <AppCard :title="t('renderTemplates.schemaPreviewTitle')" borderless class="render-templates-card">
            <template #extra>
              <span class="render-templates-card__meta">{{ schemaNodes.length }}</span>
            </template>

            <div v-if="schemaNodes.length" class="schema-list">
              <section
                v-for="node in schemaNodes"
                :key="node.key"
                class="schema-item"
                :style="{ '--schema-depth': String(node.depth) }"
              >
                <div class="schema-item__header">
                  <div class="schema-item__title">
                    <strong>{{ node.label }}</strong>
                    <code>{{ node.path || '$root' }}</code>
                  </div>
                  <div class="schema-item__badges">
                    <a-tag size="small">{{ node.type }}</a-tag>
                    <a-tag size="small" :color="node.required ? 'red' : 'default'">
                      {{ node.required ? t('renderTemplates.required.yes') : t('renderTemplates.required.no') }}
                    </a-tag>
                  </div>
                </div>
                <p>{{ node.description || t('display.empty') }}</p>
              </section>
            </div>
            <a-empty
              v-else
              :description="schemaPreviewDescription"
            />
          </AppCard>

          <AppCard :title="t('renderTemplates.validationTitle')" borderless class="render-templates-card">
            <template #extra>
              <span class="render-templates-card__meta">
                {{ currentValidation ? `${t('renderTemplates.validationIssueCount')} ${currentValidation.issues.length}` : t('display.empty') }}
              </span>
            </template>

            <div v-if="currentValidation" class="validation-panel">
              <a-alert
                :type="currentValidation.valid ? 'success' : 'warning'"
                show-icon
                :message="currentValidation.valid ? t('renderTemplates.validationPassed') : t('renderTemplates.validationInvalid')"
              />

              <div v-if="currentValidation.issues.length" class="validation-issues">
                <section v-for="issue in currentValidation.issues" :key="`${issue.code}-${issue.path || issue.message}`" class="validation-issue">
                  <div class="validation-issue__header">
                    <a-tag color="error">{{ issue.code }}</a-tag>
                    <strong>{{ issue.message }}</strong>
                  </div>
                  <small>{{ issue.path || t('display.empty') }}</small>
                </section>
              </div>

              <div class="json-preview">
                <div class="json-preview__header">
                  <strong>manifest</strong>
                </div>
                <pre>{{ JSON.stringify(currentValidation.normalized_manifest, null, 2) }}</pre>
              </div>
            </div>
            <a-empty v-else :description="t('renderTemplates.validationEmpty')" />
          </AppCard>

          <AppCard :title="t('renderTemplates.previewTitle')" borderless class="render-templates-card render-templates-card--preview">
            <template #extra>
              <span class="render-templates-card__meta">{{ currentPreviewTask?.task_id || t('display.empty') }}</span>
            </template>

            <a-form layout="vertical" class="preview-form">
              <a-form-item :label="t('renderTemplates.previewTheme')">
                <a-input v-model:value="currentPreviewTheme" :aria-label="t('renderTemplates.previewTheme')" />
              </a-form-item>
              <a-form-item :label="t('renderTemplates.previewOutput')">
                <a-radio-group v-model:value="currentPreviewOutput" button-style="solid">
                  <a-radio-button value="png">png</a-radio-button>
                  <a-radio-button value="jpeg">jpeg</a-radio-button>
                </a-radio-group>
              </a-form-item>
              <a-form-item :label="t('renderTemplates.previewData')">
                <a-textarea
                  v-model:value="currentPreviewDataText"
                  :rows="8"
                  :aria-label="t('renderTemplates.previewData')"
                  :placeholder="t('renderTemplates.previewDataPlaceholder')"
                />
              </a-form-item>
            </a-form>

            <div v-if="currentPreviewTask" class="preview-result" data-testid="render-template-preview-result">
              <div class="preview-result__meta">
                <div class="summary-item">
                  <span>{{ t('renderTemplates.previewTask') }}</span>
                  <strong>{{ currentPreviewTask.task_id }}</strong>
                </div>
                <div class="summary-item">
                  <span>{{ t('tasks.fields.status') }}</span>
                  <strong>{{ getTaskStatusLabel(currentPreviewTask.status) }}</strong>
                </div>
                <div class="summary-item">
                  <span>{{ t('renderTemplates.previewArtifact') }}</span>
                  <strong>{{ currentPreviewTask.result?.details?.artifact_id || t('display.empty') }}</strong>
                </div>
                <div class="summary-item">
                  <span>{{ t('renderTemplates.previewCache') }}</span>
                  <strong>{{ currentPreviewTask.result?.details?.from_cache ? t('renderTemplates.previewFromCache') : t('renderTemplates.previewFresh') }}</strong>
                </div>
              </div>

              <div v-if="currentPreviewTask.error" class="preview-result__error">
                <a-alert
                  type="error"
                  show-icon
                  :message="currentPreviewTask.error.code"
                  :description="currentPreviewTask.error.message"
                />
              </div>

              <img
                v-if="previewImageSrc"
                :src="previewImageSrc"
                :alt="t('renderTemplates.previewImageAlt')"
                class="preview-result__image"
              />

              <RouterLink
                class="preview-result__link"
                :to="{ name: 'tasks', query: { task_id: currentPreviewTask.task_id } }"
              >
                {{ t('renderTemplates.previewTaskDetail') }}
              </RouterLink>
            </div>
            <a-empty v-else :description="t('renderTemplates.previewEmpty')" />
          </AppCard>
        </div>
      </div>
    </div>

    <a-modal
      v-model:open="rollbackDialogOpen"
      :title="t('renderTemplates.rollbackConfirmTitle')"
      :confirm-loading="rollbackPending"
      :ok-button-props="{ disabled: !rollbackMessage.trim() }"
      :ok-text="t('renderTemplates.rollbackConfirmAction')"
      :cancel-text="t('shell.cancel')"
      @ok="confirmRollback"
    >
      <p>{{ t('renderTemplates.rollbackConfirmMessage') }}</p>
      <a-form layout="vertical">
        <a-form-item :label="t('renderTemplates.fields.revisionId')">
          <a-input :value="rollbackTarget?.revision_id || ''" disabled />
        </a-form-item>
          <a-form-item :label="t('renderTemplates.rollbackMessageLabel')">
            <a-input
              v-model:value="rollbackMessage"
              :aria-label="t('renderTemplates.rollbackMessageLabel')"
              :placeholder="t('renderTemplates.rollbackMessagePlaceholder')"
            />
          </a-form-item>
      </a-form>
    </a-modal>
  </AppPage>
</template>

<style lang="scss" scoped>
.render-templates__alerts {
  display: grid;
  gap: 12px;
}

.render-templates__issue-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding-left: 18px;

  li {
    display: grid;
    gap: 4px;
  }
}

.render-templates-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 12px;
  min-height: 0;
  height: 100%;
}

.render-templates-layout__sidebar,
.render-templates-layout__main {
  display: grid;
  gap: 12px;
  min-height: 0;
}

.render-templates-layout__main {
  align-content: start;
}

.render-templates-card {
  min-height: 0;
}

.render-templates-card__meta {
  color: var(--app-text-secondary);
  font-size: 0.82rem;
}

.render-templates-card--nav,
.render-templates-card--history {
  height: 100%;
}

.render-templates-card--editor {
  position: relative;
  z-index: 1;
}

.render-templates-card--nav :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px;
}

.render-templates-card--history :deep(.ant-card-body) {
  max-height: 420px;
  overflow: auto;
}

.template-nav-list {
  display: grid;
  gap: 8px;
  overflow: auto;
}

.template-nav-item {
  appearance: none;
  border: 1px solid var(--app-border);
  background: linear-gradient(180deg, color-mix(in srgb, var(--surface) 92%, white 8%), var(--surface-soft));
  border-radius: 14px;
  padding: 14px;
  display: grid;
  gap: 10px;
  text-align: left;
  color: var(--app-text);
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.template-nav-item:hover {
  border-color: color-mix(in srgb, var(--app-primary) 28%, var(--app-border));
  transform: translateY(-1px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.06);
}

.template-nav-item.is-active {
  border-color: color-mix(in srgb, var(--app-primary) 38%, var(--app-border));
  background: linear-gradient(180deg, color-mix(in srgb, var(--app-primary) 8%, white 92%), color-mix(in srgb, var(--app-primary) 6%, var(--surface-soft)));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--app-primary) 18%, transparent);
}

.template-nav-item__header,
.template-nav-item__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.template-nav-item__meta {
  color: var(--app-text-secondary);
  font-size: 0.82rem;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.summary-item {
  display: grid;
  gap: 4px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, var(--surface-soft) 88%, white 12%);

  span {
    color: var(--app-text-secondary);
    font-size: 0.78rem;
  }

  strong {
    font-size: 0.92rem;
    line-height: 1.45;
    word-break: break-word;
  }
}

.summary-item--full {
  grid-column: 1 / -1;
}

.version-list {
  display: grid;
  gap: 10px;
}

.version-item {
  display: grid;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, var(--surface-soft) 90%, white 10%);

  code {
    color: var(--app-text-secondary);
    font-size: 0.8rem;
  }

  p {
    margin: 0;
    color: var(--app-text);
    line-height: 1.5;
  }
}

.version-item__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;

  div {
    display: grid;
    gap: 4px;
  }

  small {
    color: var(--app-text-secondary);
  }
}

.version-item__action {
  padding-inline: 0;
}

.editor-card__title {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;

  p {
    margin: 4px 0 0;
    color: var(--app-text-secondary);
    font-size: 0.86rem;
    line-height: 1.5;
  }
}

.editor-card__badges {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.editor-workspace {
  display: grid;
  gap: 12px;
}

.editor-surface {
  border: 1px solid var(--app-border);
  border-radius: 16px;
  overflow: hidden;
  background: radial-gradient(circle at top, rgba(15, 23, 42, 0.04), transparent 55%), #f8fafc;
}

.render-template-textarea :deep(textarea.ant-input) {
  min-height: 520px;
  border: 0;
  border-radius: 0;
  padding: 18px 20px;
  background: transparent;
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 13px;
  line-height: 1.65;
  color: #0f172a;
}

.editor-actions-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: end;
}

.editor-actions-form {
  min-width: 0;
}

.editor-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.render-templates-bottom-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.schema-list,
.validation-panel,
.preview-result {
  display: grid;
  gap: 12px;
}

.schema-item {
  display: grid;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, var(--surface-soft) 92%, white 8%);
  margin-left: calc(var(--schema-depth) * 12px);

  p {
    margin: 0;
    color: var(--app-text-secondary);
    line-height: 1.5;
  }
}

.schema-item__header,
.schema-item__title,
.schema-item__badges {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}

.schema-item__title {
  justify-content: flex-start;

  code {
    color: var(--app-text-secondary);
    font-size: 0.8rem;
  }
}

.validation-issues {
  display: grid;
  gap: 10px;
}

.validation-issue {
  display: grid;
  gap: 6px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--warning) 24%, var(--app-border));
  background: color-mix(in srgb, var(--warning) 8%, transparent);

  small {
    color: var(--app-text-secondary);
  }
}

.validation-issue__header {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.json-preview {
  display: grid;
  gap: 8px;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  background: #0f172a;
  color: #e2e8f0;

  pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    font-family: "Cascadia Mono", "Consolas", monospace;
    font-size: 12px;
    line-height: 1.6;
  }
}

.json-preview__header {
  color: #93c5fd;
}

.preview-form {
  display: grid;
  gap: 10px;
}

.preview-result__meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.preview-result__image {
  display: block;
  width: 100%;
  border-radius: 14px;
  border: 1px solid var(--app-border);
  background: var(--surface-soft);
}

.preview-result__link {
  color: var(--app-primary);
  font-weight: 600;
}

@media (max-width: 1280px) {
  .render-templates-bottom-grid {
    grid-template-columns: 1fr 1fr;
  }

  .render-templates-card--preview {
    grid-column: 1 / -1;
  }
}

@media (max-width: 1080px) {
  .render-templates-layout {
    grid-template-columns: 1fr;
  }

  .editor-actions-grid {
    grid-template-columns: 1fr;
  }

  .editor-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 768px) {
  .summary-grid,
  .preview-result__meta,
  .render-templates-bottom-grid {
    grid-template-columns: 1fr;
  }

  .render-template-textarea :deep(textarea.ant-input) {
    min-height: 360px;
  }

  .editor-card__title {
    flex-direction: column;
  }
}
</style>
