<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { ChevronDown, LogOut, Power } from 'lucide-vue-next'

import { navigationItems } from '@/access/navigation'
import { notifyError, notifySuccess } from '@/adapter/feedback'
import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { t } from '@/i18n'
import { getStatusType, getSystemStatusLabel, getReadinessStatusLabel } from '@/lib/display'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()

const { shutdownPending, shutdownRequested, system, readiness } = storeToRefs(systemStore)
const shutdownDialogVisible = ref(false)
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

function isActive(path: string) {
  if (path === '/') {
    return route.path === '/'
  }

  return route.path === path || route.path.startsWith(`${path}/`)
}

function isGroupExpanded(path: string) {
  return expandedNavGroups.value.includes(path) || isActive(path)
}

function toggleGroup(path: string) {
  if (isGroupExpanded(path) && !isActive(path)) {
    expandedNavGroups.value = expandedNavGroups.value.filter((item) => item !== path)
    return
  }

  if (!expandedNavGroups.value.includes(path)) {
    expandedNavGroups.value = [...expandedNavGroups.value, path]
  }
}

async function handleLogout() {
  await sessionStore.logout()
  await router.push({ name: 'login' })
}

async function confirmShutdown() {
  try {
    await systemStore.requestShutdown()
    shutdownDialogVisible.value = false
    notifySuccess(t('shell.shutdownAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <a-layout class="shell-layout">
    <a-layout-sider class="shell-sidebar" :width="272" :trigger="null" theme="light">
      <div class="brand-block">
        <div class="brand-eyebrow">{{ t('app.brand') }}</div>
        <h1>{{ t('app.consoleName') }}</h1>
      </div>

      <nav class="shell-nav" aria-label="Primary">
        <template v-for="item in navigationItems" :key="item.path">
          <div
            v-if="item.children?.length"
            class="shell-nav-group"
            :class="{ 'is-open': isGroupExpanded(item.path) }"
          >
            <div class="shell-nav-item" :class="{ 'is-active': isActive(item.path) }">
              <button type="button" class="shell-nav-link" @click="router.push(item.path)">
                <component :is="item.icon" :size="18" class="nav-icon" />
                <span class="nav-label">{{ t(item.labelKey) }}</span>
              </button>
              <button
                type="button"
                class="shell-nav-toggle"
                :aria-label="`${t(item.labelKey)}子页面`"
                :aria-expanded="isGroupExpanded(item.path)"
                @click.stop="toggleGroup(item.path)"
              >
                <ChevronDown :size="16" class="nav-chevron" />
              </button>
            </div>

            <div v-if="isGroupExpanded(item.path)" class="shell-subnav">
              <RouterLink
                v-for="child in item.children"
                :key="child.path"
                :to="child.path"
                class="shell-subnav-item"
                :class="{ 'is-active': isActive(child.path) }"
              >
                <span>{{ t(child.labelKey) }}</span>
              </RouterLink>
            </div>
          </div>

          <RouterLink
            v-else
            :to="item.path"
            class="shell-nav-item"
            :class="{ 'is-active': isActive(item.path) }"
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
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="shell-header">
        <div class="header-heading">
          <h2>{{ headerTitle }}</h2>
        </div>

        <div class="header-actions">
          <ConnectionStatusStrip />
          <a-button danger :loading="shutdownPending" @click="shutdownDialogVisible = true">
            <template #icon><Power :size="16" /></template>
            {{ t('shell.shutdown') }}
          </a-button>
          <a-button type="primary" @click="handleLogout">
            <template #icon><LogOut :size="16" /></template>
            {{ t('shell.logout') }}
          </a-button>
        </div>
      </a-layout-header>

      <a-layout-content id="app-main" class="shell-main" tabindex="-1">
        <div class="sr-only" aria-live="polite">
          {{ shutdownRequested ? t('shell.shutdownRequestedLive') : '' }}
        </div>

        <a-alert
          v-if="shutdownRequested"
          class="section-gap"
          type="warning"
          show-icon
          :message="t('shell.shutdownRequestedTitle')"
          :description="t('shell.shutdownRequestedDescription')"
        />

        <div class="shell-body">
          <RouterView />
        </div>
      </a-layout-content>
    </a-layout>
  </a-layout>

  <a-modal
    v-model:open="shutdownDialogVisible"
    :get-container="false"
    :title="t('shell.shutdownConfirmTitle')"
    :confirm-loading="shutdownPending"
    :ok-text="t('shell.shutdownConfirmAction')"
    :cancel-text="t('dashboard.previewCancel')"
    :ok-button-props="{ danger: true }"
    @ok="confirmShutdown"
  >
    <p>{{ t('shell.shutdownConfirmBody') }}</p>
  </a-modal>
</template>
