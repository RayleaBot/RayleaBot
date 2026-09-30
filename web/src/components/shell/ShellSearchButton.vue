<script setup lang="ts">
import { SearchIcon } from '@lucide/vue'

import { t } from '@/i18n'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const shortcutLabel = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘ K' : 'Ctrl K'
</script>

<template>
  <!-- Page search sits in every page header, before the page's own actions. -->
  <button type="button" class="shell-search" :aria-label="t('shell.search')" data-testid="header-search" @click="uiShellStore.openSearch()">
    <SearchIcon aria-hidden="true" />
    <span class="shell-search__placeholder">{{ t('shell.searchPlaceholder') }}</span>
    <kbd class="shell-search__shortcut">{{ shortcutLabel }}</kbd>
  </button>
</template>

<style scoped lang="scss">
.shell-search { display: inline-flex; align-items: center; gap: 8px; width: 300px; height: 44px; padding: 0 16px; border: 1px solid transparent; border-radius: 999px; background: var(--control-fill); color: var(--muted); box-shadow: var(--shadow-xs); font: inherit; font-size: var(--font-size-sm); cursor: pointer; transition: background-color var(--motion-fast) var(--motion-easing), color var(--motion-fast) var(--motion-easing); }
.shell-search:hover { background: var(--control-fill-hover); color: var(--text); }
.shell-search:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.shell-search svg { flex: none; width: 16px; height: 16px; }
.shell-search__placeholder { flex: 1; min-width: 0; overflow: hidden; text-align: left; text-overflow: ellipsis; white-space: nowrap; }
.shell-search__shortcut { flex: none; color: var(--muted); font-family: inherit; font-size: var(--font-size-xs); font-weight: 500; }
@media (prefers-reduced-motion: reduce) { .shell-search { transition: none; } }
@media (forced-colors: active) { .shell-search { border-color: ButtonText; } }
</style>
