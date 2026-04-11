<script setup lang="ts">
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
  <AppPage :title="t('dashboard.title')">
    <template #extra>
      <div class="table-actions">
        <a-button :loading="loading" @click="refreshState()">
          {{ t('dashboard.refresh') }}
        </a-button>
      </div>
    </template>

    <template #toolbar>
      <a-card :bordered="false" class="app-view-card dashboard-hero-card">
        <div class="dashboard-hero-card__main">
          <div class="dashboard-hero-card__status">
            <div :class="['status-badge', `status-badge--${statusBadgeConfig.type}`]">
              <span class="status-badge__icon">{{ statusBadgeConfig.icon }}</span>
              <span>{{ statusBadgeConfig.label }}</span>
            </div>
            <div v-if="lastRefreshed" class="hero-meta__time">
              {{ `${t('dashboard.lastRefreshed')}: ${formatRelativeTime(lastRefreshed)}` }}
              <template v-if="autoRefresh"> · {{ countdown }}s</template>
            </div>
          </div>
          <div class="dashboard-hero-card__actions">
            <div class="hero-auto-refresh">
              <span>{{ t('dashboard.autoRefresh') }}</span>
              <a-switch
                :checked="autoRefresh"
                size="small"
                @change="toggleAutoRefresh"
              />
            </div>
          </div>
        </div>
        <div v-if="autoRefresh" class="auto-refresh-bar">
          <div class="auto-refresh-bar__fill" :style="{ width: `${(countdown / AUTO_REFRESH_INTERVAL) * 100}%` }" />
        </div>
      </a-card>
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
    </div>

    <DashboardToolsPanel
      :backup-pending="backupPending"
      :diagnostics-pending="diagnosticsPending"
      :preview-pending="previewPending"
      @create-backup="createBackup"
      @export-diagnostics="exportDiagnostics"
      @open-preview="previewVisible = true"
    />

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
.dashboard-hero-card {
  display: grid;
  gap: 14px;
}

.dashboard-hero-card__main {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  align-items: center;
  flex-wrap: wrap;
}

.dashboard-hero-card__status {
  display: grid;
  gap: 12px;
}

.dashboard-hero-card__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
</style>
