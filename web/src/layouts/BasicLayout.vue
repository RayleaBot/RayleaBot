<script setup lang="ts">
import { computed, defineComponent, h, markRaw, nextTick, onBeforeUnmount, onMounted, ref, resolveDynamicComponent, watch } from 'vue'
import type { Component as VueComponent } from 'vue'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded, type RouteRecordRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  BellOutlined,
  DownOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuOutlined,
  MenuUnfoldOutlined,
  MoreOutlined,
  PoweroffOutlined,
  ReloadOutlined,
  RightOutlined,
  SearchOutlined,
  SettingOutlined,
  TranslationOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'

import { resolveMenuIcon } from '@/access/icons'
import {
  buildMenuItems,
  collectNavigationItems,
  resolveRouteEntryPath,
  resolveRouteTitle,
  type AppMenuItem,
} from '@/access/menu'
import { notifyError, notifyInfo, notifySuccess } from '@/adapter/feedback'
import PreferencesDrawer from '@/components/shell/PreferencesDrawer.vue'
import RouteSearchPanel from '@/components/shell/RouteSearchPanel.vue'
import ThemeToggleSwitch from '@/components/shell/ThemeToggleSwitch.vue'
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

const {
  effectiveCachedViewNames,
  mobileMenuOpen,
  preferences,
  routeLoading,
  searchOpen,
  siderCollapsed,
  tabs,
} = storeToRefs(uiShellStore)
const { shutdownPending, shutdownRequested } = storeToRefs(systemStore)

const shutdownDialogVisible = ref(false)
const isFullscreen = ref(false)
const reducedMotion = ref(false)
const openMenuKeys = ref<string[]>([])
let reducedMotionMediaQuery: MediaQueryList | null = null

const menuItems = computed(() => buildMenuItems(adminRoutes[0]?.children ?? [], ''))
const staticNavigationItems = collectNavigationItems(adminRoutes[0]?.children ?? [], '')
const navigationItems = computed(() => {
  const fromTabs = tabs.value.map((item) => ({
    icon: resolveTabItemIconName(item),
    key: `tab:${item.path}`,
    path: item.path,
    title: item.title,
  }))
  const byPath = new Map<string, (typeof staticNavigationItems)[number]>()

  for (const item of [...fromTabs, ...staticNavigationItems]) {
    byPath.set(item.path, item)
  }

  return Array.from(byPath.values())
})
interface AppBreadcrumbItem {
  current: boolean
  key: string
  path: string
  title: string
}

const siderTheme = computed(() => (preferences.value.themeMode === 'dark' ? 'dark' : 'light'))
const themeToggleLabel = computed(() => (
  preferences.value.themeMode === 'dark' ? t('shell.switchLightTheme') : t('shell.switchDarkTheme')
))
const fullscreenLabel = computed(() => (
  isFullscreen.value ? t('shell.exitFullscreen') : t('shell.enterFullscreen')
))
const currentTabPath = computed(() => resolveTabPath(route))
const currentTab = computed(() => tabs.value.find((item) => item.path === currentTabPath.value) ?? null)
const activeTabIndex = computed(() => tabs.value.findIndex((item) => item.path === currentTabPath.value))
const effectiveTransitionName = computed(() => {
  if (preferences.value.pageTransition === 'none') {
    return 'route-none'
  }

  if (reducedMotion.value) {
    return 'route-fade'
  }

  return preferences.value.pageTransition === 'fade' ? 'route-fade' : 'route-fade-slide'
})
const currentRouteViewName = computed(() => resolveRouteViewIdentity(route))
const routeStageRegistry = new Map<string, VueComponent>()
const breadcrumbItems = computed<AppBreadcrumbItem[]>(() => {
  const seen = new Set<string>()
  const items = route.matched
    .map((record) => {
      const title = resolveRouteTitle(record.meta)
      return {
        key: String(record.name ?? `${record.path}:${title}`),
        path: resolveBreadcrumbPath(record),
        title,
      }
    })
    .filter((item) => {
      if (!item.title) {
        return false
      }

      const key = `${item.path}:${item.title}`
      if (seen.has(key)) {
        return false
      }

      seen.add(key)
      return true
    })

  return items.map((item, index) => ({
    ...item,
    current: index === items.length - 1,
  }))
})
const hasMultiBreadcrumb = computed(() => breadcrumbItems.value.length > 1)

