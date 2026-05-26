<script setup lang="ts">
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
  SearchOutlined,
  AppstoreOutlined,
  UnorderedListOutlined,
  MessageOutlined,
  CopyOutlined,
  InfoCircleOutlined,
  CheckOutlined,
  WarningOutlined,
} from '@ant-design/icons-vue'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import type { SchedulerJobRunStats, SchedulerJobSummary } from '@/types/api'

const schedulerStore = useSchedulerJobsStore()
const { error, loading, sortedItems, triggeringJobId } = storeToRefs(schedulerStore)

// 交互状态
const detailVisible = ref(false)
const currentJob = ref<SchedulerJobSummary | null>(null)
const searchQuery = ref('')
const statusFilter = ref<'all' | 'success' | 'error'>('all')
const sortBy = ref<'name' | 'last_run' | 'duration'>('name')
const viewMode = ref<'table' | 'grid'>('table')

// 定时更新相对时间
const timeTick = ref(0)
let timerId: any = null

onMounted(() => {
  void loadSchedulerJobs()
  timerId = setInterval(() => {
    timeTick.value++
  }, 10000) // 每 10 秒强制刷新一次相对倒计时视图
})

onUnmounted(() => {
  if (timerId) clearInterval(timerId)
})

// 极致精修合并列定义
const tableColumns = computed(() => [
  { title: `${t('scheduler.fields.plugin')} / ${t('scheduler.fields.task')}`, key: 'plugin', dataIndex: 'plugin_name', width: 300 },
  { title: `${t('scheduler.fields.label')} / ${t('scheduler.fields.conversation')}`, key: 'label', dataIndex: 'log_label', width: 250 },
  { title: `${t('scheduler.fields.cron')} / ${t('scheduler.fields.nextRun')}`, key: 'cron', dataIndex: 'cron_expr', width: 320 },
  { title: `${t('scheduler.fields.lastRun')} / ${t('scheduler.fields.duration')}`, key: 'lastRun', dataIndex: 'last_run', width: 240 },
  { title: `${t('scheduler.fields.stats')} / ${t('scheduler.fields.lastError')}`, key: 'stats', dataIndex: 'stats', width: 280 },
  { title: t('scheduler.fields.actions'), key: 'actions', dataIndex: 'actions', width: 180, fixed: 'right' as const },
])

