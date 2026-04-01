<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'

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
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('config.title') }}</h1>
      </div>

      <el-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
        {{ t('config.save') }}
      </el-button>
    </section>

    <RetryPanel
      v-if="error && !draft"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading || saving"
      @retry="loadConfig()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.actionFailed')" type="error" :description="error" show-icon />

    <el-alert
      v-if="restartRequired !== null"
      :title="restartRequired ? t('config.restartNeeded') : t('config.hotApplied')"
      :type="restartRequired ? 'warning' : 'success'"
      show-icon
    />

    <el-alert
      v-if="redactedFields.length > 0"
      :title="t('config.redactedTitle')"
      type="info"
      :description="redactedFields.join(', ')"
      show-icon
    />

    <el-skeleton :loading="loading" animated>
      <template #template>
        <el-skeleton-item variant="rect" style="height: 240px" />
      </template>

      <div v-if="draft" class="config-layout">
        <el-card class="config-nav-panel">
          <template #header>
            <div class="card-header">
              <span>{{ t('config.sectionList') }}</span>
            </div>
          </template>

          <div class="config-nav-viewport">
            <button
              v-for="section in configSections"
              :key="section.key"
              type="button"
              class="config-nav-item"
              :class="{ 'is-active': activeSectionKey === section.key }"
              @click="activeSectionKey = section.key"
            >
              <strong>{{ section.title }}</strong>
              <small>{{ section.fields.length }} {{ t('config.fieldCount') }}</small>
            </button>
          </div>
        </el-card>

        <el-card class="config-editor-panel">
          <template #header>
            <div class="card-header">
              <div>
                <strong>{{ t('config.currentSection') }}</strong>
                <p>{{ currentSection?.title }}</p>
              </div>
            </div>
          </template>

          <div class="config-editor-scroll">
            <section
              v-for="section in configSections"
              :key="section.key"
              v-show="section.key === activeSectionKey"
              class="config-section-panel"
            >
              <header class="config-section-heading">
                <div>
                  <strong>{{ section.title }}</strong>
                  <p v-if="section.description">{{ section.description }}</p>
                </div>
              </header>

              <div class="config-grid">
                <div v-for="field in section.fields" :key="field.path" class="config-field">
                  <label>{{ field.label }}</label>

                  <el-input
                    v-if="field.type === 'text'"
                    :model-value="String(readField(field.path, field.type) ?? '')"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                  />

                  <el-input-number
                    v-else-if="field.type === 'number'"
                    :model-value="Number(readField(field.path, field.type) ?? 0)"
                    :min="0"
                    :step="1"
                    controls-position="right"
                    @update:model-value="(value) => writeField(field.path, field.type, value ?? 0)"
                  />

                  <el-switch
                    v-else-if="field.type === 'boolean'"
                    :model-value="Boolean(readField(field.path, field.type))"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                  />

                  <el-select
                    v-else-if="field.type === 'select'"
                    :model-value="String(readField(field.path, field.type) ?? '')"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                  >
                    <el-option v-for="option in field.options" :key="String(option.value)" :label="option.label" :value="option.value" />
                  </el-select>

                  <el-input
                    v-else
                    :model-value="String(readField(field.path, field.type) ?? '')"
                    type="textarea"
                    :autosize="{ minRows: 3, maxRows: 6 }"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                  />

                  <small v-if="field.description">{{ field.description }}</small>
                </div>
              </div>
            </section>
          </div>
        </el-card>
      </div>
    </el-skeleton>
  </div>
</template>
