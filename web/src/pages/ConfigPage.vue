<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import { cloneConfig, configSections, getValueByPath, setValueByPath } from '@/lib/config-form'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const { document, error, loading, redactedFields, restartRequired, saving } = storeToRefs(configStore)

const draft = ref<ConfigDocument | null>(null)

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
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
  ElMessage.success(response.restart_required ? '配置已保存，需重启' : '配置已保存并即时生效')
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Config</div>
        <h1>配置表单</h1>
        <p>按 `ConfigDocument` 顶层组映射，脱敏字段保持 `__REDACTED__` round-trip。</p>
      </div>

      <el-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
        保存配置
      </el-button>
    </section>

    <RetryPanel
      v-if="error && !draft"
      title="配置读取失败"
      :description="error"
      :loading="loading || saving"
      @retry="loadConfig()"
    />

    <el-alert v-else-if="error" title="配置读取或保存失败" type="error" :description="error" show-icon />

    <el-alert
      v-if="restartRequired !== null"
      :title="restartRequired ? '保存完成，仍需重启服务' : '保存完成，已热更新生效'"
      :type="restartRequired ? 'warning' : 'success'"
      show-icon
    />

    <el-alert
      v-if="redactedFields.length > 0"
      title="脱敏字段"
      type="info"
      :description="redactedFields.join(', ')"
      show-icon
    />

    <el-skeleton :loading="loading" animated>
      <template #template>
        <el-skeleton-item variant="rect" style="height: 240px" />
      </template>

      <div v-if="draft" class="config-sections">
        <el-card v-for="section in configSections" :key="section.key">
          <template #header>
            <div class="card-header">
              <div>
                <strong>{{ section.title }}</strong>
                <p>{{ section.description }}</p>
              </div>
            </div>
          </template>

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
        </el-card>
      </div>
    </el-skeleton>
  </div>
</template>
