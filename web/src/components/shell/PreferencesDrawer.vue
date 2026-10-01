<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'

import AppDrawer from '@/components/AppDrawer.vue'
import AppTabs from '@/components/AppTabs.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppButton from '@/components/AppButton.vue'
import { t } from '@/i18n'
import { searchShortcutLabel, settingsShortcutLabel } from '@/lib/shortcuts'
import { applyThemeWithMotion } from '@/motion/runtime'
import type {
  ContentWidth,
  DensityMode,
  LayoutPreferences,
  PageTransition,
  ThemeMode,
} from '@/preferences/app'
import { useUiShellStore } from '@/stores/ui-shell'

type SettingsTabKey = 'appearance' | 'shortcuts'

withDefaults(defineProps<{ fallbackFocus?: string }>(), { fallbackFocus: '[data-testid=sidebar-account]' })
const uiShellStore = useUiShellStore()
const { preferences, settingsOpen } = storeToRefs(uiShellStore)
const activeTab = ref<SettingsTabKey>('appearance')

const themeOptions: Array<{ label: string; value: ThemeMode }> = [
  { label: t('shell.preferences.themeSystem'), value: 'system' },
  { label: t('shell.preferences.themeLight'), value: 'light' },
  { label: t('shell.preferences.themeDark'), value: 'dark' },
]

const densityOptions: Array<{ label: string; value: DensityMode }> = [
  { label: t('shell.preferences.densityDefault'), value: 'default' },
  { label: t('shell.preferences.densityCompact'), value: 'compact' },
]

const contentWidthOptions: Array<{ label: string; value: ContentWidth }> = [
  { label: t('shell.preferences.contentWidthWide'), value: 'wide' },
  { label: t('shell.preferences.contentWidthFixed'), value: 'fixed' },
]

const pageTransitionOptions: Array<{ label: string; value: PageTransition }> = [
  { label: t('shell.preferences.transitionFadeSlide'), value: 'fade-slide' },
  { label: t('shell.preferences.transitionFade'), value: 'fade' },
  { label: t('shell.preferences.transitionNone'), value: 'none' },
]


const shortcutItems = computed(() => [
  { combo: searchShortcutLabel, description: t('shell.preferences.shortcutSearch') },
  { combo: settingsShortcutLabel, description: t('shell.preferences.shortcutSettings') },
])

function patchPreference<T extends keyof LayoutPreferences>(key: T, value: LayoutPreferences[T]) {
  const update = () => uiShellStore.patchPreferences({ [key]: value })
  if (key === 'themeMode') {
    applyThemeWithMotion(update)
    return
  }
  update()
}
</script>

<template>
  <AppDrawer
    :open="settingsOpen"
    :title="t('shell.preferences.title')"
    :width="380"
    class="preferences-drawer"
    data-testid="preferences-drawer"
    :fallback-focus="fallbackFocus"
    @close="uiShellStore.closeSettings()"
  >
    <AppTabs v-model="activeTab" :items="[{ value: 'appearance', label: t('shell.preferences.appearance') }, { value: 'shortcuts', label: t('shell.preferences.shortcuts') }]" :label="t('shell.preferences.title')" class="preferences-drawer__tabs">
      <template #appearance>
        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.themeMode') }}</strong>
            <span>{{ t('shell.preferences.themeModeHelp') }}</span>
          </div>
          <AppSegmented
            :options="themeOptions"
            :model-value="preferences.themeMode" :label="t('shell.preferences.themeMode')"
            @update:model-value="patchPreference('themeMode', $event as ThemeMode)"
          />
        </div>

        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.density') }}</strong>
            <span>{{ t('shell.preferences.densityHelp') }}</span>
          </div>
          <AppSegmented
            :options="densityOptions"
            :model-value="preferences.density" :label="t('shell.preferences.density')"
            @update:model-value="patchPreference('density', $event as DensityMode)"
          />
        </div>

        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.contentWidth') }}</strong>
            <span>{{ t('shell.preferences.contentWidthHelp') }}</span>
          </div>
          <AppSegmented
            :options="contentWidthOptions"
            :model-value="preferences.contentWidth" :label="t('shell.preferences.contentWidth')"
            @update:model-value="patchPreference('contentWidth', $event as ContentWidth)"
          />
        </div>

        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.pageTransition') }}</strong>
            <span>{{ t('shell.preferences.pageTransitionHelp') }}</span>
          </div>
          <AppSegmented
            :options="pageTransitionOptions"
            :model-value="preferences.pageTransition" :label="t('shell.preferences.pageTransition')"
            @update:model-value="patchPreference('pageTransition', $event as PageTransition)"
          />
        </div>

        <!-- HarmonyOS Sans requires the software to state that the fonts are used; the agreement ships with the package.
             It is a fact about the interface, not a setting, so it reads as a note rather than a group with a control. -->
        <p class="preferences-note" data-testid="preferences-font-notice">{{ t('shell.preferences.uiFont') }}：{{ t('shell.preferences.uiFontName') }}</p>
      </template>

      <template #shortcuts>
        <div class="shortcut-list">
          <div v-for="item in shortcutItems" :key="item.combo" class="shortcut-item">
            <kbd>{{ item.combo }}</kbd>
            <span>{{ item.description }}</span>
          </div>
        </div>
      </template>
    </AppTabs>

    <template #footer>
      <AppButton class="w-full" @click="uiShellStore.resetPreferences()">
        {{ t('shell.preferences.reset') }}
      </AppButton>
    </template>
  </AppDrawer>
</template>

<style scoped lang="scss">
.preferences-drawer__tabs :deep(.app-tabs__list) {
  margin-bottom: 24px;
}

.preferences-group,
.shortcut-list {
  display: grid;
  gap: 12px;
}

.preferences-group {
  margin-bottom: 28px;
}

.preferences-note {
  margin: 0;
  padding-top: 16px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 13px;
}

.preferences-group__heading {
  display: grid;
  gap: 4px;
}

.preferences-group__heading strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.preferences-group__heading span,
.shortcut-item span {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}

.shortcut-item {
  display: grid;
  grid-template-columns: minmax(130px, auto) minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}

.shortcut-item kbd {
  width: fit-content;
  padding: 4px 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--text);
  background: var(--surface-raised);
  box-shadow: var(--shadow-raised);
  font-family: var(--font-mono);
  font-size: 12px;
}
</style>
