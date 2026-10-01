import { i18n, t } from '@/i18n'
import { describeAdapterStates } from '@/lib/adapter-status'
import type { StatusType } from '@/lib/display'
import { resolveStatusTone } from '@/lib/status-tone'
import type { AdapterDescriptor, EventsPayload, ReadinessStatusResponse, RuntimeBootstrapResource, SystemDiagnosticsResponse } from '@/types/api'

type DiagnosticIssue = SystemDiagnosticsResponse['issues'][number]
type ToastLevel = 'warning' | 'error'

// Row and lens tones on the status page: the status colours plus a quiet muted tone.
export type StatusRowTone = 'success' | 'warning' | 'danger' | 'info' | 'muted'

export function toRowTone(status?: string | null): StatusRowTone {
  const tone = resolveStatusTone(status)
  if (tone === 'neutral') return 'muted'
  if (tone === 'attention') return 'warning'
  return tone
}

export function describeOverallStatus(systemStatus?: string, readinessStatus?: string): { label: string; tone: StatusRowTone } {
  if (systemStatus === 'shutting_down') return { label: t('dashboard.overall.stopping'), tone: 'warning' }
  if (readinessStatus === 'failed') return { label: t('dashboard.alertFailed'), tone: 'danger' }
  if (readinessStatus === 'degraded') return { label: t('dashboard.alertDegraded'), tone: 'warning' }
  if (readinessStatus === 'ready' || systemStatus === 'running') return { label: t('dashboard.overall.running'), tone: 'success' }
  return { label: t('dashboard.overall.unknown'), tone: 'muted' }
}

export interface StatusAttention {
  count: number
  detail: string
  runtimeResources: RuntimeBootstrapResource[]
  title: string
  tone: 'warning' | 'danger'
  view: 'readiness' | 'diagnostics'
}

// The status page leads with what needs a person: readiness issues first, then diagnostics problems that
// readiness has not already reported, each counted once.
export function summarizeAttention(
  readinessIssues: readonly DiagnosticIssue[],
  diagnosticsIssues: readonly DiagnosticIssue[],
  passedCheckCount: number,
): StatusAttention | null {
  const pending = readinessIssues.filter(issue => issue.severity === 'error' || issue.severity === 'warning')
  const readinessCodes = new Set(pending.map(issue => issue.code))
  const diagnostics = dedupeIssues(diagnosticsIssues)
    .filter(issue => (issue.severity === 'error' || issue.severity === 'warning') && !readinessCodes.has(issue.code))
  const count = pending.length + diagnostics.length
  if (count === 0) return null

  const tone = [...pending, ...diagnostics].some(issue => issue.severity === 'error') ? 'danger' : 'warning'
  // One preparation covers every missing runtime resource, whichever issue reported it.
  const runtimeResources = Array.from(new Set([...pending, ...diagnostics].flatMap(issue => issue.runtime_resources ?? [])))
  const readinessTop = pending.find(issue => issue.severity === 'error') ?? pending[0]
  if (readinessTop) {
    const passed = passedCheckCount > 0 ? t('dashboard.attentionPassed', { count: passedCheckCount }) : ''
    return {
      count,
      detail: `${readinessTop.remediation ?? ''}${passed}`,
      runtimeResources,
      title: t('dashboard.attentionTitle', { count, summary: readinessTop.summary }),
      tone,
      view: 'readiness',
    }
  }

  const diagnosticsTop = diagnostics.find(issue => issue.severity === 'error') ?? diagnostics[0]!
  return {
    count,
    detail: diagnosticsTop.remediation || t('dashboard.diagnosticsRemediationUnavailable'),
    runtimeResources,
    title: t('dashboard.attentionTitle', { count, summary: diagnosticsTop.user_message || diagnosticsTop.summary }),
    tone,
    view: 'diagnostics',
  }
}

// Plugin lifecycle belongs to the plugin center and the logs; the status page lists system changes only.
export function isSystemEvent(payload: EventsPayload) {
  return !('plugin_id' in payload)
}

export function describeEventTone(payload: EventsPayload): StatusRowTone {
  if ('service_status' in payload) return toRowTone(payload.service_status)
  if ('connection_status' in payload) return toRowTone(payload.connection_status)
  return 'muted'
}

const knownCheckNames = new Set(['database', 'runtime', 'render'])
const knownCheckStates = new Set(['ok', 'unavailable', 'preparing', 'resource_missing'])

