<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

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
const healthDetailText = computed(() => health.value?.status === 'ok' ? '管理面可用' : t('display.empty'))
const readinessDetailText = computed(() => readiness.value?.reason || getReadinessStatusLabel(readiness.value?.status))
const systemDetailText = computed(() => getSystemStatusLabel(system.value?.status))
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

function getCheckIcon(status: StatusType): string {
  const map: Record<StatusType, string> = {
    success: '\u2705',
    warning: '\u26A0',
    danger: '\u274C',
    muted: '\u2014',
  }
  return map[status]
}

function getEventSeverityClass(severity?: string): string {
  if (severity === 'error' || severity === 'danger') return 'event-item--danger'
  if (severity === 'warning') return 'event-item--warning'
  if (severity === 'success') return 'event-item--success'
  return ''
}

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
    <section class="hero-panel">
      <div>
        <h1>{{ t('dashboard.title') }}</h1>
        <div class="hero-meta">
          <div :class="['status-badge', `status-badge--${statusBadgeConfig.type}`]">
            <span class="status-badge__icon">{{ statusBadgeConfig.icon }}</span>
            <span>{{ statusBadgeConfig.label }}</span>
          </div>
          <div v-if="lastRefreshed" class="hero-meta__time">
            {{ t('dashboard.lastRefreshed') }}: {{ formatRelativeTime(lastRefreshed) }}
            <template v-if="autoRefresh"> · {{ countdown }}s</template>
          </div>
          <div v-if="autoRefresh" class="auto-refresh-bar">
            <div class="auto-refresh-bar__fill" :style="{ width: `${(countdown / AUTO_REFRESH_INTERVAL) * 100}%` }" />
          </div>
          <div class="hero-auto-refresh">
            <span>{{ t('dashboard.autoRefresh') }}</span>
            <el-switch
              :model-value="autoRefresh"
              size="small"
              @change="toggleAutoRefresh"
            />
          </div>
        </div>
      </div>

      <div class="table-actions">
        <el-button :loading="loading" @click="refreshState()">
          {{ t('dashboard.refresh') }}
        </el-button>
      </div>
    </section>

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

    <div class="stats-grid">
      <el-card :class="['stat-card', `stat-card--${healthStatusType}`]">
        <span class="stat-label">{{ t('dashboard.health') }}</span>
        <strong>{{ health?.status === 'ok' ? '正常' : t('display.empty') }}</strong>
        <small>{{ healthDetailText }}</small>
      </el-card>
      <el-card :class="['stat-card', `stat-card--${readinessStatusType}`]">
        <span class="stat-label">{{ t('dashboard.readiness') }}</span>
        <strong>{{ getReadinessStatusLabel(readiness?.status) }}</strong>
        <small>{{ readinessDetailText }}</small>
      </el-card>
      <el-card :class="['stat-card', `stat-card--${systemStatusType}`]">
        <span class="stat-label">{{ t('dashboard.service') }}</span>
        <strong>{{ getSystemStatusLabel(system?.status) }}</strong>
        <small>{{ systemDetailText }}</small>
      </el-card>
      <el-card :class="['stat-card', `stat-card--${adapterStatusType}`]">
        <span class="stat-label">{{ t('dashboard.adapter') }}</span>
        <strong>{{ getAdapterStateLabel(system?.adapter_state) }}</strong>
        <small>{{ adapterDetailText }}</small>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.activePlugins') }}</span>
        <strong>{{ system?.active_plugins ?? 0 }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.uptime') }}</span>
        <strong>{{ formatDurationSeconds(system?.uptime_seconds) }}</strong>
      </el-card>
    </div>

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('dashboard.readinessSection') }}</span>
          </div>
        </template>

        <div v-if="checkItems.length" class="readiness-checks">
          <div
            v-for="item in checkItems"
            :key="item.key"
            :class="['readiness-check', `readiness-check--${item.status}`]"
          >
            <div class="readiness-check__header">
              <span class="readiness-check__icon">{{ getCheckIcon(item.status) }}</span>
              <span class="readiness-check__name">{{ item.key }}</span>
            </div>
            <div class="readiness-check__value">{{ item.value }}</div>
          </div>
        </div>

        <el-empty v-else :description="t('display.empty')" />

        <div class="readiness-note">
          <small style="color: var(--muted);">
            {{ health?.status === 'ok' && readiness?.status === 'degraded' ? t('dashboard.readinessLimitedHint') : t('dashboard.readinessHint') }}
          </small>
        </div>

        <div v-if="visibleReasonCodes.length" style="margin-top: 14px;">
          <small style="color: var(--muted);">{{ t('dashboard.reasonCodes') }}: {{ visibleReasonCodes.join(', ') }}</small>
        </div>

        <div v-if="readinessIssues.length" class="issues-list" :class="{ 'issues-list--collapsed': !issuesExpanded && readinessIssues.length > 3 }">
          <div
            v-for="issue in readinessIssues"
            :key="`${issue.code}-${issue.summary}`"
            :class="['issue-alert-card', { 'issue-alert-card--warning': issue.severity === 'warning' }]"
          >
            <div class="issue-alert-card__header">
              <el-tag :type="issue.severity === 'error' ? 'danger' : issue.severity === 'warning' ? 'warning' : 'success'" size="small">
                {{ issue.code }}
              </el-tag>
              <span class="issue-alert-card__summary">{{ issue.summary }}</span>
            </div>
            <div v-if="issue.remediation" class="issue-alert-card__remediation">
              {{ issue.remediation }}
            </div>
          </div>
        </div>

        <div v-if="readinessIssues.length > 3" class="issues-toggle">
          <el-button size="small" text @click="issuesExpanded = !issuesExpanded">
            {{ issuesExpanded ? t('dashboard.collapseIssues') : t('dashboard.expandIssues', { count: readinessIssues.length - 3 }) }}
          </el-button>
        </div>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>恢复兼容性</span>
          </div>
        </template>

        <el-empty v-if="!recoverySummary" :description="t('display.empty')" />

        <div v-else class="events-section">
          <div class="table-actions" style="justify-content: flex-start; margin-bottom: 12px;">
            <el-button
              data-testid="recovery-recheck-button"
              size="small"
              :loading="recoveryRecheckPending"
              @click="recheckRecoverySummary"
            >
              {{ t('dashboard.recoveryRecheck') }}
            </el-button>
            <el-button
              data-testid="runtime-bootstrap-button"
              size="small"
              :loading="runtimeBootstrapPending"
              @click="bootstrapRuntimeResources"
            >
              {{ t('dashboard.runtimeBootstrap') }}
            </el-button>
          </div>

          <div class="issue-alert-card" :class="{ 'issue-alert-card--warning': recoverySummary.status !== 'compatible' }">
            <div class="issue-alert-card__header">
              <el-tag :type="recoverySummary.status === 'blocked' ? 'danger' : recoverySummary.status === 'compatible' ? 'success' : 'warning'" size="small">
                {{ recoveryStatusLabel }}
              </el-tag>
              <span class="issue-alert-card__summary">
                {{ recoverySummary.operation }} · {{ recoverySummary.phase }}
              </span>
            </div>
            <div class="issue-alert-card__remediation">
              core {{ recoverySummary.source_core_version ?? t('display.empty') }} -> {{ recoverySummary.target_core_version ?? t('display.empty') }}
            </div>
          </div>

          <div v-for="issue in recoverySummary.issues ?? []" :key="issue.code" class="issue-alert-card" :class="{ 'issue-alert-card--warning': issue.severity === 'warning' }">
            <div class="issue-alert-card__header">
              <el-tag :type="issue.severity === 'error' ? 'danger' : 'warning'" size="small">
                {{ issue.code }}
              </el-tag>
              <span class="issue-alert-card__summary">{{ issue.summary }}</span>
            </div>
            <div v-if="issue.remediation" class="issue-alert-card__remediation">
              {{ issue.remediation }}
            </div>
          </div>

          <div v-for="plugin in recoverySummary.skipped_plugins ?? []" :key="plugin.plugin_id" class="issue-alert-card issue-alert-card--warning">
            <div class="issue-alert-card__header">
              <el-tag :type="plugin.review_status === 'confirmed' ? 'success' : 'warning'" size="small">
                {{ plugin.reason_code }}
              </el-tag>
              <el-button
                link
                type="primary"
                class="issue-alert-card__summary issue-alert-card__summary--link"
                :data-testid="`recovery-plugin-link-${plugin.plugin_id}`"
                @click="openRecoveryPlugin(plugin.plugin_id)"
              >
                {{ plugin.plugin_id }}
              </el-button>
              <el-tag :type="plugin.review_status === 'confirmed' ? 'success' : 'warning'" size="small">
                {{ plugin.review_status === 'confirmed' ? t('dashboard.recoveryConfirmed') : t('dashboard.recoveryPending') }}
              </el-tag>
            </div>
            <div class="issue-alert-card__remediation">{{ plugin.summary }}</div>
            <div v-if="plugin.manual_action" class="issue-alert-card__remediation">{{ plugin.manual_action }}</div>
            <div v-if="plugin.review_status === 'confirmed'" class="issue-alert-card__remediation">
              {{ t('dashboard.recoveryReviewedBy') }}：{{ plugin.reviewed_by || t('display.empty') }}
              · {{ t('dashboard.recoveryReviewedAt') }}：{{ plugin.reviewed_at ? formatRelativeTime(plugin.reviewed_at) : t('display.empty') }}
            </div>
            <div v-else style="margin-top: 10px;">
              <el-checkbox-group v-model="selectedRecoveryReviewIds">
                <el-checkbox :value="plugin.review_id" :data-testid="`recovery-confirm-checkbox-${plugin.review_id}`">
                  {{ t('dashboard.recoveryConfirm') }}
                </el-checkbox>
              </el-checkbox-group>
            </div>
          </div>

          <div v-if="pendingRecoveryPlugins.length" class="issue-alert-card issue-alert-card--warning" style="margin-top: 12px;">
            <div class="issue-alert-card__header">
              <span class="issue-alert-card__summary">{{ t('dashboard.recoveryConfirmSection') }}</span>
              <small style="color: var(--muted);">{{ selectedRecoveryReviewCountLabel }}</small>
            </div>
            <el-input
              v-model="recoveryConfirmNote"
              type="textarea"
              :rows="3"
              :maxlength="500"
              show-word-limit
              :placeholder="t('dashboard.recoveryConfirmNotePlaceholder')"
            />
            <div class="table-actions" style="justify-content: flex-start; margin-top: 12px;">
              <el-button
                data-testid="recovery-confirm-button"
                size="small"
                type="primary"
                :loading="recoveryConfirmPending"
                :disabled="selectedRecoveryReviewIds.length === 0"
                @click="confirmRecoverySelection"
              >
                {{ t('dashboard.recoveryConfirm') }}
              </el-button>
            </div>
          </div>

          <div v-else-if="recoverySummary.skipped_plugins?.length" class="readiness-note">
            <small style="color: var(--muted);">{{ t('dashboard.recoveryConfirmEmpty') }}</small>
          </div>

          <div v-if="recoverySummary.manual_actions?.length" style="margin-top: 12px;">
            <small style="color: var(--muted); display: block; margin-bottom: 6px;">处理建议</small>
            <ul style="margin: 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
              <li
                v-for="action in recoverySummary.manual_actions"
                :key="action"
                data-testid="recovery-manual-action"
              >
                {{ action }}
              </li>
            </ul>
          </div>

          <div v-if="recoverySummary.next_steps?.length" style="margin-top: 12px;">
            <small style="color: var(--muted); display: block; margin-bottom: 6px;">下一步</small>
            <ul style="margin: 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
              <li
                v-for="step in recoverySummary.next_steps"
                :key="step"
                data-testid="recovery-next-step"
              >
                {{ step }}
              </li>
            </ul>
          </div>

          <div v-if="recoveryAuditEntries.length" style="margin-top: 12px;">
            <small style="color: var(--muted); display: block; margin-bottom: 6px;">{{ t('dashboard.recoveryAudit') }}</small>
            <div
              v-for="entry in recoveryAuditEntries"
              :key="`${entry.task_id}-${entry.created_at}`"
              class="issue-alert-card"
            >
              <div class="issue-alert-card__header">
                <el-tag type="info" size="small">{{ entry.operator_id }}</el-tag>
                <span class="issue-alert-card__summary">{{ formatRelativeTime(entry.created_at) }}</span>
              </div>
              <div class="issue-alert-card__remediation">{{ entry.note || t('display.empty') }}</div>
              <ul style="margin: 8px 0 0; padding-left: 18px; color: var(--muted); display: grid; gap: 6px;">
                <li v-for="item in entry.items" :key="item.review_id">
                  {{ item.plugin_id }} · {{ item.reason_code }}
                </li>
              </ul>
            </div>
          </div>
          <div v-else class="readiness-note" style="margin-top: 12px;">
            <small style="color: var(--muted);">{{ t('dashboard.recoveryAuditEmpty') }}</small>
          </div>
        </div>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('dashboard.recentEvents') }}</span>
          </div>
        </template>

        <el-empty v-if="recentEvents.length === 0" :description="t('dashboard.recentEventsEmpty')" />

        <div v-else class="events-section">
          <div
            v-for="event in recentEvents"
            :key="`${event.timestamp}-${event.summary}`"
            :class="['event-item', getEventSeverityClass(event.severity)]"
          >
            <strong>{{ event.summary }}</strong>
            <span
              class="event-item__time"
              :data-absolute="event.timestamp"
            >{{ formatRelativeTime(event.timestamp) }}</span>
          </div>
        </div>
      </el-card>
    </div>

    <el-card class="tools-panel">
      <template #header>
        <div class="card-header">
          <span>{{ t('dashboard.tools') }}</span>
        </div>
      </template>

      <div class="table-actions">
        <el-button type="primary" plain :loading="backupPending" @click="createBackup">
          {{ t('dashboard.createBackup') }}
        </el-button>
        <el-button type="primary" plain :loading="diagnosticsPending" @click="exportDiagnostics">
          {{ t('dashboard.exportDiagnostics') }}
        </el-button>
        <el-button type="primary" plain :loading="previewPending" @click="previewVisible = true">
          {{ t('dashboard.renderPreview') }}
        </el-button>
      </div>
    </el-card>

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

<style scoped lang="scss">
.readiness-note {
  margin-top: 14px;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.04);
  border: 1px solid rgba(148, 163, 184, 0.18);
  line-height: 1.5;
}
</style>
