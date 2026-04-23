<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'

import {
  themeColorPresets,
  type ContentWidth,
  type LayoutPreferences,
  type DensityMode,
  type FontScale,
  type PageTransition,
  type RadiusLevel,
} from '@/preferences/app'
import { t } from '@/i18n'
import { useUiShellStore } from '@/stores/ui-shell'

type SettingsTabKey = 'appearance' | 'general' | 'layout' | 'shortcuts'

const uiShellStore = useUiShellStore()
const { preferences, settingsOpen } = storeToRefs(uiShellStore)
const activeTab = ref<SettingsTabKey>('appearance')

const themeOptions = [
  { label: t('shell.preferences.themeLight'), value: 'light' },
  { label: t('shell.preferences.themeDark'), value: 'dark' },
]

const radiusOptions: Array<{ label: string; value: RadiusLevel }> = [
  { label: t('shell.preferences.radiusSm'), value: 'sm' },
  { label: t('shell.preferences.radiusMd'), value: 'md' },
  { label: t('shell.preferences.radiusLg'), value: 'lg' },
  { label: t('shell.preferences.radiusXl'), value: 'xl' },
]

const fontScaleOptions: Array<{ label: string; value: FontScale }> = [
  { label: t('shell.preferences.fontSm'), value: 'sm' },
  { label: t('shell.preferences.fontMd'), value: 'md' },
  { label: t('shell.preferences.fontLg'), value: 'lg' },
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
  uiShellStore.patchPreferences({ [key]: value })
}
</script>

<template>
  <a-drawer
    :open="settingsOpen"
    :title="t('shell.preferences.title')"
    :width="360"
    class="preferences-drawer"
    data-testid="preferences-drawer"
    @close="uiShellStore.closeSettings()"
  >
    <a-tabs v-model:activeKey="activeTab" class="preferences-drawer__tabs" size="small">
      <a-tab-pane key="appearance" :tab="t('shell.preferences.appearance')">
        <div class="preferences-group">
          <label>{{ t('shell.preferences.themeMode') }}</label>
          <a-segmented
            :options="themeOptions"
            :value="preferences.themeMode"
            block
            @change="patchPreference('themeMode', $event as 'light' | 'dark')"
          />
        </div>

        <div class="preferences-group">
          <label>{{ t('shell.preferences.primaryColor') }}</label>
          <div class="preferences-colors">
            <button
              v-for="color in themeColorPresets"
              :key="color"
              type="button"
              :class="['preferences-color', { 'is-active': preferences.colorPrimary === color }]"
              :style="{ '--preferences-color': color }"
              @click="patchPreference('colorPrimary', color)"
            />
          </div>
        </div>

        <div class="preferences-group">
          <label>{{ t('shell.preferences.radius') }}</label>
          <a-segmented
            :options="radiusOptions"
            :value="preferences.radiusLevel"
            block
            @change="patchPreference('radiusLevel', $event as RadiusLevel)"
          />
        </div>

        <div class="preferences-group">
          <label>{{ t('shell.preferences.fontScale') }}</label>
          <a-segmented
            :options="fontScaleOptions"
            :value="preferences.fontScale"
            block
            @change="patchPreference('fontScale', $event as FontScale)"
          />
        </div>

        <div class="preferences-group">
          <label>{{ t('shell.preferences.density') }}</label>
          <a-segmented
            :options="densityOptions"
            :value="preferences.density"
            block
            @change="patchPreference('density', $event as DensityMode)"
          />
        </div>
      </a-tab-pane>

      <a-tab-pane key="layout" :tab="t('shell.preferences.layout')">
        <div class="preferences-group">
          <label>{{ t('shell.preferences.contentWidth') }}</label>
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
              <strong>{{ t('shell.preferences.fixedHeader') }}</strong>
              <span>{{ t('shell.preferences.fixedHeaderHelp') }}</span>
            </div>
            <a-switch :checked="preferences.fixedHeader" @change="patchPreference('fixedHeader', $event)" />
          </div>

          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.breadcrumb') }}</strong>
              <span>{{ t('shell.preferences.breadcrumbHelp') }}</span>
            </div>
            <a-switch :checked="preferences.breadcrumb" @change="patchPreference('breadcrumb', $event)" />
          </div>

          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.chromeTabbar') }}</strong>
              <span>{{ t('shell.preferences.chromeTabbarHelp') }}</span>
            </div>
            <a-switch :checked="preferences.chromeTabbar" @change="patchPreference('chromeTabbar', $event)" />
          </div>
        </div>
      </a-tab-pane>

      <a-tab-pane key="general" :tab="t('shell.preferences.general')">
        <div class="preferences-group">
          <label>{{ t('shell.preferences.pageTransition') }}</label>
          <a-segmented
            :options="pageTransitionOptions"
            :value="preferences.pageTransition"
            block
            @change="patchPreference('pageTransition', $event as PageTransition)"
          />
        </div>

        <div class="preferences-switches">
          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.pageLoading') }}</strong>
              <span>{{ t('shell.preferences.pageLoadingHelp') }}</span>
            </div>
            <a-switch :checked="preferences.pageLoading" @change="patchPreference('pageLoading', $event)" />
          </div>

          <div class="preferences-switch">
            <div>
              <strong>{{ t('shell.preferences.rememberTabs') }}</strong>
              <span>{{ t('shell.preferences.rememberTabsHelp') }}</span>
            </div>
            <a-switch :checked="preferences.rememberTabs" @change="patchPreference('rememberTabs', $event)" />
          </div>
        </div>

        <a-button block @click="uiShellStore.resetPreferences()">
          {{ t('shell.preferences.reset') }}
        </a-button>
      </a-tab-pane>

      <a-tab-pane key="shortcuts" :tab="t('shell.preferences.shortcuts')">
        <div class="shortcut-list">
          <div v-for="item in shortcutItems" :key="item.combo" class="shortcut-item">
            <strong>{{ item.combo }}</strong>
            <span>{{ item.description }}</span>
          </div>
        </div>
      </a-tab-pane>
    </a-tabs>
  </a-drawer>
</template>

<style scoped lang="scss">
.preferences-drawer__tabs {
  :deep(.ant-tabs-nav) {
    margin-bottom: 16px;
  }
}

.preferences-group,
.preferences-switches,
.shortcut-list {
  display: grid;
  gap: 12px;
}

.preferences-group {
  margin-bottom: 20px;

  label {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--text);
  }
}

.preferences-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);

  strong,
  span {
    display: block;
  }

  strong {
    font-size: 0.9rem;
  }

  span {
    margin-top: 4px;
    color: var(--muted);
    font-size: 0.8rem;
    line-height: 1.5;
  }
}

.preferences-colors {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.preferences-color {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  border: 2px solid transparent;
  background: var(--preferences-color);
  cursor: pointer;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.45);

  &.is-active {
    border-color: color-mix(in srgb, var(--preferences-color) 60%, var(--surface-strong) 40%);
  }
}

.shortcut-item {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface);

  strong {
    font-family: var(--font-mono);
    font-size: 0.84rem;
  }

  span {
    color: var(--muted);
    font-size: 0.82rem;
    line-height: 1.5;
  }
}
</style>
