import { computed, type Ref } from 'vue'

import {
  getAdapterStateLabel,
  getReadinessStatusLabel,
  getSystemStatusLabel,
  getStatusType,
  type StatusType,
} from '@/lib/display'
import { t } from '@/i18n'

type ReadinessInput = {
  health: Ref<any>
  readiness: Ref<any>
  system: Ref<any>
}

export function useDashboardReadinessState(input: ReadinessInput) {
  const healthStatusType = computed<StatusType>(() => getStatusType(input.health.value?.status))
  const readinessStatusType = computed<StatusType>(() => getStatusType(input.readiness.value?.status))
  const systemStatusType = computed<StatusType>(() => getStatusType(input.system.value?.status))
  const adapterStatusType = computed<StatusType>(() => getStatusType(input.system.value?.adapter_state))
  const healthValueText = computed(() => (input.health.value?.status === 'ok' ? '正常' : t('display.empty')))
  const healthDetailText = computed(() => (input.health.value?.status === 'ok' ? '管理面可用' : t('display.empty')))
  const readinessValueText = computed(() => getReadinessStatusLabel(input.readiness.value?.status))
  const readinessDetailText = computed(() => input.readiness.value?.reason || getReadinessStatusLabel(input.readiness.value?.status))
  const systemValueText = computed(() => getSystemStatusLabel(input.system.value?.status))
  const systemDetailText = computed(() => getSystemStatusLabel(input.system.value?.status))
  const adapterValueText = computed(() => getAdapterStateLabel(input.system.value?.adapter_state))
  const adapterDetailText = computed(() => getAdapterStateLabel(input.system.value?.adapter_state))
  const readinessIssues = computed(() => {
    const issues = input.readiness.value?.issues ?? []
    const seen = new Set<string>()
    return issues.filter((issue: any) => {
      const key = `${issue.code}::${issue.severity}::${issue.summary}::${issue.remediation ?? ''}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
  })
  const visibleReasonCodes = computed(() => {
    const reasonCodes = input.readiness.value?.reason_codes ?? []
    if (!reasonCodes.length) return []

    const issueCodes = new Set(readinessIssues.value.map((issue: any) => issue.code))
    return reasonCodes.filter((code: string, index: number) => reasonCodes.indexOf(code) === index && !issueCodes.has(code))
  })
  const statusBadgeConfig = computed(() => {
    const status = input.readiness.value?.status
    const type = readinessStatusType.value
    const iconMap: Record<StatusType, string> = {
      success: '\u2714',
      warning: '\u26A0',
      danger: '\u2717',
      muted: '\u2014',
    }
    const labelMap: Record<string, string> = {
      ready: '系统正常',
      failed: '系统异常',
      setup_required: '需要配置',
    }
    return {
      type,
      icon: iconMap[type],
      label: status ? (labelMap[status] ?? getReadinessStatusLabel(status)) : t('display.empty'),
    }
  })
  const checkItems = computed(() => {
    const checks = input.readiness.value?.checks ?? {}
    return Object.entries(checks).map(([key, value]) => {
      let status: StatusType = 'muted'
      if (value && (value === 'ok' || value === 'passed' || value === 'ready')) {
        status = 'success'
      } else if (value && (value === 'error' || value === 'failed' || value === 'unavailable')) {
        status = 'danger'
      } else if (value) {
        status = 'warning'
      }
      return { key, value, status }
    })
  })

  return {
    adapterDetailText,
    adapterStatusType,
    adapterValueText,
    checkItems,
    healthDetailText,
    healthStatusType,
    healthValueText,
    readinessDetailText,
    readinessIssues,
    readinessStatusType,
    readinessValueText,
    statusBadgeConfig,
    systemDetailText,
    systemStatusType,
    systemValueText,
    visibleReasonCodes,
  }
}
