<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'

import { formatDateTime } from '@/lib/format'
import { useTasksStore } from '@/stores/tasks'

const tasksStore = useTasksStore()
const { currentTask, error, loading, sortedItems } = storeToRefs(tasksStore)
const detailVisible = ref(false)

onMounted(() => {
  void tasksStore.fetchList()
})

async function inspect(taskId: string) {
  await tasksStore.fetchDetail(taskId)
  detailVisible.value = true
}

async function cancelCurrent() {
  if (!currentTask.value) {
    return
  }

  await tasksStore.cancelTask(currentTask.value.task_id)
  ElMessage.success('取消请求已发送')
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Tasks</div>
        <h1>后台任务</h1>
        <p>列表先读 HTTP，再吃 `/ws/tasks` 增量更新。</p>
      </div>

      <el-button :loading="loading" @click="tasksStore.fetchList()">
        刷新任务
      </el-button>
    </section>

    <el-alert v-if="error" title="任务列表读取失败" type="error" :description="error" show-icon />

    <el-table :data="sortedItems" stripe @row-click="(row) => inspect(row.task_id)">
      <el-table-column prop="task_id" label="Task ID" min-width="220" />
      <el-table-column prop="task_type" label="Type" min-width="150" />
      <el-table-column prop="status" label="Status" width="130" />
      <el-table-column prop="progress" label="Progress" width="120" />
      <el-table-column prop="summary" label="Summary" min-width="260" />
    </el-table>

    <el-drawer v-model="detailVisible" title="任务详情" size="40%" :modal="false">
      <el-descriptions v-if="currentTask" :column="1" border>
        <el-descriptions-item label="Task ID">{{ currentTask.task_id }}</el-descriptions-item>
        <el-descriptions-item label="Type">{{ currentTask.task_type }}</el-descriptions-item>
        <el-descriptions-item label="Status">{{ currentTask.status }}</el-descriptions-item>
        <el-descriptions-item label="Progress">{{ currentTask.progress ?? '—' }}</el-descriptions-item>
        <el-descriptions-item label="Summary">{{ currentTask.summary }}</el-descriptions-item>
        <el-descriptions-item label="Started">{{ formatDateTime(currentTask.started_at) }}</el-descriptions-item>
        <el-descriptions-item label="Finished">{{ formatDateTime(currentTask.finished_at) }}</el-descriptions-item>
      </el-descriptions>

      <template #footer>
        <el-button type="danger" plain @click="cancelCurrent">
          请求取消
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>
