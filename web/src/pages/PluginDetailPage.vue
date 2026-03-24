<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import { formatDateTime } from '@/lib/format'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const socketStore = useSocketStore()

const { actionPending, current, detailLoading, grantsLoading } = storeToRefs(pluginsStore)

const pluginId = computed(() => String(route.params.id))
const consoleFrames = computed(() => pluginsStore.getConsole(pluginId.value))
const currentGrants = computed(() => pluginsStore.getGrants(pluginId.value))
const consoleSnapshot = computed(() => socketStore.snapshots.pluginConsole)
const grantBusy = computed(() => grantsLoading.value[pluginId.value] ?? false)
const loadError = ref<string | null>(null)
const operationError = ref<string | null>(null)
const grantDialogVisible = ref(false)
const uninstallDialogVisible = ref(false)
const grantForm = reactive({
  capability: '',
  expires_at: '',
})

async function loadDetail() {
  loadError.value = null
  try {
    await Promise.all([
      pluginsStore.fetchDetail(pluginId.value),
      pluginsStore.fetchGrants(pluginId.value),
    ])
    socketStore.setConsolePlugin(pluginId.value)
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : 'plugin detail load failed'
  }
}

async function runAction(action: 'enable' | 'disable' | 'reload') {
  operationError.value = null
  try {
    await pluginsStore.executeAction(pluginId.value, action)
    ElMessage.success(`${pluginId.value} ${action} accepted`)
  } catch (error) {
    operationError.value = error instanceof Error ? error.message : `${action} failed`
  }
}

async function submitGrant() {
  operationError.value = null
  try {
    await pluginsStore.grantCapability(pluginId.value, {
      capability: grantForm.capability,
      expires_at: grantForm.expires_at || undefined,
    })
    grantDialogVisible.value = false
    grantForm.capability = ''
    grantForm.expires_at = ''
    ElMessage.success('授权已保存')
  } catch (error) {
    operationError.value = error instanceof Error ? error.message : 'grant save failed'
  }
}

async function revokeGrant(capability: string) {
  operationError.value = null
  try {
    await pluginsStore.revokeGrant(pluginId.value, capability)
    ElMessage.success('授权已撤销')
  } catch (error) {
    operationError.value = error instanceof Error ? error.message : 'grant revoke failed'
  }
}

async function uninstallPlugin() {
  operationError.value = null
  try {
    const response = await pluginsStore.uninstallPlugin(pluginId.value)
    uninstallDialogVisible.value = false
    ElMessage.success('卸载任务已接受')
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    operationError.value = error instanceof Error ? error.message : 'uninstall failed'
  }
}

function clearConsole() {
  pluginsStore.clearConsole(pluginId.value)
}

watch(pluginId, () => {
  void loadDetail()
})

onMounted(() => {
  void loadDetail()
})

onUnmounted(() => {
  socketStore.setConsolePlugin(null)
})
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Plugin Detail</div>
        <h1>{{ pluginId }}</h1>
        <p>详情页覆盖 lifecycle、当前生效 grants、console 与卸载任务入口。</p>
      </div>

      <div class="table-actions">
        <el-button type="success" :loading="actionPending[pluginId] === 'enable'" @click="runAction('enable')">Enable</el-button>
        <el-button type="warning" :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">Reload</el-button>
        <el-button type="danger" plain :loading="actionPending[pluginId] === 'disable'" @click="runAction('disable')">Disable</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true">Uninstall</el-button>
      </div>
    </section>

    <RetryPanel
      v-if="loadError && !current"
      title="插件详情读取失败"
      :description="loadError"
      :loading="detailLoading"
      @retry="loadDetail()"
    />

    <el-alert v-else-if="loadError" title="插件详情读取失败" type="error" :description="loadError" show-icon />

    <el-alert v-if="operationError" title="插件操作失败" type="error" :description="operationError" show-icon />

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>Current Snapshot</span>
          </div>
        </template>

        <el-skeleton :loading="detailLoading" animated>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Name">{{ current?.name ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Role">{{ current?.role ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Registration">{{ current?.registration_state ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Desired">{{ current?.desired_state ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Runtime">{{ current?.runtime_state ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Display">{{ current?.display_state ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Trust">{{ current?.trust?.label ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Source Root">{{ current?.source?.root ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Source Ref">{{ current?.source?.package_source_ref ?? current?.source?.package_source_type ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="命令冲突">
              <div v-if="current?.command_conflicts?.length" class="table-actions">
                <el-tag v-for="command in current.command_conflicts" :key="command" size="small" type="warning">
                  {{ command }}
                </el-tag>
              </div>
              <span v-else>—</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-skeleton>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>Effective Grants</span>
            <div class="table-actions">
              <el-tag size="small">{{ currentGrants.length }}</el-tag>
              <el-button size="small" type="primary" @click="grantDialogVisible = true">新增授权</el-button>
            </div>
          </div>
        </template>

        <el-skeleton :loading="grantBusy" animated>
          <el-empty v-if="currentGrants.length === 0" description="当前没有生效 grants" />

          <div v-else class="grant-list">
            <div v-for="grant in currentGrants" :key="`${grant.capability}-${grant.granted_at}`" class="grant-item">
              <div>
                <strong>{{ grant.capability }}</strong>
                <small>授予时间：{{ formatDateTime(grant.granted_at) }}</small>
                <small>过期时间：{{ formatDateTime(grant.expires_at ?? undefined) }}</small>
              </div>
              <el-button size="small" type="danger" plain @click="revokeGrant(grant.capability)">撤销</el-button>
            </div>
          </div>
        </el-skeleton>
      </el-card>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>Console</span>
          <div class="table-actions">
            <el-tag size="small">{{ consoleSnapshot.status }}</el-tag>
            <el-button size="small" plain @click="socketStore.reconnectConsole()">重连</el-button>
            <el-button size="small" plain @click="clearConsole">清空输出</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="consoleSnapshot.lastError"
        title="Console 连接异常"
        type="warning"
        :description="consoleSnapshot.lastError"
        show-icon
        class="section-gap"
      />

      <el-empty v-if="consoleFrames.length === 0" description="等待 console 输出" />

      <div v-else class="console-feed">
        <div v-for="frame in consoleFrames" :key="`${frame.timestamp}-${frame.text}`" class="console-line">
          <span class="console-meta">{{ formatDateTime(frame.timestamp) }} · {{ frame.stream }}</span>
          <pre>{{ frame.text }}</pre>
        </div>
      </div>
    </el-card>
  </div>

  <el-dialog v-model="grantDialogVisible" title="新增授权" width="440px">
    <el-form label-position="top">
      <el-form-item label="Capability">
        <el-input v-model="grantForm.capability" placeholder="http.request" />
      </el-form-item>
      <el-form-item label="Expires At (UTC RFC3339)">
        <el-input v-model="grantForm.expires_at" placeholder="2026-03-23T10:05:00Z" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="table-actions">
        <el-button @click="grantDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="grantBusy" :disabled="!grantForm.capability" @click="submitGrant">
          保存授权
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="uninstallDialogVisible" title="确认卸载插件" width="420px">
    <p>卸载会进入异步任务流。页面会跳转到任务详情继续跟踪执行状态。</p>

    <template #footer>
      <div class="table-actions">
        <el-button @click="uninstallDialogVisible = false">取消</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallPlugin">
          确认卸载
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>
