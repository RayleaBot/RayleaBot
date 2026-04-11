<script setup lang="ts">
import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import DashboardRecentEventsCard from '@/components/DashboardRecentEventsCard.vue'
import DashboardReadinessCard from '@/components/DashboardReadinessCard.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import { useDashboardPage } from '@/views/dashboard/useDashboardPage'

const {
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
  <AppPage :title="t('dashboard.title')">
    <template #extra>
      <div class="dashboard-page__actions">
        <span v-if="lastRefreshed || autoRefresh" class="dashboard-page__refresh-meta">
          <template v-if="lastRefreshed">
            {{ `${t('dashboard.lastRefreshed')}: ${formatRelativeTime(lastRefreshed)}` }}
          </template>
          <template v-if="autoRefresh">
            <span v-if="lastRefreshed"> · </span>
            {{ `${t('dashboard.autoRefresh')}: ${countdown}s` }}
          </template>
        </span>
        <div class="dashboard-page__refresh-controls">
          <label class="dashboard-page__refresh-toggle">
            <span>{{ t('dashboard.autoRefresh') }}</span>
            <a-switch
              :checked="autoRefresh"
              size="small"
              @change="toggleAutoRefresh"
            />
          </label>
        </div>
        <a-button :loading="loading" @click="refreshState()">
          {{ t('dashboard.refresh') }}
        </a-button>
      </div>
    </template>

    <a-alert
      v-if="alertBannerType"
      :type="alertBannerType"
      :message="alertBannerTitle"
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

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

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

    <ConnectionStatusStrip />

    <div class="content-grid">
      <DashboardReadinessCard
        :section-title="t('dashboard.readinessSection')"
        :check-items="checkItems"
        readiness-note-text=""
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

      <DashboardToolsPanel
        :backup-pending="backupPending"
        :diagnostics-pending="diagnosticsPending"
        :preview-pending="previewPending"
        @create-backup="createBackup"
        @export-diagnostics="exportDiagnostics"
        @open-preview="previewVisible = true"
      />
    </div>

    <a-modal
      v-model:open="previewVisible"
      :get-container="false"
      :title="t('dashboard.previewTitle')"
      :confirm-loading="previewPending"
      :ok-text="t('dashboard.previewSubmit')"
      :cancel-text="t('dashboard.previewCancel')"
      @ok="submitRenderPreview"
    >
      <a-form layout="vertical">
        <a-form-item :label="t('dashboard.previewTemplate')">
          <a-input v-model:value="previewForm.template" placeholder="help.menu" />
        </a-form-item>
        <a-form-item :label="t('dashboard.previewTheme')">
          <a-input v-model:value="previewForm.theme" placeholder="default" />
        </a-form-item>
        <a-form-item :label="t('dashboard.previewOutput')">
          <a-radio-group v-model:value="previewForm.output" button-style="solid">
            <a-radio-button value="png">png</a-radio-button>
            <a-radio-button value="jpeg">jpeg</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item :label="t('dashboard.previewData')">
          <a-textarea
            v-model:value="previewForm.dataText"
            :rows="10"
            placeholder="{&quot;title&quot;:&quot;帮助菜单&quot;}"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </AppPage>
</template>

<style scoped lang="scss">
.dashboard-page__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}

.dashboard-page__refresh-meta {
  color: var(--muted);
  font-size: 0.84rem;
}

.dashboard-page__refresh-controls {
  display: flex;
  align-items: center;
}

.dashboard-page__refresh-toggle {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--muted);
  font-size: 0.84rem;
}

@media (max-width: 720px) {
  .dashboard-page__actions {
    justify-content: flex-start;
  }
}
</style>
