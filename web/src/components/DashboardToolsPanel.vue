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
  <el-card class="tools-panel">
    <template #header>
      <div class="card-header">
        <span>{{ t('dashboard.tools') }}</span>
      </div>
    </template>

    <div class="table-actions">
      <el-button type="primary" plain :loading="backupPending" @click="$emit('createBackup')">
        {{ t('dashboard.createBackup') }}
      </el-button>
      <el-button type="primary" plain :loading="diagnosticsPending" @click="$emit('exportDiagnostics')">
        {{ t('dashboard.exportDiagnostics') }}
      </el-button>
      <el-button type="primary" plain :loading="previewPending" @click="$emit('openPreview')">
        {{ t('dashboard.renderPreview') }}
      </el-button>
    </div>
  </el-card>
</template>
