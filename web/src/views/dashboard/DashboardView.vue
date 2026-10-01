<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from 'vue'
import { storeToRefs } from 'pinia'
import {
  ArchiveIcon,
  CalendarClockIcon,
  ChevronRightIcon,
  CircleCheckIcon,
  CircleMinusIcon,
  CircleXIcon,
  DatabaseIcon,
  FileDownIcon,
  FilmIcon,
  FolderCheckIcon,
  ImageIcon,
  ListChecksIcon,
  PackageCheckIcon,
  PackagePlusIcon,
  PowerIcon,
  RefreshCwIcon,
  ServerIcon,
  TriangleAlertIcon,
} from '@lucide/vue'

import AppButton from '@/components/AppButton.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import StatusRow from '@/components/dashboard/StatusRow.vue'
import StatusSection from '@/components/dashboard/StatusSection.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { t } from '@/i18n'
import { getConnectionStatusLabel } from '@/lib/display'
import { formatDurationSeconds, formatTime } from '@/lib/format'
import { buildDashboardEventActions } from '@/lib/management-links'
import { useSocketStore } from '@/stores/sockets'
import { useSystemStore, type ServiceStopIntent } from '@/stores/system'
import {
  describeEventTone,
  describeOverallStatus,
  isSystemEvent,
  summarizeAttention,
  toRowTone,
  type StatusRowTone,
} from '@/views/dashboard/dashboard-status'
import DashboardConnectionsCard from '@/views/dashboard/DashboardConnectionsCard.vue'
import { useDashboardPage } from '@/views/dashboard/useDashboardPage'
import { useUpdateStatus } from '@/views/dashboard/useUpdateStatus'

const {
  backupPending,
  bootstrapRuntimeResources,
  checkItems,
  createBackup,
  diagnosticsIssueCards,
  diagnosticsPending,
  diagnosticsSubsystemItems,
  error,
  exportDiagnostics,
  liveUptimeSeconds,
  loading,
  readinessIssues,
  recentEvents,
  refreshState,
  runtimeBootstrapPending,
  system,
  visibleReasonCodes,
} = useDashboardPage()
const { diagnostics, readiness, stopIntent } = storeToRefs(useSystemStore())
const systemUnread = computed(() => Boolean(error.value && !system.value))
const socketStore = useSocketStore()
const { snapshots } = storeToRefs(socketStore)
const update = useUpdateStatus()

// Connections have their own box and plugins stay in the plugin center, so the checks leave them out.
const hiddenCheckKeys = new Set(['adapter', 'plugins'])
const checkIcons: Record<string, Component> = {
  database: DatabaseIcon,
  dependencies: PackageCheckIcon,
  filesystem: FolderCheckIcon,
  render: ImageIcon,
  runtime: FilmIcon,
  scheduler: CalendarClockIcon,
  system: ServerIcon,
  tasks: ListChecksIcon,
}

const overall = computed(() => describeOverallStatus(system.value?.status, readiness.value?.status))
const headerSummary = computed(() => [
  overall.value.label,
  liveUptimeSeconds.value === undefined ? '' : t('dashboard.uptimeText', { duration: formatDurationSeconds(liveUptimeSeconds.value) }),
  diagnostics.value?.generated_at ? t('dashboard.verifiedAt', { time: formatTime(diagnostics.value.generated_at) }) : '',
].filter(Boolean).join(' · '))

const readinessRows = computed(() => checkItems.value
  .filter(item => !hiddenCheckKeys.has(item.key))
  .map(item => ({ ...item, detail: readinessCheckDetail(item.key), icon: checkIcons[item.key] ?? ListChecksIcon })))
const diagnosticsRows = computed(() => diagnosticsSubsystemItems.value
  .filter(item => !hiddenCheckKeys.has(item.key))
  .map(item => ({ ...item, icon: checkIcons[item.key] ?? ListChecksIcon })))

