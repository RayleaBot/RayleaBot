<script setup lang="ts">
import AppTabs from '@/components/AppTabs.vue'
import AppTag from '@/components/AppTag.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import {
  CircleCheckIcon,
  CircleXIcon,
  CircleAlertIcon,
  CircleMinusIcon,
  DatabaseIcon,
} from '@lucide/vue'

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
const overviewTabs = computed(() => [
  { value: 'events', label: t('dashboard.overviewEvents') },
  { value: 'readiness', label: t('dashboard.overviewReadiness') },
  { value: 'diagnostics', label: t('dashboard.overviewDiagnostics') },
])
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
  adapters,
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
  recoveryBootstrapResources,
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

const readinessCheckGroups = computed(() => [
  { key: 'issues', collapsed: false, items: checkItems.value.filter(item => item.status !== 'success') },
  { key: 'passed', collapsed: true, items: checkItems.value.filter(item => item.status === 'success') },
])

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
    danger: CircleXIcon,
    muted: CircleMinusIcon,
    success: CircleCheckIcon,
    warning: CircleAlertIcon,
  } as const
  return map[status]
}

function getStatusTagColor(status: typeof healthStatusType.value) {
  if (status === 'success') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'danger') return 'danger'
  return 'neutral'
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
  if (severity === 'error' || severity === 'danger') return CircleXIcon
  if (severity === 'warning') return CircleAlertIcon
  if (severity === 'success') return CircleCheckIcon
  return undefined
}

