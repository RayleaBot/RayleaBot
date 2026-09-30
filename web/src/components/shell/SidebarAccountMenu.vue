<script setup lang="ts">
import {
  ChevronUpIcon,
  KeyRoundIcon,
  LogOutIcon,
  MaximizeIcon,
  MinimizeIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  PowerIcon,
  SettingsIcon,
  UserRoundIcon,
} from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import ThemeModeMenu from './ThemeModeMenu.vue'
import { t } from '@/i18n'
import type { ResolvedThemeMode, ThemeMode } from '@/preferences/app'
import type { ThemeMotionOrigin } from '@/motion/runtime'

withDefaults(defineProps<{
  collapsed?: boolean; fullscreen?: boolean; mobile?: boolean; mode: ThemeMode; resolvedMode: ResolvedThemeMode
}>(), { collapsed: false, fullscreen: false, mobile: false })
defineEmits<{
  collapse: []; fullscreen: []; logout: []; manage: []; settings: []; shutdown: []; theme: [mode: ThemeMode, origin: ThemeMotionOrigin]
}>()
</script>

<template>
  <!-- The sidebar foot holds everything that is not a page: account, preferences, full screen, service shutdown, theme and the sidebar toggle. -->
  <div class="sidebar-account" :data-collapsed="collapsed" data-testid="sidebar-footer">
    <AppDropdown side="top" align="start">
      <AppButton class="sidebar-account__trigger" variant="ghost" :size="collapsed ? 'icon' : 'default'" :aria-label="t('shell.account')" :data-testid="mobile ? 'mobile-account' : 'sidebar-account'">
        <span class="sidebar-account__avatar" aria-hidden="true"><UserRoundIcon /></span>
        <span v-if="!collapsed" class="sidebar-account__label">{{ t('shell.account') }}</span>
        <ChevronUpIcon v-if="!collapsed" class="sidebar-account__chevron" />
      </AppButton>
      <template #content>
        <AppDropdownItem @select="$emit('manage')"><KeyRoundIcon />{{ t('shell.credentials.title') }}</AppDropdownItem>
        <AppDropdownItem data-testid="shell-settings" @select="$emit('settings')"><SettingsIcon />{{ t('shell.settings') }}</AppDropdownItem>
        <AppDropdownItem v-if="!mobile" data-testid="shell-fullscreen" @select="$emit('fullscreen')">
          <MinimizeIcon v-if="fullscreen" /><MaximizeIcon v-else />{{ fullscreen ? t('shell.exitFullscreen') : t('shell.enterFullscreen') }}
        </AppDropdownItem>
        <div class="app-menu-separator" role="separator" />
        <AppDropdownItem @select="$emit('logout')"><LogOutIcon />{{ t('shell.logout') }}</AppDropdownItem>
        <AppDropdownItem danger data-testid="shell-shutdown" @select="$emit('shutdown')"><PowerIcon />{{ t('shell.shutdown') }}</AppDropdownItem>
      </template>
    </AppDropdown>
    <ThemeModeMenu class="sidebar-account__tool" :mode="mode" :resolved-mode="resolvedMode" side="top" align="start" :test-id="mobile ? 'mobile-theme-toggle' : 'theme-toggle'" @change="(mode, origin) => $emit('theme', mode, origin)" />
    <AppButton
      v-if="!mobile"
      class="sidebar-account__tool"
      variant="ghost"
      size="icon"
      :aria-label="collapsed ? t('shell.expandSidebar') : t('shell.toggleSidebar')"
      :title="collapsed ? t('shell.expandSidebar') : t('shell.toggleSidebar')"
      data-testid="sidebar-collapse"
      @click="$emit('collapse')"
    >
      <PanelLeftOpenIcon v-if="collapsed" /><PanelLeftCloseIcon v-else />
    </AppButton>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.sidebar-account { display: flex; align-items: center; gap: 2px; padding: 10px; border-top: 1px solid var(--border); }
.sidebar-account__trigger { flex: 1; min-width: 0; justify-content: flex-start; gap: 10px; height: 44px; padding-inline: 6px 10px; color: var(--chrome-text); font-weight: 500; }
.sidebar-account__avatar { display: grid; flex: none; place-items: center; width: 30px; height: 30px; border-radius: 50%; background: var(--text); color: var(--bg); }
.sidebar-account__avatar svg { width: 16px; height: 16px; }
.sidebar-account__label { flex: 1; text-align: left; }
.sidebar-account__chevron { width: 14px; height: 14px; color: var(--chrome-muted); }
.sidebar-account__tool { flex: none; width: 36px; height: 36px; color: var(--chrome-muted); }
.sidebar-account__trigger:hover, .sidebar-account__tool:hover { background: var(--nav-hover); color: var(--chrome-text); }
.sidebar-account[data-collapsed=true] { flex-direction: column; gap: 4px; padding: 10px 0; }
.sidebar-account[data-collapsed=true] .sidebar-account__trigger { flex: none; justify-content: center; width: 44px; padding: 0; }
@media (max-width: #{bp.$desktop - 1px}), (pointer: coarse) {
  .sidebar-account__trigger, .sidebar-account__tool { min-height: 44px; }
  .sidebar-account__tool { min-width: 44px; }
}
@media (forced-colors: active) {
  .sidebar-account__avatar { border: 1px solid CanvasText; }
}
</style>
