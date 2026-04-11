<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter, type RouteRecordRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  BellOutlined,
  BulbOutlined,
  DownOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  PoweroffOutlined,
  SearchOutlined,
  SettingOutlined,
  TranslationOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'

import { resolveMenuIcon } from '@/access/icons'
import { buildMenuItems, getMatchedBreadcrumbs, resolveRouteTitle, type AppMenuItem } from '@/access/menu'
import { notifyError, notifyInfo, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { adminRoutes } from '@/router/routes/modules/admin'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useUiShellStore, type ShellTabItem } from '@/stores/ui-shell'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()
const uiShellStore = useUiShellStore()

const { mobileMenuOpen, preferences, siderCollapsed, tabs } = storeToRefs(uiShellStore)
const { shutdownPending, shutdownRequested } = storeToRefs(systemStore)
const shutdownDialogVisible = ref(false)
const isFullscreen = ref(false)
const menuItems = computed(() => buildMenuItems(adminRoutes[0]?.children ?? [], ''))
const breadcrumbItems = computed(() => getMatchedBreadcrumbs(route.matched))
const siderTheme = computed(() => (preferences.value.themeMode === 'dark' ? 'dark' : 'light'))
const themeToggleLabel = computed(() => (
  preferences.value.themeMode === 'dark' ? t('shell.switchLightTheme') : t('shell.switchDarkTheme')
))
const fullscreenLabel = computed(() => (
  isFullscreen.value ? t('shell.exitFullscreen') : t('shell.enterFullscreen')
))

function joinRoutePath(parentPath: string, childPath: string) {
  if (!childPath) {
    return parentPath || '/'
  }

  if (childPath.startsWith('/')) {
    return childPath
  }

  const prefix = parentPath === '/' ? '' : parentPath
  return `${prefix}/${childPath}` || '/'
}

function collectAffixTabs(items: RouteRecordRaw[], parentPath = ''): ShellTabItem[] {
  return items.flatMap((item) => {
    const path = joinRoutePath(parentPath, item.path)
    const title = resolveRouteTitle(item.meta)
    const children = item.children ? collectAffixTabs(item.children, path) : []
    const current = item.meta?.affixTab && title
      ? [{
          affix: true,
          fullPath: path,
          name: String(item.name ?? path),
          path,
          title,
        }]
      : []

    return [...current, ...children]
  })
}

const affixTabs = collectAffixTabs(adminRoutes)
uiShellStore.syncTabs(affixTabs)

function resolveCurrentTabTitle() {
  if (route.name === 'plugin-detail') {
    const pluginId = route.params.id
    return typeof pluginId === 'string' && pluginId ? pluginId : resolveRouteTitle(route.meta)
  }

  return resolveRouteTitle(route.meta)
}

function flattenMenu(items: AppMenuItem[], lineage: Array<{ key: string; path: string }> = []) {
  return items.flatMap((item) => {
    const currentLineage = [...lineage, { key: item.key, path: item.path }]
    const current = [{ item, lineage: currentLineage }]
    return item.children ? [...current, ...flattenMenu(item.children, currentLineage)] : current
  })
}

const menuLineage = computed(() => {
  const targetPath = typeof route.meta.activePath === 'string' && route.meta.activePath
    ? route.meta.activePath
    : route.path

  const flattened = flattenMenu(menuItems.value)
  return flattened.find(({ item }) => item.path === targetPath)?.lineage ?? []
})

const selectedMenuKeys = computed(() => {
  const last = menuLineage.value.at(-1)
  return last ? [last.key] : []
})

const openMenuKeys = ref<string[]>([])

watch(
  menuLineage,
  (lineage) => {
    openMenuKeys.value = lineage.slice(0, -1).map((item) => item.key)
  },
  { immediate: true },
)

watch(
  () => route.fullPath,
  () => {
    if (route.meta.hideInTab || !route.name) {
      return
    }

    const title = resolveCurrentTabTitle()
    if (!title) {
      return
    }

    uiShellStore.upsertTab({
      affix: Boolean(route.meta.affixTab),
      fullPath: route.fullPath,
      name: String(route.name),
      path: route.path,
      title,
    })
    uiShellStore.setMobileMenuOpen(false)
  },
  { immediate: true },
)

const tabItems = computed(() => tabs.value.map((item) => ({
  closable: !item.affix,
  key: item.path,
  label: item.title,
})))

async function navigateTo(path: string) {
  await router.push(path)
}

function handleOpenChange(keys: string[]) {
  openMenuKeys.value = keys
}

function onTabChange(targetKey: string) {
  void router.push(targetKey)
}

