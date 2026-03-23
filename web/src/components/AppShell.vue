<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { ElMessage } from 'element-plus'

import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()

const { shutdownPending, shutdownRequested, system, readiness } = storeToRefs(systemStore)
const navDrawerOpen = ref(false)
const shutdownDialogVisible = ref(false)

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

async function confirmShutdown() {
  try {
    await systemStore.requestShutdown()
    shutdownDialogVisible.value = false
    ElMessage.success('停机请求已发送，服务将进入 graceful shutdown')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'shutdown failed')
  }
}
</script>

<template>
  <a class="skip-link" href="#app-main">跳到主内容</a>

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
          <el-button class="mobile-only" plain @click="navDrawerOpen = true">
            导航
          </el-button>
          <ConnectionStatusStrip />
          <el-button type="danger" plain :loading="shutdownPending" @click="shutdownDialogVisible = true">
            关闭服务
          </el-button>
          <el-button type="primary" plain @click="handleLogout">
            退出登录
          </el-button>
        </div>
      </el-header>

      <el-main id="app-main" class="shell-main" tabindex="-1">
        <div class="sr-only" aria-live="polite">
          {{ shutdownRequested ? '服务已收到关闭请求，连接断开属于预期行为。' : '' }}
        </div>

        <el-alert
          v-if="shutdownRequested"
          title="服务正在停止"
          type="warning"
          description="平台已接受 shutdown 请求，后续 WebSocket 断开或管理接口不可用属于预期行为。"
          show-icon
          class="section-gap"
        />

        <RouterView />
      </el-main>
    </el-container>
  </el-container>

  <el-drawer v-model="navDrawerOpen" direction="ltr" size="280px" :with-header="false" class="mobile-nav-drawer">
    <div class="brand-block">
      <div class="brand-eyebrow">RayleaBot</div>
      <h1>Management Surface</h1>
      <p>当前基于既有 HTTP / WebSocket contract 运行。</p>
    </div>

    <nav class="shell-nav drawer-nav" aria-label="Primary">
      <RouterLink
        v-for="item in navigationItems"
        :key="item.index"
        :to="item.index"
        class="shell-nav-item"
        :class="{ 'is-active': isActive(item.index) }"
        @click="navDrawerOpen = false"
      >
        {{ item.label }}
      </RouterLink>
    </nav>
  </el-drawer>

  <el-dialog v-model="shutdownDialogVisible" title="确认关闭服务" width="420px">
    <p>服务会先进入 graceful shutdown，再逐步停止管理接口与 WebSocket 连接。</p>

    <template #footer>
      <div class="table-actions">
        <el-button @click="shutdownDialogVisible = false">取消</el-button>
        <el-button type="danger" :loading="shutdownPending" @click="confirmShutdown">
          确认关闭
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>