function getRouteStageComponent(viewRoute: RouteLocationNormalizedLoaded) {
  const stageName = String(viewRoute.name ?? viewRoute.path)
  const cached = routeStageRegistry.get(stageName)
  if (cached) {
    return cached
  }

  const stageComponent = markRaw(defineComponent({
    name: stageName,
    props: {
      routeComponent: {
        required: true,
        type: [Function, Object, String],
      },
    },
    setup(props) {
      const stageRouteComponent = props.routeComponent

      return () => h(
        'div',
        { class: 'admin-layout__route-stage' },
        [h(resolveDynamicComponent(stageRouteComponent))],
      )
    },
  }))

  routeStageRegistry.set(stageName, stageComponent)
  return stageComponent
}

function resolveRouteViewIdentity(viewRoute: Pick<RouteLocationNormalizedLoaded, 'matched' | 'name' | 'path'>) {
  const leafMeta = viewRoute.matched.at(-1)?.meta ?? null
  if (typeof leafMeta?.viewKey === 'string' && leafMeta.viewKey) {
    return leafMeta.viewKey
  }

  return String(viewRoute.name ?? viewRoute.path)
}

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

function getLeafMatchedRecord(viewRoute: RouteLocationNormalizedLoaded) {
  return viewRoute.matched.at(-1) ?? null
}

function getLeafRouteMeta(viewRoute: RouteLocationNormalizedLoaded) {
  return getLeafMatchedRecord(viewRoute)?.meta ?? null
}

function resolveRouteIconName(meta?: Record<string, unknown> | null) {
  return typeof meta?.icon === 'string' && meta.icon ? meta.icon : undefined
}

function resolveResolvedRouteIconName(viewRoute: Pick<RouteLocationNormalizedLoaded, 'matched'>) {
  const leafMeta = viewRoute.matched.at(-1)?.meta ?? null
  const directIcon = resolveRouteIconName(leafMeta)
  if (directIcon) {
    return directIcon
  }

  const activePath = typeof leafMeta?.activePath === 'string' && leafMeta.activePath
    ? leafMeta.activePath
    : null
  if (!activePath) {
    return undefined
  }

  try {
    const activeRoute = router.resolve(activePath)
    return resolveRouteIconName(activeRoute.matched.at(-1)?.meta ?? null)
  } catch {
    return undefined
  }
}

function resolveTabItemIconName(item: ShellTabItem) {
  if (item.icon) {
    return item.icon
  }

  try {
    return resolveResolvedRouteIconName(router.resolve(item.path))
  } catch {
    return undefined
  }
}

function resolveTabItemIconComponent(item: ShellTabItem) {
  return resolveMenuIcon(resolveTabItemIconName(item))
}

function resolveLeafRouteComponent(viewRoute: RouteLocationNormalizedLoaded) {
  return getLeafMatchedRecord(viewRoute)?.components?.default ?? null
}

function resolveTabPath(viewRoute: RouteLocationNormalizedLoaded) {
  return resolveRouteEntryPath(getLeafRouteMeta(viewRoute), viewRoute.path)
}

function resolveBreadcrumbPath(record: RouteLocationNormalizedLoaded['matched'][number]) {
  if (!record.redirect || typeof record.redirect === 'function') {
    return record.path
  }

  try {
    return router.resolve(record.redirect).path
  } catch {
    return record.path
  }
}

function collectAffixTabs(items: RouteRecordRaw[], parentPath = ''): ShellTabItem[] {
  return items.flatMap((item) => {
    const routePath = joinRoutePath(parentPath, item.path)
    const path = resolveRouteEntryPath(item.meta, routePath)
    const title = resolveRouteTitle(item.meta)
    const children = item.children ? collectAffixTabs(item.children, routePath) : []
    const current = item.meta?.affixTab && title && item.name
      ? [{
        affix: true,
        fullPath: path,
        icon: resolveRouteIconName(item.meta),
        keepAlive: Boolean(item.meta?.keepAlive),
        name: String(item.name),
        path,
        title,
      }]
      : []

    return [...current, ...children]
  })
}

const affixTabs = collectAffixTabs(adminRoutes)
uiShellStore.syncTabs(affixTabs)

function resolveCurrentTabTitle(viewRoute: RouteLocationNormalizedLoaded) {
  if (viewRoute.name === 'plugin-detail') {
    const pluginId = viewRoute.params.id
    return typeof pluginId === 'string' && pluginId ? pluginId : resolveRouteTitle(getLeafRouteMeta(viewRoute))
  }

  return resolveRouteTitle(getLeafRouteMeta(viewRoute))
}

function resolveCurrentTabIcon(viewRoute: RouteLocationNormalizedLoaded) {
  return resolveResolvedRouteIconName(viewRoute)
}