const protocolIssue = computed(() => {
  const issues = adapters.value.flatMap(adapter => {
    const snapshot = adapter.onebot11
    if (!adapter.enabled || !snapshot || !['degraded', 'failed'].includes(snapshot.readiness_status)) return []
    return snapshot.recent_transport_issues.map(issue => ({ ...issue, summary: `${adapter.display_name}：${issue.summary}` }))
  })
  if (issues.length === 0) return null
  return { code: issues.map(issue => issue.code).join(','), severity: issues.some(issue => issue.severity === 'error') ? 'error' : 'warning', summary: issues.map(issue => issue.summary).join('；') }
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
        <AppTabs v-model="activeOverviewTab" :items="overviewTabs" label="状态概览" keep-alive>
          <template #events>
            <AppEmptyState v-if="recentEvents.length === 0" :description="t('dashboard.recentEventsEmpty')" />

            <div
              v-else
              class="events-timeline-wrapper"
              :class="{ 'events-timeline-wrapper--collapsed': !eventsExpanded && recentEvents.length > 4 }"
            >
              <ol class="events-timeline">
                <li
                  v-for="event in recentEvents"
                  :key="`${event.timestamp}-${event.summary}`"
                  :style="{ '--event-color': getEventSeverityColor(getEventSeverity(event.payload)) }" class="events-timeline__row"
                >
                  <span class="events-timeline__marker">
                    <component
                      :is="getEventSeverityIcon(getEventSeverity(event.payload))"
                      v-if="getEventSeverityIcon(getEventSeverity(event.payload))"
                      class="events-timeline__dot-icon"
                      role="img"
                      :aria-label="`事件级别：${getEventSeverity(event.payload) ?? 'info'}`"
                    />
                    <span v-else class="events-timeline__dot" role="img" aria-label="事件级别：info" />
                  </span>
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
                </li>
              </ol>
            </div>
            <div v-if="recentEvents.length > 4" class="events-toggle">
              <AppButton size="sm" variant="link" @click="eventsExpanded = !eventsExpanded">
                {{ eventsExpanded ? t('dashboard.collapseEvents') : t('dashboard.expandEvents', { count: recentEvents.length - 4 }) }}
              </AppButton>
            </div>
          </template>

          <template #readiness>
            <div class="readiness-actions"><AppButton size="sm" :loading="loading" @click="refreshState">{{ t('dashboard.refreshReadiness') }}</AppButton></div>
            <template v-for="group in readinessCheckGroups" :key="group.key">
              <component :is="group.collapsed ? 'details' : 'div'" v-if="group.items.length" class="readiness-check-group">
                <summary v-if="group.collapsed">{{ t('dashboard.passedCheckCount', { count: group.items.length }) }}</summary>
                <div class="readiness-checks">
                  <div v-for="item in group.items" :key="item.key" :class="['readiness-check', `readiness-check--${item.status}`]">
                    <div class="readiness-check__header">
                      <component :is="getCheckIcon(item.status)" class="readiness-check__icon" aria-hidden="true" />
                      <span class="readiness-check__name">{{ item.label }}</span>
                    </div>
                    <div class="readiness-check__value">{{ item.displayValue }}</div>
                  </div>
                </div>
              </component>
            </template>
            <AppEmptyState v-if="!checkItems.length && !readinessIssues.length" :description="t('display.empty')" />

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
                  <AppTag :tone="issue.severity === 'error' ? 'danger' : issue.severity === 'warning' ? 'warning' : 'success'">
                    {{ t(`dashboard.issueSeverity.${issue.severity === 'error' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}`) }}
                  </AppTag>
                  <span class="issue-alert-card__summary">{{ issue.summary }}</span>
                </div>
                <div v-if="issue.remediation" class="issue-alert-card__remediation">
                  {{ issue.remediation }}
                </div>
                <div v-if="issue.runtime_resources?.length" class="issue-alert-card__actions">
                  <AppButton size="sm" variant="default" :loading="runtimeBootstrapPending" data-testid="readiness-prepare-runtime" @click="bootstrapRuntimeResources(issue.runtime_resources)">{{ t('dashboard.runtimeBootstrap') }}</AppButton>
                </div>
              </div>
            </div>

            <div v-if="readinessIssues.length > 3" class="issues-toggle">
              <AppButton
                size="sm"
                variant="link"
                :aria-label="issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 })"
                @click="issuesExpanded = !issuesExpanded"
              >
                {{ issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 }) }}
              </AppButton>
            </div>
            <details v-if="visibleReasonCodes.length || readinessIssues.length" class="readiness-technical">
              <summary>{{ t('dashboard.readinessTechnicalDetails') }}</summary>
              <p v-if="visibleReasonCodes.length"><code>{{ visibleReasonCodes.join(', ') }}</code></p>
              <ul v-if="readinessIssues.length"><li v-for="issue in readinessIssues" :key="issue.code"><code>{{ issue.code }}</code> · {{ issue.summary }}</li></ul>
            </details>
          </template>

          <template #diagnostics>
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
                <AppTag :tone="getStatusTagColor(item.status)" class="diagnostics-subsystem__tag">
                  {{ item.value }}
                </AppTag>
                <div class="diagnostics-subsystem__detail">{{ item.detail }}</div>
              </div>
            </div>
            <AppEmptyState v-else :description="t('dashboard.diagnosticsEmpty')" />

            <div v-if="diagnosticsIssueCards.length" class="diagnostics-issues">
              <div
                v-for="issue in diagnosticsIssueCards"
                :key="issue.key"
                :class="['diagnostics-issue-card', `diagnostics-issue-card--${issue.status}`]"
              >
                <div class="diagnostics-issue-card__header">
                  <AppTag :tone="getStatusTagColor(issue.status)">
                    {{ issue.code }}
                  </AppTag>
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
            <AppEmptyState v-else class="diagnostics-empty-issues" :description="t('dashboard.diagnosticsNoIssues')" />
          </template>
        </AppTabs>
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
        :can-bootstrap="recoveryBootstrapResources.length > 0"
        @recheck="recheckRecoverySummary"
        @bootstrap="bootstrapRuntimeResources()"
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
          <DatabaseIcon class="dashboard-panel-icon" aria-hidden="true" />
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
.dashboard-activity-card :deep(.app-card__body) { padding: 6px 20px 16px; }
.events-timeline { padding: 6px 0 0; margin: 0; list-style: none; }
.events-timeline__row { display: grid; grid-template-columns: 18px minmax(0, 1fr); gap: 10px; }
.events-timeline__marker { display: grid; place-items: start center; padding-top: 14px; color: var(--event-color); }
.events-timeline__dot-icon { width: 18px; height: 18px; }
.events-timeline__dot-icon { font-size: 18px; line-height: 1; }
.events-timeline__dot { display: block; width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
.events-timeline__item { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 12px; min-width: 0; padding: 10px 0; border-bottom: 1px solid var(--border); }
.events-timeline__summary { font-size: 14px; font-weight: 500; line-height: 1.5; color: var(--text); overflow-wrap: anywhere; }
.events-timeline__time { color: var(--muted); font-size: 12px; grid-column: 1; }
.events-timeline__actions { grid-column: 2; grid-row: 1 / span 2; align-self: center; }
.events-timeline-wrapper--collapsed { max-height: 284px; overflow: hidden; }
.events-toggle, .issues-toggle { margin-top: 8px; text-align: center; }
.readiness-checks { grid-template-columns: 1fr; gap: 0; border: 0; background: transparent; border-radius: 0; }
.readiness-actions { display: flex; justify-content: flex-end; margin-bottom: 12px; }
.readiness-check-group summary, .readiness-technical summary { padding: 12px 0; color: var(--muted); font-size: 13px; cursor: pointer; }
.readiness-technical { margin-top: 16px; font-size: 12px; color: var(--muted); overflow-wrap: anywhere; }
.readiness-technical p { margin: 0 0 8px; }
.readiness-technical ul { display: grid; gap: 8px; margin: 0; padding-left: 20px; }
.issue-alert-card__actions { margin-top: 12px; }
.readiness-check { grid-template-columns: minmax(120px, .8fr) minmax(0, 1fr); padding: 10px 0; gap: 12px; border-bottom: 1px solid var(--border); background: transparent; }
.readiness-check__name { color: var(--text); font-weight: 500; }
.readiness-check__value { text-align: right; color: var(--muted); }
.readiness-check__icon, .diagnostics-subsystem__icon { width: 17px; height: 17px; }
.diagnostics-subsystem-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 20px; }
.diagnostics-subsystem { display: grid; grid-template-columns: minmax(0, 1fr) auto; min-width: 0; gap: 6px 10px; padding: 12px 0; border-bottom: 1px solid var(--border); }
.diagnostics-subsystem__header { display: flex; align-items: center; gap: 8px; min-width: 0; font-weight: 500; }
.diagnostics-subsystem__label { overflow-wrap: anywhere; }
.diagnostics-subsystem__detail { grid-column: 1 / -1; color: var(--muted); font-size: 13px; overflow-wrap: anywhere; }
.diagnostics-subsystem--success .diagnostics-subsystem__icon { color: var(--success); }
.diagnostics-subsystem--warning .diagnostics-subsystem__icon { color: var(--warning); }
.diagnostics-subsystem--danger .diagnostics-subsystem__icon { color: var(--danger); }
.diagnostics-issues, .issues-list { display: grid; gap: 12px; margin-top: 16px; }
.diagnostics-issue-card, .issue-alert-card { min-width: 0; padding: 12px 0; border-radius: 0; border: 0; border-bottom: 1px solid var(--border); background: transparent; color: var(--text); }
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
 .dashboard-activity-card :deep(.app-card__body) { padding-inline: 14px; }
 .events-timeline__item { grid-template-columns: minmax(0, 1fr); }
 .events-timeline__actions { grid-column: 1; grid-row: auto; }
 .events-timeline-wrapper--collapsed { max-height: 390px; }
 .dashboard-panel-icon { width: 44px; height: 44px; font-size: 24px; }
}
</style>
