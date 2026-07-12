<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'

import { t } from '@/i18n'
import { applyThemeWithMotion } from '@/motion/runtime'
import type {
  ContentWidth,
  DensityMode,
  LayoutPreferences,
  PageTransition,
  ThemeMode,
} from '@/preferences/app'
import { useUiShellStore } from '@/stores/ui-shell'

type SettingsTabKey = 'appearance' | 'workspace' | 'shortcuts'

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
  { combo: 'Ctrl / Cmd + K', description: t('shell.preferences.shortcutSearch') },
  { combo: 'Ctrl / Cmd + W', description: t('shell.preferences.shortcutCloseCurrent') },
  { combo: 'Ctrl / Cmd + Shift + W', description: t('shell.preferences.shortcutCloseOther') },
  { combo: 'Alt + Shift + S', description: t('shell.preferences.shortcutSettings') },
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
  <a-drawer
    :open="settingsOpen"
    :title="t('shell.preferences.title')"
    :width="380"
    class="preferences-drawer"
    data-testid="preferences-drawer"
    @close="uiShellStore.closeSettings()"
  >
    <a-tabs v-model:activeKey="activeTab" class="preferences-drawer__tabs" size="small">
      <a-tab-pane key="appearance" :tab="t('shell.preferences.appearance')">
        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.themeMode') }}</strong>
            <span>{{ t('shell.preferences.themeModeHelp') }}</span>
          </div>
          <a-segmented
            :options="themeOptions"
            :value="preferences.themeMode"
            block
            @change="patchPreference('themeMode', $event as ThemeMode)"
          />
        </div>

        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.density') }}</strong>
            <span>{{ t('shell.preferences.densityHelp') }}</span>
          </div>
          <a-segmented
            :options="densityOptions"
            :value="preferences.density"
            block
            @change="patchPreference('density', $event as DensityMode)"
          />
        </div>

        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.pageTransition') }}</strong>
            <span>{{ t('shell.preferences.pageTransitionHelp') }}</span>
          </div>
          <a-segmented
            :options="pageTransitionOptions"
            :value="preferences.pageTransition"
            block
            @change="patchPreference('pageTransition', $event as PageTransition)"
          />
        </div>
      </a-tab-pane>

      <a-tab-pane key="workspace" :tab="t('shell.preferences.workspace')">
        <div class="preferences-group">
          <div class="preferences-group__heading">
            <strong>{{ t('shell.preferences.contentWidth') }}</strong>
            <span>{{ t('shell.preferences.contentWidthHelp') }}</span>
          </div>
          <a-segmented
            :options="contentWidthOptions"
            :value="preferences.contentWidth"
            block
            @change="patchPreference('contentWidth', $event as ContentWidth)"
          />
        </div>

        <div class="preferences-switches">
          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.chromeTabbar') }}</strong>
              <span>{{ t('shell.preferences.chromeTabbarHelp') }}</span>
            </div>
            <a-switch :checked="preferences.chromeTabbar" @change="patchPreference('chromeTabbar', $event)" />
          </div>

          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.rememberTabs') }}</strong>
              <span>{{ t('shell.preferences.rememberTabsHelp') }}</span>
            </div>
            <a-switch :checked="preferences.rememberTabs" @change="patchPreference('rememberTabs', $event)" />
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="shortcuts" :tab="t('shell.preferences.shortcuts')">
        <div class="shortcut-list">
          <div v-for="item in shortcutItems" :key="item.combo" class="shortcut-item">
            <kbd>{{ item.combo }}</kbd>
            <span>{{ item.description }}</span>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>

    <template #footer>
      <a-button block @click="uiShellStore.resetPreferences()">
        {{ t('shell.preferences.reset') }}
      </a-button>
    </template>
  </a-drawer>
</template>

<style scoped lang="scss">
.preferences-drawer__tabs :deep(.ant-tabs-nav) {
  margin-bottom: 24px;
}

.preferences-group,
.preferences-switches,
.shortcut-list {
  display: grid;
  gap: 12px;
}

.preferences-group {
  margin-bottom: 28px;
}

.preferences-group__heading {
  display: grid;
  gap: 4px;
}

.preferences-group__heading strong,
.preferences-switch strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
}

.preferences-group__heading span,
.preferences-switch span,
.shortcut-item span {
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}

.preferences-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 16px 0;
  border-bottom: 1px solid var(--border);
}

.preferences-switch > div {
  display: grid;
  gap: 4px;
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
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  background: var(--surface-soft);
  font-family: var(--font-mono);
  font-size: 12px;
}
</style>
