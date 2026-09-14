import { computed, type Ref } from 'vue'

import { t } from '@/i18n'

type AlertInput = {
  readinessIssues: Ref<any[]>
  readiness: Ref<any>
}

export function useDashboardAlertState(input: AlertInput) {
  const topIssue = computed(() => input.readinessIssues.value.find((issue: any) => issue.severity === 'error') ?? input.readinessIssues.value[0] ?? null)
  const adapterWarningIssue = computed(() => input.readinessIssues.value.find((issue: any) => issue.code.startsWith('adapter.')) ?? null)
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
    if (adapterWarningIssue.value) return adapterWarningIssue.value.summary
    if (topIssue.value) return topIssue.value.summary
    if (input.readiness.value.reason) return input.readiness.value.reason
    return ''
  })

  return {
    readinessToastLevel,
    readinessToastMessage,
    readinessToastTitle,
  }
}
