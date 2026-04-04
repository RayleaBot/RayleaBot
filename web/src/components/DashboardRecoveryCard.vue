<script setup lang="ts">
import { formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import type {
  RecoveryCompatibilityAuditEntry,
  RecoveryCompatibilitySkippedPlugin,
  RecoveryCompatibilitySummary,
} from '@/types/api'

const selectedRecoveryReviewIds = defineModel<string[]>('selectedRecoveryReviewIds', { default: () => [] })
const recoveryConfirmNote = defineModel<string>('recoveryConfirmNote', { default: '' })

defineProps<{
  recoverySummary: RecoveryCompatibilitySummary | null
  recoveryStatusLabel: string
  pendingRecoveryPlugins: RecoveryCompatibilitySkippedPlugin[]
  selectedRecoveryReviewCountLabel: string
  recoveryAuditEntries: RecoveryCompatibilityAuditEntry[]
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

      <div class="issue-alert-card" :class="{ 'issue-alert-card--warning': recoverySummary.status !== 'compatible' }">
        <div class="issue-alert-card__header">
          <el-tag :type="recoverySummary.status === 'blocked' ? 'danger' : recoverySummary.status === 'compatible' ? 'success' : 'warning'" size="small">
            {{ recoveryStatusLabel }}
          </el-tag>
          <span class="issue-alert-card__summary">
            {{ recoverySummary.operation }} · {{ recoverySummary.phase }}
          </span>
        </div>
        <div class="issue-alert-card__remediation">
          core {{ recoverySummary.source_core_version ?? t('display.empty') }} -> {{ recoverySummary.target_core_version ?? t('display.empty') }}
        </div>
      </div>

      <div v-for="issue in recoverySummary.issues ?? []" :key="issue.code" class="issue-alert-card" :class="{ 'issue-alert-card--warning': issue.severity === 'warning' }">
        <div class="issue-alert-card__header">
          <el-tag :type="issue.severity === 'error' ? 'danger' : 'warning'" size="small">
            {{ issue.code }}
          </el-tag>
          <span class="issue-alert-card__summary">{{ issue.summary }}</span>
        </div>
        <div v-if="issue.remediation" class="issue-alert-card__remediation">
          {{ issue.remediation }}
        </div>
      </div>

      <div v-for="plugin in recoverySummary.skipped_plugins ?? []" :key="plugin.plugin_id" class="issue-alert-card issue-alert-card--warning">
        <div class="issue-alert-card__header">
          <el-tag :type="plugin.review_status === 'confirmed' ? 'success' : 'warning'" size="small">
            {{ plugin.reason_code }}
          </el-tag>
          <el-button
            link
            type="primary"
            class="issue-alert-card__summary issue-alert-card__summary--link"
            :data-testid="`recovery-plugin-link-${plugin.plugin_id}`"
            @click="$emit('openPlugin', plugin.plugin_id)"
          >
            {{ plugin.plugin_id }}
          </el-button>
          <el-tag :type="plugin.review_status === 'confirmed' ? 'success' : 'warning'" size="small">
            {{ plugin.review_status === 'confirmed' ? t('dashboard.recoveryConfirmed') : t('dashboard.recoveryPending') }}
          </el-tag>
        </div>
        <div class="issue-alert-card__remediation">{{ plugin.summary }}</div>
        <div v-if="plugin.manual_action" class="issue-alert-card__remediation">{{ plugin.manual_action }}</div>
        <div v-if="plugin.review_status === 'confirmed'" class="issue-alert-card__remediation">
          {{ t('dashboard.recoveryReviewedBy') }}：{{ plugin.reviewed_by || t('display.empty') }}
          · {{ t('dashboard.recoveryReviewedAt') }}：{{ plugin.reviewed_at ? formatRelativeTime(plugin.reviewed_at) : t('display.empty') }}
        </div>
        <div v-else style="margin-top: 10px;">
          <el-checkbox-group v-model="selectedRecoveryReviewIds">
            <el-checkbox :value="plugin.review_id" :data-testid="`recovery-confirm-checkbox-${plugin.review_id}`">
              {{ t('dashboard.recoveryConfirm') }}
            </el-checkbox>
          </el-checkbox-group>
        </div>
      </div>

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

      <div v-if="recoverySummary.manual_actions?.length" style="margin-top: 12px;">
        <small style="color: var(--muted); display: block; margin-bottom: 6px;">处理建议</small>
        <ul style="margin: 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
          <li
            v-for="action in recoverySummary.manual_actions"
            :key="action"
            data-testid="recovery-manual-action"
          >
            {{ action }}
          </li>
        </ul>
      </div>

      <div v-if="recoverySummary.next_steps?.length" style="margin-top: 12px;">
        <small style="color: var(--muted); display: block; margin-bottom: 6px;">下一步</small>
        <ul style="margin: 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
          <li
            v-for="step in recoverySummary.next_steps"
            :key="step"
            data-testid="recovery-next-step"
          >
            {{ step }}
          </li>
        </ul>
      </div>

      <div v-if="recoveryAuditEntries.length" style="margin-top: 12px;">
        <small style="color: var(--muted); display: block; margin-bottom: 6px;">{{ t('dashboard.recoveryAudit') }}</small>
        <div
          v-for="entry in recoveryAuditEntries"
          :key="`${entry.task_id}-${entry.created_at}`"
          class="issue-alert-card"
        >
          <div class="issue-alert-card__header">
            <el-tag type="info" size="small">{{ entry.operator_id }}</el-tag>
            <span class="issue-alert-card__summary">{{ formatRelativeTime(entry.created_at) }}</span>
          </div>
          <div class="issue-alert-card__remediation">{{ entry.note || t('display.empty') }}</div>
          <ul style="margin: 8px 0 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
            <li v-for="item in entry.items" :key="item.review_id">
              {{ item.plugin_id }} · {{ item.reason_code }}
            </li>
          </ul>
        </div>
      </div>
      <div v-else class="readiness-note" style="margin-top: 12px;">
        <small style="color: var(--muted);">{{ t('dashboard.recoveryAuditEmpty') }}</small>
      </div>
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
