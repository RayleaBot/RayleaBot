<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons-vue'

import AppCard from '@/components/AppCard.vue'
import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getReadinessStatusLabel, getStatusType } from '@/lib/display'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { buildDashboardEventActions, buildDashboardProtocolActions } from '@/lib/management-links'
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
  eventsExpanded,
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
  protocolSnapshot,
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

function getEventSeverityColor(severity?: string): 'blue' | 'red' | 'orange' | 'green' | 'gray' {
  if (severity === 'error' || severity === 'danger') return 'red'
  if (severity === 'warning') return 'orange'
  if (severity === 'success') return 'green'
  return 'blue'
}

function getEventSeverityIcon(severity?: string) {
  if (severity === 'error' || severity === 'danger') return CloseCircleOutlined
  if (severity === 'warning') return ExclamationCircleOutlined
  if (severity === 'success') return CheckCircleOutlined
  return undefined
}

const protocolIssue = computed(() => {
  const snapshot = protocolSnapshot.value
  if (!snapshot) {
    return null
  }
  if (!['degraded', 'failed'].includes(snapshot.readiness_status)) {
    return null
  }
  return snapshot.recent_transport_issues[0] ?? null
})

const protocolIssueStatusLabel = computed(() => (
  protocolSnapshot.value ? getReadinessStatusLabel(protocolSnapshot.value.readiness_status) : t('display.empty')
))

const protocolIssueStatusType = computed(() => getStatusType(protocolSnapshot.value?.readiness_status))

const protocolIssueExtraCount = computed(() => {
  const count = protocolSnapshot.value?.recent_transport_issues.length ?? 0
  return count > 0 ? count - 1 : 0
})
const protocolAlertActions = computed(() => buildDashboardProtocolActions(protocolSnapshot.value))

function getProtocolIssueTagColor(status: typeof protocolIssueStatusType.value) {
  if (status === 'danger') return 'error'
  if (status === 'success') return 'success'
  return 'warning'
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
            :aria-label="t('dashboard.autoRefresh')"
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
      v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 0 } } }"
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
      <AppCard
        borderless
        class="dashboard-activity-card"
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 50 } } }"
      >
        <a-tabs v-model:activeKey="activeOverviewTab" size="small">
          <a-tab-pane key="events" :tab="t('dashboard.overviewEvents')">
            <a-empty v-if="recentEvents.length === 0" :description="t('dashboard.recentEventsEmpty')" />

            <div
              v-else
              class="events-timeline-wrapper"
              :class="{ 'events-timeline-wrapper--collapsed': !eventsExpanded && recentEvents.length > 4 }"
            >
              <a-timeline class="events-timeline">
                <a-timeline-item
                  v-for="(event, index) in recentEvents"
                  :key="`${event.timestamp}-${event.summary}`"
                  :color="getEventSeverityColor(getEventSeverity(event.payload))"
                  v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: index * 50 } } }"
                >
                  <template #dot>
                    <component
                      :is="getEventSeverityIcon(getEventSeverity(event.payload))"
                      v-if="getEventSeverityIcon(getEventSeverity(event.payload))"
                      class="events-timeline__dot-icon"
                      role="img"
                      :aria-label="`事件级别：${getEventSeverity(event.payload) ?? 'info'}`"
                    />
                    <span v-else class="events-timeline__dot" role="img" aria-label="事件级别：info" />
                  </template>
                  <div class="events-timeline__item">
                    <div class="events-timeline__summary">{{ event.summary }}</div>
                    <div class="events-timeline__time" :data-absolute="event.timestamp">
                      {{ formatRelativeTime(event.timestamp) }}
                    </div>
                    <ManagementContextActions
                      :actions="buildDashboardEventActions(event.payload)"
                      class="events-timeline__actions"
                    />
                  </div>
                </a-timeline-item>
              </a-timeline>
            </div>
            <div v-if="recentEvents.length > 4" class="events-toggle">
              <a-button size="small" type="link" @click="eventsExpanded = !eventsExpanded">
                {{ eventsExpanded ? t('dashboard.collapseEvents') : t('dashboard.expandEvents', { count: recentEvents.length - 4 }) }}
              </a-button>
            </div>
          </a-tab-pane>

          <a-tab-pane key="readiness" :tab="t('dashboard.overviewReadiness')">
            <div v-if="checkItems.length" class="readiness-checks">
              <div
                v-for="(item, index) in checkItems"
                :key="item.key"
                :class="['readiness-check', `readiness-check--${item.status}`]"
                v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: index * 50 } } }"
              >
                <div class="readiness-check__header">
                  <span class="readiness-check__icon" role="img" :aria-label="`检查状态：${item.status}`">{{ getCheckIcon(item.status) }}</span>
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
              <a-button
                size="small"
                type="link"
                :aria-label="issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 })"
                @click="issuesExpanded = !issuesExpanded"
              >
                {{ issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 }) }}
              </a-button>
            </div>
          </a-tab-pane>
        </a-tabs>
      </AppCard>

      <DashboardRecoveryCard
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 100 } } }"
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
      <ConnectionStatusStrip
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 150 } } }"
      />

      <DashboardToolsPanel
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 200 } } }"
        :backup-pending="backupPending"
        :diagnostics-pending="diagnosticsPending"
        :preview-pending="previewPending"
        @create-backup="createBackup"
        @export-diagnostics="exportDiagnostics"
        @open-preview="previewVisible = true"
      />

      <AppCard
        :title="t('dashboard.runtimeInfo')"
        borderless
        class="dashboard-runtime-card"
        v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 250 } } }"
      >
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

        <div v-if="protocolIssue" class="dashboard-protocol-alert" data-testid="dashboard-protocol-alert">
          <div class="dashboard-protocol-alert__header">
            <div class="dashboard-protocol-alert__heading">
              <span>{{ t('dashboard.protocolAlertTitle') }}</span>
              <strong>{{ protocolIssue.summary }}</strong>
            </div>
            <div class="dashboard-protocol-alert__actions">
              <a-tag :color="getProtocolIssueTagColor(protocolIssueStatusType)">{{ protocolIssueStatusLabel }}</a-tag>
              <ManagementContextActions :actions="protocolAlertActions" />
            </div>
          </div>

          <div class="dashboard-protocol-alert__meta">
            <a-tag color="warning">{{ protocolIssue.code }}</a-tag>
            <small v-if="protocolIssueExtraCount > 0">
              {{ t('dashboard.protocolIssueMore', { count: protocolIssueExtraCount }) }}
            </small>
          </div>
        </div>
      </AppCard>
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
  font-size: 0.78rem;
  background: var(--surface-soft);
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  font-weight: 500;
}

