<script setup lang="ts">
import AppTabs from '@/components/AppTabs.vue'
import AppTag from '@/components/AppTag.vue'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onMounted, ref, watch, type Component } from 'vue'
import { storeToRefs } from 'pinia'

import {
  ArchiveIcon,
  BlocksIcon,
  BotIcon,
  CalendarClockIcon,
  CircleCheckIcon,
  CircleXIcon,
  CircleAlertIcon,
  CircleMinusIcon,
  FolderCheckIcon,
  ImageIcon,
  ListChecksIcon,
  PackageCheckIcon,
  PlusIcon,
  RadioTowerIcon,
  ServerIcon,
  SlidersHorizontalIcon,
  StethoscopeIcon,
} from '@lucide/vue'

import AppCard from '@/components/AppCard.vue'
import ConnectionStatusStrip from '@/components/dashboard/ConnectionStatusStrip.vue'
import DashboardStatusGrid from '@/components/dashboard/DashboardStatusGrid.vue'
import DashboardUpdateCard from '@/components/dashboard/DashboardUpdateCard.vue'
import HubSection from '@/components/dashboard/HubSection.vue'
import HubTile, { type HubTileGlyph, type HubTileTone } from '@/components/dashboard/HubTile.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { getAdapterStateLabel, getPluginStateLabel, type StatusType } from '@/lib/display'
import { buildDashboardEventActions, buildPluginDetailLocation } from '@/lib/management-links'
import { resolveStatusTone } from '@/lib/status-tone'
import { usePluginCollection } from '@/lib/use-plugin-collection'
import { useAdaptersStore } from '@/stores/adapters'
import { t } from '@/i18n'
import { useDashboardPage } from '@/views/dashboard/useDashboardPage'

const activeOverviewTab = ref('events')
const overviewTabs = computed(() => [
  { value: 'events', label: t('dashboard.overviewEvents') },
  { value: 'readiness', label: t('dashboard.overviewReadiness') },
  { value: 'diagnostics', label: t('dashboard.overviewDiagnostics') },
])

const {
  backupPending,
  bootstrapRuntimeResources,
  checkItems,
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
  liveUptimeSeconds,
  loading,
  readinessDetailText,
  readinessIssues,
  readinessStatusType,
  readinessValueText,
  recentEvents,
  refreshState,
  runtimeBootstrapPending,
  system,
  visibleReasonCodes,
} = useDashboardPage()

// Hub tiles: running, connected or ready objects are lit; stopped ones are glass; problems get an outline.
function tileTone(status?: string): HubTileTone {
  const tone = resolveStatusTone(status)
  return tone === 'neutral' || tone === 'attention' ? 'muted' : tone
}

const { adapters } = storeToRefs(useAdaptersStore())
const connectionTiles = computed(() => adapters.value.map((adapter) => {
  const tone = tileTone(adapter.state)
  return {
    id: adapter.id,
    title: adapter.identity?.name || adapter.display_name || adapter.id,
    detail: [adapter.protocol === 'qqofficial' ? t('protocols.qqTitle') : 'OneBot11', adapter.identity?.id].filter(Boolean).join(' · '),
    status: adapter.enabled ? getAdapterStateLabel(adapter.state) : t('dashboard.hub.connectionDisabled'),
    tone: adapter.enabled ? tone : 'muted' as HubTileTone,
    lit: adapter.enabled && adapter.state === 'connected',
    attention: adapter.enabled && tone === 'danger',
    glyph: (adapter.protocol === 'qqofficial' ? 'violet' : 'blue') as HubTileGlyph,
    icon: adapter.protocol === 'qqofficial' ? BotIcon : RadioTowerIcon,
  }
}))
const connectedCount = computed(() => connectionTiles.value.filter(tile => tile.lit).length)