function flattenMenu(items: AppMenuItem[], lineage: Array<{ key: string; path: string }> = []) {
  return items.flatMap((item) => {
    const currentLineage = [...lineage, { key: item.key, path: item.path }]
    const current = [{ item, lineage: currentLineage }]
    return item.children ? [...current, ...flattenMenu(item.children, currentLineage)] : current
  })
}

const flattenedMenu = flattenMenu(menuItems.value)
const menuLineage = computed(() => {
  const leafMeta = getLeafRouteMeta(route)
  const targetPath = typeof leafMeta?.activePath === 'string' && leafMeta.activePath
    ? leafMeta.activePath
    : route.path

  return flattenedMenu.find(({ item }) => item.path === targetPath)?.lineage ?? []
})

const selectedMenuKeys = computed(() => {
  const last = menuLineage.value.at(-1)
  return last ? [last.key] : []
})

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
    nextTick(() => {
      const leafRecord = getLeafMatchedRecord(route)
      const leafMeta = getLeafRouteMeta(route)

      if (!leafRecord || leafMeta?.hideInTab || !route.name) {
        return
      }

      const title = resolveCurrentTabTitle(route)
      if (!title) {
        return
      }

      if (typeof leafMeta?.viewKey === 'string' && leafMeta.viewKey) {
        uiShellStore.removeTabsByName(String(route.name), { exceptPath: resolveTabPath(route) })
      }

      uiShellStore.upsertTab({
        affix: Boolean(leafMeta?.affixTab),
        fullPath: route.fullPath,
        icon: resolveCurrentTabIcon(route),
        keepAlive: Boolean(leafMeta?.keepAlive),
        name: String(route.name),
        path: resolveTabPath(route),
        title,
      })
      uiShellStore.setMobileMenuOpen(false)
    })
  },
  { immediate: true },
)

function navigateTo(path: string) {
  void router.push(path)
}

function handleOpenChange(keys: string[]) {
  openMenuKeys.value = keys
}

function onTabChange(targetKey: string) {
  const targetTab = tabs.value.find((item) => item.path === targetKey)
  void router.push(targetTab?.fullPath ?? targetKey)
}

function closeTab(targetPath: string) {
  const items = tabs.value
  const targetIndex = items.findIndex((item) => item.path === targetPath)
  if (targetIndex < 0) {
    return
  }

  const closingCurrent = currentTabPath.value === targetPath
  const fallback = items[targetIndex - 1] ?? items[targetIndex + 1] ?? affixTabs[0]
  uiShellStore.removeTab(targetPath)

  if (closingCurrent && fallback) {
    void router.push(fallback.fullPath)
  }
}

function onTabEdit(targetKey: string | MouseEvent, action: 'add' | 'remove') {
  if (action !== 'remove' || typeof targetKey !== 'string') {
    return
  }

  closeTab(targetKey)
}

async function refreshCurrentRoute() {
  await uiShellStore.refreshView(currentRouteViewName.value)
}

function closeOtherTabs() {
  uiShellStore.closeOtherTabs(currentTabPath.value)
}

function closeTabsToLeft() {
  uiShellStore.closeTabsToLeft(currentTabPath.value)
}

function closeTabsToRight() {
  uiShellStore.closeTabsToRight(currentTabPath.value)
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

function onSearchNavigate(path: string) {
  navigateTo(path)
}

function onSearchOpenUpdate(open: boolean) {
  if (open) {
    uiShellStore.openSearch()
  } else {
    uiShellStore.closeSearch()
  }
}

function handleTabAction(key: string) {
  switch (key) {
    case 'refresh':
      void refreshCurrentRoute()
      return
    case 'close-current':
      if (currentTab.value && !currentTab.value.affix) {
        closeTab(currentTab.value.path)
      }
      return
    case 'close-other':
      closeOtherTabs()
      return
    case 'close-left':
      closeTabsToLeft()
      return
    case 'close-right':
      closeTabsToRight()
      return
    default:
      return
  }
}

const tabActionItems = computed(() => [
  { key: 'refresh', label: t('shell.tabActions.refresh') },
  {
    disabled: !currentTab.value || currentTab.value.affix,
    key: 'close-current',
    label: t('shell.tabActions.closeCurrent'),
  },
  { key: 'close-other', label: t('shell.tabActions.closeOther') },
  {
    disabled: activeTabIndex.value <= 0,
    key: 'close-left',
    label: t('shell.tabActions.closeLeft'),
  },
  {
    disabled: activeTabIndex.value < 0 || activeTabIndex.value >= tabs.value.length - 1,
    key: 'close-right',
    label: t('shell.tabActions.closeRight'),
  },
])

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) {
    return false
  }

  return target.isContentEditable
    || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

