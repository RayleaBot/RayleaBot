<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  MinusCircleOutlined,
  DatabaseOutlined,
} from '@ant-design/icons-vue'

import AppCard from '@/components/AppCard.vue'
import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import DashboardUpdateCard from '@/components/DashboardUpdateCard.vue'
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
    danger: CloseCircleOutlined,
    muted: MinusCircleOutlined,
    success: CheckCircleOutlined,
    warning: ExclamationCircleOutlined,
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

function getEventSeverityColor(severity?: string) {
  if (severity === 'error' || severity === 'danger') return 'var(--danger)'
  if (severity === 'warning') return 'var(--warning)'
  if (severity === 'success') return 'var(--success)'
  return 'var(--brand-foreground)'
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
  <AppPage :title="t('dashboard.title')" width="detail">
    <RetryPanel
      v-if="error && !system"
      :title="t('routes.status')"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

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
      :active-plugins-detail-text="t('dashboard.pluginStateCounts', { running: system?.running_plugins ?? 0, failed: system?.failed_plugins ?? 0 })"
      :active-plugins-to="{ name: 'plugins' }"
      :active-plugins-aria-label="t('dashboard.openPluginList')"
      :uptime-label="t('dashboard.uptime')"
      :uptime-text="formatDurationSeconds(liveUptimeSeconds)"
      :runtime-meta-text="t('dashboard.dbSchemaVersion', { version: system?.db_schema_version ?? t('display.empty') })"
    />

    <ConnectionStatusStrip />

    <div class="dashboard-main-grid">
      <div class="dashboard-primary-column">
      <AppCard
        borderless
        class="dashboard-activity-card"
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
                  v-for="event in recentEvents"
                  :key="`${event.timestamp}-${event.summary}`"
                  :color="getEventSeverityColor(getEventSeverity(event.payload))"
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
                v-for="item in checkItems"
                :key="item.key"
                :class="['readiness-check', `readiness-check--${item.status}`]"
              >
                <div class="readiness-check__header">
                  <component :is="getCheckIcon(item.status)" class="readiness-check__icon" role="img" :aria-label="`检查状态：${item.status}`" />
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
                v-for="item in diagnosticsSubsystemItems"
                :key="item.key"
                :class="['diagnostics-subsystem', `diagnostics-subsystem--${item.status}`]"
              >
                <div class="diagnostics-subsystem__header">
                  <component :is="getCheckIcon(item.status)" class="diagnostics-subsystem__icon" role="img" :aria-label="`子系统状态：${item.status}`" />
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

      <aside class="dashboard-support-column">
      <AppCard
        :title="t('dashboard.runtimeInfo')"
        borderless
        class="dashboard-runtime-card"
      >
        <div class="dashboard-runtime-body">
          <DatabaseOutlined class="dashboard-panel-icon" aria-hidden="true" />
          <div class="dashboard-runtime-grid">
          <div class="dashboard-runtime-item">
            <span>{{ t('dashboard.service') }}</span>
            <strong>{{ systemValueText }}</strong>
            <small v-if="systemDetailText !== systemValueText">{{ systemDetailText }}</small>
          </div>
          <div class="dashboard-runtime-item">
            <span>{{ t('dashboard.adapter') }}</span>
            <strong :class="`text-${adapterStatusType}`">{{ adapterValueText }}</strong>
            <small v-if="adapterDetailText !== adapterValueText">{{ adapterDetailText }}</small>
          </div>
          </div>
        </div>
      </AppCard>

      <DashboardToolsPanel
        :backup-pending="backupPending"
        :diagnostics-pending="diagnosticsPending"
        @create-backup="createBackup"
        @export-diagnostics="exportDiagnostics"
      />

      <DashboardUpdateCard />

      </aside>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
.dashboard-main-grid { display: grid; grid-template-columns: minmax(0, 1.8fr) minmax(300px, .85fr); gap: 16px; align-items: start; }
.dashboard-primary-column, .dashboard-support-column { display: grid; min-width: 0; gap: 16px; align-content: start; }
.dashboard-activity-card { min-width: 0; align-self: stretch; }
.dashboard-activity-card :deep(.ant-card-body) { padding: 6px 20px 16px; }
.dashboard-activity-card :deep(.ant-tabs-nav) { margin-bottom: 10px; }
.dashboard-activity-card :deep(.ant-tabs-tab) { font-size: 14px; color: var(--muted); }
.dashboard-activity-card :deep(.ant-tabs-tab-active) { font-weight: 600; }
.events-timeline { padding-top: 6px; }
.events-timeline :deep(.ant-timeline-item) { padding-bottom: 0; }
.events-timeline :deep(.ant-timeline-item-tail) { display: none; }
.events-timeline :deep(.ant-timeline-item-content) { margin-inline-start: 28px; min-height: 58px; top: 0; }
.events-timeline :deep(.ant-timeline-item-head) { inset-inline-start: 0; top: 14px; width: 16px; height: 16px; background: transparent; border: 0; display: grid; place-items: center; transform: none; }
.events-timeline__dot-icon { font-size: 18px; line-height: 1; }
.events-timeline__dot { display: block; width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
.events-timeline__item { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 12px; min-width: 0; padding: 10px 0; border-bottom: 1px solid var(--border); }
.events-timeline__summary { font-size: 14px; font-weight: 500; line-height: 1.5; color: var(--text); overflow-wrap: anywhere; }
.events-timeline__time { color: var(--muted); font-size: 12px; grid-column: 1; }
.events-timeline__actions { grid-column: 2; grid-row: 1 / span 2; align-self: center; }
.events-timeline-wrapper--collapsed { max-height: 284px; overflow: hidden; }
.events-toggle, .issues-toggle { margin-top: 8px; text-align: center; }
.dashboard-reason-codes { margin-top: 12px; color: var(--muted); overflow-wrap: anywhere; }
.readiness-checks { grid-template-columns: 1fr; gap: 0; border: 0; background: transparent; border-radius: 0; }
.readiness-check { grid-template-columns: minmax(120px, .8fr) minmax(0, 1fr); padding: 10px 0; gap: 12px; border-bottom: 1px solid var(--border); background: transparent; }
.readiness-check__name { color: var(--text); font-weight: 500; }
.readiness-check__value { text-align: right; color: var(--muted); }
.readiness-check__icon, .diagnostics-subsystem__icon { font-size: 17px; }
.diagnostics-subsystem-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 20px; }
.diagnostics-subsystem { display: grid; grid-template-columns: minmax(0, 1fr) auto; min-width: 0; gap: 6px 10px; padding: 12px 0; border-bottom: 1px solid var(--border); }
.diagnostics-subsystem__header { display: flex; align-items: center; gap: 8px; min-width: 0; font-weight: 500; }
.diagnostics-subsystem__label { overflow-wrap: anywhere; }
.diagnostics-subsystem__detail { grid-column: 1 / -1; color: var(--muted); font-size: 13px; overflow-wrap: anywhere; }
.diagnostics-subsystem--success .diagnostics-subsystem__icon { color: var(--success); }
.diagnostics-subsystem--warning .diagnostics-subsystem__icon { color: var(--warning); }
.diagnostics-subsystem--danger .diagnostics-subsystem__icon { color: var(--danger); }
.diagnostics-issues, .issues-list { display: grid; gap: 12px; margin-top: 16px; }
.diagnostics-issue-card, .issue-alert-card { min-width: 0; padding: 12px; border-radius: 8px; border: 0; background: var(--surface-danger); color: var(--text-danger); }
.diagnostics-issue-card--warning, .issue-alert-card--warning { background: var(--surface-warning); color: var(--text-warning); }
.diagnostics-issue-card--success { background: var(--surface-success); }
.diagnostics-issue-card__header, .issue-alert-card__header { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 10px; }
.issue-alert-card__summary { font-weight: 600; }
.issue-alert-card__remediation { margin-top: 8px; color: inherit; font-size: 13px; line-height: 1.5; }
.diagnostics-issue-card__facts { display: grid; gap: 8px; margin: 12px 0 0; }
.diagnostics-issue-card__facts > div { display: grid; grid-template-columns: 48px minmax(0, 1fr); gap: 8px; }
.diagnostics-issue-card__facts dt, .diagnostics-issue-card__facts dd { margin: 0; font-size: 13px; line-height: 1.5; overflow-wrap: anywhere; }
.diagnostics-issue-card__facts dt { font-weight: 500; }
.diagnostics-empty-issues { margin-top: 12px; }
.dashboard-runtime-body { display: flex; align-items: center; gap: 18px; }
.dashboard-panel-icon { width: 28px; height: 32px; flex: 0 0 auto; display: grid; place-items: center; background: transparent; color: var(--muted); font-size: 26px; }
.dashboard-runtime-grid { display: grid; flex: 1; min-width: 0; }
.dashboard-runtime-item { display: grid; grid-template-columns: minmax(78px, .6fr) minmax(0, 1fr); gap: 3px 12px; padding: 8px 0; min-width: 0; }
.dashboard-runtime-item + .dashboard-runtime-item { border-top: 1px solid var(--border); }
.dashboard-runtime-item span { grid-row: span 2; font-size: 13px; color: var(--muted); }
.dashboard-runtime-item strong { font-size: 13px; font-weight: 500; text-align: right; overflow-wrap: anywhere; }
.dashboard-runtime-item small { font-size: 12px; line-height: 1.4; color: var(--muted); text-align: right; overflow-wrap: anywhere; }
.text-success { color: var(--text-success) !important; }
.text-warning { color: var(--text-warning) !important; }
.text-danger { color: var(--text-danger) !important; }
@media (max-width: 1100px) {
 .dashboard-main-grid { grid-template-columns: 1fr; }
 .dashboard-support-column { grid-template-columns: repeat(2, minmax(0, 1fr)); }
 .dashboard-runtime-card { grid-column: 1 / -1; }
}
@media (max-width: 640px) {
 .dashboard-support-column, .diagnostics-subsystem-grid { grid-template-columns: 1fr; }
 .dashboard-activity-card :deep(.ant-card-body) { padding-inline: 14px; }
 .events-timeline__item { grid-template-columns: minmax(0, 1fr); }
 .events-timeline__actions { grid-column: 1; grid-row: auto; }
 .events-timeline-wrapper--collapsed { max-height: 390px; }
 .dashboard-panel-icon { width: 44px; height: 44px; font-size: 24px; }
}
</style>
