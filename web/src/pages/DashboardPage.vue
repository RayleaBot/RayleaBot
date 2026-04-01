<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import RetryPanel from '@/components/RetryPanel.vue'
import {
  getAdapterStateLabel,
  getReadinessStatusLabel,
  getSystemStatusLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDurationSeconds } from '@/lib/format'
import { t } from '@/i18n'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const systemStore = useSystemStore()
const { backupPending, diagnosticsPending, error, health, loading, previewPending, readiness, recentEvents, system } = storeToRefs(systemStore)
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

async function refreshState() {
  try {
    await systemStore.refresh()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void refreshState()
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
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <h1>{{ t('dashboard.title') }}</h1>
      </div>

      <div class="table-actions">
        <el-button :loading="loading" @click="refreshState()">
          {{ t('dashboard.refresh') }}
        </el-button>
      </div>
    </section>

    <RetryPanel
      v-if="error && !system"
      :title="t('routes.status')"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <div class="stats-grid">
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.health') }}</span>
        <strong>{{ health?.status === 'ok' ? '正常' : t('display.empty') }}</strong>
        <small>{{ health?.status ?? t('display.empty') }}</small>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.readiness') }}</span>
        <strong>{{ getReadinessStatusLabel(readiness?.status) }}</strong>
        <small>{{ readiness?.status ?? t('display.empty') }}</small>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.service') }}</span>
        <strong>{{ getSystemStatusLabel(system?.status) }}</strong>
        <small>{{ system?.status ?? t('display.empty') }}</small>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">{{ t('dashboard.adapter') }}</span>
        <strong>{{ getAdapterStateLabel(system?.adapter_state) }}</strong>
        <small>{{ system?.adapter_state ?? t('display.empty') }}</small>
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

        <el-descriptions :column="1" border>
          <el-descriptions-item label="原因">
            {{ readiness?.reason ?? t('display.empty') }}
          </el-descriptions-item>
          <el-descriptions-item label="原因代码">
            {{ readiness?.reason_codes?.join(', ') || t('display.empty') }}
          </el-descriptions-item>
          <el-descriptions-item label="检查项" v-if="!readiness?.issues?.length">
            <div class="mono-list">
              <div v-for="(value, key) in readiness?.checks" :key="key">
                {{ key }} = {{ value }}
              </div>
            </div>
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="readiness?.issues?.length" class="issues-list">
          <div v-for="issue in readiness.issues" :key="issue.code" class="issue-item">
            <el-tag :type="issue.severity === 'error' ? 'danger' : issue.severity === 'warning' ? 'warning' : 'success'" size="small">{{ issue.code }}</el-tag>
            <span class="issue-summary">{{ issue.summary }}</span>
            <span v-if="issue.remediation" class="issue-remediation">{{ issue.remediation }}</span>
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

        <div v-else class="event-feed">
          <div v-for="event in recentEvents" :key="`${event.timestamp}-${event.summary}`" class="event-item">
            <strong>{{ event.summary }}</strong>
            <span>{{ event.timestamp }}</span>
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