function handleGlobalShortcut(event: KeyboardEvent) {
  const targetIsEditable = isEditableTarget(event.target)
  const commandKey = event.metaKey || event.ctrlKey

  if (commandKey && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    uiShellStore.openSearch()
    return
  }

  if (event.altKey && event.shiftKey && event.key.toLowerCase() === 's') {
    event.preventDefault()
    uiShellStore.openSettings()
    return
  }

  if (targetIsEditable) {
    return
  }

  if (commandKey && event.shiftKey && event.key.toLowerCase() === 'w') {
    event.preventDefault()
    closeOtherTabs()
    return
  }

  if (commandKey && event.key.toLowerCase() === 'w') {
    if (!currentTab.value || currentTab.value.affix) {
      return
    }

    event.preventDefault()
    closeTab(currentTab.value.path)
  }
}

function handleReducedMotionPreference(event?: MediaQueryList | MediaQueryListEvent) {
  if ('matches' in (event ?? {})) {
    reducedMotion.value = Boolean((event as MediaQueryList | MediaQueryListEvent).matches)
  }
}

function getRouteViewKey(viewRoute: RouteLocationNormalizedLoaded) {
  const viewIdentity = resolveRouteViewIdentity(viewRoute)
  if (typeof getLeafRouteMeta(viewRoute)?.viewKey === 'string' && getLeafRouteMeta(viewRoute)?.viewKey) {
    return `${viewIdentity}:${uiShellStore.getRefreshKey(viewIdentity)}`
  }

  return `${viewIdentity}:${viewRoute.path}:${uiShellStore.getRefreshKey(viewIdentity)}`
}