// Readiness reports only a verdict per check; the diagnostics snapshot supplies the context line where it describes
// the same subsystem. Config, database and runtime have no such line, and borrowing another subsystem's count misleads.
function readinessCheckDetail(key: string) {
  return diagnosticsSubsystemItems.value.find(item => item.key === key)?.value
}

const attention = computed(() => summarizeAttention(
  readinessIssues.value,
  diagnostics.value?.issues ?? [],
  readinessRows.value.filter(item => item.status === 'success').length,
))

const checkView = ref<'readiness' | 'diagnostics'>('readiness')
const checkViewOptions = computed(() => [
  { value: 'readiness' as const, label: t('dashboard.overviewReadiness') },
  { value: 'diagnostics' as const, label: t('dashboard.overviewDiagnostics') },
])
watch(
  () => [readinessIssues.value.length, diagnosticsIssueCards.value.length] as const,
  ([readinessIssueCount, diagnosticsIssueCount]) => {
    if (readinessIssueCount === 0 && diagnosticsIssueCount > 0) checkView.value = 'diagnostics'
    else if (readinessIssueCount > 0) checkView.value = 'readiness'
  },
  { immediate: true },
)

const checksSection = ref<InstanceType<typeof StatusSection> | null>(null)
// "View checks" takes the operator to the first problem even when the checks box is already on screen:
// the row receives focus and briefly lights up.
async function showChecks() {
  if (attention.value) checkView.value = attention.value.view
  await nextTick()
  const element = checksSection.value?.$el as HTMLElement | undefined
  const target = element?.querySelector<HTMLElement>('.status-issue') ?? element
  if (!target) return
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  target.setAttribute('tabindex', '-1')
  target.focus({ preventScroll: true })
  target.scrollIntoView({ block: 'nearest', behavior: reducedMotion ? 'auto' : 'smooth' })
  target.classList.remove('is-highlighted')
  void target.offsetWidth
  target.classList.add('is-highlighted')
}

const versionText = computed(() => {
  const snapshot = update.status.value
  const reported = snapshot?.current_version || diagnostics.value?.build.core_version
  // The server reports `unknown` when the install has no readable build_info.json, as a development tree does.
  const version = !reported ? t('display.empty') : reported === 'unknown' ? t('dashboard.versionUnknown') : reported
  if (snapshot?.state === 'update_available' && snapshot.available_version) return t('dashboard.versionAvailable', { version, available: snapshot.available_version })
  return snapshot ? `${version} · ${t(`dashboard.update.states.${snapshot.state}`)}` : version
})
const updateGuidance = computed(() => {
  const snapshot = update.status.value
  if (snapshot?.state === 'update_available' && snapshot.available_version) return t('dashboard.update.guidedAvailable', { version: snapshot.available_version })
  if (snapshot?.state === 'disabled') return t('dashboard.update.checkUnavailable')
  return ''
})
// The management streams feed this page; a broken stream is shown with its retry countdown, unless the service said
// why it went away: then the row names that, and reconnecting by hand has nothing to reach.
const stopIntentTones: Record<ServiceStopIntent, StatusRowTone> = { stop: 'muted', restart: 'info', update: 'info' }
const managementStreams = computed(() => {
  const broken = (['events', 'logs'] as const)
    .map(channel => snapshots.value[channel])
    .filter(snapshot => snapshot.status !== 'authenticated')
  const first = broken[0]
  if (!first) return { label: t('dashboard.streamsHealthy'), reconnect: false, tone: 'success' as StatusRowTone }
  if (stopIntent.value) {
    return { label: t(`display.serviceStopIntents.${stopIntent.value}`), reconnect: false, tone: stopIntentTones[stopIntent.value] }
  }
  const seconds = first.status === 'reconnecting' && first.nextBackoffMs !== undefined
    ? Math.max(1, Math.round(first.nextBackoffMs / 1000))
    : null
  return {
    label: seconds === null ? getConnectionStatusLabel(first.status) : t('dashboard.streamsReconnecting', { seconds }),
    reconnect: true,
    tone: toRowTone(first.status),
  }
})

