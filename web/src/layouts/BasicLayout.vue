<script setup lang="ts">
import { computed, defineComponent, h, markRaw, onBeforeUnmount, onMounted, provide, readonly, ref, resolveDynamicComponent, watch } from 'vue'
import type { Component as VueComponent } from 'vue'
import { useRoute, useRouter, type RouteLocationNormalizedLoaded, type RouteLocationRaw, type RouteRecordRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  XIcon,
  MinimizeIcon,
  MaximizeIcon,
  PanelLeftCloseIcon,
  MenuIcon,
  PanelLeftOpenIcon,
  EllipsisIcon,
  PowerIcon,
  ChevronRightIcon,
  SearchIcon,
  SettingsIcon,
} from '@lucide/vue'
import { TabsRoot, TabsList, TabsTrigger } from 'reka-ui'
import AppButton from '@/components/AppButton.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import AppDrawer from '@/components/AppDrawer.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'

import { resolveMenuIcon } from '@/access/icons'
import { createPluginCenterTab, isPluginCenterRoute, pluginCenterPath, projectPluginCenterMenu } from '@/access/plugin-center'
import {
  buildMenuItems,
  collectNavigationItems,
  resolveRouteEntryPath,
  resolveRouteTitle,
  type AppMenuItem,
} from '@/access/menu'
import { notifyError, notifyInfo, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import RayleaMark from '@/components/brand/RayleaMark.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import AppSidebarNavigation from '@/components/shell/AppSidebarNavigation.vue'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import PreferencesDrawer from '@/components/shell/PreferencesDrawer.vue'
import RouteSearchPanel from '@/components/shell/RouteSearchPanel.vue'
import SidebarAccountMenu from '@/components/shell/SidebarAccountMenu.vue'
import AccountCredentialsDialog from '@/components/shell/AccountCredentialsDialog.vue'
import { buildVersionLabel } from '@/lib/build-info'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { adminRoutes } from '@/router/routes/modules/admin'
import { usePluginsStore } from '@/stores/plugins'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useConfigStore } from '@/stores/config'
import { useUiShellStore, type ShellTabItem } from '@/stores/ui-shell'
import type { ThemeMode } from '@/preferences/app'
import { PAGE_TRANSITION_STAGE_KEY, type PageTransitionStage } from '@/layouts/usePageTransitionStage'
import { useWorkspaceTabs, type WorkspaceTabProjection } from '@/layouts/useWorkspaceTabs'
import {
  applyThemeWithMotion,
  cancelRouteFallbackMotion,
  isManagedViewTransitionActive,
  navigateWithMotion,
  runRouteFallbackMotion,
  subscribeRouteMotion,
  type PageMotionProfile,
  type ThemeMotionOrigin,
} from '@/motion/runtime'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()
const configStore = useConfigStore()
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
const accountDialogVisible = ref(false)
const accountFallbackFocus = ref('[data-testid=sidebar-account]')
const isFullscreen = ref(false)
const reducedMotion = ref(false)
const pluginNavigationScope = ref<'root' | 'plugin-center'>('root')
const openMenuKeys = ref<string[]>(projectPluginCenterMenu(buildMenuItems(adminRoutes[0]?.children ?? [], '')).filter(item => item.children?.length).map(item => item.key))
const collapsedOpenMenuKeys = ref<string[]>([])
watch(siderCollapsed, () => { collapsedOpenMenuKeys.value = [] })
let reducedMotionMediaQuery: MediaQueryList | null = null
let unsubscribeRouteMotion: (() => void) | null = null

useToastFeedback(() => (
  shutdownRequested.value
    ? {
        key: 'shell-shutdown-requested',
        level: 'warning' as const,
        message: `${t('shell.shutdownRequestedTitle')}：${t('shell.shutdownRequestedDescription')}`,
      }
    : null
))

