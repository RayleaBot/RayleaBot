<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { getReadinessStatusLabel, getSystemStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()

const { shutdownPending, shutdownRequested, system, readiness } = storeToRefs(systemStore)
const shutdownDialogVisible = ref(false)

const navigationItems = [
  { index: '/', label: t('routes.status') },
  { index: '/plugins', label: t('routes.plugins') },
  { index: '/tasks', label: t('routes.tasks') },
  { index: '/logs', label: t('routes.logs') },
  { index: '/config', label: t('routes.config') },
]

const headerTitle = computed(() => {
  if (route.meta.titleKey) {
    return t(route.meta.titleKey)
  }

  return route.meta.title ?? t('app.consoleName')
})
const statusLabel = computed(() => getSystemStatusLabel(system.value?.status))
const readyLabel = computed(() => getReadinessStatusLabel(readiness.value?.status))

function isActive(index: string) {
  if (index === '/') {
    return route.path === '/'
  }

  return route.path === index || route.path.startsWith(`${index}/`)
}

async function handleLogout() {
  await sessionStore.logout()
  await router.push({ name: 'login' })
}

async function confirmShutdown() {
  try {
    await systemStore.requestShutdown()
    shutdownDialogVisible.value = false
    ElMessage.success(t('shell.shutdownAccepted'))
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
  }
}
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <el-container class="shell-layout">
    <el-aside width="260px" class="shell-sidebar">
      <div class="brand-block">
        <div class="brand-eyebrow">{{ t('app.brand') }}</div>
        <h1>{{ t('app.consoleName') }}</h1>
      </div>

      <nav class="shell-nav" aria-label="Primary">
        <RouterLink
          v-for="item in navigationItems"
          :key="item.index"
          :to="item.index"
          class="shell-nav-item"
          :class="{ 'is-active': isActive(item.index) }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="sidebar-metrics">
        <div class="metric-pill">
          <span>{{ t('shell.systemStatus') }}</span>
          <strong>{{ statusLabel }}</strong>
        </div>
        <div class="metric-pill">
          <span>{{ t('shell.readyStatus') }}</span>
          <strong>{{ readyLabel }}</strong>
        </div>
      </div>
    </el-aside>

    <el-container>
      <el-header class="shell-header">
        <div class="header-heading">
          <h2>{{ headerTitle }}</h2>
        </div>

        <div class="header-actions">
          <ConnectionStatusStrip />
          <el-button type="danger" plain :loading="shutdownPending" @click="shutdownDialogVisible = true">
            {{ t('shell.shutdown') }}
          </el-button>
          <el-button type="primary" plain @click="handleLogout">
            {{ t('shell.logout') }}
          </el-button>
        </div>
      </el-header>

      <el-main id="app-main" class="shell-main" tabindex="-1">
        <div class="sr-only" aria-live="polite">
          {{ shutdownRequested ? t('shell.shutdownRequestedLive') : '' }}
        </div>

        <el-alert
          v-if="shutdownRequested"
          :title="t('shell.shutdownRequestedTitle')"
          type="warning"
          :description="t('shell.shutdownRequestedDescription')"
          show-icon
          class="section-gap"
        />

        <div class="shell-body">
          <RouterView />
        </div>
      </el-main>
    </el-container>
  </el-container>

  <el-dialog v-model="shutdownDialogVisible" :title="t('shell.shutdownConfirmTitle')" width="420px">
    <p>{{ t('shell.shutdownConfirmBody') }}</p>

    <template #footer>
      <div class="table-actions">
        <el-button @click="shutdownDialogVisible = false">{{ t('dashboard.previewCancel') }}</el-button>
        <el-button type="danger" :loading="shutdownPending" @click="confirmShutdown">
          {{ t('shell.shutdownConfirmAction') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>
