<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useFullscreen } from '@vueuse/core'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'

import { notifyError, notifyInfo, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import RayleaMark from '@/components/brand/RayleaMark.vue'
import AppSidebarNavigation from '@/components/shell/AppSidebarNavigation.vue'
import PreferencesDrawer from '@/components/shell/PreferencesDrawer.vue'
import RouteSearchPanel from '@/components/shell/RouteSearchPanel.vue'
import SidebarAccountMenu from '@/components/shell/SidebarAccountMenu.vue'
import AccountCredentialsDialog from '@/components/shell/AccountCredentialsDialog.vue'
import { buildVersionLabel } from '@/lib/build-info'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { useSessionStore } from '@/stores/session'
import { useSystemStore } from '@/stores/system'
import { useConfigStore } from '@/stores/config'
import { useUiShellStore } from '@/stores/ui-shell'
import type { ThemeMode } from '@/preferences/app'
import { collectKeepAliveViewNames, createRouteStageRegistry, resolveLeafRouteComponent, resolveRouteViewKey } from '@/layouts/shell-routes'
import { providePageTransitionStage } from '@/layouts/usePageTransitionStage'
import { handleNavigationKeydown, useShellNavigation } from '@/layouts/useShellNavigation'
import { useShellShortcuts } from '@/layouts/useShellShortcuts'
import { applyThemeWithMotion, navigateWithMotion, type ThemeMotionOrigin } from '@/motion/runtime'
import { prefetchRouteComponents } from '@/router/prefetch'

const router = useRouter()
const sessionStore = useSessionStore()
const systemStore = useSystemStore()
const configStore = useConfigStore()
const uiShellStore = useUiShellStore()

const {
  preferences,
  routeLoading,
  searchOpen,
  siderCollapsed,
} = storeToRefs(uiShellStore)
const { shutdownPending, stopIntent } = storeToRefs(systemStore)

const shutdownDialogVisible = ref(false)
const accountDialogVisible = ref(false)
// Dialogs opened from the account menu return focus to its trigger.
const accountFallbackFocus = '[data-testid=sidebar-account]'

// A final stop is worth a warning; a restart or an update only explains the coming disconnect.
useToastFeedback(() => {
  switch (stopIntent.value) {
    case 'stop': return { key: 'shell-stop-intent:stop', level: 'warning' as const, message: t('shell.shutdownRequestedDescription') }
    case 'restart': return { key: 'shell-stop-intent:restart', level: 'info' as const, message: t('shell.restartRequestedDescription') }
    case 'update': return { key: 'shell-stop-intent:update', level: 'info' as const, message: t('shell.updateRequestedDescription') }
    default: return null
  }
})

const pageMotionProfile = computed(() => preferences.value.pageTransition)
function navigate(target: RouteLocationRaw) {
  return navigateWithMotion(router, target, pageMotionProfile.value)
}

const pageTransition = providePageTransitionStage(pageMotionProfile)
const getRouteStageComponent = createRouteStageRegistry()
const cachedViewNames = collectKeepAliveViewNames(router)
useShellShortcuts()
const {
  collapsedOpenMenuKeys,
  handleOpenChange,
  menuItems,
  navigateTo,
  navigationItems,
  openMenuKeys,
  pluginNavigationScope,
  selectedMenuKeys,
  setPluginNavigationScope,
} = useShellNavigation({ navigate })

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
  void configStore.refreshSharedSettings().catch(() => undefined)
  stopRoutePrefetch = prefetchRouteComponents(router)
})
onBeforeUnmount(() => stopRoutePrefetch?.())
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
        @keydown="handleNavigationKeydown"
      >
        <AppSidebarNavigation
          :collapsed="siderCollapsed"
          :menu-items="menuItems"
          :open-keys="siderCollapsed ? collapsedOpenMenuKeys : openMenuKeys"
          :scope="pluginNavigationScope"
          :selected-keys="selectedMenuKeys"
          @navigate="navigateTo"
          @open-change="handleOpenChange"
          @scope-change="setPluginNavigationScope"
        />
      </nav>
      <SidebarAccountMenu
        :collapsed="siderCollapsed"
        :fullscreen="isFullscreen"
        :mode="uiShellStore.themeMode"
        :resolved-mode="uiShellStore.resolvedThemeMode"
        @collapse="uiShellStore.toggleSider()"
        @fullscreen="toggleFullscreen"
        @logout="handleLogout"
        @manage="accountDialogVisible = true"
        @settings="uiShellStore.openSettings()"
        @shutdown="shutdownDialogVisible = true"
        @theme="setThemeModeWithMotion"
      />
    </aside>

    <div class="admin-layout__workspace">
      <div class="admin-layout__progress-track" aria-hidden="true">
        <div :class="['admin-layout__progress-bar', { 'is-active': routeLoading }]" />
      </div>

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
  <PreferencesDrawer :fallback-focus="accountFallbackFocus" />
  <AccountCredentialsDialog :open="accountDialogVisible" :fallback-focus="accountFallbackFocus" @close="accountDialogVisible = false" />

  <AppConfirmDialog :open="shutdownDialogVisible" :title="t('shell.shutdownConfirmTitle')" :description="t('shell.shutdownConfirmBody')" :busy="shutdownPending" danger :confirm-text="t('shell.shutdownConfirmAction')" :cancel-text="t('shell.cancel')" :fallback-focus="accountFallbackFocus" @confirm="confirmShutdown" @cancel="shutdownDialogVisible = false" />
</template>

<style scoped lang="scss">
.admin-layout__brand:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

@media (forced-colors: active) {
  .admin-layout__brand:focus-visible {
    outline-color: Highlight;
  }
}
</style>
