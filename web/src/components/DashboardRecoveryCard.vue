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
  <el-card>
    <template #header>
      <div class="card-header">
        <span>恢复兼容性</span>
      </div>
    </template>

    <el-empty v-if="!recoverySummary" :description="t('display.empty')" />

    <div v-else class="events-section">
      <div class="table-actions" style="justify-content: flex-start; margin-bottom: 12px;">
        <el-button
          data-testid="recovery-recheck-button"
          size="small"
          :loading="recoveryRecheckPending"
          @click="$emit('recheck')"
        >
          {{ t('dashboard.recoveryRecheck') }}
        </el-button>
        <el-button
          data-testid="runtime-bootstrap-button"
          size="small"
          :loading="runtimeBootstrapPending"
          @click="$emit('bootstrap')"
        >
          {{ t('dashboard.runtimeBootstrap') }}
        </el-button>
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
            <el-input
              v-model="recoveryConfirmNote"
              type="textarea"
              :rows="3"
              :maxlength="500"
              show-word-limit
              :placeholder="t('dashboard.recoveryConfirmNotePlaceholder')"
            />
            <div class="table-actions" style="justify-content: flex-start; margin-top: 12px;">
              <el-button
                data-testid="recovery-confirm-button"
                size="small"
                type="primary"
                :loading="recoveryConfirmPending"
                :disabled="selectedRecoveryReviewIds.length === 0"
                @click="$emit('confirm')"
              >
                {{ t('dashboard.recoveryConfirm') }}
              </el-button>
            </div>
          </div>

          <div v-else-if="recoverySummary.skipped_plugins?.length" class="readiness-note">
            <small style="color: var(--muted);">{{ t('dashboard.recoveryConfirmEmpty') }}</small>
          </div>
        </template>
      </RecoverySummaryDetails>
    </div>
  </el-card>
</template>

<style scoped lang="scss">
.readiness-note {
  margin-top: 14px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.04);
  border: 1px solid rgba(148, 163, 184, 0.18);
  line-height: 1.5;
}
</style>
