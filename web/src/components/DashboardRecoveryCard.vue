<script setup lang="ts">
import { t } from '@/i18n'
import type {
  RecoveryCompatibilitySkippedPlugin,
  RecoveryCompatibilitySummary,
} from '@/types/api'
import RecoverySummaryDetails from '@/components/RecoverySummaryDetails.vue'

const selectedRecoveryReviewIds = defineModel<string[]>('selectedRecoveryReviewIds', { default: () => [] })
const recoveryConfirmNote = defineModel<string>('recoveryConfirmNote', { default: '' })

defineProps<{
  recoverySummary: RecoveryCompatibilitySummary | null
  recoveryStatusLabel: string
  pendingRecoveryPlugins: RecoveryCompatibilitySkippedPlugin[]
  selectedRecoveryReviewCountLabel: string
  recoveryRecheckPending: boolean
  recoveryConfirmPending: boolean
  runtimeBootstrapPending: boolean
}>()

defineEmits<{
  recheck: []
  bootstrap: []
  openPlugin: [pluginId: string]
  confirm: []
}>()
</script>

<template>
  <a-card :bordered="false" class="dashboard-recovery-card">
    <template #title>
      <div class="card-header">
        <span>恢复兼容性</span>
      </div>
    </template>

    <a-empty v-if="!recoverySummary" :description="t('display.empty')" />

    <div v-else class="events-section">
      <div class="table-actions" style="justify-content: flex-start; margin-bottom: 12px;">
        <a-button
          data-testid="recovery-recheck-button"
          size="small"
          :loading="recoveryRecheckPending"
          @click="$emit('recheck')"
        >
          {{ t('dashboard.recoveryRecheck') }}
        </a-button>
        <a-button
          data-testid="runtime-bootstrap-button"
          size="small"
          :loading="runtimeBootstrapPending"
          @click="$emit('bootstrap')"
        >
          {{ t('dashboard.runtimeBootstrap') }}
        </a-button>
      </div>

      <RecoverySummaryDetails
        v-model:selected-recovery-review-ids="selectedRecoveryReviewIds"
        :recovery-summary="recoverySummary"
        :recovery-status-label="recoveryStatusLabel"
        show-plugin-links
        show-selection-controls
        @open-plugin="$emit('openPlugin', $event)"
      >
        <template #after-skipped-plugins>
          <div v-if="pendingRecoveryPlugins.length" class="issue-alert-card issue-alert-card--warning" style="margin-top: 12px;">
            <div class="issue-alert-card__header">
              <span class="issue-alert-card__summary">{{ t('dashboard.recoveryConfirmSection') }}</span>
              <small style="color: var(--muted);">{{ selectedRecoveryReviewCountLabel }}</small>
            </div>
            <a-textarea
              v-model:value="recoveryConfirmNote"
              :rows="3"
              :maxlength="500"
              :placeholder="t('dashboard.recoveryConfirmNotePlaceholder')"
            />
            <div class="table-actions" style="justify-content: flex-start; margin-top: 12px;">
              <a-button
                data-testid="recovery-confirm-button"
                size="small"
                type="primary"
                :loading="recoveryConfirmPending"
                :disabled="selectedRecoveryReviewIds.length === 0"
                @click="$emit('confirm')"
              >
                {{ t('dashboard.recoveryConfirm') }}
              </a-button>
            </div>
          </div>

          <div v-else-if="recoverySummary.skipped_plugins?.length" class="readiness-note">
            <small style="color: var(--muted);">{{ t('dashboard.recoveryConfirmEmpty') }}</small>
          </div>
        </template>
      </RecoverySummaryDetails>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.dashboard-recovery-card {
  border-radius: var(--radius-xl);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    box-shadow: var(--shadow-elevated);
    border-color: var(--border-accent);
  }
}

.dashboard-recovery-card :deep(.ant-card-body) {
  padding: var(--space-lg);
}

.card-header {
  span {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--text);
  }
}

.readiness-note {
  margin-top: 14px;
  padding: 12px 16px;
  border-radius: var(--radius-lg);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  line-height: 1.5;
  color: var(--muted);
  font-weight: 500;
}

.table-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