const menuItems = computed(() => projectPluginCenterMenu(buildMenuItems(adminRoutes[0]?.children ?? [], '')))
const openPluginTargets = computed(() => {
  const seen = new Set<string>()
  return tabs.value.flatMap((tab) => {
    if (tab.name !== 'plugin-detail') return []

    const pluginId = router.resolve(tab.fullPath).params.id
    if (typeof pluginId !== 'string' || !pluginId || seen.has(pluginId)) return []
    seen.add(pluginId)
    return [{ fullPath: tab.fullPath, pluginId }]
  })
})
watch(
  () => openPluginTargets.value.length,
  (openPluginCount) => {
    if (openPluginCount > 0) {
      void pluginsStore.ensureList().catch(() => undefined)
    }
  },
  { immediate: true },
)
const staticNavigationItems = collectNavigationItems(adminRoutes[0]?.children ?? [], '')
  .filter(item => !(item.path === '/' && item.title === t('routes.features')))
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

const fullscreenLabel = computed(() => (
  isFullscreen.value ? t('shell.exitFullscreen') : t('shell.enterFullscreen')
))
const pageMotionProfile = computed<PageMotionProfile>(() => preferences.value.pageTransition)

watch(
  [() => route.name, () => route.params.id],
  ([routeName, routePluginId]) => {
    if (routeName === 'plugin-detail' && typeof routePluginId === 'string' && routePluginId) {
      pluginNavigationScope.value = 'plugin-center'
      return
    }

    pluginNavigationScope.value = isPluginCenterRoute(routeName) ? 'plugin-center' : 'root'
  },
  { immediate: true },
)

const skipPageTransitionStage = computed(() => (
  preferences.value.pageTransition === 'none' || reducedMotion.value
))
const internalPageTransitionStage = ref<PageTransitionStage>('idle')
const pageTransitionStage = computed<PageTransitionStage>(() => (
  skipPageTransitionStage.value ? 'idle' : internalPageTransitionStage.value
))
provide(PAGE_TRANSITION_STAGE_KEY, readonly(pageTransitionStage))

function handlePageTransitionBeforeEnter() {
  if (skipPageTransitionStage.value) {
    internalPageTransitionStage.value = 'idle'
    return
  }
  internalPageTransitionStage.value = 'entering'
}

function handlePageTransitionAfterEnter() {
  if (!isManagedViewTransitionActive()) {
    internalPageTransitionStage.value = 'idle'
  }
}

function handlePageTransitionEnter(element: Element, done: () => void) {
  runRouteFallbackMotion(element as HTMLElement, 'enter', pageMotionProfile.value, done)
}

function handlePageTransitionLeave(element: Element, done: () => void) {
  runRouteFallbackMotion(element as HTMLElement, 'leave', pageMotionProfile.value, done)
}

