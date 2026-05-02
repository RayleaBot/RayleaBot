<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import AppCard from '@/components/AppCard.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { apiDownload } from '@/lib/http'
import { getTaskStatusLabel } from '@/lib/display'
import {
  buildRenderTemplatePreviewSample,
  buildRenderTemplateSchemaNodes,
  parseRenderTemplatePreviewData,
} from '@/lib/render-template-editor'
import { t } from '@/i18n'
import { useRenderTemplatesStore } from '@/stores/render-templates'
import { useSystemStore } from '@/stores/system'
import { useTasksStore } from '@/stores/tasks'
import type { RenderPreviewRequest } from '@/types/api'

const route = useRoute()
const router = useRouter()
const renderTemplatesStore = useRenderTemplatesStore()
const systemStore = useSystemStore()
const tasksStore = useTasksStore()

const { detailById, error, items, loading, workspaceLoading } = storeToRefs(renderTemplatesStore)

const hasRequestedList = ref(false)
const pageActive = ref(true)
const previewDataByTemplate = ref<Record<string, string>>({})
const previewTaskIdByTemplate = ref<Record<string, string>>({})
const previewRequestErrorByTemplate = ref<Record<string, string>>({})
const previewRequestErrorKeyByTemplate = ref<Record<string, string>>({})
const desiredPreviewKeyByTemplate = ref<Record<string, string>>({})
const lastSubmittedPreviewKeyByTemplate = ref<Record<string, string>>({})
const pendingPreviewKeysByTemplate = ref<Record<string, string[]>>({})
const previewImageSrc = ref('')
const imageModalVisible = ref(false)

let autoPreviewHandle: number | null = null
let previewImageLoadVersion = 0
let previewWatcherActive = true

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

const currentPreviewTaskId = computed(() => (
  activeTemplateId.value ? previewTaskIdByTemplate.value[activeTemplateId.value] ?? '' : ''
))

const currentPreviewTask = computed(() => (
  currentPreviewTaskId.value
    ? tasksStore.items.find((item) => item.task_id === currentPreviewTaskId.value) ?? null
    : null
))

const currentPreviewRequestError = computed(() => (
  activeTemplateId.value ? previewRequestErrorByTemplate.value[activeTemplateId.value] ?? '' : ''
))
const currentPreviewPending = computed(() => (
  Boolean(activeTemplateId.value && previewRequestKey.value && hasPendingPreviewKey(activeTemplateId.value, previewRequestKey.value))
))

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
    data: previewParseResult.value.data,
  })
})

const previewImageUrl = computed(() => {
  const imageUrl = currentPreviewTask.value?.result?.details?.image_url
  return typeof imageUrl === 'string' ? imageUrl : ''
})

