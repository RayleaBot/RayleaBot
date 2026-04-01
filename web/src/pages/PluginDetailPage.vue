<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import {
  getConnectionStatusLabel,
  getPluginDesiredStateLabel,
  getPluginRegistrationStateLabel,
  getPluginRoleLabel,
  getPluginRuntimeStateLabel,
} from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
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
    loadError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
  }
}

async function runAction(action: 'enable' | 'disable' | 'reload') {
  operationError.value = null
  try {
    await pluginsStore.executeAction(pluginId.value, action)
    ElMessage.success(t('plugins.actionAccepted'))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
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
    ElMessage.success(t('plugins.grantSaved'))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

async function revokeGrant(capability: string) {
  operationError.value = null
  try {
    await pluginsStore.revokeGrant(pluginId.value, capability)
    ElMessage.success(t('plugins.grantRevoked'))
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
  }
}

async function uninstallPlugin() {
  operationError.value = null
  try {
    const response = await pluginsStore.uninstallPlugin(pluginId.value)
    uninstallDialogVisible.value = false
    ElMessage.success(t('plugins.uninstallAccepted'))
    await router.push({ name: 'tasks', query: { task_id: response.task_id } })
  } catch (error) {
    operationError.value = getDisplayErrorMessage(error)
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
        <h1>{{ pluginId }}</h1>
      </div>

      <div class="table-actions">
        <el-button type="success" :loading="actionPending[pluginId] === 'enable'" @click="runAction('enable')">{{ t('plugins.actions.enable') }}</el-button>
        <el-button type="warning" :loading="actionPending[pluginId] === 'reload'" @click="runAction('reload')">{{ t('plugins.actions.reload') }}</el-button>
        <el-button type="danger" plain :loading="actionPending[pluginId] === 'disable'" @click="runAction('disable')">{{ t('plugins.actions.disable') }}</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallDialogVisible = true">{{ t('plugins.actions.uninstall') }}</el-button>
      </div>
    </section>

    <RetryPanel
      v-if="loadError && !current"
      :title="t('errors.common.loadFailed')"
      :description="loadError"
      :loading="detailLoading"
      @retry="loadDetail()"
    />

    <el-alert v-else-if="loadError" :title="t('errors.common.loadFailed')" type="error" :description="loadError" show-icon />

    <el-alert v-if="operationError" :title="t('errors.common.actionFailed')" type="error" :description="operationError" show-icon />

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('plugins.sections.current') }}</span>
          </div>
        </template>

        <el-skeleton :loading="detailLoading" animated>
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="t('plugins.fields.name')">{{ current?.name ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.role')">
              {{ getPluginRoleLabel(current?.role) }}
              <small v-if="current?.role"> · {{ current.role }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.registration')">
              {{ getPluginRegistrationStateLabel(current?.registration_state) }}
              <small v-if="current?.registration_state"> · {{ current.registration_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.desired')">
              {{ getPluginDesiredStateLabel(current?.desired_state) }}
              <small v-if="current?.desired_state"> · {{ current.desired_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.runtime')">
              {{ getPluginRuntimeStateLabel(current?.runtime_state) }}
              <small v-if="current?.runtime_state"> · {{ current.runtime_state }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.display')">{{ current?.display_state ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.trust')">{{ current?.trust?.label ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.sourceRoot')">{{ current?.source?.root ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.sourceRef')">{{ current?.source?.package_source_ref ?? current?.source?.package_source_type ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('plugins.fields.conflicts')">
              <div v-if="current?.command_conflicts?.length" class="table-actions">
                <el-tag v-for="command in current.command_conflicts" :key="command" size="small" type="warning">
                  {{ command }}
                </el-tag>
              </div>
              <span v-else>{{ t('display.empty') }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-skeleton>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>{{ t('plugins.sections.grants') }}</span>
            <div class="table-actions">
              <el-tag size="small">{{ currentGrants.length }}</el-tag>
              <el-button size="small" type="primary" @click="grantDialogVisible = true">{{ t('plugins.actions.addGrant') }}</el-button>
            </div>
          </div>
        </template>

        <el-skeleton :loading="grantBusy" animated>
          <el-empty v-if="currentGrants.length === 0" :description="t('plugins.empty.grants')" />

          <div v-else class="grant-list">
            <div v-for="grant in currentGrants" :key="`${grant.capability}-${grant.granted_at}`" class="grant-item">
              <div>
                <strong>{{ grant.capability }}</strong>
                <small>{{ t('plugins.fields.grantedAt') }}：{{ formatDateTime(grant.granted_at) }}</small>
                <small>{{ t('plugins.fields.expiresAt') }}：{{ formatDateTime(grant.expires_at ?? undefined) }}</small>
              </div>
              <el-button size="small" type="danger" plain @click="revokeGrant(grant.capability)">{{ t('plugins.actions.revokeGrant') }}</el-button>
            </div>
          </div>
        </el-skeleton>
      </el-card>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>{{ t('plugins.sections.console') }}</span>
          <div class="table-actions">
            <el-tag size="small">{{ getConnectionStatusLabel(consoleSnapshot.status) }}</el-tag>
            <el-button size="small" plain @click="socketStore.reconnectConsole()">{{ t('plugins.actions.reconnectConsole') }}</el-button>
            <el-button size="small" plain @click="clearConsole">{{ t('plugins.actions.clearConsole') }}</el-button>
          </div>
        </div>
      </template>

      <el-alert
        v-if="consoleSnapshot.lastError"
        :title="t('plugins.consoleUnavailable')"
        type="warning"
        :description="consoleSnapshot.lastError"
        show-icon
        class="section-gap"
      />

      <el-empty v-if="consoleFrames.length === 0" :description="t('plugins.empty.console')" />

      <div v-else class="console-feed">
        <div v-for="frame in consoleFrames" :key="`${frame.timestamp}-${frame.text}`" class="console-line">
          <span class="console-meta">{{ formatDateTime(frame.timestamp) }} · {{ frame.stream }}</span>
          <pre>{{ frame.text }}</pre>
        </div>
      </div>
    </el-card>
  </div>

  <el-dialog v-model="grantDialogVisible" :title="t('plugins.grantDialogTitle')" width="440px">
    <el-form label-position="top">
      <el-form-item :label="t('plugins.fields.capability')">
        <el-input v-model="grantForm.capability" placeholder="http.request" />
      </el-form-item>
      <el-form-item :label="t('plugins.grantExpiry')">
        <el-input v-model="grantForm.expires_at" placeholder="2026-03-23T10:05:00Z" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="table-actions">
        <el-button @click="grantDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="primary" :loading="grantBusy" :disabled="!grantForm.capability" @click="submitGrant">
          {{ t('plugins.actions.saveGrant') }}
        </el-button>
      </div>
    </template>
  </el-dialog>

  <el-dialog v-model="uninstallDialogVisible" :title="t('plugins.uninstallConfirmTitle')" width="420px">
    <p>{{ t('plugins.uninstallConfirmBody') }}</p>

    <template #footer>
      <div class="table-actions">
        <el-button @click="uninstallDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="danger" :loading="actionPending[pluginId] === 'uninstall'" @click="uninstallPlugin">
          {{ t('plugins.actions.uninstallConfirm') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>