function handlePageTransitionCancelled(element: Element) {
  cancelRouteFallbackMotion(element as HTMLElement)
  internalPageTransitionStage.value = 'idle'
}
const routeStageRegistry = new Map<string, VueComponent>()
const breadcrumbItems = computed<AppBreadcrumbItem[]>(() => {
  const seen = new Set<string>()
  const items = route.matched
    .map((record) => {
      const isPluginGroup = record.meta.titleKey === 'routes.features'
      const title = isPluginGroup ? t('routes.pluginCenter') : resolveRouteTitle(record.meta)
      return {
        key: String(record.name ?? `${record.path}:${title}`),
        path: isPluginGroup ? pluginCenterPath : resolveBreadcrumbPath(record),
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
const showWorkspaceTabs = computed(() => preferences.value.chromeTabbar && tabs.value.length > 0)

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
        [h(resolveDynamicComponent(stageRouteComponent) as VueComponent)],
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
  try {
    return resolveResolvedRouteIconName(router.resolve(item.path)) || item.icon
  } catch {
    return item.icon
  }
}

function resolveTabItemIconComponent(item: ShellTabItem) {
  return resolveMenuIcon(resolveTabItemIconName(item))
}

function resolvePluginTabIdentity(item: ShellTabItem) {
  if (item.name !== 'plugin-detail') {
    return null
  }

  try {
    const pluginId = router.resolve(item.fullPath).params.id
    if (typeof pluginId !== 'string' || !pluginId) {
      return null
    }

    const plugin = pluginsStore.detailsByPluginId[pluginId]
      ?? pluginsStore.items.find(candidate => candidate.id === pluginId)
    return {
      icon: plugin?.icon,
      pluginId,
      version: plugin?.version,
    }
  } catch {
    return null
  }
}

function resolveTabItemIconData(item: ShellTabItem) {
  const pluginIdentity = resolvePluginTabIdentity(item)
  return pluginIdentity ? `plugin:${pluginIdentity.pluginId}` : resolveTabItemIconName(item)
}

function resolveLeafRouteComponent(viewRoute: RouteLocationNormalizedLoaded) {
  return getLeafMatchedRecord(viewRoute)?.components?.default ?? null
}

function resolveTabPath(viewRoute: RouteLocationNormalizedLoaded) {
  if (isPluginCenterRoute(viewRoute.name)) return pluginCenterPath
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
    if (typeof pluginId === 'string' && pluginId) {
      const pluginName = pluginsStore.getPluginDisplayName(pluginId)
      return t('plugins.detailPageTitle', { name: pluginName })
    }
  }

  return resolveRouteTitle(getLeafRouteMeta(viewRoute))
}

function resolveCurrentTabIcon(viewRoute: RouteLocationNormalizedLoaded) {
  return resolveResolvedRouteIconName(viewRoute)
}

function resolveCurrentWorkspaceTab(viewRoute: RouteLocationNormalizedLoaded): WorkspaceTabProjection | null {
  const leafRecord = getLeafMatchedRecord(viewRoute)
  const leafMeta = getLeafRouteMeta(viewRoute)

  if (!leafRecord || leafMeta?.hideInTab || !viewRoute.name) {
    return null
  }

  if (isPluginCenterRoute(viewRoute.name)) {
    return { tab: createPluginCenterTab(viewRoute.fullPath) }
  }

  const title = resolveCurrentTabTitle(viewRoute)
  if (!title) {
    return null
  }

  const path = resolveTabPath(viewRoute)
  return {
    replaceByName: typeof leafMeta?.viewKey === 'string' && leafMeta.viewKey
      ? String(viewRoute.name)
      : undefined,
    tab: {
      affix: Boolean(leafMeta?.affixTab),
      fullPath: viewRoute.fullPath,
      icon: resolveCurrentTabIcon(viewRoute),
      keepAlive: Boolean(leafMeta?.keepAlive),
      name: String(viewRoute.name),
      path,
      title,
    },
  }
}

const {
  currentTabPath,
  getTabCloseActionItems,
  handleTabAction,
  onTabChange,
  onTabEdit,
  tabActionItems,
} = useWorkspaceTabs({
  affixTabs,
  resolveCurrentTab: resolveCurrentWorkspaceTab,
  resolveTabPath,
  route,
  router,
  tabs,
  uiShellStore,
  navigate: (target) => navigateWithMotion(router, target, pageMotionProfile.value),
})

watch(
  () => route.name === 'plugin-detail' ? resolveCurrentTabTitle(route) : null,
  () => {
    if (route.name !== 'plugin-detail') {
      return
    }

    const projection = resolveCurrentWorkspaceTab(route)
    if (projection) {
      uiShellStore.upsertTab(projection.tab)
    }
  },
)

interface FlattenedMenuItem {
  item: AppMenuItem
  lineage: Array<{ key: string; path: string }>
}

function flattenMenu(items: AppMenuItem[], lineage: Array<{ key: string; path: string }> = []): FlattenedMenuItem[] {
  return items.flatMap((item) => {
    const currentLineage = [...lineage, { key: item.key, path: item.path }]
    const current = [{ item, lineage: currentLineage }]
    return item.children ? [...current, ...flattenMenu(item.children, currentLineage)] : current
  })
}

const flattenedMenu = flattenMenu(menuItems.value)
const menuLineage = computed(() => {
  const leafMeta = getLeafRouteMeta(route)
  const targetPath = isPluginCenterRoute(route.name) || route.name === 'plugin-detail'
    ? pluginCenterPath
    : typeof leafMeta?.activePath === 'string' && leafMeta.activePath
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
    const routeKeys = lineage.slice(0, -1).map((item) => item.key)
    openMenuKeys.value = Array.from(new Set([...openMenuKeys.value, ...routeKeys]))
  },
  { immediate: true },
)

function navigateTo(path: RouteLocationRaw) {
  uiShellStore.setMobileMenuOpen(false)
  collapsedOpenMenuKeys.value = []
  void navigateWithMotion(router, path, pageMotionProfile.value)
}

function handlePluginNavigationScopeChange(scope: 'root' | 'plugin-center') {
  pluginNavigationScope.value = scope
}

function setThemeModeWithMotion(mode: ThemeMode, origin: ThemeMotionOrigin) {
  if (mode !== uiShellStore.themeMode) applyThemeWithMotion(() => uiShellStore.setThemeMode(mode), origin)
}

function openAccountDialog(mobile = false) {
  accountFallbackFocus.value = mobile ? '[data-testid=mobile-account]' : '[data-testid=sidebar-account]'
  accountDialogVisible.value = true
}

function handlePrimaryNavigationKeydown(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
    return
  }

  const container = event.currentTarget
  if (!(container instanceof HTMLElement)) {
    return
  }

  const items = Array.from(container.querySelectorAll<HTMLElement>('[data-nav-item]'))
    .filter((item) => item.getClientRects().length > 0 && item.getAttribute('aria-disabled') !== 'true')
  if (items.length === 0) {
    return
  }

  const activeIndex = items.findIndex((item) => item === document.activeElement || item.contains(document.activeElement))
  let nextIndex = 0
  if (event.key === 'End') {
    nextIndex = items.length - 1
  } else if (event.key === 'ArrowUp') {
    nextIndex = activeIndex <= 0 ? items.length - 1 : activeIndex - 1
  } else if (event.key === 'ArrowDown' && activeIndex >= 0) {
    nextIndex = (activeIndex + 1) % items.length
  }

  event.preventDefault()
  items[nextIndex]?.focus()
}

function handleOpenChange(keys: string[]) {
  if (siderCollapsed.value) collapsedOpenMenuKeys.value = keys.slice(-1)
  else openMenuKeys.value = keys
}

function syncFullscreenState() {
  if (typeof document === 'undefined') {
    isFullscreen.value = false
    return
  }

  isFullscreen.value = Boolean(document.fullscreenElement)
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

function handleReducedMotionPreference(event?: MediaQueryList | MediaQueryListEvent) {
  if ('matches' in (event ?? {})) {
    reducedMotion.value = Boolean((event as MediaQueryList | MediaQueryListEvent).matches)
  }
}

function getRouteViewKey(viewRoute: RouteLocationNormalizedLoaded) {
  const viewIdentity = resolveRouteViewIdentity(viewRoute)
  if (typeof getLeafRouteMeta(viewRoute)?.viewKey === 'string' && getLeafRouteMeta(viewRoute)?.viewKey) {
    return viewIdentity
  }

  return `${viewIdentity}:${viewRoute.path}`
}

onMounted(() => {
  void configStore.refreshEffectiveTimezone().catch(() => undefined)
  syncFullscreenState()
  if (typeof document !== 'undefined') {
    document.addEventListener('fullscreenchange', syncFullscreenState)
  }

  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    reducedMotionMediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    reducedMotion.value = reducedMotionMediaQuery.matches
    reducedMotionMediaQuery.addEventListener('change', handleReducedMotionPreference)
  }

  unsubscribeRouteMotion = subscribeRouteMotion((active) => {
    internalPageTransitionStage.value = active ? 'entering' : 'idle'
  })
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') {
    document.removeEventListener('fullscreenchange', syncFullscreenState)
  }

  if (reducedMotionMediaQuery) {
    reducedMotionMediaQuery.removeEventListener('change', handleReducedMotionPreference)
    reducedMotionMediaQuery = null
  }
  unsubscribeRouteMotion?.()
  unsubscribeRouteMotion = null
})
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <div class="admin-layout" :class="[`admin-layout--${preferences.density}`]">
    <aside class="admin-layout__sider" :data-collapsed="siderCollapsed" data-testid="app-sider">
      <button
        type="button"
        class="admin-layout__brand"
        :aria-label="`${t('app.brand')}，${buildVersionLabel}`"
        :title="siderCollapsed ? `${t('app.brand')} · ${buildVersionLabel}` : undefined"
        @click="navigateTo('/')"
      >
        <RayleaMark class="admin-layout__brand-mark" variant="chrome" />
        <span v-if="!siderCollapsed" class="admin-layout__brand-copy">
          <strong>{{ t('app.brand') }}</strong>
          <span class="admin-layout__build-version" :title="buildVersionLabel" data-testid="build-version">{{ buildVersionLabel }}</span>
        </span>
      </button>

      <nav
        class="admin-layout__sider-scroll admin-layout__primary-navigation"
        tabindex="0"
        :aria-label="t('app.mainNavigation')"
        @keydown="handlePrimaryNavigationKeydown"
      >
        <AppSidebarNavigation
          :collapsed="siderCollapsed"
          :menu-items="menuItems"
          :open-keys="siderCollapsed ? collapsedOpenMenuKeys : openMenuKeys"
          :open-plugin-targets="openPluginTargets"
          :scope="pluginNavigationScope"
          :selected-keys="selectedMenuKeys"
          @navigate="navigateTo"
          @open-change="handleOpenChange"
          @scope-change="handlePluginNavigationScopeChange"
        />
      </nav>
      <SidebarAccountMenu :collapsed="siderCollapsed" :mode="uiShellStore.themeMode" :resolved-mode="uiShellStore.resolvedThemeMode" @manage="openAccountDialog()" @logout="handleLogout" @theme="setThemeModeWithMotion" />
    </aside>

    <AppDrawer
      :open="mobileMenuOpen"
      class="admin-layout__mobile-drawer"
      placement="left"
      :title="t('app.mainNavigation')"
      :width="280"
      @close="uiShellStore.setMobileMenuOpen(false)"
    >
      <div class="admin-layout__mobile-brand">
        <RayleaMark variant="chrome" />
        <strong>{{ t('app.brand') }}</strong>
        <span class="admin-layout__build-version" :title="buildVersionLabel">{{ buildVersionLabel }}</span>
      </div>

      <nav
        class="admin-layout__primary-navigation"
        tabindex="0"
        :aria-label="t('app.mainNavigation')"
        @keydown="handlePrimaryNavigationKeydown"
      >
        <AppSidebarNavigation
          mobile
          :menu-items="menuItems"
          :open-keys="openMenuKeys"
          :open-plugin-targets="openPluginTargets"
          :scope="pluginNavigationScope"
          :selected-keys="selectedMenuKeys"
          @navigate="navigateTo"
          @open-change="handleOpenChange"
          @scope-change="handlePluginNavigationScopeChange"
        />
      </nav>
      <template #footer>
        <SidebarAccountMenu mobile :mode="uiShellStore.themeMode" :resolved-mode="uiShellStore.resolvedThemeMode" @manage="openAccountDialog(true)" @logout="handleLogout" @theme="setThemeModeWithMotion" />
      </template>
    </AppDrawer>

    <div class="admin-layout__workspace">
      <header class="admin-layout__header" data-testid="app-header">
        <div class="admin-layout__progress-track">
          <div :class="['admin-layout__progress-bar', { 'is-active': routeLoading }]" />
        </div>

        <div class="admin-layout__header-main">
          <div class="admin-layout__header-left">
            <AppButton
              class="admin-layout__icon-button admin-layout__nav-trigger desktop-only"
              variant="ghost"
              :aria-label="t('shell.toggleSidebar')"
              @click="uiShellStore.toggleSider()"
            >
              <template #icon>
                <PanelLeftOpenIcon v-if="siderCollapsed" />
                <PanelLeftCloseIcon v-else />
              </template>
            </AppButton>
            <AppButton
              class="admin-layout__icon-button admin-layout__nav-trigger mobile-only"
              variant="ghost"
              :aria-label="t('shell.openMenu')"
              @click="uiShellStore.setMobileMenuOpen(true)"
            >
              <template #icon>
                <MenuIcon />
              </template>
            </AppButton>

            <div
              v-if="breadcrumbItems.length"
              :class="[
                'admin-layout__header-breadcrumb',
                hasMultiBreadcrumb
                  ? 'admin-layout__header-breadcrumb--multi'
                  : 'admin-layout__header-breadcrumb--single',
              ]"
              data-testid="header-breadcrumb"
            >
              <nav class="admin-layout__breadcrumb-nav" :aria-label="t('shell.breadcrumbNav')">
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
                    <MotionRouterLink
                      v-if="!item.current"
                      :to="item.path"
                      class="admin-layout__breadcrumb-link"
                    >
                      <span class="admin-layout__breadcrumb-link-text">{{ item.title }}</span>
                    </MotionRouterLink>
                    <span v-else class="admin-layout__breadcrumb-current">
                      <span class="admin-layout__breadcrumb-current-text">{{ item.title }}</span>
                    </span>

                    <span v-if="!item.current" class="admin-layout__breadcrumb-separator" aria-hidden="true">
                      <ChevronRightIcon />
                    </span>
                  </li>
                </ol>
              </nav>
            </div>
          </div>

          <div class="admin-layout__header-tools">
              <AppTooltip :title="t('shell.search')">
                <AppButton
                  class="admin-layout__icon-button admin-layout__search-button"
                  variant="ghost"
                  :aria-label="t('shell.search')"
                  data-testid="header-search"
                  @click="uiShellStore.openSearch()"
                >
                  <template #icon>
                    <SearchIcon />
                  </template>
                  <span class="admin-layout__search-copy" aria-hidden="true">{{ t('shell.searchPlaceholder') }}</span>
                </AppButton>
              </AppTooltip>
          </div>

          <div class="admin-layout__header-right">
            <AppDropdown>
              <AppButton
                class="admin-layout__icon-button"
                variant="ghost"
                :aria-label="t('shell.moreActions')"
                data-testid="header-more"
              >
                <template #icon><EllipsisIcon /></template>
              </AppButton>

              <template #content>
                <div>
                  <AppDropdownItem key="settings" data-testid="header-settings" @select="uiShellStore.openSettings()">
                    <SettingsIcon />
                    {{ t('shell.settings') }}
                  </AppDropdownItem>
                  <AppDropdownItem key="fullscreen" data-testid="header-fullscreen" @select="toggleFullscreen">
                    <MinimizeIcon v-if="isFullscreen" />
                    <MaximizeIcon v-else />
                    {{ fullscreenLabel }}
                  </AppDropdownItem>
                  <div class="app-menu-separator" role="separator" />
                  <AppDropdownItem key="shutdown" danger @select="shutdownDialogVisible = true">
                    <PowerIcon />
                    {{ t('shell.shutdown') }}
                  </AppDropdownItem>
                </div>
              </template>
            </AppDropdown>
          </div>
        </div>

        <div v-if="showWorkspaceTabs" class="admin-layout__tabbar">
          <div class="admin-layout__tabbar-main">
            <TabsRoot class="workspace-tabs" :model-value="currentTabPath" activation-mode="manual" @update:model-value="onTabChange(String($event))">
              <TabsList class="workspace-tabs__list" aria-label="工作区页签">
                <AppDropdown v-for="item in tabs" :key="item.path" context align="start" data-testid="tab-context-menu">
                  <div class="workspace-tabs__item" :data-active="currentTabPath === item.path">
                    <TabsTrigger :value="item.path" class="workspace-tabs__trigger" aria-controls="app-main">
                      <span class="admin-layout__tab-label" :data-icon="resolveTabItemIconData(item) || undefined" :data-tab-path="item.path">
                        <PluginIcon v-if="resolvePluginTabIdentity(item)" class="admin-layout__tab-plugin-icon" :data-plugin-id="resolvePluginTabIdentity(item)?.pluginId" :plugin-id="resolvePluginTabIdentity(item)?.pluginId ?? ''" :icon="resolvePluginTabIdentity(item)?.icon" :version="resolvePluginTabIdentity(item)?.version" />
                        <component :is="resolveTabItemIconComponent(item)" v-else-if="resolveTabItemIconComponent(item)" class="admin-layout__tab-icon" />
                        <span>{{ item.title }}</span>
                      </span>
                    </TabsTrigger>
                    <button v-if="!item.affix" type="button" class="workspace-tabs__close" :aria-label="t('shell.tabActions.closeCurrent') + ' ' + item.title" @click.stop="onTabEdit(item.path, 'remove')"><XIcon :size="14" /></button>
                  </div>
                  <template #content>
                    <AppDropdownItem v-for="action in getTabCloseActionItems(item)" :key="action.key" :disabled="action.disabled" :data-testid="`tab-context-${action.key}`" @select="handleTabAction(action.key, item)">{{ action.label }}</AppDropdownItem>
                  </template>
                </AppDropdown>
              </TabsList>
            </TabsRoot>

            <div class="admin-layout__tabbar-actions">
              <AppDropdown>
                <AppButton
                  class="admin-layout__icon-button"
                  variant="ghost"
                  :aria-label="t('shell.tabActions.menu')"
                  data-testid="tabbar-actions"
                >
                  <template #icon>
                    <EllipsisIcon />
                  </template>
                </AppButton>

                <template #content>
                  <div>
                    <AppDropdownItem
                      v-for="item in tabActionItems"
                      :key="item.key"
                      :disabled="item.disabled"
                      @select="handleTabAction(item.key)"
                    >
                      {{ item.label }}
                    </AppDropdownItem>
                  </div>
                </template>
              </AppDropdown>
            </div>
          </div>
        </div>
      </header>

      <main id="app-main" class="admin-layout__content" tabindex="-1">
        <RouterView v-slot="{ route: currentViewRoute }">
          <Transition
            :css="false"
            mode="out-in"
            @before-enter="handlePageTransitionBeforeEnter"
            @enter="handlePageTransitionEnter"
            @leave="handlePageTransitionLeave"
            @after-enter="handlePageTransitionAfterEnter"
            @enter-cancelled="handlePageTransitionCancelled"
            @leave-cancelled="handlePageTransitionCancelled"
          >
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
      </main>
    </div>
  </div>

  <RouteSearchPanel
    :items="navigationItems"
    :open="searchOpen"
    @navigate="onSearchNavigate"
    @update:open="onSearchOpenUpdate"
  />
  <PreferencesDrawer />
  <AccountCredentialsDialog :open="accountDialogVisible" :fallback-focus="accountFallbackFocus" @close="accountDialogVisible = false" />

  <AppConfirmDialog :open="shutdownDialogVisible" :title="t('shell.shutdownConfirmTitle')" :description="t('shell.shutdownConfirmBody')" :busy="shutdownPending" danger :confirm-text="t('shell.shutdownConfirmAction')" :cancel-text="t('shell.cancel')" fallback-focus="[data-testid=header-more]" @confirm="confirmShutdown" @cancel="shutdownDialogVisible = false" />
</template>

<style scoped lang="scss">
.admin-layout__brand:focus-visible,
.admin-layout__icon-button:focus-visible,
.admin-layout__shutdown-button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

.admin-layout__brand:focus-visible {
  outline-color: var(--chrome-muted);
  outline-offset: var(--focus-outline-offset);
}

@media (forced-colors: active) {
  .admin-layout__brand:focus-visible,
  .admin-layout__icon-button:focus-visible,
  .admin-layout__shutdown-button:focus-visible {
    outline-color: Highlight;
  }
}
</style>