onMounted(() => {
  syncFullscreenState()
  if (typeof document !== 'undefined') {
    document.addEventListener('fullscreenchange', syncFullscreenState)
    document.addEventListener('keydown', handleGlobalShortcut)
  }

  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    reducedMotionMediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    reducedMotion.value = reducedMotionMediaQuery.matches
    reducedMotionMediaQuery.addEventListener('change', handleReducedMotionPreference)
  }
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') {
    document.removeEventListener('fullscreenchange', syncFullscreenState)
    document.removeEventListener('keydown', handleGlobalShortcut)
  }

  if (reducedMotionMediaQuery) {
    reducedMotionMediaQuery.removeEventListener('change', handleReducedMotionPreference)
    reducedMotionMediaQuery = null
  }
})
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <a-layout class="admin-layout" :class="[`admin-layout--${preferences.density}`]">
    <a-layout-sider
      breakpoint="lg"
      class="admin-layout__sider"
      :collapsed="siderCollapsed"
      :collapsed-width="64"
      :trigger="null"
      :theme="siderTheme"
      width="224"
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
      width="240"
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
      <a-layout-header :class="['admin-layout__header', { 'is-static': !preferences.fixedHeader }]" data-testid="app-header">
        <div v-if="preferences.pageLoading" class="admin-layout__progress-track">
          <div :class="['admin-layout__progress-bar', { 'is-active': routeLoading }]" />
        </div>

        <div class="admin-layout__header-main">
          <div class="admin-layout__header-left">
            <a-button
              class="admin-layout__icon-button admin-layout__nav-trigger desktop-only"
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
              class="admin-layout__icon-button admin-layout__nav-trigger mobile-only"
              type="text"
              :aria-label="t('shell.openMenu')"
              @click="uiShellStore.setMobileMenuOpen(true)"
            >
              <template #icon>
                <MenuOutlined />
              </template>
            </a-button>

            <div
              v-if="preferences.breadcrumb && breadcrumbItems.length"
              :class="[
                'admin-layout__header-breadcrumb',
                hasMultiBreadcrumb
                  ? 'admin-layout__header-breadcrumb--multi'
                  : 'admin-layout__header-breadcrumb--single',
              ]"
              data-testid="header-breadcrumb"
            >
              <nav class="admin-layout__breadcrumb-nav" aria-label="面包屑">
                <ol class="admin-layout__breadcrumb-list">
                  <li
                  v-for="item in breadcrumbItems"
                  :key="item.key"
                  :class="[
                    'admin-layout__breadcrumb-item',
                    {
                      'admin-layout__breadcrumb-item--ancestor': !item.current,
                      'admin-layout__breadcrumb-item--current': item.current,
                    },
                  ]"
                  >
                    <RouterLink
                      v-if="!item.current"
                      :to="item.path"
                      class="ant-breadcrumb-link admin-layout__breadcrumb-link"
                    >
                    <span class="admin-layout__breadcrumb-link-text">{{ item.title }}</span>
                    </RouterLink>
                    <span v-else class="ant-breadcrumb-link admin-layout__breadcrumb-current">
                      <span class="admin-layout__breadcrumb-current-text">{{ item.title }}</span>
                    </span>

                    <span v-if="!item.current" class="ant-breadcrumb-separator admin-layout__breadcrumb-separator" aria-hidden="true">
                      <RightOutlined />
                    </span>
                  </li>
                </ol>
              </nav>
            </div>
          </div>

          <div class="admin-layout__header-right">
            <div class="admin-layout__header-tools desktop-only">
              <a-tooltip :title="t('shell.search')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.search')"
                  data-testid="header-search"
                  @click="uiShellStore.openSearch()"
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
                  @click="uiShellStore.openSettings()"
                >
                  <template #icon>
                    <SettingOutlined />
                  </template>
                </a-button>
              </a-tooltip>

              <a-popover placement="bottom" trigger="click">
                <template #content>
                  <div class="admin-layout__pending-panel">{{ t('shell.languagePending') }}</div>
                </template>

                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.language')"
                  data-testid="header-language"
                >
                  <template #icon>
                    <TranslationOutlined />
                  </template>
                </a-button>
              </a-popover>

              <a-popover placement="bottomRight" trigger="click">
                <template #content>
                  <div class="admin-layout__pending-panel">{{ t('shell.notificationsPending') }}</div>
                </template>

                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  :aria-label="t('shell.notifications')"
                  data-testid="header-notifications"
                >
                  <template #icon>
                    <BellOutlined />
                  </template>
                </a-button>
              </a-popover>

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
              <ThemeToggleSwitch
                class="admin-layout__theme-toggle"
                :checked="preferences.themeMode === 'dark'"
                :label="themeToggleLabel"
                test-id="theme-toggle"
                @toggle="uiShellStore.toggleThemeMode()"
              />
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

        <div v-if="preferences.chromeTabbar" class="admin-layout__tabbar">
          <div class="admin-layout__tabbar-main">
            <a-tabs
              hide-add
              size="small"
              type="editable-card"
              :active-key="currentTabPath"
              @change="onTabChange"
              @edit="onTabEdit"
            >
              <a-tab-pane
                v-for="item in tabs"
                :key="item.path"
                :closable="!item.affix"
              >
                <template #tab>
                  <span class="admin-layout__tab-label" :data-icon="resolveTabItemIconName(item) || undefined">
                    <component
                      :is="resolveTabItemIconComponent(item)"
                      v-if="resolveTabItemIconComponent(item)"
                      class="admin-layout__tab-icon"
                    />
                    <span>{{ item.title }}</span>
                  </span>
                </template>
              </a-tab-pane>
            </a-tabs>

            <div class="admin-layout__tabbar-actions">
              <a-tooltip :title="t('shell.tabActions.refresh')">
                <a-button
                  class="admin-layout__icon-button"
                  type="text"
                  data-testid="tabbar-refresh"
                  @click="refreshCurrentRoute"
                >
                  <template #icon>
                    <ReloadOutlined />
                  </template>
                </a-button>
              </a-tooltip>

              <a-dropdown placement="bottomRight">
                <a-button class="admin-layout__icon-button" type="text" data-testid="tabbar-actions">
                  <template #icon>
                    <MoreOutlined />
                  </template>
                </a-button>

                <template #overlay>
                  <a-menu @click="handleTabAction($event.key)">
                    <a-menu-item
                      v-for="item in tabActionItems"
                      :key="item.key"
                      :disabled="item.disabled"
                    >
                      {{ item.label }}
                    </a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>
            </div>
          </div>
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

        <RouterView v-slot="{ route: currentViewRoute }">
          <Transition :name="effectiveTransitionName" mode="out-in">
            <KeepAlive :include="effectiveCachedViewNames">
              <component
                :is="getRouteStageComponent(currentViewRoute)"
                v-if="resolveLeafRouteComponent(currentViewRoute)"
                :key="getRouteViewKey(currentViewRoute)"
                :route-component="resolveLeafRouteComponent(currentViewRoute)"
              />
            </KeepAlive>
          </Transition>
        </RouterView>
      </a-layout-content>
    </a-layout>
  </a-layout>

  <RouteSearchPanel
    :items="navigationItems"
    :open="searchOpen"
    @navigate="onSearchNavigate"
    @update:open="onSearchOpenUpdate"
  />
  <PreferencesDrawer />

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
