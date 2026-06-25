<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

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
import { useToastFeedback } from '@/adapter/feedback'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { buildDashboardEventActions } from '@/lib/management-links'
import { t } from '@/i18n'
import { useDashboardPage } from '@/views/dashboard/useDashboardPage'

const activeOverviewTab = ref('events')
const uptimeClock = ref(Date.now())
const uptimeSnapshotAt = ref(Date.now())
let uptimeTimer: ReturnType<typeof window.setInterval> | null = null

const {
  adapterDetailText,
  adapterStatusType,
  adapterValueText,
  backupPending,
  bootstrapRuntimeResources,
  checkItems,
  confirmRecoverySelection,
  createBackup,
  diagnosticsIssueCards,
  diagnosticsPending,
  diagnosticsSubsystemItems,
  error,
  eventsExpanded,
  exportDiagnostics,
  healthDetailText,
  healthStatusType,
  healthValueText,
  issuesExpanded,
  loading,
  openRecoveryPlugin,
  pendingRecoveryPlugins,
  protocolSnapshot,
  readinessToastLevel,
  readinessToastMessage,
  readinessToastTitle,
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
  system,
  systemDetailText,
  systemValueText,
  visibleReasonCodes,
} = useDashboardPage()

watch(
  () => [readinessIssues.value.length, diagnosticsIssueCards.value.length] as const,
  ([readinessIssueCount, diagnosticsIssueCount]) => {
    if (readinessIssueCount > 0) {
      activeOverviewTab.value = 'readiness'
      return
    }

    if (diagnosticsIssueCount > 0) {
      activeOverviewTab.value = 'diagnostics'
      return
    }

    if (activeOverviewTab.value === 'readiness' || activeOverviewTab.value === 'diagnostics') {
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

function getStatusTagColor(status: typeof healthStatusType.value) {
  if (status === 'success') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'danger') return 'error'
  return 'default'
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

const readinessToast = computed(() => {
  if (!readinessToastLevel.value) {
    return null
  }
  const title = readinessToastTitle.value
  const detail = readinessToastMessage.value
  const message = detail ? `${title}：${detail}` : title
  return {
    key: `dashboard-readiness:${readinessToastLevel.value}:${title}:${detail}`,
    level: readinessToastLevel.value,
    message,
  }
})

const dashboardErrorToast = computed(() => (
  error.value && system.value
    ? {
        key: `dashboard-error:${error.value}`,
        level: 'error' as const,
        message: error.value,
      }
    : null
))

const protocolIssueToast = computed(() => (
  protocolIssue.value
    ? {
        key: `dashboard-protocol:${protocolIssue.value.code}:${protocolIssue.value.summary}`,
        level: protocolIssue.value.severity === 'error' ? 'error' as const : 'warning' as const,
        message: `${t('dashboard.protocolAlertTitle')}：${protocolIssue.value.summary}`,
      }
    : null
))
const liveUptimeSeconds = computed(() => {
  const baseUptime = system.value?.uptime_seconds
  if (baseUptime === undefined) {
    return undefined
  }

  const elapsedSeconds = Math.max(0, Math.floor((uptimeClock.value - uptimeSnapshotAt.value) / 1000))
  return baseUptime + elapsedSeconds
})

watch(
  () => system.value?.uptime_seconds,
  () => {
    uptimeClock.value = Date.now()
    uptimeSnapshotAt.value = uptimeClock.value
  },
  { immediate: true },
)

onMounted(() => {
  uptimeTimer = window.setInterval(() => {
    uptimeClock.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (uptimeTimer !== null) {
    window.clearInterval(uptimeTimer)
    uptimeTimer = null
  }
})

useToastFeedback(readinessToast)
useToastFeedback(dashboardErrorToast)
useToastFeedback(protocolIssueToast)
</script>

<template>
  <AppPage :title="t('dashboard.title')">
    <RetryPanel
      v-if="error && !system"
      :title="t('routes.status')"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

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
      :active-plugins-detail-text="t('dashboard.pluginStateCounts', { running: system?.running_plugins ?? 0, failed: system?.failed_plugins ?? 0 })"
      :active-plugins-to="{ name: 'plugins' }"
      :active-plugins-aria-label="t('dashboard.openPluginList')"
      :uptime-label="t('dashboard.uptime')"
      :uptime-text="formatDurationSeconds(liveUptimeSeconds)"
      :runtime-meta-text="t('dashboard.dbSchemaVersion', { version: system?.db_schema_version ?? t('display.empty') })"
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

          <a-tab-pane key="diagnostics" :tab="t('dashboard.overviewDiagnostics')">
            <div v-if="diagnosticsSubsystemItems.length" class="diagnostics-subsystem-grid">
              <div
                v-for="(item, index) in diagnosticsSubsystemItems"
                :key="item.key"
                :class="['diagnostics-subsystem', `diagnostics-subsystem--${item.status}`]"
                v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: index * 35 } } }"
              >
                <div class="diagnostics-subsystem__header">
                  <span class="diagnostics-subsystem__icon" role="img" :aria-label="`子系统状态：${item.status}`">{{ getCheckIcon(item.status) }}</span>
                  <span class="diagnostics-subsystem__label">{{ item.label }}</span>
                </div>
                <a-tag :color="getStatusTagColor(item.status)" class="diagnostics-subsystem__tag">
                  {{ item.value }}
                </a-tag>
                <div class="diagnostics-subsystem__detail">{{ item.detail }}</div>
              </div>
            </div>
            <a-empty v-else :description="t('dashboard.diagnosticsEmpty')" />

            <div v-if="diagnosticsIssueCards.length" class="diagnostics-issues">
              <div
                v-for="issue in diagnosticsIssueCards"
                :key="issue.key"
                :class="['diagnostics-issue-card', `diagnostics-issue-card--${issue.status}`]"
              >
                <div class="diagnostics-issue-card__header">
                  <a-tag :color="getStatusTagColor(issue.status)">
                    {{ issue.code }}
                  </a-tag>
                  <strong>{{ issue.problem }}</strong>
                </div>
                <dl class="diagnostics-issue-card__facts">
                  <div>
                    <dt>{{ t('dashboard.diagnosticsProblem') }}</dt>
                    <dd>{{ issue.problem }}</dd>
                  </div>
                  <div>
                    <dt>{{ t('dashboard.diagnosticsImpact') }}</dt>
                    <dd>{{ issue.impact }}</dd>
                  </div>
                  <div>
                    <dt>{{ t('dashboard.diagnosticsAction') }}</dt>
                    <dd>{{ issue.remediation }}</dd>
                  </div>
                </dl>
              </div>
            </div>
            <a-empty v-else class="diagnostics-empty-issues" :description="t('dashboard.diagnosticsNoIssues')" />
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
        @create-backup="createBackup"
        @export-diagnostics="exportDiagnostics"
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
        </div>
      </AppCard>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
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
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

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
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;

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
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

@media (max-width: 720px) {
  .dashboard-runtime-grid {
    grid-template-columns: 1fr;
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
  transition: transform 0.24s ease, box-shadow 0.24s ease, border-color 0.24s ease, background-color 0.24s ease, color 0.24s ease;

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
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

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

.diagnostics-subsystem-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--space-md);
}

.diagnostics-subsystem {
  display: grid;
  min-width: 0;
  gap: 8px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  box-shadow: var(--shadow-xs);
  transition: transform 0.24s ease, box-shadow 0.24s ease, border-color 0.24s ease, background-color 0.24s ease, color 0.24s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
  }
}

.diagnostics-subsystem--success {
  border-color: var(--border-success);
  background: var(--surface-success);
}

.diagnostics-subsystem--warning {
  border-color: var(--border-warning);
  background: var(--surface-warning);
}

.diagnostics-subsystem--danger {
  border-color: var(--border-danger);
  background: var(--surface-danger);
}

.diagnostics-subsystem__header {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}

.diagnostics-subsystem__icon {
  flex: 0 0 auto;
  font-size: 1.05rem;
  line-height: 1;
}

.diagnostics-subsystem__label {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.88rem;
  font-weight: 800;
  color: var(--text);
}

.diagnostics-subsystem__tag {
  width: fit-content;
  max-width: 100%;
  white-space: normal;
}

.diagnostics-subsystem__detail {
  min-width: 0;
  color: var(--muted);
  font-size: 0.8rem;
  font-weight: 500;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.diagnostics-issues {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}

.diagnostics-empty-issues {
  margin-top: 12px;
}

.diagnostics-issue-card {
  display: grid;
  min-width: 0;
  gap: 12px;
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  box-shadow: var(--shadow-xs);
  border-left-width: 4px;
}

.diagnostics-issue-card--success {
  border-left-color: var(--success);
}

.diagnostics-issue-card--warning {
  border-left-color: var(--warning);
  background: color-mix(in srgb, var(--warning) 5%, var(--surface-soft));
}

.diagnostics-issue-card--danger {
  border-left-color: var(--danger);
  background: color-mix(in srgb, var(--danger) 5%, var(--surface-soft));
}

.diagnostics-issue-card__header {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 10px;

  strong {
    min-width: 0;
    overflow-wrap: anywhere;
    font-size: 0.92rem;
    line-height: 1.45;
    color: var(--text);
  }
}

.diagnostics-issue-card__facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 0;

  div {
    min-width: 0;
    padding: 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: color-mix(in srgb, var(--surface-strong) 80%, transparent);
  }

  dt {
    margin-bottom: 4px;
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 800;
  }

  dd {
    margin: 0;
    color: var(--text);
    font-size: 0.82rem;
    font-weight: 500;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }
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
  transition: transform 0.24s ease, box-shadow 0.24s ease, border-color 0.24s ease, background-color 0.24s ease, color 0.24s ease;

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
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

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
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
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
  .diagnostics-issue-card__facts {
    grid-template-columns: 1fr;
  }
}
</style>