// The hub reads its own first page of plugins; the plugin center keeps its own query and cursor.
const pluginCollection = usePluginCollection()
const pluginsLoading = pluginCollection.loading
onMounted(() => { void pluginCollection.load({}).catch(() => undefined) })
const pluginGlyphs: HubTileGlyph[] = ['ember', 'teal', 'violet', 'rose', 'blue', 'green', 'amber', 'slate']
function pluginGlyph(pluginId: string): HubTileGlyph {
  let hash = 0
  for (const char of pluginId) hash = (hash * 31 + char.charCodeAt(0)) >>> 0
  return pluginGlyphs[hash % pluginGlyphs.length] ?? 'slate'
}
const pluginTiles = computed(() => pluginCollection.items.value.map((plugin) => {
  const tone = tileTone(plugin.state)
  const name = plugin.name?.trim() || plugin.id
  return {
    id: plugin.id,
    name,
    icon: plugin.icon,
    version: plugin.version,
    initial: Array.from(name)[0] ?? '?',
    status: getPluginStateLabel(plugin.state),
    tone: tone === 'success' ? 'muted' as HubTileTone : tone,
    lit: plugin.state === 'running',
    attention: tone === 'danger' || tone === 'warning',
    glyph: pluginGlyph(plugin.id),
  }
}))
const runningPluginCount = computed(() => pluginTiles.value.filter(tile => tile.lit).length)

const runtimeIcons: Record<string, Component> = {
  system: ServerIcon,
  config: SlidersHorizontalIcon,
  render: ImageIcon,
  scheduler: CalendarClockIcon,
  tasks: ListChecksIcon,
  dependencies: PackageCheckIcon,
  filesystem: FolderCheckIcon,
}
const runtimeGlyphs: Record<string, HubTileGlyph> = {
  system: 'slate',
  config: 'slate',
  render: 'amber',
  scheduler: 'violet',
  tasks: 'teal',
  dependencies: 'blue',
  filesystem: 'green',
}
// Connections and plugins have their own sections, so the runtime section shows the other subsystems.
const runtimeTiles = computed(() => diagnosticsSubsystemItems.value
  .filter(item => item.key in runtimeIcons)
  .map(item => ({
    ...item,
    icon: runtimeIcons[item.key],
    glyph: runtimeGlyphs[item.key] ?? 'slate',
    lit: item.status === 'success',
    attention: item.status === 'warning' || item.status === 'danger',
    tone: (item.status === 'warning' || item.status === 'danger' ? item.status : 'muted') as HubTileTone,
  })))
const readyRuntimeCount = computed(() => runtimeTiles.value.filter(tile => tile.lit).length)

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

function getCheckIcon(status: StatusType) {
  const map = {
    danger: CircleXIcon,
    muted: CircleMinusIcon,
    success: CircleCheckIcon,
    warning: CircleAlertIcon,
  } as const
  return map[status]
}