const systemEvents = computed(() => recentEvents.value.filter(event => isSystemEvent(event.payload)).slice(0, 8))
const eventIcons: Record<StatusRowTone, Component> = {
  danger: CircleXIcon,
  info: RefreshCwIcon,
  muted: CircleMinusIcon,
  success: CircleCheckIcon,
  warning: TriangleAlertIcon,
}
function eventAction(payload: Parameters<typeof buildDashboardEventActions>[0]) {
  return buildDashboardEventActions(payload)[0]
}
</script>

<template>
  <AppPage :title="t('dashboard.title')">
    <template #leading>
      <span class="status-lens" :data-tone="overall.tone" aria-hidden="true">
        <span class="status-lens__dot"><PowerIcon /></span>
      </span>
    </template>
    <template #description>{{ headerSummary }}</template>
    <template #extra>
      <div class="status-actions" role="group" :aria-label="t('dashboard.maintenanceActions')">
        <AppButton variant="ghost" :loading="loading" data-testid="dashboard-refresh" @click="refreshState">
          <template #icon><RefreshCwIcon /></template>
          {{ t('dashboard.refreshReadiness') }}
        </AppButton>
        <span class="status-actions__separator" aria-hidden="true" />
        <AppButton variant="ghost" :loading="diagnosticsPending" @click="exportDiagnostics">
          <template #icon><FileDownIcon /></template>
          {{ t('dashboard.exportDiagnostics') }}
        </AppButton>
      </div>
      <AppButton variant="default" :loading="backupPending" @click="createBackup">
        <template #icon><ArchiveIcon /></template>
        {{ t('dashboard.createBackup') }}
      </AppButton>
    </template>

    <RetryPanel
      v-if="systemUnread"
      :title="t('routes.status')"
      :description="error ?? ''"
      :loading="loading"
      @retry="refreshState()"
    />

    <section v-if="attention && !systemUnread" class="status-attention app-box" :data-tone="attention.tone" data-testid="dashboard-attention" aria-labelledby="dashboard-attention-title">
      <span class="status-attention__icon" aria-hidden="true"><TriangleAlertIcon /></span>
      <div class="status-attention__copy">
        <h2 id="dashboard-attention-title">{{ attention.title }}</h2>
        <p v-if="attention.detail">{{ attention.detail }}</p>
      </div>
      <div class="status-attention__actions">
        <AppButton data-testid="dashboard-view-checks" @click="showChecks">{{ t('dashboard.viewChecks') }}</AppButton>
        <AppButton
          v-if="attention.runtimeResources.length"
          variant="default"
          :loading="runtimeBootstrapPending"
          data-testid="readiness-prepare-runtime"
          @click="bootstrapRuntimeResources(attention.runtimeResources)"
        >
          <template #icon><PackagePlusIcon /></template>
          {{ t('dashboard.runtimeBootstrap') }}
        </AppButton>
      </div>
    </section>

    <!-- Without a system snapshot there is nothing to show below the retry panel. Connections and their messages take
         the full-width card; the other three boxes share the row below it. -->
    <DashboardConnectionsCard v-if="!systemUnread" />
    <div v-if="!systemUnread" class="status-grid">
      <StatusSection :title="t('dashboard.runtimeInfo')" data-testid="dashboard-runtime-info">
        <dl class="status-facts">
          <div class="status-facts__row">
            <dt>{{ t('dashboard.facts.version') }}</dt>
            <dd>
              <span>{{ versionText }}</span>
              <AppButton size="sm" variant="ghost" :loading="update.checking.value" data-testid="dashboard-update-check" @click="update.check">{{ t('dashboard.update.check') }}</AppButton>
            </dd>
          </div>
          <!-- Engine, schema and apply state are the same on every install; where the files live is what differs. -->
          <div class="status-facts__row">
            <dt>{{ t('dashboard.facts.configFile') }}</dt>
            <dd class="status-facts__path">{{ diagnostics?.config.config_path || t('display.empty') }}</dd>
          </div>
          <div class="status-facts__row">
            <dt>{{ t('dashboard.facts.databaseFile') }}</dt>
            <dd class="status-facts__path">{{ diagnostics?.config.database_path || t('display.empty') }}</dd>
          </div>
          <div class="status-facts__row">
            <dt>{{ t('dashboard.facts.streams') }}</dt>
            <dd>
              <span class="status-facts__state" :data-tone="managementStreams.tone"><component :is="eventIcons[managementStreams.tone]" aria-hidden="true" />{{ managementStreams.label }}</span>
              <AppButton v-if="managementStreams.reconnect" size="sm" variant="ghost" @click="socketStore.reconnectAll()">{{ t('dashboard.reconnect') }}</AppButton>
            </dd>
          </div>
        </dl>
        <p v-if="updateGuidance || update.error.value" class="status-facts__note" aria-live="polite">{{ update.error.value || updateGuidance }}</p>
      </StatusSection>

      <StatusSection ref="checksSection" :title="t('dashboard.checks')" data-testid="dashboard-checks">
        <template #actions>
          <AppSegmented v-model="checkView" :options="checkViewOptions" :label="t('dashboard.checkViewLabel')" class="status-checks__views" />
        </template>

        <template v-if="checkView === 'readiness'">
          <StatusRow
            v-for="issue in readinessIssues"
            :key="`${issue.code}-${issue.summary}`"
            :title="issue.summary"
            :detail="issue.remediation"
            :status="t(`dashboard.issueSeverity.${issue.severity === 'error' ? 'error' : issue.severity === 'warning' ? 'warning' : 'info'}`)"
            :tone="issue.severity === 'error' ? 'danger' : issue.severity === 'warning' ? 'warning' : 'info'"
            class="status-issue"
          >
            <template #icon><TriangleAlertIcon /></template>
          </StatusRow>
          <StatusRow
            v-for="item in readinessRows"
            :key="item.key"
            :title="item.label"
            :detail="item.detail"
            :status="item.displayValue"
            :tone="item.status"
          >
            <template #icon><component :is="item.icon" /></template>
          </StatusRow>
          <AppEmptyState v-if="!readinessRows.length && !readinessIssues.length" :description="t('dashboard.readinessEmpty')" />
          <details v-if="visibleReasonCodes.length || readinessIssues.length" class="status-technical">
            <summary>{{ t('dashboard.readinessTechnicalDetails') }}</summary>
            <p v-if="visibleReasonCodes.length"><code>{{ visibleReasonCodes.join(', ') }}</code></p>
            <ul v-if="readinessIssues.length"><li v-for="issue in readinessIssues" :key="issue.code"><code>{{ issue.code }}</code> · {{ issue.summary }}</li></ul>
          </details>
        </template>

        <template v-else>
          <StatusRow
            v-for="issue in diagnosticsIssueCards"
            :key="issue.key"
            :title="issue.problem"
            :detail="issue.remediation"
            :status="t(`dashboard.issueSeverity.${issue.severity === 'error' ? 'error' : 'warning'}`)"
            :tone="issue.status === 'danger' ? 'danger' : 'warning'"
            class="status-issue"
          >
            <template #icon><TriangleAlertIcon /></template>
          </StatusRow>
          <StatusRow
            v-for="item in diagnosticsRows"
            :key="item.key"
            :title="item.label"
            :detail="item.detail"
            :status="item.value"
            :tone="item.status"
          >
            <template #icon><component :is="item.icon" /></template>
          </StatusRow>
          <AppEmptyState v-if="!diagnosticsRows.length" :description="t('dashboard.diagnosticsEmpty')" />
          <!-- Issue codes belong to the technical details, as on the readiness view. -->
          <details v-if="diagnosticsIssueCards.length" class="status-technical">
            <summary>{{ t('dashboard.readinessTechnicalDetails') }}</summary>
            <ul><li v-for="issue in diagnosticsIssueCards" :key="issue.key"><code>{{ issue.code }}</code> · {{ issue.problem }}</li></ul>
          </details>
        </template>
      </StatusSection>

      <StatusSection :title="t('dashboard.overviewEvents')" data-testid="dashboard-events">
        <template #actions>
          <MotionRouterLink :to="{ name: 'logs-history' }" class="status-link">{{ t('routes.logsHistory') }}<ChevronRightIcon aria-hidden="true" /></MotionRouterLink>
        </template>
        <ol v-if="systemEvents.length" class="status-events">
          <li v-for="event in systemEvents" :key="`${event.timestamp}-${event.summary}`" class="status-event">
            <time :datetime="event.timestamp">{{ formatTime(event.timestamp) }}</time>
            <span class="status-event__tone" :data-tone="describeEventTone(event.payload)" aria-hidden="true">
              <component :is="eventIcons[describeEventTone(event.payload)]" />
            </span>
            <MotionRouterLink v-if="eventAction(event.payload)" :to="eventAction(event.payload)!.to" class="status-event__summary status-event__summary--link">{{ event.summary }}</MotionRouterLink>
            <span v-else class="status-event__summary">{{ event.summary }}</span>
          </li>
        </ol>
        <AppEmptyState v-else :description="t('dashboard.recentEventsEmpty')" />
      </StatusSection>
    </div>
  </AppPage>
