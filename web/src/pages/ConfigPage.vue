<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
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
    return type === 'boolean' ? false : ''
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
    normalized = Number(value)
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
  ElMessage.success(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <div class="config-page-wrapper">
    <header class="hero-panel">
      <div class="hero-title-group">
        <h1>{{ t('config.title') }}</h1>
        <div v-if="restartRequired !== null" class="restart-indicator">
          <el-tag :type="restartRequired ? 'warning' : 'success'" size="small" effect="dark">
            {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
          </el-tag>
        </div>
      </div>

      <div class="hero-actions">
        <el-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
          {{ t('config.save') }}
        </el-button>
        <el-button @click="loadConfig" :loading="loading">{{ t('dashboard.refresh') }}</el-button>
      </div>
    </header>

    <div class="config-alerts-container" v-if="error || redactedFields.length > 0">
      <el-alert v-if="error" :title="t('errors.common.actionFailed')" type="error" :description="error" show-icon />
      <el-alert
        v-if="redactedFields.length > 0"
        :title="t('config.redactedTitle')"
        type="info"
        :description="redactedFields.join(', ')"
        show-icon
      />
    </div>

    <div class="config-main-area">
      <el-skeleton :loading="loading" animated style="height: 100%">
        <template #template>
          <div class="skeleton-layout">
            <el-skeleton-item variant="rect" style="width: 280px; height: 100%" />
            <el-skeleton-item variant="rect" style="flex: 1; height: 100%" />
          </div>
        </template>

        <div v-if="draft" class="config-layout">
          <aside class="config-nav-panel">
            <div class="nav-header">
              <strong>{{ t('config.sectionList') }}</strong>
            </div>
            
            <div class="nav-viewport-outer">
              <VirtualDataViewport
                :items="configSections"
                :item-height="96"
                viewport-height="100%"
                :get-item-key="(row) => row.key"
              >
                <template #default="{ item: section }">
                  <div class="nav-item-wrapper">
                    <button
                      type="button"
                      class="config-nav-item-card"
                      :class="{ 'is-active': activeSectionKey === section.key }"
                      @click="activeSectionKey = section.key"
                    >
                      <div class="nav-card-content">
                        <span class="nav-item-title">{{ section.title }}</span>
                        <small class="nav-item-meta">{{ section.fields.length }} {{ t('config.fieldCount') }}</small>
                      </div>
                      <div class="active-indicator"></div>
                    </button>
                  </div>
                </template>
              </VirtualDataViewport>
            </div>
          </aside>

          <div class="config-editor-shadow-box">
            <main class="config-editor-panel glass-panel">
              <div class="panel-header">
                <div class="section-title-wrap">
                  <span class="section-key-badge">{{ activeSectionKey.toUpperCase() }}</span>
                  <strong>{{ currentSection?.title }}</strong>
                </div>
                <p v-if="currentSection?.description" class="section-desc">{{ currentSection.description }}</p>
              </div>

              <div class="config-editor-scroll">
                <el-form 
                  v-for="section in configSections" 
                  :key="section.key" 
                  v-show="section.key === activeSectionKey"
                  label-position="top"
                  class="config-form-grid"
                >
                  <div v-for="field in section.fields" :key="field.path" class="config-field-item">
                    <el-form-item>
                      <template #label>
                        <div class="field-label-wrap">
                          <span class="field-label-text">{{ field.label }}</span>
                          <el-tooltip v-if="field.description" :content="field.description" placement="top">
                            <span class="field-info-icon">?</span>
                          </el-tooltip>
                        </div>
                      </template>

                      <el-input
                        v-if="field.type === 'text'"
                        :model-value="String(readField(field.path, field.type) ?? '')"
                        class="refined-input"
                        @update:model-value="(value) => writeField(field.path, field.type, value)"
                      />

                      <el-input-number
                        v-else-if="field.type === 'number'"
                        :model-value="Number(readField(field.path, field.type) ?? 0)"
                        :min="0"
                        :step="1"
                        controls-position="right"
                        class="refined-number-input"
                        @update:model-value="(value) => writeField(field.path, field.type, value ?? 0)"
                      />

                      <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                        <el-switch
                          :model-value="Boolean(readField(field.path, field.type))"
                          @update:model-value="(value) => writeField(field.path, field.type, value)"
                        />
                      </div>

                      <el-select
                        v-else-if="field.type === 'select'"
                        :model-value="String(readField(field.path, field.type) ?? '')"
                        class="refined-input"
                        @update:model-value="(value) => writeField(field.path, field.type, value)"
                      >
                        <el-option v-for="option in field.options" :key="String(option.value)" :label="option.label" :value="option.value" />
                      </el-select>

                      <el-input
                        v-else
                        :model-value="String(readField(field.path, field.type) ?? '')"
                        type="textarea"
                        :autosize="{ minRows: 4, maxRows: 8 }"
                        class="refined-input refined-textarea"
                        @update:model-value="(value) => writeField(field.path, field.type, value)"
                      />
                    </el-form-item>
                  </div>
                </el-form>
              </div>
            </main>
          </div>
        </div>
      </el-skeleton>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.config-page-wrapper {
  height: calc(100vh - 100px); 
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow: visible;
}

.hero-panel {
  flex-shrink: 0;
  padding: 20px 28px;
  border-radius: 28px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 251, 247, 0.88));
  border: 1px solid rgba(22, 33, 39, 0.08);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.04);
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 10;
}

