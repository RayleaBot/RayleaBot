<script setup lang="ts">
import { ref, watch } from 'vue'

import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons-vue'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import { useDashboardPage } from '@/views/dashboard/useDashboardPage'

const activeOverviewTab = ref('events')

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
  systemValueText,
  toggleAutoRefresh,
  visibleReasonCodes,
} = useDashboardPage()

watch(
  () => readinessIssues.value.length,
  (issueCount) => {
    if (issueCount > 0) {
      activeOverviewTab.value = 'readiness'
      return
    }

    if (activeOverviewTab.value === 'readiness') {
      activeOverviewTab.value = 'events'
    }
  },
  { immediate: true },
)

function getCheckIcon(status: typeof healthStatusType.value) {
  const map = {
    danger: '❌',
    muted: '—',
    success: '✅',
    warning: '⚠',
  } as const
  return map[status]
}

function getEventSeverity(payload: Record<string, unknown>) {
  const severity = payload.severity
  return typeof severity === 'string' ? severity : undefined
}

function getEventSeverityClass(severity?: string) {
  if (severity === 'error' || severity === 'danger') return 'event-item--danger'
  if (severity === 'warning') return 'event-item--warning'
  if (severity === 'success') return 'event-item--success'
  return ''
}

function getEventSeverityIcon(severity?: string) {
  if (severity === 'error' || severity === 'danger') return CloseCircleOutlined
  if (severity === 'warning') return ExclamationCircleOutlined
  if (severity === 'success') return CheckCircleOutlined
  return undefined
}
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
        <label class="dashboard-page__refresh-toggle">
          <span>{{ t('dashboard.autoRefresh') }}</span>
          <a-switch
            :checked="autoRefresh"
            size="small"
            @change="toggleAutoRefresh"
          />
        </label>
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
      :health-label="t('dashboard.health')"
      :health-value-text="healthValueText"
      :health-detail-text="healthDetailText"
      :readiness-label="t('dashboard.readiness')"
      :readiness-value-text="readinessValueText"
      :readiness-detail-text="readinessDetailText"
      :active-plugins-label="t('dashboard.activePlugins')"
      :active-plugins-count="system?.active_plugins ?? 0"
      :uptime-label="t('dashboard.uptime')"
      :uptime-text="formatDurationSeconds(system?.uptime_seconds)"
    />

    <div class="dashboard-main-grid">
      <a-card :bordered="false" class="dashboard-activity-card">
        <a-tabs v-model:activeKey="activeOverviewTab" size="small">
          <a-tab-pane key="events" :tab="t('dashboard.overviewEvents')">
            <a-empty v-if="recentEvents.length === 0" :description="t('dashboard.recentEventsEmpty')" />

            <div v-else class="events-section">
              <div
                v-for="event in recentEvents"
                :key="`${event.timestamp}-${event.summary}`"
                :class="['event-item', getEventSeverityClass(getEventSeverity(event.payload))]"
              >
                <component
                  :is="getEventSeverityIcon(getEventSeverity(event.payload))"
                  v-if="getEventSeverityIcon(getEventSeverity(event.payload))"
                  class="event-item__icon"
                />
                <span v-else class="event-item__dot" />
                <div class="event-item__body">
                  <strong>{{ event.summary }}</strong>
                  <span class="event-item__time" :data-absolute="event.timestamp">
                    {{ formatRelativeTime(event.timestamp) }}
                  </span>
                </div>
              </div>
            </div>
          </a-tab-pane>

          <a-tab-pane key="readiness" :tab="t('dashboard.overviewReadiness')">
            <div v-if="checkItems.length" class="readiness-checks">
              <div
                v-for="item in checkItems"
                :key="item.key"
                :class="['readiness-check', `readiness-check--${item.status}`]"
              >
                <div class="readiness-check__header">
                  <span class="readiness-check__icon">{{ getCheckIcon(item.status) }}</span>
                  <span class="readiness-check__name">{{ item.key }}</span>
                </div>
                <div class="readiness-check__value">{{ item.value }}</div>
              </div>
            </div>
            <a-empty v-else :description="t('display.empty')" />

            <div v-if="visibleReasonCodes.length" class="dashboard-reason-codes">
              <small>{{ t('dashboard.reasonCodes') }}: {{ visibleReasonCodes.join(', ') }}</small>
            </div>

            <div
              v-if="readinessIssues.length"
              class="issues-list"
              :class="{ 'issues-list--collapsed': !issuesExpanded && readinessIssues.length > 3 }"
            >
              <div
                v-for="issue in readinessIssues"
                :key="`${issue.code}-${issue.summary}`"
                :class="['issue-alert-card', { 'issue-alert-card--warning': issue.severity === 'warning' }]"
              >
                <div class="issue-alert-card__header">
                  <a-tag :color="issue.severity === 'error' ? 'error' : issue.severity === 'warning' ? 'warning' : 'success'">
                    {{ issue.code }}
                  </a-tag>
                  <span class="issue-alert-card__summary">{{ issue.summary }}</span>
                </div>
                <div v-if="issue.remediation" class="issue-alert-card__remediation">
                  {{ issue.remediation }}
                </div>
              </div>
            </div>

            <div v-if="readinessIssues.length > 3" class="issues-toggle">
              <a-button size="small" type="link" @click="issuesExpanded = !issuesExpanded">
                {{ issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 }) }}
              </a-button>
            </div>
          </a-tab-pane>
        </a-tabs>
      </a-card>

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
    </div>

    <div class="dashboard-bottom-grid">
      <ConnectionStatusStrip />

      <DashboardToolsPanel
        :backup-pending="backupPending"
        :diagnostics-pending="diagnosticsPending"
        :preview-pending="previewPending"
        @create-backup="createBackup"
        @export-diagnostics="exportDiagnostics"
        @open-preview="previewVisible = true"
      />

      <a-card :bordered="false" class="dashboard-runtime-card">
        <template #title>
          <div class="card-header">
            <span>{{ t('dashboard.runtimeInfo') }}</span>
          </div>
        </template>

        <div class="dashboard-runtime-grid">
          <div class="dashboard-runtime-item">
            <span>{{ t('dashboard.service') }}</span>
            <strong>{{ systemValueText }}</strong>
            <small>{{ systemDetailText }}</small>
          </div>
          <div class="dashboard-runtime-item">
            <span>{{ t('dashboard.adapter') }}</span>
            <strong :class="`text-${adapterStatusType}`">{{ adapterValueText }}</strong>
            <small>{{ adapterDetailText }}</small>
          </div>
          <div class="dashboard-runtime-item">
            <span>{{ t('dashboard.lastRefreshed') }}</span>
            <strong>{{ lastRefreshed ? formatRelativeTime(lastRefreshed) : t('display.empty') }}</strong>
            <small>{{ autoRefresh ? `${t('dashboard.autoRefresh')} ${countdown}s` : t('dashboard.refresh') }}</small>
          </div>
        </div>
      </a-card>
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
  gap: 10px;
  flex-wrap: wrap;
}

