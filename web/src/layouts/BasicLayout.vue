<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter, type RouteRecordRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  BulbOutlined,
  DownOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  PoweroffOutlined,
} from '@ant-design/icons-vue'

import { resolveMenuIcon } from '@/access/icons'
import { buildMenuItems, getMatchedBreadcrumbs, resolveRouteTitle, type AppMenuItem } from '@/access/menu'
import ConnectionStatusStrip from '@/components/ConnectionStatusStrip.vue'
import { notifyError, notifySuccess } from '@/adapter/feedback'
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
const menuItems = computed(() => buildMenuItems(adminRoutes[0]?.children ?? [], ''))
const breadcrumbItems = computed(() => getMatchedBreadcrumbs(route.matched))

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

const headerTitle = computed(() => resolveCurrentTabTitle() || t('app.consoleName'))
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

  <a-layout class="admin-layout">
    <a-layout-sider
      breakpoint="lg"
      class="admin-layout__sider"
      :collapsed="siderCollapsed"
      :collapsed-width="88"
      :trigger="null"
      theme="dark"
      width="264"
    >
      <button type="button" class="admin-layout__brand" @click="navigateTo('/')">
        <span class="admin-layout__brand-mark">R</span>
        <span v-if="!siderCollapsed" class="admin-layout__brand-copy">
          <strong>RayleaBot</strong>
          <small>{{ t('app.consoleName') }}</small>
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
        <small>{{ t('app.consoleName') }}</small>
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
      <a-layout-header class="admin-layout__header">
        <div class="admin-layout__header-main">
          <div class="admin-layout__header-left">
            <a-button class="admin-layout__icon-button desktop-only" type="text" @click="uiShellStore.toggleSider()">
              <template #icon>
                <MenuUnfoldOutlined v-if="siderCollapsed" />
                <MenuFoldOutlined v-else />
              </template>
            </a-button>
            <a-button class="admin-layout__icon-button mobile-only" type="text" @click="uiShellStore.setMobileMenuOpen(true)">
              <template #icon>
                <MenuOutlined />
              </template>
            </a-button>

            <div class="admin-layout__title-block">
              <strong>{{ headerTitle }}</strong>
              <small>{{ t('shell.headerSubtitle') }}</small>
            </div>
          </div>

          <div class="admin-layout__header-right">
            <ConnectionStatusStrip />
            <a-button class="admin-layout__icon-button" type="text" @click="uiShellStore.toggleThemeMode()">
              <template #icon>
                <BulbOutlined />
              </template>
            </a-button>
            <a-button danger @click="shutdownDialogVisible = true">
              <template #icon><PoweroffOutlined /></template>
              {{ t('shell.shutdown') }}
            </a-button>
            <a-dropdown>
              <a-button>
                {{ t('shell.account') }}
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