</template>

<style scoped lang="scss">

// The header lens: a raised circle holding the overall status dot.
.status-lens { display: grid; place-items: center; width: 50px; height: 50px; border-radius: 50%; background: var(--control-fill); box-shadow: var(--shadow-xs); }
.status-lens__dot { display: grid; place-items: center; width: 26px; height: 26px; border-radius: 50%; background: var(--muted); color: var(--on-brand); }
.status-lens__dot svg { width: 13px; height: 13px; stroke-width: 2.8; }
.status-lens[data-tone=success] .status-lens__dot { background: var(--success); }
.status-lens[data-tone=warning] .status-lens__dot { background: var(--warning); }
.status-lens[data-tone=danger] .status-lens__dot { background: var(--danger); }

// Maintenance actions share one pill with a hairline between them; the backup is the page's primary action.
// The page's first row keeps one control height: 40px, like the page search and the primary action.
.status-actions { display: inline-flex; align-items: center; height: 40px; padding: 4px; border: 1px solid transparent; border-radius: 999px; background: var(--control-fill); box-shadow: var(--shadow-xs); }
.status-actions :deep(.app-button) { height: 32px; }
.status-actions__separator { width: 1px; height: 18px; background: var(--border); }

.status-attention { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 16px; padding: 18px 20px 18px 18px; }
.status-attention__icon { display: grid; place-items: center; width: 44px; height: 44px; border-radius: 14px; background: var(--warning-soft); color: var(--warning); }
.status-attention[data-tone=danger] .status-attention__icon { background: var(--danger-soft); color: var(--danger); }
.status-attention__icon svg { width: 22px; height: 22px; }
.status-attention__copy { display: grid; gap: 2px; min-width: 0; }
.status-attention h2 { margin: 0; font-size: var(--font-size-lg); font-weight: 700; letter-spacing: -0.01em; }
.status-attention p { margin: 0; color: var(--muted); font-size: var(--font-size-sm); }
.status-attention__actions { display: flex; gap: 8px; }