const previewEmptyDescription = computed(() => {
  if (previewParseResult.value.issue) {
    return previewParseResult.value.issue.message
  }
  if (currentPreviewRequestError.value) {
    return currentPreviewRequestError.value
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

function buildDefaultPreviewData(templateId: string, schema: Record<string, unknown> | null = null) {
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
        name: 'RayleaBot 测试群',
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
    const previewData = buildDefaultPreviewData(templateId, detailById.value[templateId]?.input_schema_json ?? null)
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

function resetPreviewImage() {
  if (!previewImageSrc.value) {
    return
  }

  window.URL.revokeObjectURL(previewImageSrc.value)
  previewImageSrc.value = ''
}

function clearPreviewRequestError(templateId: string) {
  previewRequestErrorByTemplate.value = {
    ...previewRequestErrorByTemplate.value,
    [templateId]: '',
  }
  previewRequestErrorKeyByTemplate.value = {
    ...previewRequestErrorKeyByTemplate.value,
    [templateId]: '',
  }
}

function setPreviewRequestError(templateId: string, requestKey: string, message: string) {
  previewRequestErrorByTemplate.value = {
    ...previewRequestErrorByTemplate.value,
    [templateId]: message,
  }
  previewRequestErrorKeyByTemplate.value = {
    ...previewRequestErrorKeyByTemplate.value,
    [templateId]: requestKey,
  }
}

function setDesiredPreviewKey(templateId: string, requestKey: string) {
  desiredPreviewKeyByTemplate.value = {
    ...desiredPreviewKeyByTemplate.value,
    [templateId]: requestKey,
  }
}

function hasPendingPreviewKey(templateId: string, requestKey: string) {
  return pendingPreviewKeysByTemplate.value[templateId]?.includes(requestKey) ?? false
}

function markPendingPreviewKey(templateId: string, requestKey: string) {
  if (hasPendingPreviewKey(templateId, requestKey)) {
    return
  }

  pendingPreviewKeysByTemplate.value = {
    ...pendingPreviewKeysByTemplate.value,
    [templateId]: [...(pendingPreviewKeysByTemplate.value[templateId] ?? []), requestKey],
  }
}

function clearPendingPreviewKey(templateId: string, requestKey: string) {
  const keys = pendingPreviewKeysByTemplate.value[templateId]
  if (!keys?.length) {
    return
  }

  if (!keys.includes(requestKey)) {
    return
  }

  const nextKeys = keys.filter((key) => key !== requestKey)
  pendingPreviewKeysByTemplate.value = {
    ...pendingPreviewKeysByTemplate.value,
    [templateId]: nextKeys,
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

  lastSubmittedPreviewKeyByTemplate.value = {
    ...lastSubmittedPreviewKeyByTemplate.value,
    [activeTemplateId.value]: '',
  }
  await loadTemplateWorkspace(activeTemplateId.value, { force: true })
  scheduleAutoPreview()
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

  if (hasPendingPreviewKey(templateId, requestKey)) {
    return
  }

  markPendingPreviewKey(templateId, requestKey)
  clearPreviewRequestError(templateId)

  const payload: RenderPreviewRequest = {
    template: templateId,
    data: previewParseResult.value.data,
  }

  try {
    const response = await systemStore.previewRender(payload)
    if (desiredPreviewKeyByTemplate.value[templateId] !== requestKey) {
      return
    }

    lastSubmittedPreviewKeyByTemplate.value = {
      ...lastSubmittedPreviewKeyByTemplate.value,
      [templateId]: requestKey,
    }
    previewTaskIdByTemplate.value = {
      ...previewTaskIdByTemplate.value,
      [templateId]: response.task_id,
    }
    await tasksStore.fetchTask(response.task_id, { makeCurrent: false })
  } catch (error) {
    if (desiredPreviewKeyByTemplate.value[templateId] !== requestKey) {
      return
    }

    setPreviewRequestError(templateId, requestKey, getDisplayErrorMessage(error))
  } finally {
    clearPendingPreviewKey(templateId, requestKey)
  }
}

function scheduleAutoPreview() {
  clearAutoPreviewTimer()

  if (!isActiveTemplateRoute.value || !activeTemplateId.value || !currentTemplate.value) {
    return
  }

  if (previewParseResult.value.data === null) {
    return
  }

  const requestKey = previewRequestKey.value
  const templateId = activeTemplateId.value
  if (previewRequestErrorKeyByTemplate.value[templateId] && previewRequestErrorKeyByTemplate.value[templateId] !== requestKey) {
    clearPreviewRequestError(templateId)
  }

  if (!requestKey || lastSubmittedPreviewKeyByTemplate.value[templateId] === requestKey || hasPendingPreviewKey(templateId, requestKey)) {
    return
  }

  autoPreviewHandle = window.setTimeout(() => {
    autoPreviewHandle = null
    if (!isActiveTemplateRoute.value || activeTemplateId.value !== templateId || previewRequestKey.value !== requestKey) {
      return
    }

    void submitPreview(templateId, requestKey)
  }, 500)
}

function openImageModal() {
  if (!previewImageSrc.value) {
    return
  }
  imageModalVisible.value = true
}

watch([items, isActiveTemplateRoute, () => route.params.templateId], () => {
  void syncRouteTemplate()
}, { immediate: true })

watch(activeTemplateId, (templateId) => {
  if (!templateId) {
    return
  }

  ensurePreviewDefaults(templateId)
  if (!(templateId in previewRequestErrorByTemplate.value)) {
    previewRequestErrorByTemplate.value = {
      ...previewRequestErrorByTemplate.value,
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
], () => {
  if (activeTemplateId.value) {
    setDesiredPreviewKey(activeTemplateId.value, previewRequestKey.value)
  }
  scheduleAutoPreview()
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

onActivated(() => {
  pageActive.value = true
})

onDeactivated(() => {
  pageActive.value = false
  clearAutoPreviewTimer()
})

onBeforeUnmount(() => {
  previewWatcherActive = false
  clearAutoPreviewTimer()
  pendingPreviewKeysByTemplate.value = {}
  previewImageLoadVersion += 1
  resetPreviewImage()
})
</script>

<template>
  <AppPage :title="t('renderTemplates.title')" :description="t('renderTemplates.subtitle')">
    <template #extra>
      <div class="table-actions">
        <a-button :loading="loading || workspaceLoading" @click="loadTemplateList">
          {{ t('dashboard.refresh') }}
        </a-button>
        <a-button :disabled="!activeTemplateId" @click="reloadCurrentTemplate">
          {{ t('renderTemplates.reloadAction') }}
        </a-button>
      </div>
    </template>

    <div v-if="error || previewParseResult.issue" class="render-templates__alerts">
      <a-alert
        v-if="error"
        type="error"
        show-icon
        :message="t('errors.common.actionFailed')"
        :description="error"
      />
      <a-alert
        v-if="previewParseResult.issue"
        type="warning"
        show-icon
        :message="t('renderTemplates.previewInvalid')"
        :description="previewParseResult.issue.message"
      />
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
        <AppCard :title="t('renderTemplates.templateList')" borderless class="render-templates-card render-templates-card--nav" size="small">
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
      </div>

      <div class="render-templates-layout__main">
        <a-skeleton
          v-if="workspaceLoading && !currentTemplate"
          active
          :paragraph="{ rows: 1 }"
          class="template-info-bar-skeleton"
        />
        <div v-else-if="currentTemplate" class="template-info-bar">
          <div class="template-info-bar__item">
            <span>{{ t('renderTemplates.fields.id') }}</span>
            <strong>{{ currentTemplate.id }}</strong>
          </div>
          <div class="template-info-bar__item">
            <span>{{ t('renderTemplates.fields.version') }}</span>
            <strong>{{ currentTemplate.version }}</strong>
          </div>
          <div class="template-info-bar__item">
            <span>{{ t('renderTemplates.fields.size') }}</span>
            <strong>{{ formatTemplateSize(currentTemplate.width, currentTemplate.height) }}</strong>
          </div>
          <div class="template-info-bar__item">
            <span>{{ t('renderTemplates.fields.updatedAt') }}</span>
            <strong>{{ formatDateTime(currentTemplate.updated_at) }}</strong>
          </div>
        </div>

        <div class="render-templates-workspace">
          <div class="render-templates-workspace__col">
            <AppCard :title="t('renderTemplates.previewData')" borderless size="small">
              <a-form layout="vertical" class="preview-form">
                <a-form-item :label="t('renderTemplates.previewData')">
                  <a-textarea
                    v-model:value="currentPreviewDataText"
                    :rows="10"
                    :aria-label="t('renderTemplates.previewData')"
                    :placeholder="t('renderTemplates.previewDataPlaceholder')"
                  />
                </a-form-item>
              </a-form>
            </AppCard>

            <AppCard :title="t('renderTemplates.schemaPreviewTitle')" borderless size="small">
              <template #extra>
                <span class="render-templates-card__meta">{{ t('renderTemplates.schemaPreviewHint') }}</span>
              </template>
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
            </AppCard>
          </div>

          <div class="render-templates-workspace__col render-templates-workspace__col--preview">
            <AppCard
              :title="t('renderTemplates.previewTitle')"
              borderless
              size="small"
              class="render-templates-card--preview"
            >
              <template #extra>
                <span class="render-templates-card__meta">{{ t('renderTemplates.previewHint') }}</span>
              </template>

              <div class="preview-pane">
                <div
                  v-if="currentPreviewTask || previewImageSrc || currentPreviewRequestError || currentPreviewPending"
                  class="preview-result"
                  data-testid="render-template-preview-result"
                >
                  <div class="preview-result__status-bar">
                    <span class="status-pill">
                      <span>{{ t('renderTemplates.previewTask') }}</span>
                      <strong>{{ currentPreviewTask?.task_id || t('display.empty') }}</strong>
                    </span>
                    <span class="status-pill">
                      <span>{{ t('tasks.fields.status') }}</span>
                      <strong>
                        {{ currentPreviewPending
                          ? t('renderTemplates.previewPending')
                          : currentPreviewTask
                            ? getTaskStatusLabel(currentPreviewTask.status)
                            : t('display.empty') }}
                      </strong>
                    </span>
                    <span class="status-pill">
                      <span>{{ t('renderTemplates.previewArtifact') }}</span>
                      <strong>{{ currentPreviewTask?.result?.details?.artifact_id || t('display.empty') }}</strong>
                    </span>
                    <span class="status-pill">
                      <span>{{ t('renderTemplates.previewCache') }}</span>
                      <strong>{{ currentPreviewTask?.result?.details?.from_cache ? t('renderTemplates.previewFromCache') : t('renderTemplates.previewFresh') }}</strong>
                    </span>
                  </div>

                  <a-alert
                    v-if="currentPreviewRequestError"
                    type="error"
                    show-icon
                    :message="t('errors.common.actionFailed')"
                    :description="currentPreviewRequestError"
                  />

                  <a-alert
                    v-if="currentPreviewTask?.error"
                    type="error"
                    show-icon
                    :message="currentPreviewTask.error.code"
                    :description="currentPreviewTask.error.message"
                  />

                  <div
                    v-if="previewImageSrc"
                    class="preview-result__image-wrap"
                    :title="t('renderTemplates.previewZoomHint')"
                    @click="openImageModal"
                  >
                    <img
                      :src="previewImageSrc"
                      :alt="t('renderTemplates.previewImageAlt')"
                      class="preview-result__image"
                    />
                    <div class="preview-result__zoom-hint">
                      {{ t('renderTemplates.previewZoomHint') }}
                    </div>
                  </div>

                  <RouterLink
                    v-if="currentPreviewTask"
                    class="preview-result__link"
                    :to="{ name: 'tasks', query: { task_id: currentPreviewTask.task_id } }"
                  >
                    {{ t('renderTemplates.previewTaskDetail') }}
                  </RouterLink>
                </div>

                <a-empty v-else :description="previewEmptyDescription" />
              </div>
            </AppCard>
          </div>
        </div>
      </div>
    </div>

    <a-modal
      v-model:open="imageModalVisible"
      :footer="null"
      :closable="true"
      width="auto"
      wrap-class-name="preview-image-modal"
      @cancel="imageModalVisible = false"
    >
      <img
        :src="previewImageSrc"
        :alt="t('renderTemplates.previewImageAlt')"
        style="display: block; max-width: 90vw; max-height: 85vh; object-fit: contain;"
      />
    </a-modal>
  </AppPage>
</template>

<style lang="scss" scoped>
.render-templates__alerts {
  display: grid;
  gap: 12px;
}

.render-templates-layout {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 16px;
  flex: 1 1 auto;
  min-height: 0;
}

.render-templates-layout__sidebar,
.render-templates-layout__main {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.render-templates-card__meta {
  color: var(--app-text-secondary);
  font-size: 0.82rem;
}

.render-templates-card--nav {
  min-height: 0;
}

.render-templates-card--nav :deep(.ant-card-body) {
  padding: 12px;
}

.template-nav-list {
  display: grid;
  gap: 8px;
}

.template-nav-item {
  appearance: none;
  border: 1px solid var(--app-border);
  background: linear-gradient(180deg, color-mix(in srgb, var(--surface) 92%, white 8%), var(--surface-soft));
  border-radius: var(--radius-lg);
  padding: 14px;
  display: grid;
  gap: 10px;
  text-align: left;
  color: var(--app-text);
  cursor: pointer;
  transition: border-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
}

.template-nav-item:hover {
  border-color: color-mix(in srgb, var(--accent) 28%, var(--app-border));
  transform: translateY(-1px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.06);
}

.template-nav-item.is-active {
  border-color: color-mix(in srgb, var(--accent) 38%, var(--app-border));
  background: linear-gradient(180deg, color-mix(in srgb, var(--accent) 8%, white 92%), color-mix(in srgb, var(--accent) 6%, var(--surface-soft)));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 18%, transparent);
}

.template-nav-item__header,
.template-nav-item__meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.template-nav-item__meta {
  flex-wrap: wrap;
  color: var(--app-text-secondary);
  font-size: 0.82rem;

  span {
    min-width: 0;
    overflow-wrap: anywhere;
  }
}

.template-info-bar-skeleton :deep(.ant-skeleton-title) {
  margin: 0;
  height: 48px;
  border-radius: var(--radius-lg);
}

.template-info-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
  padding: 12px 16px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, var(--surface-soft) 92%, white 8%);
}

.template-info-bar__item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.88rem;

  span {
    color: var(--app-text-secondary);
    font-size: 0.78rem;
  }

  strong {
    font-weight: 600;
    word-break: break-word;
  }
}

.render-templates-workspace {
  display: grid;
  grid-template-columns: minmax(300px, 1fr) minmax(300px, 1.2fr);
  gap: 16px;
  flex: 1 1 auto;
  min-height: 0;
}

.render-templates-workspace__col {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 0;
}

.render-templates-workspace__col--preview {
  min-height: 0;
}

.preview-result {
  display: grid;
  gap: 12px;
}

.schema-tree {
  display: flex;
  flex-direction: column;
}

.schema-tree-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 7px 0;
  padding-left: calc((var(--schema-depth) - 1) * 16px);
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 40%, transparent);
}

.schema-tree-row:last-child {
  border-bottom: none;
}

.schema-tree-row__content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.schema-tree-row__header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.schema-tree-row__name {
  font-weight: 600;
  font-size: 0.9rem;
  font-family: ui-monospace, 'SF Mono', Menlo, Consolas, monospace;
}

.schema-tree-row__type {
  font-size: 0.75rem;
  color: var(--app-text-secondary);
  background: color-mix(in srgb, var(--surface-soft) 80%, white 20%);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid color-mix(in srgb, var(--app-border) 50%, transparent);
}

.schema-tree-row__required {
  font-size: 0.75rem;
  color: #ff4d4f;
  font-weight: 500;
}

.schema-tree-row__desc {
  font-size: 0.8rem;
  color: var(--app-text-secondary);
  line-height: 1.4;
}

.render-templates-card--preview :deep(.ant-card-body) {
  display: grid;
  gap: 16px;
}

.preview-form {
  display: grid;
  gap: 10px;
}

.preview-form :deep(.ant-form-item) {
  margin-bottom: 0;
}

.preview-pane {
  display: grid;
  gap: 12px;
}

.preview-result__status-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  border: 1px solid var(--app-border);
  background: color-mix(in srgb, var(--surface-soft) 88%, white 12%);
  font-size: 0.82rem;

  span {
    color: var(--app-text-secondary);
  }

  strong {
    font-weight: 600;
    word-break: break-word;
  }
}

.preview-result__image-wrap {
  position: relative;
  display: inline-block;
  width: 100%;
  border-radius: var(--radius-lg);
  border: 1px solid var(--app-border);
  background: var(--surface-soft);
  cursor: zoom-in;
  overflow: hidden;
  transition: box-shadow 0.2s ease;
}

.preview-result__image-wrap:hover {
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.1);
}

.preview-result__image {
  display: block;
  width: 100%;
  max-height: 70vh;
  object-fit: contain;
  border-radius: var(--radius-lg);
}

.preview-result__zoom-hint {
  position: absolute;
  bottom: 12px;
  right: 12px;
  padding: 4px 10px;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  border-radius: var(--radius-md);
  font-size: 0.75rem;
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
  user-select: none;
}

.preview-result__image-wrap:hover .preview-result__zoom-hint {
  opacity: 1;
}

.preview-result__link {
  color: var(--accent);
  font-weight: 600;
}

@media (max-width: 1080px) {
  .render-templates-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 900px) {
  .render-templates-workspace {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .template-info-bar {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
}
</style>