.dashboard-page__refresh-meta {
  color: var(--muted);
  font-size: 0.82rem;
}

.dashboard-page__refresh-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 0.82rem;
}

.dashboard-main-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(340px, 0.88fr);
  gap: 12px;
}

.dashboard-bottom-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-activity-card :deep(.ant-card-body) {
  padding-top: 10px;
}

.dashboard-activity-card :deep(.ant-tabs-nav) {
  margin-bottom: 14px;
}

.dashboard-reason-codes {
  margin-top: 14px;

  small {
    color: var(--muted);
  }
}

.dashboard-runtime-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 720px) {
  .dashboard-runtime-grid {
    grid-template-columns: 1fr;
  }
}

.event-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.event-item__icon {
  flex-shrink: 0;
  font-size: 1rem;
  margin-top: 2px;
}

.event-item--success .event-item__icon {
  color: var(--success);
}

.event-item--warning .event-item__icon {
  color: var(--warning);
}

.event-item--danger .event-item__icon {
  color: var(--danger);
}

.event-item__dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--muted);
  flex-shrink: 0;
  margin-top: 6px;
}

.event-item__body {
  flex: 1 1 auto;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.event-item__body strong {
  font-size: 0.92rem;
  line-height: 1.4;
}

.event-item__time {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 0.78rem;
  white-space: nowrap;
}

.dashboard-runtime-item {
  display: grid;
  gap: 4px;
  padding: 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-soft);

  span {
    color: var(--muted);
    font-size: 0.8rem;
  }

  strong {
    font-size: 1rem;
    line-height: 1.3;
  }

  small {
    color: var(--muted);
    line-height: 1.45;
  }
}

.text-success {
  color: var(--success);
}

.text-warning {
  color: var(--warning);
}

.text-danger {
  color: var(--danger);
}

@media (max-width: 1200px) {
  .dashboard-main-grid,
  .dashboard-bottom-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .dashboard-page__actions {
    justify-content: flex-start;
  }
}
</style>
