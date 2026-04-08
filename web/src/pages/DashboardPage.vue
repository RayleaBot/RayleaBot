<script setup lang="ts">
import DashboardHeroPanel from '@/components/DashboardHeroPanel.vue'
import DashboardRecentEventsCard from '@/components/DashboardRecentEventsCard.vue'
import DashboardReadinessCard from '@/components/DashboardReadinessCard.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import { useDashboardPage } from '@/pages/dashboard/useDashboardPage'

const {
  AUTO_REFRESH_INTERVAL,
  adapterDetailText,
  adapterStatusType,
  adapterValueText,
  alertBannerMessage,
  alertBannerTitle,
  alertBannerType,
  autoRefresh,
  backupPending,
  bootstrapRuntimeResources,
  checkItems,
  confirmRecoverySelection,
  countdown,
  createBackup,
  diagnosticsPending,
  error,
  exportDiagnostics,
  health,
  healthDetailText,
  healthStatusType,
  healthValueText,
  issuesExpanded,
  lastRefreshed,
  loading,
  openRecoveryPlugin,
  pendingRecoveryPlugins,
  previewForm,
  previewPending,
  previewVisible,
  readiness,
  readinessDetailText,
  readinessIssues,
  readinessStatusType,
  readinessValueText,
  recentEvents,
  recoveryConfirmNote,
  recoveryConfirmPending,
  recoveryRecheckPending,
  recoveryStatusLabel,
  recoverySummary,
  refreshState,
  recheckRecoverySummary,
  runtimeBootstrapPending,
  selectedRecoveryReviewCountLabel,
  selectedRecoveryReviewIds,
  statusBadgeConfig,
  submitRenderPreview,
  system,
  systemDetailText,
  systemStatusType,
  systemValueText,
  toggleAutoRefresh,
  visibleReasonCodes,
} = useDashboardPage()
</script>

<template>
  <div class="page-grid">
    <DashboardHeroPanel
      :title="t('dashboard.title')"
      :status-badge="statusBadgeConfig"
      :last-refreshed-label="lastRefreshed ? `${t('dashboard.lastRefreshed')}: ${formatRelativeTime(lastRefreshed)}` : null"
      :auto-refresh="autoRefresh"
      :countdown="countdown"
      :auto-refresh-interval="AUTO_REFRESH_INTERVAL"
      :loading="loading"
      @refresh="refreshState()"
      @toggle-auto-refresh="toggleAutoRefresh"
    />

    <el-alert
      v-if="alertBannerType"
      :type="alertBannerType"
      :title="alertBannerTitle"
      :description="alertBannerMessage"
      show-icon
      :closable="false"
    />

    <RetryPanel
      v-if="error && !system"
      :title="t('routes.status')"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <DashboardStatusGrid
      :health-status-type="healthStatusType"
      :readiness-status-type="readinessStatusType"
      :system-status-type="systemStatusType"
      :adapter-status-type="adapterStatusType"
      :health-label="t('dashboard.health')"
      :health-value-text="healthValueText"
      :health-detail-text="healthDetailText"
      :readiness-label="t('dashboard.readiness')"
      :readiness-value-text="readinessValueText"
      :readiness-detail-text="readinessDetailText"
      :system-label="t('dashboard.service')"
      :system-value-text="systemValueText"
      :system-detail-text="systemDetailText"
      :adapter-label="t('dashboard.adapter')"
      :adapter-value-text="adapterValueText"
      :adapter-detail-text="adapterDetailText"
      :active-plugins-label="t('dashboard.activePlugins')"
      :active-plugins-count="system?.active_plugins ?? 0"
      :uptime-label="t('dashboard.uptime')"
      :uptime-text="formatDurationSeconds(system?.uptime_seconds)"
    />

    <div class="content-grid">
      <DashboardReadinessCard
        :section-title="t('dashboard.readinessSection')"
        :check-items="checkItems"
        :readiness-note-text="health?.status === 'ok' && readiness?.status === 'degraded' ? t('dashboard.readinessLimitedHint') : t('dashboard.readinessHint')"
        :reason-codes-label="t('dashboard.reasonCodes')"
        :visible-reason-codes="visibleReasonCodes"
        :readiness-issues="readinessIssues"
        :issues-expanded="issuesExpanded"
        :expand-issues-text="t('dashboard.expandIssues', { count: readinessIssues.length - 3 })"
        :collapse-issues-text="t('dashboard.collapseIssues')"
        @toggle-issues="issuesExpanded = !issuesExpanded"
      />

      <DashboardRecoveryCard
        v-model:selected-recovery-review-ids="selectedRecoveryReviewIds"
        v-model:recovery-confirm-note="recoveryConfirmNote"
        :recovery-summary="recoverySummary"
        :recovery-status-label="recoveryStatusLabel"
        :pending-recovery-plugins="pendingRecoveryPlugins"
        :selected-recovery-review-count-label="selectedRecoveryReviewCountLabel"
        :recovery-recheck-pending="recoveryRecheckPending"
        :recovery-confirm-pending="recoveryConfirmPending"
        :runtime-bootstrap-pending="runtimeBootstrapPending"
        @recheck="recheckRecoverySummary"
        @bootstrap="bootstrapRuntimeResources"
        @open-plugin="openRecoveryPlugin"
        @confirm="confirmRecoverySelection"
      />

      <DashboardRecentEventsCard :recent-events="recentEvents" />
    </div>

    <DashboardToolsPanel
      :backup-pending="backupPending"
      :diagnostics-pending="diagnosticsPending"
      :preview-pending="previewPending"
      @create-backup="createBackup"
      @export-diagnostics="exportDiagnostics"
      @open-preview="previewVisible = true"
    />

    <el-dialog v-model="previewVisible" :title="t('dashboard.previewTitle')" width="min(720px, 92vw)">
      <el-form label-position="top">
        <el-form-item :label="t('dashboard.previewTemplate')">
          <el-input v-model="previewForm.template" placeholder="help.menu" />
        </el-form-item>
        <el-form-item :label="t('dashboard.previewTheme')">
          <el-input v-model="previewForm.theme" placeholder="default" />
        </el-form-item>
        <el-form-item :label="t('dashboard.previewOutput')">
          <el-radio-group v-model="previewForm.output">
            <el-radio-button label="png" value="png" />
            <el-radio-button label="jpeg" value="jpeg" />
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('dashboard.previewData')">
          <el-input
            v-model="previewForm.dataText"
            type="textarea"
            :rows="10"
            placeholder="{&quot;title&quot;:&quot;帮助菜单&quot;}"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="table-actions">
          <el-button @click="previewVisible = false">
            {{ t('dashboard.previewCancel') }}
          </el-button>
          <el-button type="primary" :loading="previewPending" @click="submitRenderPreview">
            {{ t('dashboard.previewSubmit') }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
