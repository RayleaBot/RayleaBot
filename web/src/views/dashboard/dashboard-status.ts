import { i18n, t } from '@/i18n'
import { describeAdapterStates } from '@/lib/adapter-status'
import type { StatusType } from '@/lib/display'
import type { AdapterDescriptor, ReadinessStatusResponse, SystemDiagnosticsResponse } from '@/types/api'

type DiagnosticIssue = SystemDiagnosticsResponse['issues'][number]
type ToastLevel = 'warning' | 'error'

const knownCheckNames = new Set(['config', 'database', 'runtime', 'render', 'adapter', 'plugins', 'scheduler'])
const knownCheckStates = new Set(['ok', 'passed', 'ready', 'error', 'failed', 'unavailable', 'resource_missing', 'not_configured', 'skipped'])

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
    let status: StatusType = 'muted'
    if (value === 'ok' || value === 'passed' || value === 'ready') status = 'success'
    else if (value === 'error' || value === 'failed' || value === 'unavailable') status = 'danger'
    else if (value) status = 'warning'
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
  const filesystemIssueCount = snapshot.filesystem.filter(path => path.status !== 'ok').length
  const adapters = describeAdapterStates(snapshot.adapters)
  const failureStatus = (failed: number, tone: StatusType = 'danger'): StatusType => failed > 0 ? tone : 'success'

  return [
    {
      key: 'system',
      label: t('dashboard.diagnosticsSubsystems.system'),
      status: diagnosticsStatusType(snapshot.system.status),
      value: diagnosticsStatusLabel(snapshot.system.status),
      detail: t('dashboard.diagnosticsCoreVersion', { version: snapshot.build.core_version }),
    },
    {
      key: 'adapter',
      label: t('dashboard.diagnosticsSubsystems.adapter'),
      status: adapters.status,
      value: adapters.value,
      detail: adapters.detail,
    },
    {
      key: 'config',
      label: t('dashboard.diagnosticsSubsystems.config'),
      status: diagnosticsStatusType(snapshot.config.status),
      value: diagnosticsStatusLabel(snapshot.config.status),
      detail: t('dashboard.diagnosticsConfigApplyState', { state: diagnosticsStatusLabel(snapshot.config.apply_state) }),
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
      status: failureStatus(dependencyBlockingCount),
      value: t('dashboard.diagnosticsDependencyValue', {
        ready: snapshot.dependencies.length - dependencyBlockingCount,
        total: snapshot.dependencies.length,
      }),
      detail: issueCountDetail(dependencyBlockingCount),
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
      impact: t(`dashboard.diagnosticsIssueImpact.${issue.severity}`),
      remediation: issue.remediation || t('dashboard.diagnosticsRemediationUnavailable'),
      severity: issue.severity,
      status: issueSeverityStatus(issue.severity),
    }))
}
