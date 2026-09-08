<script setup lang="ts">
import { computed, ref } from 'vue'
import { AnimatePresence, motion } from 'motion-v'
import { CheckIcon, MonitorIcon, MoonIcon, SunIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import AppDropdown from '@/components/AppDropdown.vue'
import AppDropdownItem from '@/components/AppDropdownItem.vue'
import { t } from '@/i18n'
import type { ResolvedThemeMode, ThemeMode } from '@/preferences/app'
import { useOverlayMotion } from '@/motion/presets'
import type { ThemeMotionOrigin } from '@/motion/runtime'
defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ mode: ThemeMode; resolvedMode: ResolvedThemeMode; testId?: string; side?: 'top' | 'bottom'; align?: 'start' | 'end' }>(), { side: 'bottom', align: 'end' })
const emit = defineEmits<{ change: [mode: ThemeMode, origin: ThemeMotionOrigin] }>()
const trigger = ref<InstanceType<typeof AppButton> | null>(null)
const overlayMotion = useOverlayMotion()
function selectTheme(mode: ThemeMode) {
  const bounds = (trigger.value?.$el as HTMLElement | undefined)?.getBoundingClientRect()
  emit('change', mode, bounds ? { x: bounds.left + bounds.width / 2, y: bounds.top + bounds.height / 2 } : undefined)
}
const triggerLabel = computed(() => t('shell.themeMenuLabel', { mode: t(`shell.preferences.theme${props.mode === 'system' ? 'System' : props.mode === 'dark' ? 'Dark' : 'Light'}`) }))
const triggerIcon = computed(() => props.mode === 'system' ? MonitorIcon : props.resolvedMode === 'dark' ? MoonIcon : SunIcon)
const options: Array<{ icon: typeof MonitorIcon; label: string; value: ThemeMode }> = [
  { icon: MonitorIcon, label: t('shell.preferences.themeSystem'), value: 'system' },
  { icon: SunIcon, label: t('shell.preferences.themeLight'), value: 'light' },
  { icon: MoonIcon, label: t('shell.preferences.themeDark'), value: 'dark' },
]
</script>
<template>
  <AppDropdown :side="side" :align="align">
    <AppButton ref="trigger" v-bind="$attrs" variant="ghost" size="icon" :aria-label="triggerLabel" :data-testid="testId">
      <span class="theme-mode-icon" aria-hidden="true">
        <AnimatePresence :initial="false">
          <motion.span :key="mode" class="theme-mode-icon__layer" :initial="{ opacity: 0, rotate: overlayMotion.transition.duration ? -35 : 0, scale: .8 }" :animate="{ opacity: 1, rotate: 0, scale: 1 }" :exit="{ opacity: 0, rotate: overlayMotion.transition.duration ? 35 : 0, scale: .8 }" :transition="overlayMotion.transition"><component :is="triggerIcon" :size="18" /></motion.span>
        </AnimatePresence>
      </span>
    </AppButton>
    <template #content>
      <AppDropdownItem v-for="option in options" :key="option.value" @select="selectTheme(option.value)">
        <component :is="option.icon" /><span class="grow">{{ option.label }}</span><CheckIcon v-if="mode === option.value" class="text-accent" />
      </AppDropdownItem>
    </template>
  </AppDropdown>
</template>

<style scoped>
.theme-mode-icon { position: relative; width: 18px; height: 18px; }
.theme-mode-icon__layer { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; }
</style>
