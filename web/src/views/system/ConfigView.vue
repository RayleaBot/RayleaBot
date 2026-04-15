<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import AppCard from '@/components/AppCard.vue'
import { notifySuccess } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { cloneConfig, getConfigSections, getValueByPath, setValueByPath } from '@/lib/config-form'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const { document, error, loading, redactedFields, restartRequired, saving } = storeToRefs(configStore)

const draft = ref<ConfigDocument | null>(null)
const configSections = computed(() => getConfigSections())
const activeSectionKey = ref<keyof ConfigDocument>('server')
const currentSection = computed(
  () => configSections.value.find((section) => section.key === activeSectionKey.value) ?? configSections.value[0],
)

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

watch(configSections, (sections) => {
  if (!sections.some((section) => section.key === activeSectionKey.value)) {
    activeSectionKey.value = sections[0]?.key ?? 'server'
  }
}, { immediate: true })

async function loadConfig() {
  try {
    await configStore.fetchConfig()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadConfig()
})

function readField(path: string, type: 'text' | 'number' | 'boolean' | 'select' | 'list') {
  if (!draft.value) {
    if (type === 'boolean') {
      return false
    }

    return type === 'number' ? null : ''
  }

  const current = getValueByPath(draft.value as unknown as Record<string, unknown>, path)
  if (type === 'list') {
    return Array.isArray(current) ? toMultilineList(current as string[]) : ''
  }
  return current
}

function writeField(path: string, type: 'text' | 'number' | 'boolean' | 'select' | 'list', value: unknown) {
  if (!draft.value) {
    return
  }

  let normalized = value
  if (type === 'number') {
    if (value === null || value === undefined || value === '') {
      normalized = undefined
    } else {
      const nextNumber = Number(value)
      normalized = Number.isFinite(nextNumber) ? nextNumber : undefined
    }
  } else if (type === 'list') {
    normalized = fromMultilineList(String(value))
  }

  setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
}

const canSave = computed(() => Boolean(draft.value) && !saving.value)

async function save() {
  if (!draft.value) {
    return
  }

  const response = await configStore.saveConfig(draft.value)
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <AppPage :title="t('config.title')" full-height>
    <template #extra>
      <div class="table-actions">
        <a-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
          {{ t('config.save') }}
        </a-button>
        <a-button :loading="loading" @click="loadConfig">{{ t('dashboard.refresh') }}</a-button>
      </div>
    </template>

    <div v-if="error || redactedFields.length > 0" class="config-alerts-container">
      <a-alert v-if="error" :message="t('errors.common.actionFailed')" type="error" :description="error" show-icon />
      <a-alert
        v-if="redactedFields.length > 0"
        :message="t('config.redactedTitle')"
        type="info"
        :description="redactedFields.join(', ')"
        show-icon
      />
    </div>

    <RetryPanel
      v-if="error && !draft"
      :title="t('config.title')"
      :description="error"
      :loading="loading"
      @retry="loadConfig"
    />

    <div v-else-if="loading && !draft" class="skeleton-layout">
      <div class="skeleton-panel"></div>
      <div class="skeleton-panel"></div>
    </div>

    <div v-else-if="draft" class="config-layout">
      <AppCard :title="t('config.sectionList')" borderless class="config-nav-card">
        <nav class="config-nav-list" aria-label="配置分区">
          <button
            v-for="section in configSections"
            :key="section.key"
            type="button"
            class="config-nav-item"
            :class="{ 'is-active': activeSectionKey === section.key }"
            @click="activeSectionKey = section.key"
          >
            <span class="config-nav-item__title">{{ section.title }}</span>
            <small class="config-nav-item__meta">{{ section.fields.length }} {{ t('config.fieldCount') }}</small>
          </button>
        </nav>
      </AppCard>

      <AppCard borderless class="config-editor-card">
        <template #title>
          <div class="config-editor-card__header">
            <div class="config-editor-card__title">
              <strong>{{ currentSection?.title }}</strong>
              <p v-if="currentSection?.description">{{ currentSection.description }}</p>
            </div>
            <div v-if="restartRequired !== null" class="restart-indicator">
              <a-tag :color="restartRequired ? 'warning' : 'success'">
                {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
              </a-tag>
            </div>
          </div>
        </template>

        <a-form
          v-if="currentSection"
          layout="vertical"
          class="config-form-grid"
        >
          <div v-for="field in currentSection.fields" :key="field.path" class="config-field-item">
            <a-form-item>
              <template #label>
                <div class="field-label-wrap">
                  <span class="field-label-text">{{ field.label }}</span>
                  <a-tooltip v-if="field.description" :title="field.description">
                    <span class="field-info-icon">?</span>
                  </a-tooltip>
                </div>
              </template>

              <a-input
                v-if="field.type === 'text'"
                :value="String(readField(field.path, field.type) ?? '')"
                @update:value="(value) => writeField(field.path, field.type, value)"
              />

              <a-input-number
                v-else-if="field.type === 'number'"
                class="config-number-input"
                :value="typeof readField(field.path, field.type) === 'number' ? readField(field.path, field.type) : null"
                :min="0"
                :step="1"
                @update:value="(value) => writeField(field.path, field.type, value)"
              />

              <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                <a-switch
                  :checked="Boolean(readField(field.path, field.type))"
                  @update:checked="(value) => writeField(field.path, field.type, value)"
                />
              </div>

              <a-select
                v-else-if="field.type === 'select'"
                :value="String(readField(field.path, field.type) ?? '')"
                :options="field.options"
                @update:value="(value) => writeField(field.path, field.type, value)"
              />

              <a-textarea
                v-else
                :value="String(readField(field.path, field.type) ?? '')"
                :auto-size="{ minRows: 4, maxRows: 8 }"
                @update:value="(value) => writeField(field.path, field.type, value)"
              />
            </a-form-item>
          </div>
        </a-form>
      </AppCard>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.config-alerts-container {
  display: grid;
  gap: 12px;
}

.skeleton-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 12px;
  flex: 1;
}

.skeleton-panel {
  border-radius: 10px;
  min-height: 520px;
  background: linear-gradient(90deg, var(--surface-soft), var(--surface), var(--surface-soft));
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}

.config-layout {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 12px;
  height: 100%;
  min-height: 0;
}

.config-nav-card,
.config-editor-card {
  min-height: 0;
}

:deep(.config-nav-card) {
  display: flex;
  flex-direction: column;
  height: 100%;
}

:deep(.config-nav-card .ant-card-head) {
  flex-shrink: 0;
}

.config-nav-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  scrollbar-gutter: stable;
  padding: 12px;
}

