import { computed, type Ref } from 'vue'

import { t } from '@/i18n'
import type { StatusType } from '@/lib/display'
import type { SystemDiagnosticsResponse } from '@/types/api'

type DiagnosticsInput = {
  diagnostics: Ref<SystemDiagnosticsResponse | null>
}

type DiagnosticsIssue = SystemDiagnosticsResponse['issues'][number]

function statusToType(status?: string): StatusType {
  if (!status) return 'muted'
  if (['ok', 'ready', 'running', 'connected', 'cached', 'on_demand', 'loaded', 'applied'].includes(status)) {
    return 'success'
  }
  if (['warning', 'degraded', 'connecting', 'idle', 'disabled', 'missing', 'unknown'].includes(status)) {
    return 'warning'
  }
  if (['error', 'failed', 'unavailable', 'metadata_incomplete', 'unreadable', 'shutting_down'].includes(status)) {
    return 'danger'
  }
  return 'muted'
}

function issueSeverityToType(severity: DiagnosticsIssue['severity']): StatusType {
  if (severity === 'error') return 'danger'
  if (severity === 'warning') return 'warning'
  return 'success'
}

function statusLabel(status?: string) {
  return status ? t(`dashboard.diagnosticsStatus.${status}`) : t('display.empty')
}

function issueCountDetail(count: number) {
  return count > 0 ? t('dashboard.diagnosticsIssueCount', { count }) : t('dashboard.diagnosticsNoIssues')
}

function dedupeIssues(issues: DiagnosticsIssue[]) {
  const seen = new Set<string>()
  return issues.filter((issue) => {
    const key = `${issue.code}::${issue.severity}::${issue.summary}::${issue.remediation ?? ''}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

export function useDashboardDiagnosticsState(input: DiagnosticsInput) {
  const diagnosticsSubsystemItems = computed(() => {
    const snapshot = input.diagnostics.value
    if (!snapshot) return []

    const dependencyBlockingCount = snapshot.dependencies.filter(
      dependency => ['metadata_incomplete', 'unavailable'].includes(dependency.status),
    ).length
    const filesystemIssueCount = snapshot.filesystem.filter(path => path.status !== 'ok').length

    return [
      {
        key: 'system',
        label: t('dashboard.diagnosticsSubsystems.system'),
        status: statusToType(snapshot.system.status),
        value: statusLabel(snapshot.system.status),
        detail: t('dashboard.diagnosticsCoreVersion', { version: snapshot.build.core_version }),
      },
      {
        key: 'adapter',
        label: t('dashboard.diagnosticsSubsystems.adapter'),
        status: statusToType(snapshot.adapter.state),
        value: statusLabel(snapshot.adapter.state),
        detail: snapshot.config.onebot_configured ? t('dashboard.diagnosticsOneBotConfigured') : t('dashboard.diagnosticsOneBotMissing'),
      },
      {
        key: 'config',
        label: t('dashboard.diagnosticsSubsystems.config'),
        status: statusToType(snapshot.config.status),
        value: statusLabel(snapshot.config.status),
        detail: t('dashboard.diagnosticsConfigApplyState', { state: statusLabel(snapshot.config.apply_state) }),
      },
      {
        key: 'plugins',
        label: t('dashboard.diagnosticsSubsystems.plugins'),
        status: snapshot.plugins.failed > 0 ? 'danger' as const : 'success' as const,
        value: t('dashboard.diagnosticsPluginValue', { running: snapshot.plugins.running, active: snapshot.plugins.active }),
        detail: t('dashboard.diagnosticsPluginDetail', { failed: snapshot.plugins.failed, total: snapshot.plugins.total }),
      },
      {
        key: 'render',
        label: t('dashboard.diagnosticsSubsystems.render'),
        status: statusToType(snapshot.render.status),
        value: statusLabel(snapshot.render.status),
        detail: issueCountDetail(snapshot.render.issues.length),
      },
      {
        key: 'third-party',
        label: t('dashboard.diagnosticsSubsystems.thirdParty'),
        status: snapshot.third_party.invalid > 0 ? 'warning' as const : 'success' as const,
        value: t('dashboard.diagnosticsThirdPartyValue', { configured: snapshot.third_party.configured, total: snapshot.third_party.total }),
        detail: t('dashboard.diagnosticsThirdPartyDetail', { enabled: snapshot.third_party.enabled, invalid: snapshot.third_party.invalid }),
      },
      {
        key: 'bilibili-source',
        label: t('dashboard.diagnosticsSubsystems.bilibiliSource'),
        status: statusToType(snapshot.bilibili_source.status),
        value: statusLabel(snapshot.bilibili_source.status),
        detail: snapshot.bilibili_source.summary,
      },
      {
        key: 'scheduler',
        label: t('dashboard.diagnosticsSubsystems.scheduler'),
        status: snapshot.scheduler.failed > 0 ? 'danger' as const : 'success' as const,
        value: t('dashboard.diagnosticsSchedulerValue', { running: snapshot.scheduler.running, pending: snapshot.scheduler.pending }),
        detail: t('dashboard.diagnosticsSchedulerDetail', { enabled: snapshot.scheduler.enabled, total: snapshot.scheduler.total, failed: snapshot.scheduler.failed, disabled: snapshot.scheduler.disabled }),
      },
      {
        key: 'tasks',
        label: t('dashboard.diagnosticsSubsystems.tasks'),
        status: snapshot.tasks.failed > 0 ? 'warning' as const : 'success' as const,
        value: t('dashboard.diagnosticsTaskValue', { running: snapshot.tasks.running, pending: snapshot.tasks.pending }),
        detail: t('dashboard.diagnosticsTaskDetail', { failed: snapshot.tasks.failed }),
      },
      {
        key: 'dependencies',
        label: t('dashboard.diagnosticsSubsystems.dependencies'),
        status: dependencyBlockingCount > 0 ? 'danger' as const : 'success' as const,
        value: t('dashboard.diagnosticsDependencyValue', {
          ready: snapshot.dependencies.length - dependencyBlockingCount,
          total: snapshot.dependencies.length,
        }),
        detail: issueCountDetail(dependencyBlockingCount),
      },
      {
        key: 'filesystem',
        label: t('dashboard.diagnosticsSubsystems.filesystem'),
        status: filesystemIssueCount > 0 ? 'danger' as const : 'success' as const,
        value: t('dashboard.diagnosticsFilesystemValue', {
          ok: snapshot.filesystem.length - filesystemIssueCount,
          total: snapshot.filesystem.length,
        }),
        detail: issueCountDetail(filesystemIssueCount),
      },
    ]
  })

  const diagnosticsIssueCards = computed(() => dedupeIssues(input.diagnostics.value?.issues ?? [])
    .filter(issue => issue.severity !== 'ok')
    .map(issue => ({
      key: `${issue.code}:${issue.severity}:${issue.summary}`,
      code: issue.code,
      problem: issue.user_message || issue.summary,
      impact: t(`dashboard.diagnosticsIssueImpact.${issue.severity}`),
      remediation: issue.remediation || t('dashboard.diagnosticsRemediationUnavailable'),
      severity: issue.severity,
      status: issueSeverityToType(issue.severity),
    })))

  return {
    diagnosticsIssueCards,
    diagnosticsSubsystemItems,
  }
}