function closeTab(targetPath: string) {
  const items = tabs.value
  const targetIndex = items.findIndex((item) => item.path === targetPath)
  if (targetIndex < 0) {
    return
  }

  const closingCurrent = route.path === targetPath
  const fallback = items[targetIndex - 1] ?? items[targetIndex + 1] ?? affixTabs[0]
  uiShellStore.removeTab(targetPath)

  if (closingCurrent && fallback) {
    void router.push(fallback.path)
  }
}

function onTabEdit(targetKey: string | MouseEvent, action: 'add' | 'remove') {
  if (action !== 'remove' || typeof targetKey !== 'string') {
    return
  }

  closeTab(targetKey)
}

function syncFullscreenState() {
  if (typeof document === 'undefined') {
    isFullscreen.value = false
    return
  }

  isFullscreen.value = Boolean(document.fullscreenElement)
}

function notifyFeaturePending(feature: string) {
  notifyInfo(t('shell.featurePending', { feature }))
}

async function toggleFullscreen() {
  if (typeof document === 'undefined' || typeof document.documentElement.requestFullscreen !== 'function') {
    notifyInfo(t('shell.fullscreenUnsupported'))
    return
  }

  try {
    if (document.fullscreenElement) {
      await document.exitFullscreen()
    } else {
      await document.documentElement.requestFullscreen()
    }
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  } finally {
    syncFullscreenState()
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

onMounted(() => {
  syncFullscreenState()
  if (typeof document !== 'undefined') {
    document.addEventListener('fullscreenchange', syncFullscreenState)
  }
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') {
    document.removeEventListener('fullscreenchange', syncFullscreenState)
  }
})
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <a-layout class="admin-layout">
    <a-layout-sider
      breakpoint="lg"
      class="admin-layout__sider"
      :collapsed="siderCollapsed"
      :collapsed-width="88"
      :trigger="null"
      :theme="siderTheme"
      width="264"
      data-testid="app-sider"
    >
      <button type="button" class="admin-layout__brand" @click="navigateTo('/')">
        <span class="admin-layout__brand-mark">R</span>
        <span v-if="!siderCollapsed" class="admin-layout__brand-copy">
          <strong>RayleaBot</strong>
        </span>
      </button>

      <div class="admin-layout__sider-scroll">
        <a-menu
          mode="inline"
          :inline-collapsed="siderCollapsed"
          :open-keys="openMenuKeys"
          :selected-keys="selectedMenuKeys"
          @openChange="handleOpenChange"
        >
          <template v-for="item in menuItems" :key="item.key">
            <a-sub-menu v-if="item.children?.length" :key="item.key">
              <template #title>
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                  <span>{{ item.title }}</span>
                </span>
              </template>

              <a-menu-item
                v-for="child in item.children"
                :key="child.key"
                @click="navigateTo(child.path)"
              >
                <span class="admin-layout__menu-label">
                  <component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" class="admin-layout__menu-icon" />
                  <span>{{ child.title }}</span>
                </span>
              </a-menu-item>
            </a-sub-menu>

            <a-menu-item v-else :key="item.key" @click="navigateTo(item.path)">
              <span class="admin-layout__menu-label">
                <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                <span>{{ item.title }}</span>
              </span>
            </a-menu-item>
          </template>
        </a-menu>
      </div>
    </a-layout-sider>

    <a-drawer
      :open="mobileMenuOpen"
      class="admin-layout__mobile-drawer"
      placement="left"
      width="280"
      @close="uiShellStore.setMobileMenuOpen(false)"
    >
      <div class="admin-layout__mobile-brand">
        <strong>RayleaBot</strong>
      </div>

      <a-menu mode="inline" :selected-keys="selectedMenuKeys">
        <template v-for="item in menuItems" :key="item.key">
          <a-sub-menu v-if="item.children?.length" :key="item.key">
            <template #title>
              <span class="admin-layout__menu-label">
                <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
                <span>{{ item.title }}</span>
              </span>
            </template>
            <a-menu-item
              v-for="child in item.children"
              :key="child.key"
              @click="navigateTo(child.path)"
            >
              <span class="admin-layout__menu-label">
                <component :is="resolveMenuIcon(child.icon)" v-if="resolveMenuIcon(child.icon)" class="admin-layout__menu-icon" />
                <span>{{ child.title }}</span>
              </span>
            </a-menu-item>
          </a-sub-menu>

          <a-menu-item v-else :key="item.key" @click="navigateTo(item.path)">
            <span class="admin-layout__menu-label">
              <component :is="resolveMenuIcon(item.icon)" v-if="resolveMenuIcon(item.icon)" class="admin-layout__menu-icon" />
              <span>{{ item.title }}</span>
            </span>
          </a-menu-item>
        </template>
      </a-menu>
    </a-drawer>

    <a-layout>
      <a-layout-header class="admin-layout__header" data-testid="app-header">
        <div class="admin-layout__header-main">
          <div class="admin-layout__header-left">
            <a-button
              class="admin-layout__icon-button desktop-only"
              type="text"
              :aria-label="t('shell.toggleSidebar')"
              @click="uiShellStore.toggleSider()"
            >
              <template #icon>
                <MenuUnfoldOutlined v-if="siderCollapsed" />
                <MenuFoldOutlined v-else />
              </template>
            </a-button>
            <a-button
              class="admin-layout__icon-button mobile-only"
              type="text"
              :aria-label="t('shell.openMenu')"
              @click="uiShellStore.setMobileMenuOpen(true)"
            >
              <template #icon>
                <MenuOutlined />
              </template>
            </a-button>
          </div>

          <div class="admin-layout__header-right">
            <div class="admin-layout__header-tools desktop-only">
              <a-tooltip :title="t('shell.search')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.search')"
                  data-testid="header-search"
                  @click="notifyFeaturePending(t('shell.search'))"
                >
                  <template #icon>
                    <SearchOutlined />
                  </template>
                </a-button>
              </a-tooltip>
              <a-tooltip :title="t('shell.settings')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.settings')"
                  data-testid="header-settings"
                  @click="notifyFeaturePending(t('shell.settings'))"
                >
                  <template #icon>
                    <SettingOutlined />
                  </template>
                </a-button>
              </a-tooltip>
              <a-tooltip :title="t('shell.language')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.language')"
                  data-testid="header-language"
                  @click="notifyFeaturePending(t('shell.language'))"
                >
                  <template #icon>
                    <TranslationOutlined />
                  </template>
                </a-button>
              </a-tooltip>
              <a-tooltip :title="t('shell.notifications')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.notifications')"
                  data-testid="header-notifications"
                  @click="notifyFeaturePending(t('shell.notifications'))"
                >
                  <template #icon>
                    <BellOutlined />
                  </template>
                </a-button>
              </a-tooltip>
              <a-tooltip :title="fullscreenLabel">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="fullscreenLabel"
                  data-testid="header-fullscreen"
                  @click="toggleFullscreen"
                >
                  <template #icon>
                    <FullscreenExitOutlined v-if="isFullscreen" />
                    <FullscreenOutlined v-else />
                  </template>
                </a-button>
              </a-tooltip>
            </div>

            <a-tooltip :title="themeToggleLabel">
              <a-button
                class="admin-layout__icon-button"
                type="text"
                :aria-label="themeToggleLabel"
                data-testid="theme-toggle"
                @click="uiShellStore.toggleThemeMode()"
              >
                <template #icon>
                  <BulbOutlined />
                </template>
              </a-button>
            </a-tooltip>

            <a-button class="admin-layout__shutdown-button" danger @click="shutdownDialogVisible = true">
              <template #icon><PoweroffOutlined /></template>
              <span class="desktop-only">{{ t('shell.shutdown') }}</span>
            </a-button>

            <a-dropdown placement="bottomRight">
              <a-button class="admin-layout__account-button">
                <UserOutlined />
                <span class="desktop-only">{{ t('shell.account') }}</span>
                <DownOutlined />
              </a-button>

              <template #overlay>
                <a-menu>
                  <a-menu-item key="logout" @click="handleLogout">
                    <LogoutOutlined />
                    {{ t('shell.logout') }}
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </div>
        </div>

        <div v-if="preferences.breadcrumb" class="admin-layout__breadcrumb-row">
          <a-breadcrumb>
            <a-breadcrumb-item v-for="item in breadcrumbItems" :key="item.path">
              <RouterLink v-if="item.path !== route.path" :to="item.path">{{ item.title }}</RouterLink>
              <span v-else>{{ item.title }}</span>
            </a-breadcrumb-item>
          </a-breadcrumb>
        </div>

        <div v-if="preferences.chromeTabbar" class="admin-layout__tabbar">
          <a-tabs
            hide-add
            size="small"
            type="editable-card"
            :active-key="route.path"
            :items="tabItems"
            @change="onTabChange"
            @edit="onTabEdit"
          />
        </div>
      </a-layout-header>

      <a-layout-content id="app-main" class="admin-layout__content" tabindex="-1">
        <a-alert
          v-if="shutdownRequested"
          class="admin-layout__shutdown-banner"
          type="warning"
          show-icon
          :message="t('shell.shutdownRequestedTitle')"
          :description="t('shell.shutdownRequestedDescription')"
        />

        <RouterView />
      </a-layout-content>
    </a-layout>
  </a-layout>

  <a-modal
    v-model:open="shutdownDialogVisible"
    :get-container="false"
    :title="t('shell.shutdownConfirmTitle')"
    :confirm-loading="shutdownPending"
    :ok-button-props="{ danger: true }"
    :ok-text="t('shell.shutdownConfirmAction')"
    :cancel-text="t('shell.cancel')"
    @ok="confirmShutdown"
  >
    <p>{{ t('shell.shutdownConfirmBody') }}</p>
  </a-modal>
</template>
