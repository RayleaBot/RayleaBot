<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import {
  getPluginDesiredStateLabel,
  getPluginDisplayStateLabel,
  getPluginRegistrationStateLabel,
  getPluginRoleLabel,
  getPluginRuntimeStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { t } from '@/i18n'
import type { PluginInstallRequest } from '@/types/api'
import { usePluginsStore } from '@/stores/plugins'

type HealthNoticeTone = '' | 'info' | 'warning' | 'danger'

interface PluginHealthNotice {
  label: string
  tone: HealthNoticeTone
}

const router = useRouter()
const pluginsStore = usePluginsStore()
const { actionPending, error, installPending, loading, sortedItems } = storeToRefs(pluginsStore)
const installDialogVisible = ref(false)
const installError = ref<string | null>(null)
const summaryDrawerVisible = ref(false)
const summaryPluginId = ref<string | null>(null)
const installForm = reactive<PluginInstallRequest>({
  source_type: 'local_zip',
  source: '',
})
const summaryPlugin = computed(() => sortedItems.value.find((item) => item.id === summaryPluginId.value) ?? null)

function getConflictNotice(count: number) {
  return t('plugins.health.commandConflicts', { count })
}

function getPluginHealthNotices(row: (typeof sortedItems.value)[number]) {
  const notices: PluginHealthNotice[] = []
  const conflicts = row.command_conflicts?.length ?? 0

  if (conflicts > 0) {
    notices.push({ label: getConflictNotice(conflicts), tone: 'warning' })
  }

  if (row.source?.verified === false || row.trust?.level === 'unverified') {
    notices.push({ label: t('plugins.health.unverifiedSource'), tone: 'info' })
  }

  if (row.registration_state === 'removed') {
    notices.push({ label: t('plugins.health.removed'), tone: 'danger' })
  }

  if (row.runtime_state === 'crashed' || row.runtime_state === 'dead_letter') {
    notices.push({ label: t('plugins.health.runtimeIssue'), tone: 'danger' })
  } else if (row.runtime_state === 'backoff') {
    notices.push({ label: t('plugins.health.retrying'), tone: 'warning' })
  } else if (row.desired_state === 'enabled' && row.runtime_state === 'stopped') {
    notices.push({ label: t('plugins.health.enabledButStopped'), tone: 'warning' })
  }

  return notices.slice(0, 3)
}

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

function openSummary(id: string) {
  summaryPluginId.value = id
  summaryDrawerVisible.value = true
}

async function submitInstall() {
  installError.value = null
  try {
    const response = await pluginsStore.installPlugin(installForm)
    installDialogVisible.value = false
    installForm.source_type = 'local_zip'
    installForm.source = ''
    delete installForm.allow_install_scripts
    ElMessage.success(t('plugins.installAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    installError.value = getDisplayErrorMessage(error)
  }
}
</script>

<template>
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('plugins.title') }}</h1>
      </div>

      <div class="table-actions">
        <el-button type="primary" @click="installDialogVisible = true">
          {{ t('plugins.install') }}
        </el-button>
        <el-button :loading="loading" @click="loadPlugins()">
          {{ t('plugins.refresh') }}
        </el-button>
      </div>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadPlugins()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <el-alert v-if="installError" :title="t('errors.common.actionFailed')" type="error" :description="installError" show-icon />

    <VirtualDataViewport
      :items="sortedItems"
      :item-height="164"
      :get-item-key="(row) => row.id"
      :empty-label="t('display.empty')"
    >
      <template #default="{ item: row }">
        <article class="plugin-summary-row">
          <div class="plugin-summary-identity">
            <div class="plugin-summary-heading">
              <div class="mono-list">
                <strong>{{ row.name }}</strong>
                <small>{{ row.id }}</small>
              </div>
            </div>

            <div class="plugin-summary-facts">
              <div class="plugin-summary-fact">
                <span>{{ t('plugins.fields.source') }}</span>
                <strong :title="row.source?.root ?? t('display.empty')">{{ row.source?.root ?? t('display.empty') }}</strong>
              </div>

              <div class="plugin-summary-fact">
                <span>{{ t('plugins.fields.trust') }}</span>
                <strong>{{ row.trust?.label ?? t('display.empty') }}</strong>
              </div>
            </div>
          </div>

          <div class="plugin-summary-statuses">
            <div class="plugin-status-grid">
              <div class="plugin-status-card">
                <span>{{ t('plugins.fields.desired') }}</span>
                <strong>{{ getPluginDesiredStateLabel(row.desired_state) }}</strong>
              </div>

              <div class="plugin-status-card">
                <span>{{ t('plugins.fields.runtime') }}</span>
                <strong>{{ getPluginRuntimeStateLabel(row.runtime_state) }}</strong>
              </div>
            </div>

            <div class="plugin-summary-health">
              <el-tag
                v-for="notice in getPluginHealthNotices(row)"
                :key="notice.label"
                size="small"
                effect="plain"
                :type="notice.tone"
              >
                {{ notice.label }}
              </el-tag>
            </div>
          </div>

          <div class="plugin-summary-actions">
            <el-button size="small" plain @click="openSummary(row.id)">
              {{ t('plugins.actions.summary') }}
            </el-button>
            <el-button size="small" plain @click="openDetail(row.id)">
              {{ t('plugins.actions.detail') }}
            </el-button>
            <el-button size="small" type="success" :loading="actionPending[row.id] === 'enable'" @click="pluginsStore.executeAction(row.id, 'enable')">
              {{ t('plugins.actions.enable') }}
            </el-button>
            <el-button size="small" type="warning" :loading="actionPending[row.id] === 'reload'" @click="pluginsStore.executeAction(row.id, 'reload')">
              {{ t('plugins.actions.reload') }}
            </el-button>
            <el-button size="small" type="danger" plain :loading="actionPending[row.id] === 'disable'" @click="pluginsStore.executeAction(row.id, 'disable')">
              {{ t('plugins.actions.disable') }}
            </el-button>
          </div>
        </article>
      </template>
    </VirtualDataViewport>
  </div>

  <el-dialog v-model="installDialogVisible" :title="t('plugins.installDialogTitle')" width="520px">
    <el-form label-position="top">
      <el-alert v-if="installError" :title="t('errors.common.actionFailed')" type="error" :description="installError" show-icon class="section-gap" />

      <el-form-item :label="t('plugins.sourceType')">
        <el-select v-model="installForm.source_type">
          <el-option :label="t('plugins.localZip')" value="local_zip" />
          <el-option :label="t('plugins.localDirectory')" value="local_directory" />
          <el-option :label="t('plugins.remoteUrl')" value="remote_url" />
        </el-select>
      </el-form-item>

      <el-form-item :label="installForm.source_type === 'remote_url' ? t('plugins.remoteUrlLabel') : t('plugins.serverPath')">
        <el-input v-model="installForm.source" />
      </el-form-item>

      <el-form-item>
        <el-checkbox v-model="installForm.allow_install_scripts">
          {{ t('plugins.allowScripts') }}
        </el-checkbox>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="table-actions">
        <el-button @click="installDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="primary" :loading="installPending" :disabled="!installForm.source" @click="submitInstall">
          {{ t('plugins.installSubmit') }}
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="summaryDrawerVisible" :title="t('plugins.actions.summary')" width="min(560px, 92vw)">
    <template v-if="summaryPlugin">
      <div class="drawer-section drawer-section--dense">
        <div class="mono-list">
          <strong>{{ summaryPlugin.name }}</strong>
          <small>{{ summaryPlugin.id }}</small>
        </div>
      </div>

      <el-descriptions :column="1" border>
        <el-descriptions-item :label="t('plugins.fields.role')">{{ getPluginRoleLabel(summaryPlugin.role) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.trust')">{{ summaryPlugin.trust?.label ?? t('display.empty') }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.registration')">{{ getPluginRegistrationStateLabel(summaryPlugin.registration_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.desired')">{{ getPluginDesiredStateLabel(summaryPlugin.desired_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.runtime')">{{ getPluginRuntimeStateLabel(summaryPlugin.runtime_state) }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.display')">
          {{ getPluginDisplayStateLabel(summaryPlugin.display_state) }}
          <small v-if="summaryPlugin.display_state"> · {{ summaryPlugin.display_state }}</small>
        </el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.source')">{{ summaryPlugin.source?.root ?? t('display.empty') }}</el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.sourceRef')">
          {{ summaryPlugin.source?.package_source_ref ?? summaryPlugin.source?.package_source_type ?? t('display.empty') }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('plugins.fields.conflicts')">
          <div v-if="summaryPlugin.command_conflicts?.length" class="table-actions">
            <el-tag v-for="command in summaryPlugin.command_conflicts" :key="command" size="small" type="warning">
              {{ command }}
            </el-tag>
          </div>
          <span v-else>{{ t('display.empty') }}</span>
        </el-descriptions-item>
      </el-descriptions>
    </template>
  </el-dialog>
</template>