.status-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; align-items: stretch; }

.status-link { display: inline-flex; align-items: center; gap: 2px; border-radius: var(--radius-sm); color: var(--brand-foreground); font-size: var(--font-size-sm); font-weight: 500; }
.status-link:hover { text-decoration: underline; text-underline-offset: 3px; }
.status-link:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.status-link svg { width: 15px; height: 15px; }


.status-facts { display: grid; margin: 0; }
.status-facts__row { display: flex; align-items: center; gap: 16px; min-height: 46px; padding: 6px 0; }
.status-facts__row + .status-facts__row { border-top: 1px solid var(--border); }
.status-facts dt { flex: none; color: var(--muted); font-size: var(--font-size-sm); }
.status-facts dd { display: flex; align-items: center; gap: 8px; min-width: 0; margin: 0 0 0 auto; font-weight: 500; font-variant-numeric: tabular-nums; text-align: right; }
// Text buttons give back their inner padding so their words end on the same edge as the values above.
// Paths are data: the mono face, wrapping anywhere instead of pushing the label out of the row.
.status-facts__path { font-family: var(--font-mono); font-size: var(--font-size-sm); font-weight: 400; overflow-wrap: anywhere; }
.status-facts dd :deep(.app-button) { height: 30px; margin-inline-end: -10px; padding-inline: 10px; color: var(--brand-foreground); }
.status-facts__state { display: inline-flex; align-items: center; gap: 6px; }
.status-facts__state svg { width: 15px; height: 15px; }
.status-facts__state[data-tone=success] { color: var(--success); }
.status-facts__state[data-tone=warning] { color: var(--warning); }
.status-facts__state[data-tone=danger] { color: var(--danger); }
.status-facts__state[data-tone=info] { color: var(--info); }
.status-facts__note { margin: 4px 0 8px; color: var(--muted); font-size: var(--font-size-xs); line-height: 1.6; }

