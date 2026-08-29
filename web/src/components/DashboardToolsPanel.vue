<script setup lang="ts">
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  FileZipOutlined,
} from '@ant-design/icons-vue'
import { t } from '@/i18n'

defineProps<{
  backupPending: boolean
  diagnosticsPending: boolean
}>()

defineEmits<{
  createBackup: []
  exportDiagnostics: []
}>()
</script>

<template>
  <a-card :bordered="false" class="tools-panel">
    <template #title>
      <div class="card-header">
        <span>{{ t('dashboard.tools') }}</span>
      </div>
    </template>

    <div class="tools-panel__body">
      <DatabaseOutlined class="tools-panel__icon" aria-hidden="true" />
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
        class="tool-button tool-button--diagnostics"
        :loading="diagnosticsPending"
        @click="$emit('exportDiagnostics')"
      >
        <template #icon><FileZipOutlined v-if="!diagnosticsPending" /></template>
        {{ t('dashboard.exportDiagnostics') }}
      </a-button>
      </div>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.tools-panel {
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: none;
}

.tools-panel :deep(.ant-card-body) {
  padding: var(--space-lg);
}

.card-header {
  span {
    font-size: 16px;
    font-weight: 600;
    color: var(--text);
  }
}

.tools-panel__body { display: flex; align-items: center; gap: 20px; }
.tools-panel__icon { display: grid; place-items: center; width: 28px; height: 32px; flex: none; color: var(--muted); font-size: 24px; }
.table-actions {
  flex: 1;
  min-width: 0;
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
  box-shadow: none;
  transition: border-color var(--motion-fast) var(--motion-easing), background-color var(--motion-fast) var(--motion-easing);

  &--backup {
    background: var(--brand-fill) !important;
    border-color: var(--brand-fill);
    color: var(--on-brand);

    &:hover {
      border-color: var(--brand-fill-hover);
      background: var(--brand-fill-hover) !important;
    }

    &:active {
      border-color: var(--brand-fill-pressed);
      background: var(--brand-fill-pressed) !important;
    }
  }

}
</style>