// Readiness and diagnostics can report the same issue more than once.
export function dedupeIssues<T extends Pick<DiagnosticIssue, 'code' | 'severity' | 'summary' | 'remediation'>>(issues: readonly T[] = []) {
  const seen = new Set<string>()
  return issues.filter((issue) => {
    const key = `${issue.code}::${issue.severity}::${issue.summary}::${issue.remediation ?? ''}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

export function buildReadinessCheckItems(checks: ReadinessStatusResponse['checks'] = {}) {
  return Object.entries(checks as Record<string, unknown>).map(([key, value]) => {
    // A resource still being prepared is neither a pass nor a problem yet.
    let status: StatusType = 'muted'
    if (value === 'ok') status = 'success'
    else if (value === 'unavailable') status = 'danger'
    else if (value && value !== 'preparing') status = 'warning'
    return {
      key,
      value,
      status,
      label: knownCheckNames.has(key) ? t(`dashboard.readinessCheckNames.${key}`) : key,
      displayValue: typeof value === 'string' && knownCheckStates.has(value) ? t(`dashboard.readinessCheckStates.${value}`) : String(value ?? t('display.empty')),
    }
  })
}

// Reason codes already explained by an issue card are not repeated in technical details.
export function listUnexplainedReasonCodes(reasonCodes: readonly string[] = [], issues: readonly Pick<DiagnosticIssue, 'code'>[]) {
  const issueCodes = new Set(issues.map(issue => issue.code))
  return reasonCodes.filter((code, index) => reasonCodes.indexOf(code) === index && !issueCodes.has(code))
}

export function describeReadinessAlert(readiness: ReadinessStatusResponse | null, issues: readonly DiagnosticIssue[]) {
  if (!readiness) return null
  const adapterIssue = issues.find(issue => issue.code.startsWith('adapter.'))
  let level: ToastLevel
  let title: string
  if (readiness.status === 'failed') {
    level = 'error'
    title = t('dashboard.alertFailed')
  } else if (readiness.status === 'degraded') {
    level = 'warning'
    title = t('dashboard.alertDegraded')
  } else if (adapterIssue) {
    level = 'warning'
    title = t('dashboard.alertProtocolWarning')
  } else {
    return null
  }
  const topIssue = issues.find(issue => issue.severity === 'error') ?? issues[0]
  const detail = adapterIssue ? adapterIssue.summary : topIssue ? topIssue.summary : readiness.reason ?? ''
  return { level, title, detail }
}

export function describeProtocolIssue(adapters: readonly AdapterDescriptor[]) {
  const issues = adapters.flatMap((adapter) => {
    const snapshot = adapter.onebot11
    if (!adapter.enabled || !snapshot || !['degraded', 'failed'].includes(snapshot.readiness_status)) return []
    return snapshot.recent_transport_issues.map(issue => ({ ...issue, summary: `${adapter.display_name}：${issue.summary}` }))
  })
  if (issues.length === 0) return null
  return {
    code: issues.map(issue => issue.code).join(','),
    level: issues.some(issue => issue.severity === 'error') ? 'error' as const : 'warning' as const,
    summary: issues.map(issue => issue.summary).join('；'),
  }
}

function diagnosticsStatusType(status?: string): StatusType {
  if (!status) return 'muted'
  if (['ok', 'ready', 'running', 'connected', 'cached', 'on_demand', 'loaded', 'applied'].includes(status)) return 'success'
  if (['warning', 'degraded', 'connecting', 'idle', 'disabled', 'missing', 'unknown'].includes(status)) return 'warning'
  if (['error', 'failed', 'unavailable', 'metadata_incomplete', 'unreadable', 'shutting_down'].includes(status)) return 'danger'
  return 'muted'
}

function diagnosticsStatusLabel(status?: string) {
  if (!status) return t('display.empty')
  const key = `dashboard.diagnosticsStatus.${status}`
  return i18n.global.te(key) ? t(key) : status
}

function issueCountDetail(count: number) {
  return count > 0 ? t('dashboard.diagnosticsIssueCount', { count }) : t('dashboard.diagnosticsNoIssues')
}

export function buildDiagnosticsSubsystemItems(snapshot: SystemDiagnosticsResponse | null) {
  if (!snapshot) return []

  const dependencyBlockingCount = snapshot.dependencies.filter(
    dependency => ['metadata_incomplete', 'unavailable'].includes(dependency.status),
  ).length
  // Only a ready dependency is usable now; on_demand and cached ones still have to be prepared.
  const dependencyReadyCount = snapshot.dependencies.filter(dependency => dependency.status === 'ready').length
  const dependencyPendingCount = snapshot.dependencies.length - dependencyReadyCount - dependencyBlockingCount
  // The dependency row takes the severity the server gave its issues instead of assuming a blocking failure.
  const dependencyTone: StatusType = snapshot.issues.some(issue => issue.code.startsWith('dependency.') && issue.severity === 'error') ? 'danger' : 'warning'
  const dependencyStatus: StatusType = dependencyBlockingCount > 0 ? dependencyTone : dependencyPendingCount > 0 ? 'warning' : 'success'
  const filesystemIssueCount = snapshot.filesystem.filter(path => path.status !== 'ok').length
  const adapters = describeAdapterStates(snapshot.adapters)
  const failureStatus = (failed: number, tone: StatusType = 'danger'): StatusType => failed > 0 ? tone : 'success'

  // The service state and version already lead the page, and the config is always loaded and applied, so neither
  // gets a row here.
  return [
    {
      key: 'adapter',
      label: t('dashboard.diagnosticsSubsystems.adapter'),
      status: adapters.status,
      value: adapters.value,
      detail: adapters.detail,
    },
    {
      key: 'plugins',
      label: t('dashboard.diagnosticsSubsystems.plugins'),
      status: failureStatus(snapshot.plugins.failed),
      value: t('dashboard.diagnosticsPluginValue', { running: snapshot.plugins.running, active: snapshot.plugins.active }),
      detail: t('dashboard.diagnosticsPluginDetail', { failed: snapshot.plugins.failed, total: snapshot.plugins.total }),
    },
    {
      key: 'render',
      label: t('dashboard.diagnosticsSubsystems.render'),
      status: diagnosticsStatusType(snapshot.render.status),
      value: diagnosticsStatusLabel(snapshot.render.status),
      detail: issueCountDetail(snapshot.render.issues.length),
    },
    {
      key: 'scheduler',
      label: t('dashboard.diagnosticsSubsystems.scheduler'),
      status: failureStatus(snapshot.scheduler.failed),
      value: t('dashboard.diagnosticsSchedulerValue', { running: snapshot.scheduler.running, pending: snapshot.scheduler.pending }),
      detail: t('dashboard.diagnosticsSchedulerDetail', { enabled: snapshot.scheduler.enabled, total: snapshot.scheduler.total, failed: snapshot.scheduler.failed, disabled: snapshot.scheduler.disabled }),
    },
    {
      key: 'tasks',
      label: t('dashboard.diagnosticsSubsystems.tasks'),
      status: failureStatus(snapshot.tasks.failed, 'warning'),
      value: t('dashboard.diagnosticsTaskValue', { running: snapshot.tasks.running, pending: snapshot.tasks.pending }),
      detail: t('dashboard.diagnosticsTaskDetail', { failed: snapshot.tasks.failed }),
    },
    {
      key: 'dependencies',
      label: t('dashboard.diagnosticsSubsystems.dependencies'),
      status: dependencyStatus,
      value: t('dashboard.diagnosticsDependencyValue', {
        ready: dependencyReadyCount,
        total: snapshot.dependencies.length,
      }),
      detail: dependencyBlockingCount > 0 || dependencyPendingCount === 0
        ? issueCountDetail(dependencyBlockingCount)
        : t('dashboard.diagnosticsDependencyPending', { count: dependencyPendingCount }),
    },
    {
      key: 'filesystem',
      label: t('dashboard.diagnosticsSubsystems.filesystem'),
      status: failureStatus(filesystemIssueCount),
      value: t('dashboard.diagnosticsFilesystemValue', {
        ok: snapshot.filesystem.length - filesystemIssueCount,
        total: snapshot.filesystem.length,
      }),
      detail: issueCountDetail(filesystemIssueCount),
    },
  ]
}

function issueSeverityStatus(severity: DiagnosticIssue['severity']): StatusType {
  if (severity === 'error') return 'danger'
  if (severity === 'warning') return 'warning'
  return 'success'
}

export function buildDiagnosticsIssueCards(issues: readonly DiagnosticIssue[] = []) {
  return dedupeIssues(issues)
    .filter(issue => issue.severity !== 'ok')
    .map(issue => ({
      key: `${issue.code}:${issue.severity}:${issue.summary}`,
      code: issue.code,
      problem: issue.user_message || issue.summary,
      remediation: issue.remediation || t('dashboard.diagnosticsRemediationUnavailable'),
      severity: issue.severity,
      status: issueSeverityStatus(issue.severity),
    }))
}