.hero-title-group {
  display: flex;
  align-items: center;
  gap: 16px;
  h1 { margin: 0; font-size: 1.5rem; }
}

.config-main-area {
  flex: 1;
  min-height: 0;
  overflow: visible;
}

.config-layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 32px;
  height: 100%;
  overflow: visible;
}

.config-nav-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: visible;
}

.nav-header {
  padding: 0 16px 12px;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.nav-viewport-outer {
  flex: 1;
  min-height: 0;
}

.nav-item-wrapper {
  padding: 8px 16px; // Left/Right padding to keep shadow inside the scroller
}

.config-nav-item-card {
  appearance: none;
  border: 1px solid rgba(22, 33, 39, 0.06);
  background: rgba(255, 255, 255, 0.6);
  width: 100%;
  text-align: left;
  padding: 18px 22px;
  border-radius: 24px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  color: var(--text);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.03);
  position: relative;

  &:hover {
    background: #fff;
    transform: translateY(-3px);
    box-shadow: 0 12px 28px rgba(0, 0, 0, 0.06);
    border-color: rgba(15, 111, 112, 0.15);
  }

  &.is-active {
    background: #fff;
    border-color: rgba(15, 111, 112, 0.3);
    box-shadow: 0 16px 36px rgba(15, 111, 112, 0.12);
    
    .nav-item-title { color: #0f6f70; font-weight: 700; }
    .active-indicator { transform: scaleY(1); opacity: 1; }
  }

  .nav-card-content { display: flex; flex-direction: column; gap: 4px; }
  .nav-item-title { font-size: 1rem; }
  .nav-item-meta { font-size: 0.75rem; color: var(--muted); }

  .active-indicator {
    position: absolute;
    left: 0; top: 25%; bottom: 25%;
    width: 5px;
    background: #0f6f70;
    border-radius: 0 10px 10px 0;
    transform: scaleY(0); opacity: 0;
    transition: all 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  }
}

.config-editor-shadow-box {
  flex: 1;
  min-height: 0;
  padding: 20px 40px 40px 20px;
  margin: -20px -40px -40px -20px;
  overflow: visible;
  display: flex;
  flex-direction: column;
}

.config-editor-panel {
  flex: 1;
  border-radius: 32px;
  background: rgba(247, 250, 246, 0.88);
  border: 1px solid rgba(22, 33, 39, 0.08);
  box-shadow: 0 20px 60px rgba(18, 32, 38, 0.12);
  display: flex;
  flex-direction: column;
  overflow: visible;
}

.panel-header {
  padding: 24px 32px;
  border-bottom: 1px solid rgba(22, 33, 39, 0.04);
  background: rgba(255, 255, 255, 0.5);
  border-radius: 32px 32px 0 0;
  flex-shrink: 0;
  backdrop-filter: blur(12px);
  z-index: 5;

  .section-title-wrap { display: flex; align-items: center; gap: 12px; }
  .section-key-badge {
    font-size: 10px; font-family: "Cascadia Mono", monospace;
    background: rgba(15, 111, 112, 0.12); color: #0f6f70;
    padding: 3px 8px; border-radius: 8px; font-weight: bold;
  }
  strong { font-size: 1.25rem; color: #1a2a33; }
  .section-desc { margin: 8px 0 0; font-size: 0.9rem; color: var(--muted); line-height: 1.5; }
}

.config-editor-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 32px;
  scroll-behavior: smooth;
}

.config-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 32px 40px;
}

.refined-input {
  :deep(.el-input__wrapper),
  :deep(.el-textarea__inner) {
    border-radius: 16px;
    background: rgba(255, 255, 255, 0.7);
    border: 1px solid rgba(0, 0, 0, 0.08);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.02) inset;
    transition: all 0.2s;
    
    &:hover { background: #fff; border-color: #0f6f70; }
    &.is-focus { background: #fff; box-shadow: 0 0 0 1px #0f6f70 inset, 0 10px 25px rgba(15, 111, 112, 0.1); }
  }
}

@media (max-width: 1024px) {
  .config-layout { grid-template-columns: 1fr; }
  .config-nav-panel { display: none; }
}
</style>