function getStatusTagColor(status: StatusType) {
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

    <HubSection
      :title="t('dashboard.hub.connections')"
      :meta="connectionTiles.length ? t('dashboard.hub.connectionsMeta', { connected: connectedCount, total: connectionTiles.length }) : undefined"
      :to="{ name: 'protocols' }"
      :link-label="t('dashboard.hub.openProtocols')"
    >
      <HubTile
        v-for="tile in connectionTiles"
        :key="tile.id"
        wide
        :lit="tile.lit"
        :attention="tile.attention"
        :tone="tile.tone"
        :glyph="tile.glyph"
        :to="{ name: 'protocols' }"
        :label="`${tile.title} · ${tile.status}`"
      >
        <template #glyph><component :is="tile.icon" /></template>
        <template #title>{{ tile.title }}</template>
        <template #detail>{{ tile.detail }}</template>
        <template #status>{{ tile.status }}</template>
      </HubTile>
      <HubTile v-if="!connectionTiles.length" wide :to="{ name: 'protocols' }" :label="t('dashboard.hub.addConnection')">
        <template #glyph><PlusIcon /></template>
        <template #title>{{ t('dashboard.hub.addConnection') }}</template>
        <template #detail>{{ t('dashboard.hub.addConnectionDetail') }}</template>
        <template #status>{{ t('dashboard.hub.openProtocols') }}</template>
      </HubTile>
    </HubSection>

    <HubSection
      :title="t('dashboard.hub.plugins')"
      :meta="pluginTiles.length ? t('dashboard.hub.pluginsMeta', { running: runningPluginCount, total: pluginTiles.length }) : undefined"
      :to="{ name: 'plugins' }"
      :link-label="t('dashboard.hub.openPlugins')"
    >
      <HubTile
        v-for="tile in pluginTiles"
        :key="tile.id"
        :lit="tile.lit"
        :attention="tile.attention"
        :tone="tile.tone"
        :glyph="tile.icon ? 'image' : tile.glyph"
        :to="buildPluginDetailLocation(tile.id)"
        :label="`${tile.name} · ${tile.status}`"
      >
        <template #glyph>
          <PluginIcon v-if="tile.icon" :plugin-id="tile.id" :icon="tile.icon" :version="tile.version" />
          <template v-else>{{ tile.initial }}</template>
        </template>
        <template #title>{{ tile.name }}</template>
        <template #status>{{ tile.status }}</template>
      </HubTile>
      <HubTile v-if="!pluginTiles.length && !pluginsLoading" :to="{ name: 'plugins' }" :label="t('dashboard.hub.installPlugin')">
        <template #glyph><BlocksIcon /></template>
        <template #title>{{ t('dashboard.hub.installPlugin') }}</template>
        <template #status>{{ t('dashboard.hub.openPlugins') }}</template>
      </HubTile>
    </HubSection>

    <HubSection
      :title="t('dashboard.hub.runtime')"
      :meta="runtimeTiles.length ? t('dashboard.hub.runtimeMeta', { ready: readyRuntimeCount, total: runtimeTiles.length }) : undefined"
    >
      <HubTile
        v-for="tile in runtimeTiles"
        :key="tile.key"
        :lit="tile.lit"
        :attention="tile.attention"
        :tone="tile.tone"
        :glyph="tile.glyph"
      >
        <template #glyph><component :is="tile.icon" /></template>
        <template #title>{{ tile.label }}</template>
        <template #status>{{ tile.value }}</template>
      </HubTile>
      <HubTile action :disabled="backupPending" @click="createBackup">
        <template #glyph><ArchiveIcon /></template>
        <template #title>{{ t('dashboard.createBackup') }}</template>
        <template #status>{{ backupPending ? t('dashboard.hub.inProgress') : t('dashboard.hub.runNow') }}</template>
      </HubTile>
      <HubTile action :disabled="diagnosticsPending" @click="exportDiagnostics">
        <template #glyph><StethoscopeIcon /></template>
        <template #title>{{ t('dashboard.exportDiagnostics') }}</template>
        <template #status>{{ diagnosticsPending ? t('dashboard.hub.inProgress') : t('dashboard.hub.runNow') }}</template>
      </HubTile>
    </HubSection>

    <div class="dashboard-detail-grid">
      <AppCard
        borderless
        class="dashboard-activity-card"
      >
        <AppTabs v-model="activeOverviewTab" :items="overviewTabs" :label="t('dashboard.overviewLabel')" keep-alive>
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
                      :aria-label="t('dashboard.eventSeverityLabel', { severity: getEventSeverity(event.payload) ?? 'info' })"
                    />
                    <span v-else class="events-timeline__dot" role="img" :aria-label="t('dashboard.eventSeverityLabel', { severity: 'info' })" />
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
            <AppEmptyState v-if="!diagnosticsSubsystemItems.length" :description="t('dashboard.diagnosticsEmpty')" />

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

      <DashboardUpdateCard />
    </div>
  </AppPage>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.dashboard-detail-grid { display: grid; grid-template-columns: minmax(0, 1.8fr) minmax(300px, .85fr); gap: 16px; align-items: start; }
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
.readiness-check__icon { width: 17px; height: 17px; }
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
@media (max-width: #{bp.$splitPanel}) {
 .dashboard-detail-grid { grid-template-columns: 1fr; }
}
@media (max-width: #{bp.$phone}) {
 .dashboard-activity-card :deep(.app-card__body) { padding-inline: 14px; }
 .events-timeline__item { grid-template-columns: minmax(0, 1fr); }
 .events-timeline__actions { grid-column: 1; grid-row: auto; }
 .events-timeline-wrapper--collapsed { max-height: 390px; }
}
</style>
