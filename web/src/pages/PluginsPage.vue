<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import type { PluginInstallRequest } from '@/types/api'
import { usePluginsStore } from '@/stores/plugins'

const router = useRouter()
const pluginsStore = usePluginsStore()
const { actionPending, error, installPending, loading, sortedItems } = storeToRefs(pluginsStore)
const installDialogVisible = ref(false)
const installError = ref<string | null>(null)
const installForm = reactive<PluginInstallRequest>({
  source_type: 'local_zip',
  source: '',
})

async function loadPlugins() {
  try {
    await pluginsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadPlugins()
})

function openDetail(id: string) {
  void router.push({ name: 'plugin-detail', params: { id } })
}

async function submitInstall() {
  installError.value = null
  try {
    const response = await pluginsStore.installPlugin(installForm)
    installDialogVisible.value = false
    installForm.source_type = 'local_zip'
    installForm.source = ''
    delete installForm.allow_install_scripts
    ElMessage.success('安装任务已接受')
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    installError.value = error instanceof Error ? error.message : 'install failed'
  }
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Plugins</div>
        <h1>插件主流程</h1>
        <p>当前覆盖 install、详情、lifecycle、grants 与 console 管理流。</p>
      </div>

      <div class="table-actions">
        <el-button type="primary" @click="installDialogVisible = true">
          安装插件
        </el-button>
        <el-button :loading="loading" @click="loadPlugins()">
          刷新列表
        </el-button>
      </div>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      title="插件列表读取失败"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <el-alert v-else-if="error" title="插件列表读取失败" type="error" :description="error" show-icon />

    <el-alert v-if="installError" title="安装请求失败" type="error" :description="installError" show-icon />

    <el-table class="desktop-table" :data="sortedItems" stripe @row-click="(row) => openDetail(row.id)">
      <el-table-column prop="id" label="Plugin ID" min-width="180" />
      <el-table-column prop="name" label="Name" min-width="180" />
      <el-table-column prop="role" label="Role" width="120" />
      <el-table-column label="Trust" min-width="160">
        <template #default="{ row }">
          <div class="mono-list">
            <strong>{{ row.trust?.label ?? '—' }}</strong>
            <small>{{ row.source?.verified ? 'verified' : 'unverified' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Source" min-width="240">
        <template #default="{ row }">
          <div class="mono-list">
            <div>{{ row.source?.root ?? '—' }}</div>
            <small>{{ row.source?.package_source_ref ?? row.source?.package_source_type ?? '—' }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Command Conflicts" min-width="180">
        <template #default="{ row }">
          <div v-if="row.command_conflicts?.length" class="table-actions">
            <el-tag v-for="command in row.command_conflicts" :key="command" size="small" type="warning">
              {{ command }}
            </el-tag>
          </div>
          <span v-else>—</span>
        </template>
      </el-table-column>
      <el-table-column prop="registration_state" label="Registration" width="140" />
      <el-table-column prop="desired_state" label="Desired" width="140" />
      <el-table-column prop="runtime_state" label="Runtime" width="140" />
      <el-table-column prop="display_state" label="Display" width="160" />
      <el-table-column label="Actions" min-width="260">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button size="small" type="success" :loading="actionPending[row.id] === 'enable'" @click.stop="pluginsStore.executeAction(row.id, 'enable')">
              Enable
            </el-button>
            <el-button size="small" type="warning" :loading="actionPending[row.id] === 'reload'" @click.stop="pluginsStore.executeAction(row.id, 'reload')">
              Reload
            </el-button>
            <el-button size="small" type="danger" plain :loading="actionPending[row.id] === 'disable'" @click.stop="pluginsStore.executeAction(row.id, 'disable')">
              Disable
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <div class="mobile-card-list">
      <el-card v-for="row in sortedItems" :key="row.id" class="mobile-data-card">
        <div class="mobile-data-header">
          <strong>{{ row.name }}</strong>
          <el-tag size="small">{{ row.runtime_state }}</el-tag>
        </div>
        <div class="mobile-data-grid">
          <div><span>Plugin ID</span><strong>{{ row.id }}</strong></div>
          <div><span>Role</span><strong>{{ row.role }}</strong></div>
          <div><span>Registration</span><strong>{{ row.registration_state }}</strong></div>
          <div><span>Desired</span><strong>{{ row.desired_state }}</strong></div>
          <div><span>Display</span><strong>{{ row.display_state ?? '—' }}</strong></div>
          <div><span>Trust</span><strong>{{ row.trust?.label ?? '—' }}</strong></div>
        </div>
        <p class="mobile-data-copy">{{ row.source?.root ?? '—' }}</p>
        <div v-if="row.command_conflicts?.length" class="table-actions">
          <el-tag v-for="command in row.command_conflicts" :key="command" size="small" type="warning">
            {{ command }}
          </el-tag>
        </div>
        <div class="table-actions">
          <el-button size="small" plain @click="openDetail(row.id)">详情</el-button>
          <el-button size="small" type="success" :loading="actionPending[row.id] === 'enable'" @click="pluginsStore.executeAction(row.id, 'enable')">
            Enable
          </el-button>
          <el-button size="small" type="warning" :loading="actionPending[row.id] === 'reload'" @click="pluginsStore.executeAction(row.id, 'reload')">
            Reload
          </el-button>
          <el-button size="small" type="danger" plain :loading="actionPending[row.id] === 'disable'" @click="pluginsStore.executeAction(row.id, 'disable')">
            Disable
          </el-button>
        </div>
      </el-card>
    </div>
  </div>

  <el-dialog v-model="installDialogVisible" title="安装插件" width="520px">
    <el-form label-position="top">
      <el-alert v-if="installError" title="安装请求失败" type="error" :description="installError" show-icon class="section-gap" />

      <el-form-item label="Source Type">
        <el-select v-model="installForm.source_type">
          <el-option label="Local ZIP" value="local_zip" />
          <el-option label="Local Directory" value="local_directory" />
          <el-option label="Remote URL" value="remote_url" />
        </el-select>
      </el-form-item>

      <el-form-item :label="installForm.source_type === 'remote_url' ? 'HTTPS URL' : 'Server Path'">
        <el-input v-model="installForm.source" />
      </el-form-item>

      <el-form-item>
        <el-checkbox v-model="installForm.allow_install_scripts">
          允许安装脚本
        </el-checkbox>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="table-actions">
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="installPending" :disabled="!installForm.source" @click="submitInstall">
          开始安装
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>
