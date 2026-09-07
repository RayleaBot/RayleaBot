<script setup lang="ts">
import { computed } from 'vue'
import { CheckIcon, MonitorIcon, MoonIcon, SunIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import { t } from '@/i18n'
import type { ResolvedThemeMode, ThemeMode } from '@/preferences/app'
defineOptions({ inheritAttrs: false })
const props = defineProps<{ mode: ThemeMode; resolvedMode: ResolvedThemeMode; testId?: string }>()
defineEmits<{ change: [mode: ThemeMode] }>()
const triggerLabel = computed(() => t('shell.themeMenuLabel', { mode: t(`shell.preferences.theme${props.mode === 'system' ? 'System' : props.mode === 'dark' ? 'Dark' : 'Light'}`) }))
const triggerIcon = computed(() => props.mode === 'system' ? MonitorIcon : props.resolvedMode === 'dark' ? MoonIcon : SunIcon)
const options: Array<{ icon: typeof MonitorIcon; label: string; value: ThemeMode }> = [
  { icon: MonitorIcon, label: t('shell.preferences.themeSystem'), value: 'system' },
  { icon: SunIcon, label: t('shell.preferences.themeLight'), value: 'light' },
  { icon: MoonIcon, label: t('shell.preferences.themeDark'), value: 'dark' },
]
</script>
<template>
  <AppDropdown>
    <AppButton v-bind="$attrs" variant="ghost" size="icon" :aria-label="triggerLabel" :data-testid="testId"><component :is="triggerIcon" /></AppButton>
    <template #content>
      <AppDropdownItem v-for="option in options" :key="option.value" @select="$emit('change', option.value)">
        <component :is="option.icon" /><span class="grow">{{ option.label }}</span><CheckIcon v-if="mode === option.value" class="text-accent" />
      </AppDropdownItem>
    </template>
  </AppDropdown>
</template>