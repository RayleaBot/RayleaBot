<script setup lang="ts">
import AppTextarea from '@/components/AppTextarea.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import { t } from '@/i18n'
import { RotateCwIcon, DatabaseIcon } from '@lucide/vue'
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
  canBootstrap: boolean
}>()

defineEmits<{
  recheck: []
  bootstrap: []
  openPlugin: [pluginId: string]
  confirm: []
}>()
</script>

<template>
  <AppCard borderless class="dashboard-recovery-card">
    <template #title>
      <div class="card-header">
        <span>恢复兼容性</span>
      </div>
    </template>

    <p v-if="!recoverySummary" class="dashboard-recovery-card__empty">暂无恢复记录</p>

    <div v-else class="events-section">


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
            <AppTextarea
              v-model="recoveryConfirmNote"
              :aria-label="t('dashboard.recoveryConfirmNotePlaceholder')"
              :rows="3"
              :maxlength="500"
              :placeholder="t('dashboard.recoveryConfirmNotePlaceholder')"
            />
            <div class="table-actions" style="justify-content: flex-start; margin-top: 12px;">
              <AppButton
                data-testid="recovery-confirm-button"
                size="sm"
                variant="default"
                :loading="recoveryConfirmPending"
                :disabled="selectedRecoveryReviewIds.length === 0"
                @click="$emit('confirm')"
              >
                {{ t('dashboard.recoveryConfirm') }}
              </AppButton>
            </div>
          </div>

          <div v-else-if="recoverySummary.skipped_plugins?.length" class="readiness-note">
            <small style="color: var(--muted);">{{ t('dashboard.recoveryConfirmEmpty') }}</small>
          </div>
        </template>
      </RecoverySummaryDetails>
      <div class="dashboard-recovery-actions">
        <AppButton
          data-testid="recovery-recheck-button"
          size="sm"
          :loading="recoveryRecheckPending"
          @click="$emit('recheck')"
        >
          <template #icon><RotateCwIcon v-if="!recoveryRecheckPending" /></template>
          {{ t('dashboard.recoveryRecheck') }}
        </AppButton>
        <AppButton
          v-if="canBootstrap"
          data-testid="runtime-bootstrap-button"
          size="sm"
          :loading="runtimeBootstrapPending"
          @click="$emit('bootstrap')"
        >
          <template #icon><DatabaseIcon v-if="!runtimeBootstrapPending" /></template>
          {{ t('dashboard.runtimeBootstrap') }}
        </AppButton>
      </div>
    </div>
  </AppCard>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.dashboard-recovery-card__empty { margin: 0; padding: 12px 0; color: var(--muted); font-size: 13px; }
.dashboard-recovery-card { border: 1px solid var(--border); background: var(--surface); box-shadow: none; min-width: 0; }
.dashboard-recovery-card :deep(.app-card__body) { padding: 16px 20px; }
.card-header span { font-size: 16px; font-weight: 600; color: var(--text); }
.readiness-note { padding: 4px 0; color: var(--muted); line-height: 1.5; }
.dashboard-recovery-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 8px; }
.table-actions { display: flex; gap: 8px; flex-wrap: wrap; }
@media (max-width: #{bp.$phone}) {
 .dashboard-recovery-card :deep(.app-card__body) { padding: 14px; }
 .dashboard-recovery-actions { grid-template-columns: 1fr; gap: 8px; }
}
</style>
