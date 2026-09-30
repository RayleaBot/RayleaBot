<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useFullscreen } from '@vueuse/core'
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
import { usePluginsStore } from '@/stores/plugins'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useConfigStore } from '@/stores/config'
import { useUiShellStore } from '@/stores/ui-shell'
import type { ThemeMode } from '@/preferences/app'
import { createRouteStageRegistry, resolveLeafRouteComponent, resolveRouteViewKey } from '@/layouts/shell-routes'
import { providePageTransitionStage } from '@/layouts/usePageTransitionStage'
import { handleNavigationKeydown, useShellNavigation } from '@/layouts/useShellNavigation'
import { useWorkspaceTabs } from '@/layouts/useWorkspaceTabs'
import { applyThemeWithMotion, navigateWithMotion, type ThemeMotionOrigin } from '@/motion/runtime'
import { prefetchRouteComponents } from '@/router/prefetch'

const router = useRouter()
const pluginsStore = usePluginsStore()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()
const configStore = useConfigStore()
const uiShellStore = useUiShellStore()

const {
  cachedViewNames,
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

useToastFeedback(() => (
  shutdownRequested.value
    ? {
        key: 'shell-shutdown-requested',
        level: 'warning' as const,
        message: `${t('shell.shutdownRequestedTitle')}：${t('shell.shutdownRequestedDescription')}`,
      }
    : null
))

const pageMotionProfile = computed(() => preferences.value.pageTransition)
function navigate(target: RouteLocationRaw) {
  return navigateWithMotion(router, target, pageMotionProfile.value)
}

const pageTransition = providePageTransitionStage(pageMotionProfile)
const getRouteStageComponent = createRouteStageRegistry()
const {
  currentTabPath,
  getTabCloseActionItems,
  handleTabAction,
  onTabChange,
  onTabEdit,
  openPluginTargets,
  tabActionItems,
  tabViews,
} = useWorkspaceTabs(navigate)
const {
  breadcrumbItems,
  collapsedOpenMenuKeys,
  handleOpenChange,
  menuItems,
  navigateTo,
  navigationItems,
  openMenuKeys,
  pluginNavigationScope,
  selectedMenuKeys,
  setPluginNavigationScope,
} = useShellNavigation({
  navigate,
  tabItems: computed(() => tabViews.value.map(({ tab, iconName }) => ({
    icon: iconName,
    key: `tab:${tab.path}`,
    path: tab.path,
    title: tab.title,
  }))),
})
const showWorkspaceTabs = computed(() => preferences.value.chromeTabbar && tabs.value.length > 0)

const { isFullscreen, isSupported: fullscreenSupported, toggle: toggleDocumentFullscreen } = useFullscreen()

async function toggleFullscreen() {
  if (!fullscreenSupported.value) {
    notifyInfo(t('shell.fullscreenUnsupported'))
    return
  }

  try {
    await toggleDocumentFullscreen()
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
  }
}

function setThemeModeWithMotion(mode: ThemeMode, origin: ThemeMotionOrigin) {
  if (mode !== uiShellStore.themeMode) applyThemeWithMotion(() => uiShellStore.setThemeMode(mode), origin)
}

function openAccountDialog(mobile = false) {
  accountFallbackFocus.value = mobile ? '[data-testid=mobile-account]' : '[data-testid=sidebar-account]'
  accountDialogVisible.value = true
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

function onSearchOpenUpdate(open: boolean) {
  if (open) {
    uiShellStore.openSearch()
  } else {
    uiShellStore.closeSearch()
  }
}

let stopRoutePrefetch: (() => void) | undefined
onMounted(() => {
  void configStore.refreshEffectiveTimezone().catch(() => undefined)
  stopRoutePrefetch = prefetchRouteComponents(router)
})
onBeforeUnmount(() => stopRoutePrefetch?.())
</script>

<template>
  <a class="skip-link" href="#app-main">{{ t('app.skipToMain') }}</a>

  <div class="admin-layout" :class="[`admin-layout--${preferences.density}`]">
    <aside class="admin-layout__sider liquid-glass" data-glass="clear" :data-collapsed="siderCollapsed" data-testid="app-sider">
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
        @keydown="handleNavigationKeydown"
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
          @scope-change="setPluginNavigationScope"
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
        @keydown="handleNavigationKeydown"
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
          @scope-change="setPluginNavigationScope"
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
              class="admin-layout__icon-button admin-layout__nav-trigger desktop-only liquid-glass liquid-glass--strong"
              data-glass="clear"
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
              class="admin-layout__icon-button admin-layout__nav-trigger mobile-only liquid-glass liquid-glass--strong"
              data-glass="clear"
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
                breadcrumbItems.length > 1
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
                  class="admin-layout__icon-button admin-layout__search-button liquid-glass liquid-glass--strong"
                  data-glass="clear"
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
                class="admin-layout__icon-button liquid-glass liquid-glass--strong"
                data-glass="clear"
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
                    {{ isFullscreen ? t('shell.exitFullscreen') : t('shell.enterFullscreen') }}
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
          <div class="admin-layout__tabbar-main liquid-glass liquid-glass--strong" data-glass="clear">
            <TabsRoot class="workspace-tabs" :model-value="currentTabPath" activation-mode="manual" @update:model-value="onTabChange(String($event))">
              <TabsList class="workspace-tabs__list" :aria-label="t('shell.workspaceTabs')">
                <AppDropdown v-for="{ tab: item, plugin, icon, iconData } in tabViews" :key="item.path" context align="start" data-testid="tab-context-menu">
                  <div class="workspace-tabs__item" :data-active="currentTabPath === item.path">
                    <TabsTrigger :value="item.path" class="workspace-tabs__trigger" aria-controls="app-main">
                      <span class="admin-layout__tab-label" :data-icon="iconData" :data-tab-path="item.path">
                        <PluginIcon v-if="plugin" :refresh-key="pluginsStore.iconRevision" class="admin-layout__tab-plugin-icon" :data-plugin-id="plugin.id" :plugin-id="plugin.id" :icon="plugin.icon" :version="plugin.version" />
                        <component :is="icon" v-else-if="icon" class="admin-layout__tab-icon" />
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
            @before-enter="pageTransition.onBeforeEnter"
            @enter="pageTransition.onEnter"
            @leave="pageTransition.onLeave"
            @after-enter="pageTransition.onAfterEnter"
            @enter-cancelled="pageTransition.onCancelled"
            @leave-cancelled="pageTransition.onCancelled"
          >
            <KeepAlive :include="cachedViewNames">
              <component
                :is="getRouteStageComponent(currentViewRoute)"
                v-if="resolveLeafRouteComponent(currentViewRoute)"
                :key="resolveRouteViewKey(currentViewRoute)"
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
    @navigate="navigateTo"
    @update:open="onSearchOpenUpdate"
  />
  <PreferencesDrawer />
  <AccountCredentialsDialog :open="accountDialogVisible" :fallback-focus="accountFallbackFocus" @close="accountDialogVisible = false" />

  <AppConfirmDialog :open="shutdownDialogVisible" :title="t('shell.shutdownConfirmTitle')" :description="t('shell.shutdownConfirmBody')" :busy="shutdownPending" danger :confirm-text="t('shell.shutdownConfirmAction')" :cancel-text="t('shell.cancel')" fallback-focus="[data-testid=header-more]" @confirm="confirmShutdown" @cancel="shutdownDialogVisible = false" />
</template>

<style scoped lang="scss">
.admin-layout__brand:focus-visible,
.admin-layout__icon-button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

.admin-layout__brand:focus-visible {
  outline-color: var(--chrome-muted);
  outline-offset: var(--focus-outline-offset);
}

@media (forced-colors: active) {
  .admin-layout__brand:focus-visible,
  .admin-layout__icon-button:focus-visible {
    outline-color: Highlight;
  }
}
</style>