.config-editor-card :deep(.ant-card-head) {
  min-height: 50px;
}

.config-editor-card :deep(.ant-card-body) {
  min-height: 0;
  overflow: auto;
  padding: 16px;
}

.config-nav-list {
  display: grid;
  width: 100%;
  gap: 6px;
  min-height: min-content;
}

.config-nav-item {
  appearance: none;
  border: 1px solid transparent;
  background: transparent;
  width: 100%;
  text-align: left;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  display: grid;
  gap: 4px;
  transition: border-color 0.2s ease, background-color 0.2s ease;
  color: var(--text);
}

.config-nav-item:hover {
  background: var(--surface-soft);
  border-color: var(--border);
}

.config-nav-item.is-active {
  background: color-mix(in srgb, var(--accent) 8%, var(--surface));
  border-color: color-mix(in srgb, var(--accent) 24%, var(--border));
  font-weight: 600;
}

.config-nav-item__title {
  font-size: 0.94rem;
  font-weight: 500;
}

.config-nav-item__meta {
  font-size: 0.78rem;
  color: var(--muted);
}

.config-editor-card__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}

.config-editor-card__title {
  display: grid;
  gap: 4px;

  strong {
    font-size: 1rem;
  }

  p {
    margin: 0;
    color: var(--muted);
    font-size: 0.86rem;
    line-height: 1.5;
  }
}

.config-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px 20px;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field-label-text {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--theme-text, var(--text));
}

.field-info-icon {
  color: var(--muted);
  cursor: help;
  font-size: 0.8rem;
  font-weight: bold;
  opacity: 0.7;
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  border: 1px solid var(--border);
}

.field-info-icon:hover {
  opacity: 1;
  color: var(--accent);
  border-color: var(--accent);
}

.config-number-input {
  width: 100%;
}

.switch-wrap {
  display: flex;
  min-height: 36px;
  align-items: center;
}

@keyframes shimmer {
  0% {
    background-position: 200% 0;
  }

  100% {
    background-position: -200% 0;
  }
}

@media (max-width: 1024px) {
  .config-layout,
  .skeleton-layout {
    grid-template-columns: 1fr;
  }

  .config-nav-card :deep(.ant-card-body) {
    max-height: 200px;
  }
}
</style>
