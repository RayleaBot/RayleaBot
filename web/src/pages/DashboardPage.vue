<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import DashboardHeroPanel from '@/components/DashboardHeroPanel.vue'
import DashboardRecentEventsCard from '@/components/DashboardRecentEventsCard.vue'
import DashboardReadinessCard from '@/components/DashboardReadinessCard.vue'
import DashboardRecoveryCard from '@/components/DashboardRecoveryCard.vue'
import DashboardStatusGrid from '@/components/DashboardStatusGrid.vue'
import DashboardToolsPanel from '@/components/DashboardToolsPanel.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getAdapterStateLabel,
  getReadinessStatusLabel,
  getSystemStatusLabel,
  getStatusType,
  type StatusType,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDurationSeconds, formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'
import { useSystemStore } from '@/stores/system'
import type { RecoveryCompatibilitySkippedPlugin, RuntimeBootstrapResource } from '@/types/api'

const AUTO_REFRESH_INTERVAL = 10

const router = useRouter()
const systemStore = useSystemStore()
const { backupPending, diagnosticsPending, error, health, loading, previewPending, readiness, recentEvents, recoveryConfirmPending, recoveryRecheckPending, runtimeBootstrapPending, system } = storeToRefs(systemStore)

const previewVisible = ref(false)
const previewForm = reactive({
  template: 'help.menu',
  theme: 'default',
  output: 'png' as 'png' | 'jpeg',
  dataText: JSON.stringify({
    title: '帮助菜单',
    subtitle: '系统页渲染调试入口',
    items: [
      {
        name: 'weather',
        description: '查询天气',
        usage: '/weather <城市>',
      },
    ],
  }, null, 2),
})

const autoRefresh = ref(false)
const lastRefreshed = ref<string | null>(null)
const countdown = ref(AUTO_REFRESH_INTERVAL)
const issuesExpanded = ref(false)
const selectedRecoveryReviewIds = ref<string[]>([])
const recoveryConfirmNote = ref('')
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null
let countdownTimer: ReturnType<typeof setInterval> | null = null

