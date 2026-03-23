<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()

const { system, readiness } = storeToRefs(systemStore)

const navigationItems = [
  { index: '/', label: '系统状态' },
  { index: '/plugins', label: '插件' },
  { index: '/tasks', label: '任务' },
  { index: '/logs', label: '日志' },
  { index: '/config', label: '配置' },
]

const headerTitle = computed(() => route.meta.title ?? 'RayleaBot Web')
const statusLabel = computed(() => system.value?.status ?? 'unknown')
const readyLabel = computed(() => readiness.value?.status ?? 'unknown')

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
</script>

<template>
  <el-container class="shell-layout">
    <el-aside width="260px" class="shell-sidebar">
      <div class="brand-block">
        <div class="brand-eyebrow">RayleaBot</div>
        <h1>Management Surface</h1>
        <p>当前基于既有 HTTP / WebSocket contract 运行。</p>
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
          <span>System</span>
          <strong>{{ statusLabel }}</strong>
        </div>
        <div class="metric-pill">
          <span>Ready</span>
          <strong>{{ readyLabel }}</strong>
        </div>
      </div>
    </el-aside>

    <el-container>
      <el-header class="shell-header">
        <div>
          <div class="page-eyebrow">Control Plane</div>
          <h2>{{ headerTitle }}</h2>
        </div>

        <div class="header-actions">
          <ConnectionStatusStrip />
          <el-button type="primary" plain @click="handleLogout">
            退出登录
          </el-button>
        </div>
      </el-header>

      <el-main class="shell-main">
        <RouterView />
      </el-main>
    </el-container>
  </el-container>
</template>
