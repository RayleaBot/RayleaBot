<script setup lang="ts">
import { CheckIcon, MonitorIcon, MoonIcon, SunIcon } from "@lucide/vue";
import {
  DropdownMenuContent,
  DropdownMenuItemIndicator,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  type AcceptableValue,
} from "reka-ui";
import { computed, ref, type Component } from "vue";
import { isLauncherThemeMode, type LauncherThemeMode } from "@shared/launcher-theme";

import { useTheme } from "./useTheme";

const modeConfig: Record<LauncherThemeMode, { icon: Component; label: string }> = {
  system: { icon: MonitorIcon, label: "跟随系统" },
  light: { icon: SunIcon, label: "浅色" },
  dark: { icon: MoonIcon, label: "深色" },
};
const modes = Object.keys(modeConfig) as LauncherThemeMode[];

const { mode, setMode, syncError } = useTheme();
const current = computed(() => modeConfig[mode.value]);
const trigger = ref<{ $el: Element } | null>(null);

// The menu closes as soon as an item is chosen and returns focus to the trigger; the new theme grows out of
// the trigger's center.
function selectMode(value: AcceptableValue) {
  if (!isLauncherThemeMode(value) || value === mode.value) {
    return;
  }
  const bounds = trigger.value?.$el.getBoundingClientRect();
  setMode(value, bounds ? { x: bounds.left + bounds.width / 2, y: bounds.top + bounds.height / 2 } : undefined);
}
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger
      ref="trigger"
      class="theme-menu-trigger"
      :aria-label="`主题：${current.label}`"
      :title="`主题：${current.label}`"
    >
      <component :is="current.icon" aria-hidden="true" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent class="theme-menu-surface" side="top" align="start" :side-offset="8" :collision-padding="8">
        <DropdownMenuRadioGroup :model-value="mode" aria-label="选择主题" @update:model-value="selectMode">
          <DropdownMenuRadioItem v-for="itemMode in modes" :key="itemMode" class="theme-menu-item" :value="itemMode">
            <component :is="modeConfig[itemMode].icon" class="theme-menu-item__icon" aria-hidden="true" />
            <span>{{ modeConfig[itemMode].label }}</span>
            <DropdownMenuItemIndicator class="theme-menu-item__check">
              <CheckIcon aria-hidden="true" />
            </DropdownMenuItemIndicator>
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <p v-if="syncError" class="theme-menu-error" role="status">{{ syncError }}</p>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