.dashboard-page__refresh-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 0.8rem;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  transition: background-color 0.2s ease;

  &:hover {
    background: var(--surface-soft);
  }
}

.dashboard-main-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(340px, 0.85fr);
  gap: var(--space-lg);
  margin-bottom: var(--space-lg);
}

.dashboard-bottom-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-lg);
}

.dashboard-activity-card {
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

.dashboard-activity-card :deep(.ant-card-body) {
  padding: var(--space-lg);
  padding-top: 6px;
}

.dashboard-activity-card :deep(.ant-tabs-nav) {
  margin-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.dashboard-activity-card :deep(.ant-tabs-tab) {
  font-size: 0.9rem;
  font-weight: 500;
  color: var(--muted);
  transition: all 0.2s ease;

  &:hover {
    color: var(--accent);
  }
}

.dashboard-activity-card :deep(.ant-tabs-tab-active) {
  font-weight: 700;

  .ant-tabs-tab-btn {
    color: var(--accent) !important;
  }
}

.dashboard-reason-codes {
  margin-top: 14px;
  padding: 6px 12px;
  background: var(--surface-soft);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);

  small {
    color: var(--muted);
    font-weight: 500;
  }
}

.dashboard-runtime-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-md);
}

.dashboard-protocol-alert {
  display: grid;
  gap: 12px;
  margin-top: 16px;
  padding: 16px;
  border-radius: var(--radius-lg);
  border: 1px solid color-mix(in srgb, var(--warning) 30%, var(--border));
  background: color-mix(in srgb, var(--warning) 6%, var(--surface-soft));
  box-shadow: var(--shadow-sm);
  transition: all 0.24s ease;

  &:hover {
    border-color: var(--warning);
    box-shadow: var(--shadow-elevated);
  }
}

.dashboard-protocol-alert__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.dashboard-protocol-alert__heading {
  display: grid;
  gap: 4px;

  span {
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  strong {
    font-size: 0.92rem;
    line-height: 1.4;
    color: var(--text);
  }
}

.dashboard-protocol-alert__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.dashboard-protocol-alert__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;

  small {
    color: var(--muted);
    line-height: 1.4;
    font-weight: 500;
  }
}

@media (max-width: 720px) {
  .dashboard-runtime-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-protocol-alert__header {
    flex-direction: column;
  }

  .dashboard-protocol-alert__actions {
    justify-content: flex-start;
  }
}

.events-timeline-wrapper--collapsed {
  max-height: 330px;
  overflow: hidden;
  position: relative;
}

.events-timeline-wrapper--collapsed::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 60px;
  background: linear-gradient(transparent, var(--surface-strong));
  pointer-events: none;
}

.events-toggle {
  margin-top: 14px;
  text-align: center;
}

.events-timeline {
  padding-top: 6px;
}

