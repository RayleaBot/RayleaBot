<script setup lang="ts">
import {
  CloudUploadOutlined,
  FileZipOutlined,
  EyeOutlined,
} from '@ant-design/icons-vue'
import { t } from '@/i18n'

defineProps<{
  backupPending: boolean
  diagnosticsPending: boolean
  previewPending: boolean
}>()

defineEmits<{
  createBackup: []
  exportDiagnostics: []
  openPreview: []
}>()
</script>

<template>
  <a-card :bordered="false" class="tools-panel">
    <template #title>
      <div class="card-header">
        <span>{{ t('dashboard.tools') }}</span>
      </div>
    </template>

    <div class="table-actions">
      <a-button
        type="primary"
        class="tool-button tool-button--backup"
        :loading="backupPending"
        @click="$emit('createBackup')"
      >
        <template #icon><CloudUploadOutlined v-if="!backupPending" /></template>
        {{ t('dashboard.createBackup') }}
      </a-button>
      <a-button
        type="primary"
        class="tool-button tool-button--diagnostics"
        :loading="diagnosticsPending"
        @click="$emit('exportDiagnostics')"
      >
        <template #icon><FileZipOutlined v-if="!diagnosticsPending" /></template>
        {{ t('dashboard.exportDiagnostics') }}
      </a-button>
      <a-button
        type="primary"
        class="tool-button tool-button--preview"
        :loading="previewPending"
        @click="$emit('openPreview')"
      >
        <template #icon><EyeOutlined v-if="!previewPending" /></template>
        {{ t('dashboard.renderPreview') }}
      </a-button>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.tools-panel {
  border-radius: var(--radius-xl);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    box-shadow: var(--shadow-elevated);
    border-color: var(--border-accent);
  }
}

.tools-panel :deep(.ant-card-body) {
  padding: var(--space-lg);
}

.card-header {
  span {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--text);
  }
}

.table-actions {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.tool-button {
  width: 100%;
  height: 38px;
  border-radius: var(--radius-md);
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  box-shadow: var(--shadow-xs);
  border: 1px solid transparent;
  transition: transform 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1);

  &--backup {
    background: linear-gradient(135deg, var(--accent) 0%, color-mix(in srgb, var(--accent) 80%, #ffffff) 100%) !important;
    border-color: var(--accent);
    color: #ffffff;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px -3px color-mix(in srgb, var(--accent) 40%, transparent);
    }
  }

  &--diagnostics {
    background: linear-gradient(135deg, #17a2b8 0%, color-mix(in srgb, #17a2b8 80%, #ffffff) 100%) !important;
    border-color: #17a2b8;
    color: #ffffff;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px -3px color-mix(in srgb, #17a2b8 40%, transparent);
    }
  }

  &--preview {
    background: linear-gradient(135deg, var(--success) 0%, color-mix(in srgb, var(--success) 80%, #ffffff) 100%) !important;
    border-color: var(--success);
    color: #ffffff;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px -3px color-mix(in srgb, var(--success) 40%, transparent);
    }
  }
}
</style>
