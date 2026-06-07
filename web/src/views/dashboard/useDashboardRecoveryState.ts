import { computed, type Ref } from 'vue'

import { getRecoveryStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import type { RecoveryCompatibilitySkippedPlugin, RuntimeBootstrapResource } from '@/types/api'

type RecoveryInput = {
  readinessIssues: Ref<any[]>
  readiness: Ref<any>
  selectedRecoveryReviewIds: Ref<string[]>
  system: Ref<any>
}

function isPythonRuntimeIssue(issue: { code?: string; summary?: string; remediation?: string }) {
  const joined = `${issue.code ?? ''} ${issue.summary ?? ''} ${issue.remediation ?? ''}`
  return joined.includes('python') || joined.includes('Python')
}

export function useDashboardRecoveryState(input: RecoveryInput) {
  const recoverySummary = computed(() => input.system.value?.recovery_summary ?? input.readiness.value?.recovery_summary ?? null)
  const topIssue = computed(() => input.readinessIssues.value.find((issue: any) => issue.severity === 'error') ?? input.readinessIssues.value[0] ?? null)
  const adapterWarningIssue = computed(() => input.readinessIssues.value.find((issue: any) => issue.code.startsWith('adapter.')) ?? null)
  const pythonRuntimeIssue = computed(() => input.readinessIssues.value.find((issue: any) => isPythonRuntimeIssue(issue)) ?? null)
  const recoveryStatusLabel = computed(() => getRecoveryStatusLabel(recoverySummary.value?.status))
  const recoveryBootstrapResources = computed<RuntimeBootstrapResource[]>(() => {
    const resources = new Set<RuntimeBootstrapResource>()
    for (const issue of [...(recoverySummary.value?.issues ?? []), ...input.readinessIssues.value]) {
      const code = issue.code ?? ''
      const summary = issue.summary ?? ''
      if (code.includes('python') || summary.includes('Python')) resources.add('python-runtime')
      if (code.includes('node') || summary.includes('Node')) resources.add('nodejs-runtime')
      if (code === 'platform.resource_missing' || code.includes('chromium') || summary.includes('Chromium')) {
        resources.add('chromium')
      }
    }
    if (resources.size === 0) {
      resources.add('chromium')
      resources.add('python-runtime')
      resources.add('nodejs-runtime')
    }
    return [...resources]
  })
  const pendingRecoveryPlugins = computed<RecoveryCompatibilitySkippedPlugin[]>(() => {
    return (recoverySummary.value?.skipped_plugins ?? []).filter((plugin: RecoveryCompatibilitySkippedPlugin) => plugin.review_status !== 'confirmed')
  })
  const selectedRecoveryReviewCountLabel = computed(() => t('dashboard.recoveryConfirmSelection', { count: input.selectedRecoveryReviewIds.value.length }))
  const readinessToastLevel = computed<'warning' | 'error' | null>(() => {
    if (input.readiness.value?.status === 'failed') return 'error'
    if (input.readiness.value?.status === 'degraded') return 'warning'
    if (adapterWarningIssue.value) return 'warning'
    return null
  })
  const readinessToastTitle = computed(() => {
    if (input.readiness.value?.status === 'failed') return t('dashboard.alertFailed')
    if (input.readiness.value?.status === 'degraded') return t('dashboard.alertDegraded')
    if (adapterWarningIssue.value) return t('dashboard.alertProtocolWarning')
    return ''
  })
  const readinessToastMessage = computed(() => {
    if (!input.readiness.value) return ''
    if (pythonRuntimeIssue.value) return t('dashboard.pythonRuntimeLimited')
    if (adapterWarningIssue.value) return adapterWarningIssue.value.summary
    if (topIssue.value) return topIssue.value.summary
    if (input.readiness.value.reason) return input.readiness.value.reason
    return ''
  })

  return {
    pendingRecoveryPlugins,
    readinessToastLevel,
    readinessToastMessage,
    readinessToastTitle,
    recoveryBootstrapResources,
    recoveryStatusLabel,
    recoverySummary,
    selectedRecoveryReviewCountLabel,
  }
}
