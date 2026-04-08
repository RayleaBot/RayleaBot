<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'
import {
  Activity,
  ChevronDown,
  Command,
  LayoutDashboard,
  LogOut,
  LucideIcon,
  Plug,
  Settings,
  SquareTerminal,
  Sword,
} from 'lucide-vue-next'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { getReadinessStatusLabel, getStatusType, getSystemStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()

const { shutdownPending, shutdownRequested, system, readiness } = storeToRefs(systemStore)
const shutdownDialogVisible = ref(false)

interface NavItem {
  index: string
  labelKey: string
  icon: LucideIcon
  children?: Array<{
    index: string
    labelKey: string
  }>
}

const navigationItems: NavItem[] = [
  { index: '/', labelKey: 'routes.status', icon: LayoutDashboard },
  { index: '/plugins', labelKey: 'routes.plugins', icon: Plug },
  { index: '/commands', labelKey: 'routes.commands', icon: Command },
  { index: '/tasks', labelKey: 'routes.tasks', icon: Sword },
  { index: '/logs', labelKey: 'routes.logs', icon: SquareTerminal },
  {
    index: '/protocols',
    labelKey: 'routes.protocols',
    icon: Activity,
    children: [
      { index: '/protocols/logs', labelKey: 'routes.protocolLogs' },
    ],
  },
  { index: '/config', labelKey: 'routes.config', icon: Settings },
]
const expandedNavGroups = ref<string[]>([])

const headerTitle = computed(() => {
  if (route.meta.titleKey) {
    return t(route.meta.titleKey)
  }

  return route.meta.title ?? t('app.consoleName')
})

const systemStatusType = computed(() => getStatusType(system.value?.status))
const readinessStatusType = computed(() => getStatusType(readiness.value?.status))

const statusLabel = computed(() => getSystemStatusLabel(system.value?.status))
const readyLabel = computed(() => getReadinessStatusLabel(readiness.value?.status))

function isActive(index: string) {
  if (index === '/') {
    return route.path === '/'
  }

  return route.path === index || route.path.startsWith(`${index}/`)
}

function getNavItemClass(index: string) {
  return {
    'shell-nav-item': true,
    'shell-nav-item--group': navigationItems.some((item) => item.index === index && item.children?.length),
    'is-active': isActive(index),
  }
}

function isGroupExpanded(index: string) {
  return expandedNavGroups.value.includes(index) || isActive(index)
}

function toggleGroup(index: string) {
  if (isGroupExpanded(index) && !isActive(index)) {
    expandedNavGroups.value = expandedNavGroups.value.filter((item) => item !== index)
    return
  }

  if (!expandedNavGroups.value.includes(index)) {
    expandedNavGroups.value = [...expandedNavGroups.value, index]
  }
}

function getSubNavItemClass(index: string) {
  return {
    'shell-subnav-item': true,
    'is-active': isActive(index),
  }
}

function navigateTo(index: string) {
  if (route.path === index) {
    return
  }
  void router.push(index)
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
        <template v-for="item in navigationItems" :key="item.index">
          <div v-if="item.children?.length" class="shell-nav-group" :class="{ 'is-open': isGroupExpanded(item.index) }">
            <div :class="getNavItemClass(item.index)">
              <button type="button" class="shell-nav-link" @click="navigateTo(item.index)">
                <component :is="item.icon" :size="18" class="nav-icon" />
                <span class="nav-label">{{ t(item.labelKey) }}</span>
              </button>
              <button
                type="button"
                class="shell-nav-toggle"
                :aria-label="`${t(item.labelKey)}子页面`"
                :aria-expanded="isGroupExpanded(item.index)"
                @click.stop="toggleGroup(item.index)"
              >
                <ChevronDown :size="16" class="nav-chevron" />
              </button>
            </div>

            <div v-if="isGroupExpanded(item.index)" class="shell-subnav">
              <RouterLink
                v-for="child in item.children"
                :key="child.index"
                :to="child.index"
                :class="getSubNavItemClass(child.index)"
              >
                <span>{{ t(child.labelKey) }}</span>
              </RouterLink>
            </div>
          </div>

          <RouterLink
            v-else
            :to="item.index"
            :class="getNavItemClass(item.index)"
          >
            <component :is="item.icon" :size="18" class="nav-icon" />
            <span class="nav-label">{{ t(item.labelKey) }}</span>
          </RouterLink>
        </template>
      </nav>

      <div class="sidebar-metrics">
        <div :class="['metric-pill', `metric-pill--${systemStatusType}`]">
          <span>{{ t('shell.systemStatus') }}</span>
          <strong>{{ statusLabel }}</strong>
        </div>
        <div :class="['metric-pill', `metric-pill--${readinessStatusType}`]">
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
