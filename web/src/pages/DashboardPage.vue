<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import RetryPanel from '@/components/RetryPanel.vue'
import { formatDurationSeconds } from '@/lib/format'
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
    ElMessage.success('在线备份任务已接受')
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'backup failed')
  }
}

async function exportDiagnostics() {
  try {
    await systemStore.exportDiagnostics()
    ElMessage.success('诊断包导出已开始')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'diagnostics export failed')
  }
}

async function submitRenderPreview() {
  let data: Record<string, unknown>
  try {
    const parsed = JSON.parse(previewForm.dataText)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      throw new Error('render preview data must be a JSON object')
    }
    data = parsed as Record<string, unknown>
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'render preview payload is invalid')
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
    ElMessage.success('渲染预览任务已接受')
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'render preview failed')
  }
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Status</div>
        <h1>系统状态</h1>
        <p>聚合 health、ready、system status 与 `/ws/events` 的管理摘要。</p>
      </div>

      <div class="table-actions">
        <el-button :loading="loading" @click="refreshState()">
          刷新状态
        </el-button>
        <el-button type="primary" plain :loading="backupPending" @click="createBackup">
          创建在线备份
        </el-button>
        <el-button type="primary" plain :loading="diagnosticsPending" @click="exportDiagnostics">
          导出诊断包
        </el-button>
        <el-button type="primary" plain :loading="previewPending" @click="previewVisible = true">
          渲染预览
        </el-button>
      </div>
    </section>

    <RetryPanel
      v-if="error && !system"
      title="状态读取失败"
      :description="error"
      :loading="loading"
      @retry="refreshState()"
    />

    <el-alert v-else-if="error" title="状态读取失败" type="error" :description="error" show-icon />

    <div class="stats-grid">
      <el-card class="stat-card">
        <span class="stat-label">Health</span>
        <strong>{{ health?.status ?? '—' }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">Ready</span>
        <strong>{{ readiness?.status ?? '—' }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">System</span>
        <strong>{{ system?.status ?? '—' }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">Adapter</span>
        <strong>{{ system?.adapter_state ?? '—' }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">Active Plugins</span>
        <strong>{{ system?.active_plugins ?? 0 }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">Uptime</span>
        <strong>{{ formatDurationSeconds(system?.uptime_seconds) }}</strong>
      </el-card>
    </div>

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>Readiness Checks</span>
          </div>
        </template>

        <el-descriptions :column="1" border>
          <el-descriptions-item label="Reason">
            {{ readiness?.reason ?? '—' }}
          </el-descriptions-item>
          <el-descriptions-item label="Reason Codes">
            {{ readiness?.reason_codes?.join(', ') || '—' }}
          </el-descriptions-item>
          <el-descriptions-item label="Checks" v-if="!readiness?.issues?.length">
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
            <span>Recent Events</span>
          </div>
        </template>

        <el-empty v-if="recentEvents.length === 0" description="暂无 events 摘要" />

        <div v-else class="event-feed">
          <div v-for="event in recentEvents" :key="`${event.timestamp}-${event.summary}`" class="event-item">
            <strong>{{ event.summary }}</strong>
            <span>{{ event.timestamp }}</span>
          </div>
        </div>
      </el-card>
    </div>

    <el-dialog v-model="previewVisible" title="渲染预览" width="min(720px, 92vw)">
      <el-form label-position="top">
        <el-form-item label="Template">
          <el-input v-model="previewForm.template" placeholder="help.menu" />
        </el-form-item>
        <el-form-item label="Theme">
          <el-input v-model="previewForm.theme" placeholder="default" />
        </el-form-item>
        <el-form-item label="Output">
          <el-radio-group v-model="previewForm.output">
            <el-radio-button label="png" value="png" />
            <el-radio-button label="jpeg" value="jpeg" />
          </el-radio-group>
        </el-form-item>
        <el-form-item label="Data JSON">
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
            取消
          </el-button>
          <el-button type="primary" :loading="previewPending" @click="submitRenderPreview">
            开始预览
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