const healthStatusType = computed<StatusType>(() => getStatusType(health.value?.status))
const readinessStatusType = computed<StatusType>(() => getStatusType(readiness.value?.status))
const systemStatusType = computed<StatusType>(() => getStatusType(system.value?.status))
const adapterStatusType = computed<StatusType>(() => getStatusType(system.value?.adapter_state))
const recoverySummary = computed(() => system.value?.recovery_summary ?? readiness.value?.recovery_summary ?? null)
const healthValueText = computed(() => health.value?.status === 'ok' ? '正常' : t('display.empty'))
const healthDetailText = computed(() => health.value?.status === 'ok' ? '管理面可用' : t('display.empty'))
const readinessValueText = computed(() => getReadinessStatusLabel(readiness.value?.status))
const readinessDetailText = computed(() => readiness.value?.reason || getReadinessStatusLabel(readiness.value?.status))
const systemValueText = computed(() => getSystemStatusLabel(system.value?.status))
const systemDetailText = computed(() => getSystemStatusLabel(system.value?.status))
const adapterValueText = computed(() => getAdapterStateLabel(system.value?.adapter_state))
const adapterDetailText = computed(() => getAdapterStateLabel(system.value?.adapter_state))
const readinessIssues = computed(() => {
  const issues = readiness.value?.issues ?? []
  const seen = new Set<string>()
  return issues.filter((issue) => {
    const key = `${issue.code}::${issue.severity}::${issue.summary}::${issue.remediation ?? ''}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
})
const visibleReasonCodes = computed(() => {
  const reasonCodes = readiness.value?.reason_codes ?? []
  if (!reasonCodes.length) return []

  const issueCodes = new Set(readinessIssues.value.map(issue => issue.code))
  return reasonCodes.filter((code, index) => reasonCodes.indexOf(code) === index && !issueCodes.has(code))
})

const topIssue = computed(() => {
  if (!readinessIssues.value.length) return null
  return readinessIssues.value.find(i => i.severity === 'error') ?? readinessIssues.value[0]
})

const adapterWarningIssue = computed(() => readinessIssues.value.find((issue) => issue.code.startsWith('adapter.')) ?? null)

function isPythonRuntimeIssue(issue: { code?: string; summary?: string; remediation?: string }) {
  const joined = `${issue.code ?? ''} ${issue.summary ?? ''} ${issue.remediation ?? ''}`
  return joined.includes('python') || joined.includes('Python')
}

const pythonRuntimeIssue = computed(() => readinessIssues.value.find((issue) => isPythonRuntimeIssue(issue)) ?? null)

const recoveryStatusLabel = computed(() => {
  const status = recoverySummary.value?.status
  if (status === 'compatible') return '兼容通过'
  if (status === 'pending') return '待完成检查'
  if (status === 'degraded') return '需要人工处理'
  if (status === 'blocked') return '恢复被阻止'
  return t('display.empty')
})

const recoveryBootstrapResources = computed<RuntimeBootstrapResource[]>(() => {
  const resources = new Set<RuntimeBootstrapResource>()
  for (const issue of [...(recoverySummary.value?.issues ?? []), ...readinessIssues.value]) {
    const code = issue.code ?? ''
    const summary = issue.summary ?? ''
    if (code.includes('python') || summary.includes('Python')) {
      resources.add('python-runtime')
    }
    if (code.includes('node') || summary.includes('Node')) {
      resources.add('nodejs-runtime')
    }
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
  return (recoverySummary.value?.skipped_plugins ?? []).filter((plugin) => plugin.review_status !== 'confirmed')
})

const selectedRecoveryReviewCountLabel = computed(() => t('dashboard.recoveryConfirmSelection', { count: selectedRecoveryReviewIds.value.length }))

const recoveryAuditEntries = computed(() => recoverySummary.value?.audit ?? [])

const alertBannerType = computed<'warning' | 'error' | null>(() => {
  if (readiness.value?.status === 'failed') return 'error'
  if (readiness.value?.status === 'degraded') return 'warning'
  if (adapterWarningIssue.value) return 'warning'
  return null
})

const alertBannerTitle = computed(() => {
  if (readiness.value?.status === 'failed') return t('dashboard.alertFailed')
  if (readiness.value?.status === 'degraded') return t('dashboard.alertDegraded')
  if (adapterWarningIssue.value) return t('dashboard.alertProtocolWarning')
  return ''
})

const alertBannerMessage = computed(() => {
  if (!readiness.value) return ''
  if (pythonRuntimeIssue.value) return t('dashboard.pythonRuntimeLimited')
  if (adapterWarningIssue.value) return adapterWarningIssue.value.summary
  if (topIssue.value) return topIssue.value.summary
  if (readiness.value.reason) return readiness.value.reason
  return ''
})

const statusBadgeConfig = computed(() => {
  const status = readiness.value?.status
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
  const checks = readiness.value?.checks ?? {}
  const items: Array<{ key: string; value: string; status: StatusType }> = []
  for (const [key, value] of Object.entries(checks)) {
    if (value && (value === 'ok' || value === 'passed' || value === 'ready')) {
      items.push({ key, value, status: 'success' })
    } else if (value && (value === 'error' || value === 'failed' || value === 'unavailable')) {
      items.push({ key, value, status: 'danger' })
    } else {
      items.push({ key, value, status: value ? 'warning' : 'muted' })
    }
  }
  return items
})

async function refreshState() {
  try {
    await systemStore.refresh()
    lastRefreshed.value = new Date().toISOString()
    countdown.value = AUTO_REFRESH_INTERVAL
  } catch {
    // store error state drives the page
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  autoRefresh.value = true
  countdown.value = AUTO_REFRESH_INTERVAL

  countdownTimer = setInterval(() => {
    countdown.value = Math.max(0, countdown.value - 1)
  }, 1000)

  autoRefreshTimer = setInterval(() => {
    void refreshState()
  }, AUTO_REFRESH_INTERVAL * 1000)
}

function stopAutoRefresh() {
  if (autoRefreshTimer !== null) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
  if (countdownTimer !== null) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  autoRefresh.value = false
}

function toggleAutoRefresh(val: boolean) {
  if (val) {
    void refreshState()
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

onMounted(() => {
  void refreshState()
})

onUnmounted(() => {
  stopAutoRefresh()
})

watch(recoverySummary, (nextSummary) => {
  const pendingIds = new Set((nextSummary?.skipped_plugins ?? []).filter((plugin) => plugin.review_status !== 'confirmed').map((plugin) => plugin.review_id))
  selectedRecoveryReviewIds.value = selectedRecoveryReviewIds.value.filter((reviewID) => pendingIds.has(reviewID))
  if (selectedRecoveryReviewIds.value.length === 0) {
    recoveryConfirmNote.value = ''
  }
})

async function createBackup() {
  try {
    const response = await systemStore.createBackup()
    ElMessage.success(t('dashboard.backupAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function exportDiagnostics() {
  try {
    await systemStore.exportDiagnostics()
    ElMessage.success(t('dashboard.diagnosticsAccepted'))
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function submitRenderPreview() {
  let data: Record<string, unknown>
  try {
    const parsed = JSON.parse(previewForm.dataText)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      throw new Error(t('errors.platform.invalidRequest'))
    }
    data = parsed as Record<string, unknown>
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
    return
  }

  try {
    const response = await systemStore.previewRender({
      template: previewForm.template,
      theme: previewForm.theme || undefined,
      output: previewForm.output,
      data,
    })
    previewVisible.value = false
    ElMessage.success(t('dashboard.previewAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function recheckRecoverySummary() {
  try {
    const response = await systemStore.recheckRecovery()
    ElMessage.success(t('dashboard.recoveryRecheckAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function confirmRecoverySelection() {
  if (selectedRecoveryReviewIds.value.length === 0) return

  try {
    const response = await systemStore.confirmRecovery({
      review_ids: [...selectedRecoveryReviewIds.value],
      note: recoveryConfirmNote.value.trim() || undefined,
    })
    ElMessage.success(t('dashboard.recoveryConfirmAccepted'))
    selectedRecoveryReviewIds.value = []
    recoveryConfirmNote.value = ''
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function bootstrapRuntimeResources() {
  try {
    const response = await systemStore.bootstrapManagedRuntime(recoveryBootstrapResources.value)
    ElMessage.success(t('dashboard.runtimeBootstrapAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function openRecoveryPlugin(pluginID: string) {
  await router.push({ name: 'plugin-detail', params: { id: pluginID } })
}
</script>

<template>
  <div class="page-grid">
    <DashboardHeroPanel
      :title="t('dashboard.title')"
      :status-badge="statusBadgeConfig"
      :last-refreshed-label="lastRefreshed ? `${t('dashboard.lastRefreshed')}: ${formatRelativeTime(lastRefreshed)}` : null"
      :auto-refresh="autoRefresh"
      :countdown="countdown"
      :auto-refresh-interval="AUTO_REFRESH_INTERVAL"
      :loading="loading"
      @refresh="refreshState()"
      @toggle-auto-refresh="toggleAutoRefresh"
    />

    <el-alert
      v-if="alertBannerType"
      :type="alertBannerType"
      :title="alertBannerTitle"
      :description="alertBannerMessage"
      show-icon
      :closable="false"
    />

    <RetryPanel
      v-if="error && !system"
      :title="t('routes.status')"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <DashboardStatusGrid
      :health-status-type="healthStatusType"
      :readiness-status-type="readinessStatusType"
      :system-status-type="systemStatusType"
      :adapter-status-type="adapterStatusType"
      :health-label="t('dashboard.health')"
      :health-value-text="healthValueText"
      :health-detail-text="healthDetailText"
      :readiness-label="t('dashboard.readiness')"
      :readiness-value-text="readinessValueText"
      :readiness-detail-text="readinessDetailText"
      :system-label="t('dashboard.service')"
      :system-value-text="systemValueText"
      :system-detail-text="systemDetailText"
      :adapter-label="t('dashboard.adapter')"
      :adapter-value-text="adapterValueText"
      :adapter-detail-text="adapterDetailText"
      :active-plugins-label="t('dashboard.activePlugins')"
      :active-plugins-count="system?.active_plugins ?? 0"
      :uptime-label="t('dashboard.uptime')"
      :uptime-text="formatDurationSeconds(system?.uptime_seconds)"
    />

    <div class="content-grid">
      <DashboardReadinessCard
        :section-title="t('dashboard.readinessSection')"
        :check-items="checkItems"
        :readiness-note-text="health?.status === 'ok' && readiness?.status === 'degraded' ? t('dashboard.readinessLimitedHint') : t('dashboard.readinessHint')"
        :reason-codes-label="t('dashboard.reasonCodes')"
        :visible-reason-codes="visibleReasonCodes"
        :readiness-issues="readinessIssues"
        :issues-expanded="issuesExpanded"
        :expand-issues-text="t('dashboard.expandIssues', { count: readinessIssues.length - 3 })"
        :collapse-issues-text="t('dashboard.collapseIssues')"
        @toggle-issues="issuesExpanded = !issuesExpanded"
      />

      <DashboardRecoveryCard
        v-model:selected-recovery-review-ids="selectedRecoveryReviewIds"
        v-model:recovery-confirm-note="recoveryConfirmNote"
        :recovery-summary="recoverySummary"
        :recovery-status-label="recoveryStatusLabel"
        :pending-recovery-plugins="pendingRecoveryPlugins"
        :selected-recovery-review-count-label="selectedRecoveryReviewCountLabel"
        :recovery-audit-entries="recoveryAuditEntries"
        :recovery-recheck-pending="recoveryRecheckPending"
        :recovery-confirm-pending="recoveryConfirmPending"
        :runtime-bootstrap-pending="runtimeBootstrapPending"
        @recheck="recheckRecoverySummary"
        @bootstrap="bootstrapRuntimeResources"
        @open-plugin="openRecoveryPlugin"
        @confirm="confirmRecoverySelection"
      />

      <DashboardRecentEventsCard :recent-events="recentEvents" />
    </div>

    <DashboardToolsPanel
      :backup-pending="backupPending"
      :diagnostics-pending="diagnosticsPending"
      :preview-pending="previewPending"
      @create-backup="createBackup"
      @export-diagnostics="exportDiagnostics"
      @open-preview="previewVisible = true"
    />

    <el-dialog v-model="previewVisible" :title="t('dashboard.previewTitle')" width="min(720px, 92vw)">
      <el-form label-position="top">
        <el-form-item :label="t('dashboard.previewTemplate')">
          <el-input v-model="previewForm.template" placeholder="help.menu" />
        </el-form-item>
        <el-form-item :label="t('dashboard.previewTheme')">
          <el-input v-model="previewForm.theme" placeholder="default" />
        </el-form-item>
        <el-form-item :label="t('dashboard.previewOutput')">
          <el-radio-group v-model="previewForm.output">
            <el-radio-button label="png" value="png" />
            <el-radio-button label="jpeg" value="jpeg" />
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('dashboard.previewData')">
          <el-input
            v-model="previewForm.dataText"
            type="textarea"
            :rows="10"
            placeholder="{&quot;title&quot;:&quot;帮助菜单&quot;}"
          />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="table-actions">
          <el-button @click="previewVisible = false">
            {{ t('dashboard.previewCancel') }}
          </el-button>
          <el-button type="primary" :loading="previewPending" @click="submitRenderPreview">
            {{ t('dashboard.previewSubmit') }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