.events-timeline :deep(.ant-timeline-item-tail) {
  inset-inline-start: 13px;
  border-inline-start: 2px solid var(--border);
}

.events-timeline :deep(.ant-timeline-item-head) {
  inset-inline-start: 4px;
  width: 20px;
  height: 20px;
  background: var(--surface-strong);
  border: 0;
}

.events-timeline__dot-icon {
  font-size: 1.15rem;
  line-height: 1;
}

.events-timeline__dot {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--border-accent);
  margin: 5px;
  border: 2px solid var(--surface-strong);
  box-shadow: 0 0 0 1px var(--border);
}

.events-timeline__item {
  display: grid;
  gap: var(--space-xs);
  min-width: 0;
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  background: transparent;
  transition: all 0.24s ease;

  &:hover {
    background: var(--surface-soft);
    border-color: var(--border);
    transform: translateX(4px);
    box-shadow: var(--shadow-xs);
  }
}

.events-timeline__summary {
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.45;
  color: var(--text);
}

.events-timeline__time {
  flex-shrink: 0;
  color: var(--muted);
  font-size: 0.76rem;
  font-weight: 500;
  white-space: nowrap;
}

.events-timeline__actions {
  margin-top: 4px;
}

.readiness-checks {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--space-md);
}

.readiness-check {
  display: grid;
  gap: 6px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
    border-color: var(--border-accent);
  }
}

.readiness-check--success {
  border-color: var(--border-success);
  background: var(--surface-success);
  &:hover {
    border-color: var(--success);
    box-shadow: 0 4px 12px -4px color-mix(in srgb, var(--success) 20%, transparent);
  }
}

.readiness-check--warning {
  border-color: var(--border-warning);
  background: var(--surface-warning);
  &:hover {
    border-color: var(--warning);
    box-shadow: 0 4px 12px -4px color-mix(in srgb, var(--warning) 20%, transparent);
  }
}

.readiness-check--danger {
  border-color: var(--border-danger);
  background: var(--surface-danger);
  &:hover {
    border-color: var(--danger);
    box-shadow: 0 4px 12px -4px color-mix(in srgb, var(--danger) 20%, transparent);
  }
}

.readiness-check__header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.readiness-check__icon {
  font-size: 1.1rem;
  line-height: 1;
}

.readiness-check__name {
  font-weight: 700;
  font-size: 0.9rem;
  color: var(--text);
}

.readiness-check__value {
  font-size: 0.84rem;
  color: var(--muted);
  font-weight: 500;
  line-height: 1.4;
}

.issues-list {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}

.issues-list--collapsed {
  max-height: 240px;
  overflow: hidden;
  position: relative;
}

.issues-list--collapsed::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 60px;
  background: linear-gradient(transparent, var(--surface-strong));
  pointer-events: none;
}

.issues-toggle {
  margin-top: 14px;
  text-align: center;
}

.issue-alert-card {
  display: grid;
  gap: 8px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--danger) 5%, var(--surface-soft));
  box-shadow: var(--shadow-xs);
  position: relative;
  overflow: hidden;
  transition: all 0.24s ease;

  &::before {
    content: '';
    position: absolute;
    inset: 0 auto 0 0;
    width: 4px;
    background: var(--danger);
  }

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
    border-color: color-mix(in srgb, var(--danger) 30%, var(--border));
  }
}

.issue-alert-card--warning {
  background: color-mix(in srgb, var(--warning) 5%, var(--surface-soft));

  &::before {
    background: var(--warning);
  }

  &:hover {
    border-color: color-mix(in srgb, var(--warning) 30%, var(--border));
  }
}

.issue-alert-card__header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.issue-alert-card__summary {
  flex: 1;
  font-weight: 700;
  font-size: 0.9rem;
  color: var(--text);
}

.issue-alert-card__remediation {
  font-size: 0.84rem;
  color: var(--muted);
  line-height: 1.5;
  padding-left: 2px;
}

.dashboard-runtime-card {
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

.dashboard-runtime-item {
  position: relative;
  display: grid;
  gap: 4px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  box-shadow: var(--shadow-xs);
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  overflow: hidden;

  &::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    width: 3px;
    height: 100%;
    background: var(--border-accent);
    opacity: 0.7;
  }

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
    border-color: var(--border-accent);
  }

  span {
    color: var(--muted);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  strong {
    font-size: 1rem;
    font-weight: 800;
    line-height: 1.35;
    color: var(--text);
  }

  small {
    color: var(--muted);
    line-height: 1.45;
    font-size: 0.78rem;
    font-weight: 500;
  }
}

.text-success {
  color: var(--text-success) !important;
}

.text-warning {
  color: var(--text-warning) !important;
}

.text-danger {
  color: var(--text-danger) !important;
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