.status-checks__views :deep(.app-segmented__item) { min-height: 28px; padding: 3px 14px; }
.status-technical { margin-top: 8px; padding: 8px 0 6px; border-top: 1px solid var(--border); color: var(--muted); font-size: var(--font-size-xs); overflow-wrap: anywhere; }
.status-technical summary { cursor: pointer; }
.status-technical summary:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; border-radius: var(--radius-xs); }
.status-technical p { margin: 8px 0 0; }
.status-technical ul { display: grid; gap: 6px; margin: 8px 0 0; padding-left: 20px; }

.status-events { margin: 0; padding: 0; list-style: none; }
.status-event { display: grid; grid-template-columns: 72px 20px minmax(0, 1fr); align-items: center; gap: 8px; min-height: 46px; padding: 7px 0; }
.status-event + .status-event { border-top: 1px solid var(--border); }
.status-event time { color: var(--muted); font-size: var(--font-size-sm); font-variant-numeric: tabular-nums; }
.status-event__tone { display: grid; place-items: center; color: var(--muted); }
.status-event__tone svg { width: 16px; height: 16px; }
.status-event__tone[data-tone=success] { color: var(--success); }
.status-event__tone[data-tone=warning] { color: var(--warning); }
.status-event__tone[data-tone=danger] { color: var(--danger); }
.status-event__tone[data-tone=info] { color: var(--info); }
.status-event__summary { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.status-event__summary--link { border-radius: var(--radius-xs); color: var(--text); }
.status-event__summary--link:hover { color: var(--brand-foreground); text-decoration: underline; text-underline-offset: 3px; }
.status-event__summary--link:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }

@media (forced-colors: active) {
  .status-lens, .status-actions, .status-attention__icon { border: 1px solid CanvasText; }
}
// Issue rows share the link rows' inset so the highlight from "view checks" has room around the text.
.status-issue { margin-inline: -8px; padding-inline: 8px; border-radius: var(--radius-md); }
.status-issue:focus { outline: none; }
.status-issue:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.status-issue.is-highlighted { animation: status-issue-highlight 1.6s var(--motion-easing); }
@keyframes status-issue-highlight { 0%, 45% { background: var(--warning-soft); } 100% { background: transparent; } }
@media (prefers-reduced-motion: reduce) { .status-issue.is-highlighted { animation: none; } }
</style>
