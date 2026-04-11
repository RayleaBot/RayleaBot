<script setup lang="ts">
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
      <a-button type="primary" :loading="backupPending" @click="$emit('createBackup')">
        {{ t('dashboard.createBackup') }}
      </a-button>
      <a-button type="primary" :loading="diagnosticsPending" @click="$emit('exportDiagnostics')">
        {{ t('dashboard.exportDiagnostics') }}
      </a-button>
      <a-button type="primary" :loading="previewPending" @click="$emit('openPreview')">
        {{ t('dashboard.renderPreview') }}
      </a-button>
    </div>
  </a-card>
</template>
