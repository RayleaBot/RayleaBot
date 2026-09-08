<script setup lang="ts">
import { ChevronUpIcon, KeyRoundIcon, LogOutIcon, UserRoundIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import ThemeModeMenu from './ThemeModeMenu.vue'
import { t } from '@/i18n'
import type { ResolvedThemeMode, ThemeMode } from '@/preferences/app'
import type { ThemeMotionOrigin } from '@/motion/runtime'

withDefaults(defineProps<{
  collapsed?: boolean; mobile?: boolean; mode: ThemeMode; resolvedMode: ResolvedThemeMode
}>(), { collapsed: false, mobile: false })
defineEmits<{
  manage: []; logout: []; theme: [mode: ThemeMode, origin: ThemeMotionOrigin]
}>()
</script>

<template>
  <div class="sidebar-account" :data-collapsed="collapsed" data-testid="sidebar-footer">
    <AppDropdown side="top" align="start">
      <AppButton class="sidebar-account__trigger" variant="ghost" :size="collapsed ? 'icon' : 'default'" :aria-label="t('shell.account')" :data-testid="mobile ? 'mobile-account' : 'sidebar-account'">
        <UserRoundIcon />
        <span v-if="!collapsed" class="sidebar-account__label">{{ t('shell.account') }}</span>
        <ChevronUpIcon v-if="!collapsed" class="sidebar-account__chevron" />
      </AppButton>
      <template #content>
        <AppDropdownItem @select="$emit('manage')"><KeyRoundIcon />{{ t('shell.credentials.title') }}</AppDropdownItem>
        <div class="app-menu-separator" role="separator" />
        <AppDropdownItem @select="$emit('logout')"><LogOutIcon />{{ t('shell.logout') }}</AppDropdownItem>
      </template>
    </AppDropdown>
    <ThemeModeMenu class="sidebar-account__theme" :mode="mode" :resolved-mode="resolvedMode" side="top" align="start" :test-id="mobile ? 'mobile-theme-toggle' : 'theme-toggle'" @change="(mode, origin) => $emit('theme', mode, origin)" />
  </div>
</template>

<style scoped>
.sidebar-account { display: flex; align-items: center; gap: 8px; padding: 12px; border-top: 1px solid var(--sider-brand-border); }
.sidebar-account__trigger { flex: 1; min-width: 0; justify-content: flex-start; gap: 10px; height: 40px; padding-inline: 10px; color: var(--sider-menu-text); }
.sidebar-account__label { flex: 1; text-align: left; }
.sidebar-account__chevron { width: 14px; height: 14px; color: var(--sider-menu-text); }
.sidebar-account__theme { flex: none; width: 40px; height: 40px; color: var(--sider-menu-text); }
.sidebar-account__trigger:hover, .sidebar-account__theme:hover { background: var(--sider-menu-hover-bg); }
.sidebar-account[data-collapsed=true] { flex-direction: column; padding: 12px 10px; }
.sidebar-account[data-collapsed=true] .sidebar-account__trigger { flex: none; justify-content: center; width: 40px; padding: 0; }
@media (max-width: 991px), (pointer: coarse) {
  .sidebar-account__trigger, .sidebar-account__theme { min-height: 44px; }
  .sidebar-account__theme { min-width: 44px; }
}
</style>