async function loadSchedulerJobs() {
  try {
    await schedulerStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

async function triggerJob(job: SchedulerJobSummary) {
  try {
    await schedulerStore.trigger(job.job_id)
    notifySuccess(t('scheduler.triggerAccepted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function showJobDetail(job: SchedulerJobSummary) {
  currentJob.value = job
  detailVisible.value = true
}

// 格式化耗时胶囊
function formatDurationMs(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return t('display.empty')
  }
  if (value < 1000) {
    return `${value} ms`
  }
  return `${(value / 1000).toFixed(value < 10_000 ? 1 : 0)} s`
}

function getDurationClass(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 'duration-empty'
  if (value < 1000) return 'duration-fast' // 1s 内绿色
  if (value < 5000) return 'duration-normal' // 5s 内蓝色
  return 'duration-slow' // 超过 5s 警示橙黄
}

function displayText(value?: string | null) {
  return value?.trim() || t('display.empty')
}

function conversationText(job: SchedulerJobSummary) {
  const payload = job.payload_summary
  if (payload.conversation_id) {
    return payload.conversation_id
  }
  if (payload.target_type && payload.target_id) {
    return `${payload.target_type}:${payload.target_id}`
  }
  return ''
}

// 动态色彩 Avatar 算法
function getPluginAvatarStyle(pluginName: string) {
  let hash = 0
  for (let i = 0; i < pluginName.length; i++) {
    hash = pluginName.charCodeAt(i) + ((hash << 5) - hash)
  }
  const h1 = Math.abs(hash) % 360
  const h2 = (h1 + 50) % 360
  return {
    background: `linear-gradient(135deg, hsl(${h1}, 75%, 55%) 0%, hsl(${h2}, 80%, 45%) 100%)`,
    color: '#ffffff',
    textShadow: '0 1px 2px rgba(0, 0, 0, 0.15)',
    boxShadow: 'var(--shadow-xs)',
  }
}

function getPluginInitials(pluginName: string): string {
  if (!pluginName) return 'RB'
  const cleanName = pluginName.startsWith('raylea.') ? pluginName.substring(7) : pluginName
  return cleanName.substring(0, 2).toUpperCase()
}

// 智能 Cron 中文解析
function parseCronToChinese(cron?: string): string {
  if (!cron) return '未配置'
  const parts = cron.trim().split(/\s+/)
  if (parts.length < 5) return cron

  const [min, hour, day, month, week] = parts

  if (min === '*' && hour === '*' && day === '*' && month === '*' && week === '*') {
    return '每分钟'
  }
  if (min.startsWith('*/') && hour === '*' && day === '*' && month === '*' && week === '*') {
    return `每 ${min.substring(2)} 分钟`
  }
  if (min === '0' && hour.startsWith('*/') && day === '*' && month === '*' && week === '*') {
    return `每 ${hour.substring(2)} 小时整`
  }
  if (!min.includes('*') && !min.includes('/') && !hour.includes('*') && !hour.includes('/') && day === '*' && month === '*' && week === '*') {
    return `每天 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  return cron
}

// 距离下次执行的动态相对倒计时
function getNextRunRelativeText(nextRunTime?: string) {
  if (!nextRunTime) return ''
  const next = new Date(nextRunTime).getTime()
  const now = Date.now()
  const diffMs = next - now
  if (diffMs <= 0) return '即将执行'
  const diffMin = Math.round(diffMs / 60000)
  if (diffMin < 1) {
    const diffSec = Math.round(diffMs / 1000)
    return `${diffSec > 0 ? diffSec : 1} 秒后`
  }
  if (diffMin < 60) {
    return `${diffMin} 分钟后`
  }
  const diffHour = Math.floor(diffMin / 60)
  const remainMin = diffMin % 60
  if (diffHour < 24) {
    return `${diffHour} 小时 ${remainMin} 分钟后`
  }
  const diffDay = Math.floor(diffHour / 24)
  return `${diffDay} 天后`
}

// 健康百分比及 Conic-gradient 环形算法
function getSuccessRate(stats: SchedulerJobRunStats): number {
  if (!stats.total) return 0
  return Math.round((stats.success / stats.total) * 100)
}

function successRateText(stats: SchedulerJobRunStats): string {
  if (!stats.total) return '未执行'
  return `${getSuccessRate(stats)}% 成功`
}

function getHealthRingStyle(stats: SchedulerJobRunStats) {
  const rate = getSuccessRate(stats)
  return {
    background: `conic-gradient(var(--success) 0% ${rate}%, var(--border) ${rate}% 100%)`
  }
}

// 复制报错信息到剪切板
async function copyToClipboard(text?: string) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    notifySuccess('错误信息已复制到剪切板')
  } catch {
    notifyError('复制失败，请手动选择复制')
  }
}

// 多维过滤与排序 computed 数据集
const filteredItems = computed(() => {
  const _ = timeTick.value
  let result = [...sortedItems.value]

  // 1. 搜索词检索
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    result = result.filter(
      (item) =>
        item.plugin_name.toLowerCase().includes(q) ||
        item.plugin_id.toLowerCase().includes(q) ||
        item.task_name.toLowerCase().includes(q) ||
        item.job_id.toLowerCase().includes(q) ||
        (item.log_label && item.log_label.toLowerCase().includes(q)) ||
        (item.payload_summary.content && item.payload_summary.content.toLowerCase().includes(q))
    )
  }

  // 2. 状态胶囊筛选
  if (statusFilter.value === 'success') {
    result = result.filter((item) => !item.last_error)
  } else if (statusFilter.value === 'error') {
    result = result.filter((item) => !!item.last_error)
  }

  // 3. 多维排序
  if (sortBy.value === 'name') {
    result.sort((left, right) => {
      if (left.plugin_name === right.plugin_name) {
        return left.task_name.localeCompare(right.task_name)
      }
      return left.plugin_name.localeCompare(right.plugin_name)
    })
  } else if (sortBy.value === 'last_run') {
    result.sort((left, right) => {
      const tLeft = left.last_run ? new Date(left.last_run).getTime() : 0
      const tRight = right.last_run ? new Date(right.last_run).getTime() : 0
      return tRight - tLeft
    })
  } else if (sortBy.value === 'duration') {
    result.sort((left, right) => (right.last_duration_ms || 0) - (left.last_duration_ms || 0))
  }

  return result
})
</script>

<template>
  <AppPage :title="t('scheduler.title')" full-height>
    <template #extra>
      <div class="scheduler-toolbar">
        <a-button :loading="loading" :aria-label="t('scheduler.refresh')" class="refresh-btn" @click="loadSchedulerJobs">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ t('scheduler.refresh') }}
        </a-button>
      </div>
    </template>

    <div class="scheduler-page-container">
      <!-- 高级交互搜索过滤栏 -->
      <div class="scheduler-filter-card">
      <div class="filter-left">
        <a-input
          v-model:value="searchQuery"
          placeholder="搜索插件、任务或自定义内容..."
          allow-clear
          class="filter-search-input"
        >
          <template #prefix>
            <SearchOutlined class="search-icon" />
          </template>
        </a-input>

        <a-segmented
          v-model:value="statusFilter"
          :options="[
            { label: '全部', value: 'all' },
            { label: '正常运行', value: 'success' },
            { label: '异常警告', value: 'error' },
          ]"
          class="filter-segmented"
        />
      </div>

      <div class="filter-right">
        <div class="sort-wrapper">
          <span class="sort-label">排序方式:</span>
          <a-select v-model:value="sortBy" class="sort-select" :bordered="false">
            <a-select-option value="name">按任务字母排序</a-select-option>
            <a-select-option value="last_run">按最近执行时间</a-select-option>
            <a-select-option value="duration">按执行耗时排序</a-select-option>
          </a-select>
        </div>

        <a-segmented
          v-model:value="viewMode"
          :options="[
            { value: 'table', payload: 'table' },
            { value: 'grid', payload: 'grid' },
          ]"
          class="view-mode-segmented"
        >
          <template #label="{ value: val }">
            <template v-if="val === 'table'">
              <UnorderedListOutlined />
            </template>
            <template v-else-if="val === 'grid'">
              <AppstoreOutlined />
            </template>
          </template>
        </a-segmented>
      </div>
    </div>

    <div class="scheduler-content-stage">
      <!-- 异常状态加载及重试面板 -->
      <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadSchedulerJobs"
    />

    <a-card v-else-if="loading && sortedItems.length === 0" class="scheduler-loading-card" :bordered="false">
      <a-skeleton active :paragraph="{ rows: 6 }" />
    </a-card>

    <AppEmptyState
      v-else-if="filteredItems.length === 0"
      icon="box"
      :title="t('scheduler.empty.title')"
      :description="searchQuery ? '未找到符合筛选条件的定时任务' : t('scheduler.empty.description')"
    />

    <!-- 双视图之：表格式 (Table View) 极致精修 -->
    <div v-else-if="viewMode === 'table'" class="table-container-wrapper">
      <a-table
        class="scheduler-data-table app-data-table refactored-table"
        :columns="tableColumns"
        :data-source="filteredItems"
        :pagination="false"
        :row-key="(row) => row.job_id"
        :scroll="{ x: 1570 }"
        :row-class-name="(record) => (record.last_error ? 'row-item row-error' : 'row-item row-success')"
      >
        <template #emptyText>
          {{ t('display.empty') }}
        </template>

        <template #bodyCell="{ column, record }">
          <!-- 1. 插件与任务合并列 -->
          <template v-if="column.key === 'plugin'">
            <div class="scheduler-cell-plugin-task">
              <div class="plugin-avatar" :style="getPluginAvatarStyle(record.plugin_name)">
                {{ getPluginInitials(record.plugin_name) }}
              </div>
              <div class="meta-content">
                <div class="top-row">
                  <strong class="plugin-name">{{ record.plugin_name }}</strong>
                  <span class="task-tag">{{ record.task_name }}</span>
                </div>
                <div class="bottom-row">
                  <span class="plugin-id" title="插件 ID">{{ record.plugin_id }}</span>
                  <span class="divider">/</span>
                  <span class="job-id" title="任务 ID">{{ record.job_id }}</span>
                </div>
              </div>
            </div>
          </template>

          <!-- 2. 自定义内容与会话 ID 合并列 -->
          <template v-else-if="column.key === 'label'">
            <div class="scheduler-cell-label-conv">
              <div class="label-text" :title="record.log_label || record.payload_summary.content">
                {{ displayText(record.log_label || record.payload_summary.content) }}
              </div>
              <div class="conv-tag-row">
                <template v-if="conversationText(record)">
                  <span class="conv-badge">
                    <MessageOutlined class="badge-icon" />
                    <span class="badge-text">{{ conversationText(record) }}</span>
                  </span>
                </template>
                <template v-else>
                  <span class="conv-badge global">全局会话</span>
                </template>
              </div>
            </div>
          </template>

          <!-- 3. 定时计划与下一次执行列 -->
          <template v-else-if="column.key === 'cron'">
            <div class="scheduler-cell-cron-next">
              <div class="cron-expr-row" :title="`表达式: ${record.cron_expr} (${record.timezone})`">
                <span class="chinese-cron">{{ parseCronToChinese(record.cron_expr) }}</span>
                <span class="raw-cron">{{ record.cron_expr }}</span>
              </div>
              <div class="next-run-row">
                <ClockCircleOutlined class="clock-icon" />
                <span class="next-time" :title="formatDateTime(record.next_run)">
                  {{ formatDateTime(record.next_run) }}
                </span>
                <span class="relative-time-pill" v-if="record.next_run">
                  {{ getNextRunRelativeText(record.next_run) }}
                </span>
              </div>
            </div>
          </template>

          <!-- 4. 最近执行与耗时列 -->
          <template v-else-if="column.key === 'lastRun'">
            <div class="scheduler-cell-run-duration">
              <div class="last-run-time">
                {{ record.last_run ? formatDateTime(record.last_run) : '尚未执行' }}
              </div>
              <div class="duration-row" v-if="record.last_run">
                <span class="duration-badge" :class="getDurationClass(record.last_duration_ms)">
                  {{ formatDurationMs(record.last_duration_ms) }}
                </span>
              </div>
            </div>
          </template>

          <!-- 5. 执行情况与最近错误列 -->
          <template v-else-if="column.key === 'stats'">
            <div class="scheduler-cell-health-stats">
              <div class="stats-header">
                <span class="total-count">{{ t('scheduler.stats.total', { count: record.stats.total }) }}</span>
                <span class="success-rate-pct">{{ successRateText(record.stats) }}</span>
              </div>

              <!-- 微型堆叠条 (Mini Stacked Bar) -->
              <div class="mini-stacked-bar" v-if="record.stats.total > 0">
                <div
                  class="bar-success"
                  :style="{ width: `${(record.stats.success / record.stats.total) * 100}%` }"
                  :title="`成功: ${record.stats.success}次`"
                ></div>
                <div
                  class="bar-failed"
                  :style="{ width: `${(record.stats.failed / record.stats.total) * 100}%` }"
                  :title="`失败: ${record.stats.failed}次`"
                ></div>
                <div
                  class="bar-other"
                  :style="{ width: `${((record.stats.total - record.stats.success - record.stats.failed) / record.stats.total) * 100}%` }"
                  :title="`其他: ${record.stats.total - record.stats.success - record.stats.failed}次`"
                ></div>
              </div>

              <!-- 错误气泡 -->
              <div class="error-badge-row" v-if="record.last_error">
                <a-popover placement="left" trigger="hover" overlay-class-name="scheduler-error-popover">
                  <template #content>
                    <div class="error-popover-content">
                      <div class="err-title">
                        <WarningOutlined class="err-icon" />
                        <strong>{{ record.last_error.code }}</strong>
                      </div>
                      <div class="err-msg">{{ record.last_error.message }}</div>
                      <a-button size="small" type="link" class="copy-err-btn" @click="copyToClipboard(`${record.last_error.code}: ${record.last_error.message}`)">
                        <template #icon><CopyOutlined /></template>
                        复制错误信息
                      </a-button>
                    </div>
                  </template>
                  <span class="error-capsule">
                    {{ record.last_error.code }}
                  </span>
                </a-popover>
              </div>
              <div class="success-dot-row" v-else-if="record.stats.total > 0">
                <span class="success-dot"><CheckOutlined class="ok-icon" /> 正常运行</span>
              </div>
            </div>
          </template>

          <!-- 6. 操作列 -->
          <template v-else-if="column.key === 'actions'">
            <div class="scheduler-actions">
              <a-button size="small" class="action-btn view-btn" @click="showJobDetail(record)">
                <template #icon>
                  <EyeOutlined />
                </template>
                {{ t('scheduler.view') }}
              </a-button>
              <a-button
                size="small"
                class="action-btn trigger-btn"
                :loading="triggeringJobId === record.job_id"
                @click="triggerJob(record)"
              >
                <template #icon>
                  <ThunderboltOutlined />
                </template>
                {{ t('scheduler.trigger') }}
              </a-button>
            </div>
          </template>
        </template>
      </a-table>
    </div>

    <!-- 双视图之：卡片看板 (Grid View) -->
    <div v-else-if="viewMode === 'grid'" class="scheduler-grid-container">
      <div
        v-for="job in filteredItems"
        :key="job.job_id"
        class="scheduler-card"
        :class="{ 'card-has-error': job.last_error }"
      >
        <div class="card-header">
          <div class="plugin-avatar" :style="getPluginAvatarStyle(job.plugin_name)">
            {{ getPluginInitials(job.plugin_name) }}
          </div>
          <div class="card-meta">
            <div class="plugin-info">
              <span class="p-name">{{ job.plugin_name }}</span>
              <span class="p-id">{{ job.plugin_id }}</span>
            </div>
            <div class="task-info">
              <span class="t-tag">{{ job.task_name }}</span>
              <span class="t-id">{{ job.job_id }}</span>
            </div>
          </div>
        </div>

        <div class="card-body">
          <div class="card-label-section">
            <div class="section-title">自定义内容 / 会话</div>
            <div class="label-desc">{{ displayText(job.log_label || job.payload_summary.content) }}</div>
            <span class="conv-badge" v-if="conversationText(job)">
              <MessageOutlined class="badge-icon" />
              <span>{{ conversationText(job) }}</span>
            </span>
            <span class="conv-badge global" v-else>全局会话</span>
          </div>

          <div class="card-cron-section">
            <div class="section-title">定时计划</div>
            <div class="cron-text-row">
              <span class="cron-c-text">{{ parseCronToChinese(job.cron_expr) }}</span>
              <span class="cron-expr-tag">{{ job.cron_expr }}</span>
            </div>
            <div class="next-time-row" v-if="job.next_run">
              <ClockCircleOutlined />
              <span class="next-val">{{ formatDateTime(job.next_run) }}</span>
              <span class="next-rel-pill">{{ getNextRunRelativeText(job.next_run) }}</span>
            </div>
          </div>

          <div class="card-stats-section">
            <div class="stats-summary">
              <span>已执行: <strong>{{ job.stats.total }}</strong>次</span>
              <span>成功率: <strong class="pct">{{ job.stats.total ? `${getSuccessRate(job.stats)}%` : '未执行' }}</strong></span>
            </div>
            <div class="mini-stacked-bar">
              <div class="bar-success" :style="{ width: `${(job.stats.success / (job.stats.total || 1)) * 100}%` }"></div>
              <div class="bar-failed" :style="{ width: `${(job.stats.failed / (job.stats.total || 1)) * 100}%` }"></div>
              <div class="bar-other" :style="{ width: `${((job.stats.total - job.stats.success - job.stats.failed) / (job.stats.total || 1)) * 100}%` }"></div>
            </div>
          </div>

          <!-- 看板错误通知 -->
          <div class="card-error-alert" v-if="job.last_error">
            <div class="err-head">
              <WarningOutlined />
              <span>{{ job.last_error.code }}</span>
            </div>
            <p class="err-msg" :title="job.last_error.message">{{ job.last_error.message }}</p>
          </div>
        </div>

        <div class="card-actions">
          <a-button class="card-action-btn view" @click="showJobDetail(job)">
            <template #icon><EyeOutlined /></template>
            查看详情
          </a-button>
          <a-button
            class="card-action-btn trigger"
            :loading="triggeringJobId === job.job_id"
            @click="triggerJob(job)"
          >
            <template #icon><ThunderboltOutlined /></template>
            立即执行
          </a-button>
        </div>
      </div>
    </div>
  </div> <!-- .scheduler-content-stage 闭合 -->
</div> <!-- .scheduler-page-container 闭合 -->

    <!-- 极客控制台详情弹窗 (Console Details Modal) -->
    <a-modal
      v-model:open="detailVisible"
      :footer="null"
      width="750px"
      wrap-class-name="scheduler-detail-modal-wrapper"
      :destroy-on-close="true"
    >
      <template #title>
        <div class="modal-header-title">
          <div class="pulse-dot"></div>
          <span>定时任务详情控制台</span>
        </div>
      </template>

      <div class="modal-console-layout" v-if="currentJob">
        <!-- 左侧：系统参数面板 -->
        <div class="console-pane-left">
          <div class="pane-group">
            <div class="pane-group-title">标识与归属</div>
            <div class="info-block">
              <span class="label">插件模块</span>
              <span class="value bold">{{ currentJob.plugin_name }}<span class="sr-only"> / </span></span>
              <span class="sub-val">{{ currentJob.plugin_id }}</span>
            </div>
            <div class="info-block">
              <span class="label">任务名称</span>
              <span class="value bold">{{ currentJob.task_name }}<span class="sr-only"> / </span></span>
              <span class="sub-val">{{ currentJob.job_id }}</span>
            </div>
          </div>

          <div class="pane-group">
            <div class="pane-group-title">执行调度配置</div>
            <div class="info-block inline">
              <div>
                <span class="label">Cron 规则</span>
                <span class="value code">{{ currentJob.cron_expr }}</span>
              </div>
              <div>
                <span class="label">时区</span>
                <span class="value code">{{ currentJob.timezone }}</span>
              </div>
            </div>
            <div class="info-block">
              <span class="label">智能中文语义</span>
              <span class="value highlight">{{ parseCronToChinese(currentJob.cron_expr) }}</span>
            </div>
          </div>

          <div class="pane-group">
            <div class="pane-group-title">上下文载荷</div>
            <div class="info-block inline">
              <div>
                <span class="label">会话 ID</span>
                <span class="value code">{{ conversationText(currentJob) || 'N/A (全局任务)' }}</span>
              </div>
            </div>
            <div class="info-block">
              <span class="label">内容标识</span>
              <span class="value">{{ displayText(currentJob.log_label || currentJob.payload_summary.content) }}</span>
            </div>
          </div>
        </div>

        <!-- 右侧：健康运行分析仪 -->
        <div class="console-pane-right">
          <div class="pane-group-title">运行状态分析</div>

          <div class="health-instrument">
            <!-- 仪表圆环 -->
            <div class="health-gauge" :style="getHealthRingStyle(currentJob.stats)">
              <div class="gauge-center">
                <span class="gauge-pct">{{ currentJob.stats.total ? `${getSuccessRate(currentJob.stats)}%` : '-' }}</span>
                <span class="gauge-desc">健康度</span>
              </div>
            </div>

            <!-- 数据列项 -->
            <div class="gauge-stats-list">
              <div class="stat-item success">
                <CheckCircleOutlined />
                <span class="lbl">成功执行</span>
                <span class="val">{{ currentJob.stats.success }} 次</span>
              </div>
              <div class="stat-item failed">
                <CloseCircleOutlined />
                <span class="lbl">失败运行</span>
                <span class="val">{{ currentJob.stats.failed }} 次</span>
              </div>
              <div class="stat-item warning">
                <ClockCircleOutlined />
                <span class="lbl">执行超时</span>
                <span class="val">{{ currentJob.stats.timeout }} 次</span>
              </div>
              <div class="stat-item other">
                <InfoCircleOutlined />
                <span class="lbl">重试次数</span>
                <span class="val">{{ currentJob.stats.retry }} 次</span>
              </div>
            </div>
          </div>

          <div class="pane-group margin-top">
            <div class="pane-group-title">时间与性能</div>
            <div class="info-block inline">
              <div>
                <span class="label">上一次执行</span>
                <span class="value small-text">{{ currentJob.last_run ? formatDateTime(currentJob.last_run) : '未跑' }}</span>
              </div>
              <div>
                <span class="label">单次耗时</span>
                <span class="value highlight">{{ formatDurationMs(currentJob.last_duration_ms) }}</span>
              </div>
            </div>
            <div class="info-block">
              <span class="label">下一次预计调度</span>
              <span class="value small-text">
                {{ currentJob.next_run ? formatDateTime(currentJob.next_run) : '未排期' }}
                <span class="rel-time" v-if="currentJob.next_run">({{ getNextRunRelativeText(currentJob.next_run) }})</span>
              </span>
            </div>
          </div>

          <!-- 最近运行错误报告区 -->
          <div class="console-error-report" v-if="currentJob.last_error">
            <div class="report-head">
              <WarningOutlined />
              <span>运行故障报告</span>
            </div>
            <div class="report-body">
              <div class="err-code">错误代码: <code>{{ currentJob.last_error.code }}</code></div>
              <p class="err-msg">{{ currentJob.last_error.message }}</p>
              <a-button size="small" type="primary" danger class="copy-console-err-btn" @click="copyToClipboard(`${currentJob.last_error.code}: ${currentJob.last_error.message}`)">
                <template #icon><CopyOutlined /></template>
                复制报错诊断堆栈
              </a-button>
            </div>
          </div>
        </div>
      </div>
    </a-modal>
  </AppPage>
</template>

<style lang="scss" scoped>
/* ----------------------------------------------------
 * 高阶 Flex-box 弹性布局包裹容器
 * ---------------------------------------------------- */
.scheduler-page-container {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  gap: var(--space-md);
  height: 100%;
}

.scheduler-content-stage {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
}

/* ----------------------------------------------------
 * 主题基础变量与公用混合宏
 * ---------------------------------------------------- */
.scheduler-toolbar {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.refresh-btn {
  height: 38px;
  border-radius: var(--radius-md);
  border-color: var(--border);
  background: var(--surface);
  transition: color 0.25s ease, border-color 0.25s ease, background-color 0.25s ease;

  &:hover {
    color: var(--accent) !important;
    border-color: var(--accent) !important;
    background: var(--surface-accent) !important;
  }
}

/* ----------------------------------------------------
 * 高级交互搜索过滤栏 (Glassmorphism Control Panel)
 * ---------------------------------------------------- */
.scheduler-filter-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-md);
  padding: 14px 18px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  margin-bottom: var(--space-md);
  transition: border-color 0.3s ease, box-shadow 0.3s ease;

  &:hover {
    border-color: var(--border-accent);
    box-shadow: var(--shadow);
  }

  .filter-left,
  .filter-right {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    flex-wrap: wrap;
  }

  .filter-search-input {
    width: 260px;
    height: 36px;
    border-radius: var(--radius-md);
    border-color: var(--border);
    transition: border-color 0.25s ease;

    &:hover, &:focus {
      border-color: var(--accent);
    }

    .search-icon {
      color: var(--muted);
    }
  }

  .filter-segmented,
  .view-mode-segmented {
    background: color-mix(in srgb, var(--text) 5%, transparent);
    padding: 2px;
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
  }

  .sort-wrapper {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
    background: color-mix(in srgb, var(--text) 3%, transparent);
    padding-inline: 10px;
    border-radius: var(--radius-md);
    border: 1px solid var(--border);
    height: 36px;

    .sort-label {
      font-size: 12px;
      color: var(--muted);
      white-space: nowrap;
    }

    .sort-select {
      width: 140px;
      font-size: 13px;
      font-weight: 500;
      color: var(--text);
    }
  }
}

.scheduler-loading-card {
  border-radius: var(--radius-lg);
  background: var(--surface);
  border: 1px solid var(--border);
  padding: 24px;
}

/* ----------------------------------------------------
 * 双视图之：表格式 (Table View) 极致精修
 * ---------------------------------------------------- */
.table-container-wrapper {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: color-mix(in srgb, var(--surface) 90%, transparent);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  transition: box-shadow 0.3s ease;

  &:hover {
    box-shadow: var(--shadow);
  }
}

.refactored-table {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;

  :deep(.ant-spin-nested-loading),
  :deep(.ant-spin-container),
  :deep(.ant-table) {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    min-height: 0;
    background: transparent !important;
  }

  :deep(.ant-table-container) {
    display: flex;
    flex: 1 1 auto;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  :deep(.ant-table-content) {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto !important;
  }

  /* 极致清爽边框线 */
  :deep(.ant-table-thead > tr > th) {
    background: color-mix(in srgb, var(--text) 3%, var(--surface)) !important;
    border-bottom: 2px solid var(--border) !important;
    color: var(--text) !important;
    font-weight: 600 !important;
    font-size: 13px;
    padding: 12px 16px;
  }

  :deep(.ant-table-tbody > tr:not(.ant-table-measure-row) > td) {
    border-bottom: 1px solid var(--border) !important;
    padding: 14px 16px !important;
    background: transparent !important;
    transition: background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }

  /* 表格行 Hover 左侧指示条与底色 */
  :deep(.ant-table-tbody > tr:not(.ant-table-measure-row)) {
    position: relative;
    transition: background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1);

    &:hover {
      background-color: var(--surface-accent) !important;

      td {
        background-color: var(--surface-accent) !important;
      }
    }
  }

  :deep(.ant-table-tbody > tr:not(.ant-table-measure-row) > td:first-child) {
    position: relative;

    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: 4px;
      background: transparent;
      transform: scaleY(0);
      transform-origin: center;
      transition: transform 0.22s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.22s ease;
      z-index: 10;
    }
  }

  :deep(.ant-table-tbody > tr:not(.ant-table-measure-row):hover > td:first-child::before) {
    transform: scaleY(1);
  }

  :deep(.ant-table-tbody > tr.row-success > td:first-child::before) {
      background: var(--success);
  }

  :deep(.ant-table-tbody > tr.row-error > td:first-child::before) {
      background: var(--danger);
  }
}

/* 单元格排版设计 */
.scheduler-cell-plugin-task {
  display: flex;
  align-items: center;
  gap: 12px;
  white-space: nowrap;

  .plugin-avatar {
    width: 36px;
    height: 36px;
    border-radius: var(--radius-md);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 700;
    flex-shrink: 0;
    user-select: none;
  }

  .meta-content {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .top-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;

    .plugin-name {
      font-size: 14px;
      color: var(--text);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 600;
    }

    .task-tag {
      font-size: 11px;
      background: color-mix(in srgb, var(--accent) 8%, transparent);
      color: var(--accent);
      border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent);
      padding: 1px 6px;
      border-radius: 4px;
      font-weight: 500;
      white-space: nowrap;
    }
  }

  .bottom-row {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: var(--muted);
    font-family: var(--font-mono);

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .divider {
      color: color-mix(in srgb, var(--border) 60%, transparent);
    }
  }
}

.scheduler-cell-label-conv {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  white-space: nowrap;

  .label-text {
    font-size: 13px;
    color: var(--text);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .conv-tag-row {
    display: flex;
    align-items: center;
  }

  .conv-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    background: color-mix(in srgb, var(--text) 5%, transparent);
    border: 1px solid var(--border);
    color: var(--muted);
    padding: 1px 6px;
    border-radius: 6px;
    max-width: 160px;
    font-family: var(--font-mono);

    .badge-icon {
      font-size: 10px;
    }
    .badge-text {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &.global {
      background: color-mix(in srgb, var(--success) 6%, transparent);
      border-color: color-mix(in srgb, var(--success) 18%, transparent);
      color: var(--success);
      font-weight: 500;
    }
  }
}

.scheduler-cell-cron-next {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
  white-space: nowrap;

  .cron-expr-row {
    display: flex;
    align-items: center;
    gap: 8px;
    white-space: nowrap;
    flex-shrink: 0;

    .chinese-cron {
      font-size: 13px;
      font-weight: 600;
      color: var(--text);
    }

    .raw-cron {
      font-size: 11px;
      font-family: var(--font-mono);
      color: var(--muted);
      background: color-mix(in srgb, var(--text) 4%, transparent);
      padding: 0 4px;
      border-radius: 3px;
    }
  }

  .next-run-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);

    .clock-icon {
      font-size: 11px;
    }

    .next-time {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .relative-time-pill {
      font-size: 10px;
      background: var(--accent-soft);
      color: var(--accent);
      padding: 0px 5px;
      border-radius: 4px;
      font-weight: 600;
      white-space: nowrap;
    }
  }
}

.scheduler-cell-run-duration {
  display: flex;
  flex-direction: column;
  gap: 6px;
  white-space: nowrap;

  .last-run-time {
    font-size: 12px;
    color: var(--text);
  }

  .duration-row {
    display: flex;
  }

  .duration-badge {
    font-size: 11px;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: 5px;
    font-family: var(--font-mono);
    border: 1px solid transparent;

    &.duration-fast {
      background: color-mix(in srgb, var(--success) 8%, transparent);
      border-color: color-mix(in srgb, var(--success) 20%, transparent);
      color: var(--success);
    }

    &.duration-normal {
      background: color-mix(in srgb, var(--accent) 8%, transparent);
      border-color: color-mix(in srgb, var(--accent) 20%, transparent);
      color: var(--accent);
    }

    &.duration-slow {
      background: color-mix(in srgb, var(--warning) 8%, transparent);
      border-color: color-mix(in srgb, var(--warning) 20%, transparent);
      color: var(--warning);
    }
  }
}

.scheduler-cell-health-stats {
  display: flex;
  flex-direction: column;
  gap: 5px;
  width: 100%;
  white-space: nowrap;

  .stats-header {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--muted);
    font-weight: 500;
    white-space: nowrap;
    padding-right: 0;
  }

  /* 微型堆叠条 (Mini Stacked Bar) */
  .mini-stacked-bar {
    display: flex;
    height: 6px;
    width: 140px;
    background: color-mix(in srgb, var(--text) 8%, transparent);
    border-radius: 3px;
    overflow: hidden;

    .bar-success {
      height: 100%;
      background: var(--success);
      transition: width 0.3s ease;
    }

    .bar-failed {
      height: 100%;
      background: var(--danger);
      transition: width 0.3s ease;
    }

    .bar-other {
      height: 100%;
      background: var(--warning);
      transition: width 0.3s ease;
    }
  }

  .error-badge-row {
    display: flex;
    margin-top: 2px;
  }

  .error-capsule {
    font-size: 10px;
    font-weight: 700;
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 24%, transparent);
    color: var(--danger);
    padding: 1px 6px;
    border-radius: 4px;
    cursor: pointer;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.02em;
    transition: background-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;

    &:hover {
      background: var(--danger);
      color: #ffffff;
      box-shadow: 0 2px 6px color-mix(in srgb, var(--danger) 30%, transparent);
    }
  }

  .success-dot-row {
    display: flex;
    align-items: center;
    margin-top: 2px;
  }

  .success-dot {
    font-size: 11px;
    color: var(--success);
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 4px;

    .ok-icon {
      font-size: 10px;
    }
  }
}

/* 气泡故障面板样式 */
.error-popover-content {
  max-width: 280px;
  padding: 4px;

  .err-title {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--danger);
    font-size: 13px;
    margin-bottom: 6px;

    .err-icon {
      font-size: 14px;
    }
  }

  .err-msg {
    font-size: 12px;
    color: var(--text);
    background: color-mix(in srgb, var(--text) 4%, transparent);
    padding: 6px 8px;
    border-radius: 6px;
    word-break: break-all;
    font-family: var(--font-mono);
    max-height: 120px;
    overflow-y: auto;
  }

  .copy-err-btn {
    padding: 0;
    height: auto;
    font-size: 11px;
    margin-top: 8px;
  }
}

.scheduler-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  .action-btn {
    height: 28px;
    font-size: 12px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
    background: var(--surface);
    transition: transform 0.22s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.22s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.22s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.22s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.22s cubic-bezier(0.25, 0.8, 0.25, 1);

    &:hover {
      transform: translateY(-1px);
      box-shadow: var(--shadow-xs);
    }

    &.view-btn:hover {
      color: var(--accent) !important;
      border-color: var(--accent) !important;
      background: var(--surface-accent) !important;
    }

    &.trigger-btn:hover {
      color: var(--success) !important;
      border-color: var(--success) !important;
      background: var(--surface-success) !important;
    }
  }
}

/* ----------------------------------------------------
 * 双视图之：卡片看板 (Grid View)
 * ---------------------------------------------------- */
.scheduler-grid-container {
  flex: 1 1 auto;
  overflow-y: auto;
  min-height: 0;
  padding: 4px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
  align-content: start;
}

.scheduler-card {
  display: flex;
  flex-direction: column;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    transform: translateY(-4px);
    border-color: var(--border-accent);
    box-shadow: var(--shadow-lg);
  }

  /* 异常行脉冲呼吸灯阴影 */
  &.card-has-error {
    border-color: color-mix(in srgb, var(--danger) 30%, var(--border));

    &:hover {
      border-color: var(--danger);
      box-shadow: 0 8px 24px color-mix(in srgb, var(--danger) 12%, transparent);
    }
  }

  .card-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--text) 2%, transparent);

    .plugin-avatar {
      width: 42px;
      height: 42px;
      border-radius: var(--radius-md);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 15px;
      font-weight: 800;
      flex-shrink: 0;
    }

    .card-meta {
      display: flex;
      flex-direction: column;
      gap: 3px;
      min-width: 0;
    }

    .plugin-info {
      display: flex;
      align-items: baseline;
      gap: 6px;
      min-width: 0;

      .p-name {
        font-weight: 700;
        color: var(--text);
        font-size: 14px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .p-id {
        font-size: 10px;
        color: var(--muted);
        font-family: var(--font-mono);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .task-info {
      display: flex;
      align-items: center;
      gap: 6px;
      min-width: 0;

      .t-tag {
        font-size: 11px;
        background: var(--accent-soft);
        color: var(--accent);
        border: 1px solid color-mix(in srgb, var(--accent) 18%, transparent);
        padding: 0 5px;
        border-radius: 4px;
        font-weight: 600;
      }
      .t-id {
        font-size: 10px;
        color: var(--muted);
        font-family: var(--font-mono);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }
  }

  .card-body {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    flex: 1 1 auto;

    .section-title {
      font-size: 11px;
      color: var(--muted);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.02em;
      margin-bottom: 4px;
    }

    .label-desc {
      font-size: 14px;
      font-weight: 600;
      color: var(--text);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .conv-badge {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 11px;
      background: color-mix(in srgb, var(--text) 5%, transparent);
      border: 1px solid var(--border);
      color: var(--muted);
      padding: 1px 6px;
      border-radius: 6px;
      margin-top: 4px;
      width: fit-content;
      font-family: var(--font-mono);

      &.global {
        background: color-mix(in srgb, var(--success) 6%, transparent);
        border-color: color-mix(in srgb, var(--success) 18%, transparent);
        color: var(--success);
        font-weight: 500;
      }
    }

    .cron-text-row {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 3px;

      .cron-c-text {
        font-size: 14px;
        font-weight: 700;
        color: var(--text);
      }
      .cron-expr-tag {
        font-size: 11px;
        font-family: var(--font-mono);
        color: var(--muted);
        background: color-mix(in srgb, var(--text) 4%, transparent);
        padding: 0 4px;
        border-radius: 3px;
      }
    }

    .next-time-row {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 12px;
      color: var(--muted);

      .next-rel-pill {
        background: var(--accent-soft);
        color: var(--accent);
        font-size: 10px;
        font-weight: 700;
        padding: 0 5px;
        border-radius: 4px;
      }
    }

    .stats-summary {
      display: flex;
      justify-content: space-between;
      font-size: 12px;
      color: var(--muted);
      margin-bottom: 5px;

      strong {
        color: var(--text);
      }
      .pct {
        color: var(--success);
      }
    }

    .mini-stacked-bar {
      display: flex;
      height: 6px;
      width: 100%;
      background: color-mix(in srgb, var(--text) 8%, transparent);
      border-radius: 3px;
      overflow: hidden;

      .bar-success { height: 100%; background: var(--success); }
      .bar-failed { height: 100%; background: var(--danger); }
      .bar-other { height: 100%; background: var(--warning); }
    }

    .card-error-alert {
      background: color-mix(in srgb, var(--danger) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--danger) 18%, transparent);
      border-radius: 8px;
      padding: 8px 12px;
      display: flex;
      flex-direction: column;
      gap: 4px;

      .err-head {
        display: flex;
        align-items: center;
        gap: 6px;
        font-weight: 700;
        font-size: 12px;
        color: var(--danger);
      }

      .err-msg {
        font-size: 11px;
        color: var(--text);
        margin: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        font-family: var(--font-mono);
      }
    }
  }

  .card-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    background: color-mix(in srgb, var(--text) 2%, transparent);

    .card-action-btn {
      height: 32px;
      font-size: 12px;
      border-radius: var(--radius-md);
      transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;

      &.view {
        border-color: var(--border);
        background: var(--surface);

        &:hover {
          color: var(--accent) !important;
          border-color: var(--accent) !important;
          background: var(--surface-accent) !important;
        }
      }

      &.trigger {
        background: var(--success);
        color: #ffffff;
        border: none;

        &:hover {
          background: color-mix(in srgb, var(--success) 85%, #ffffff) !important;
          box-shadow: 0 4px 10px rgba(63, 190, 115, 0.25);
        }
      }
    }
  }
}

/* ----------------------------------------------------
 * 科技感控制台详情弹窗 (Console Details Dashboard)
 * ---------------------------------------------------- */
:deep(.scheduler-detail-modal-wrapper) {
  .ant-modal-content {
    background: var(--surface) !important;
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-floating);
    padding: 20px 24px;
  }

  .ant-modal-header {
    background: transparent !important;
    border-bottom: 1px solid var(--border);
    padding-bottom: 12px;
    margin-bottom: 16px;
  }

  .ant-modal-close {
    color: var(--muted);
    transition: color 0.2s ease, background-color 0.2s ease;
    &:hover {
      color: var(--text);
      background: color-mix(in srgb, var(--text) 5%, transparent);
    }
  }
}

.modal-header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 700;
  color: var(--text);

  .pulse-dot {
    width: 8px;
    height: 8px;
    background: var(--success);
    border-radius: 50%;
    box-shadow: 0 0 0 0 rgba(63, 190, 115, 0.4);
    animation: pulse-green 2s infinite;
  }
}

@keyframes pulse-green {
  0% { box-shadow: 0 0 0 0 rgba(63, 190, 115, 0.4); }
  70% { box-shadow: 0 0 0 6px rgba(63, 190, 115, 0); }
  100% { box-shadow: 0 0 0 0 rgba(63, 190, 115, 0); }
}

.modal-console-layout {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 20px;
}

.console-pane-left {
  border-right: 1px solid var(--border);
  padding-right: 20px;
}

.console-pane-left,
.console-pane-right {
  display: flex;
  flex-direction: column;
  gap: 16px;

  .pane-group-title {
    font-size: 11px;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 8px;
    border-left: 2px solid var(--accent);
    padding-left: 6px;
  }

  .pane-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .info-block {
    display: flex;
    flex-direction: column;
    background: color-mix(in srgb, var(--text) 3%, transparent);
    padding: 8px 12px;
    border-radius: 8px;
    border: 1px solid var(--border);

    .label {
      font-size: 11px;
      color: var(--muted);
      margin-bottom: 3px;
    }

    .value {
      font-size: 13px;
      font-weight: 600;
      color: var(--text);

      &.bold {
        font-weight: 700;
        font-size: 14px;
      }
      &.code {
        font-family: var(--font-mono);
        color: var(--text);
        background: color-mix(in srgb, var(--text) 5%, transparent);
        padding-inline: 4px;
        border-radius: 3px;
        font-size: 12px;
        width: fit-content;
      }
      &.highlight {
        color: var(--accent);
        font-weight: 700;
      }
    }

    .sub-val {
      font-size: 11px;
      color: var(--muted);
      font-family: var(--font-mono);
      margin-top: 2px;
    }

    &.inline {
      flex-direction: row;
      justify-content: space-between;
      gap: 12px;

      & > div {
        display: flex;
        flex-direction: column;
        flex: 1 1 0%;
      }
    }
  }
}

.console-pane-right {
  .margin-top {
    margin-top: 4px;
  }

  .small-text {
    font-size: 12px !important;
  }

  .rel-time {
    color: var(--accent);
    font-weight: 600;
    margin-left: 4px;
  }

  /* 运行健康仪表盘 */
  .health-instrument {
    display: flex;
    align-items: center;
    gap: 16px;
    background: color-mix(in srgb, var(--text) 3%, transparent);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px;
  }

  .health-gauge {
    width: 90px;
    height: 90px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 8px; // 圆环粗细
    flex-shrink: 0;

    .gauge-center {
      width: 100%;
      height: 100%;
      border-radius: 50%;
      background: var(--surface) !important;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      box-shadow: var(--shadow-sm);
    }

    .gauge-pct {
      font-size: 18px;
      font-weight: 800;
      color: var(--text);
      line-height: 1;
    }

    .gauge-desc {
      font-size: 10px;
      color: var(--muted);
      margin-top: 2px;
    }
  }

  .gauge-stats-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1 1 auto;

    .stat-item {
      display: flex;
      align-items: center;
      font-size: 12px;
      font-weight: 500;

      span.lbl {
        margin-left: 6px;
        color: var(--muted);
      }

      span.val {
        margin-left: auto;
        font-family: var(--font-mono);
        font-weight: 700;
      }

      &.success { color: var(--success); }
      &.failed { color: var(--danger); }
      &.warning { color: var(--warning); }
      &.other { color: var(--muted); }
    }
  }

  /* 科技感故障诊断堆栈报告 */
  .console-error-report {
    background: color-mix(in srgb, var(--danger) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
    border-radius: 10px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .report-head {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
      font-weight: 700;
      color: var(--danger);
    }

    .report-body {
      display: flex;
      flex-direction: column;
      gap: 6px;

      .err-code {
        font-size: 12px;
        color: var(--text);

        code {
          font-family: var(--font-mono);
          background: color-mix(in srgb, var(--danger) 15%, transparent);
          padding: 1px 5px;
          border-radius: 4px;
          color: var(--danger);
          font-weight: 700;
        }
      }

      .err-msg {
        font-size: 11px;
        color: var(--text);
        font-family: var(--font-mono);
        background: color-mix(in srgb, var(--surface) 60%, transparent);
        padding: 8px;
        border-radius: 6px;
        max-height: 80px;
        overflow-y: auto;
        word-break: break-all;
        margin: 0;
        border: 1px solid color-mix(in srgb, var(--danger) 10%, transparent);
      }

      .copy-console-err-btn {
        margin-top: 4px;
        font-size: 11px;
        height: 28px;
        width: fit-content;
        align-self: flex-end;
      }
    }
  }
}
</style>
