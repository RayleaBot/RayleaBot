<script setup lang="ts">
import { computed } from 'vue'
import {
  BulbFilled,
  BulbOutlined,
  CheckOutlined,
  DesktopOutlined,
} from '@ant-design/icons-vue'

import { t } from '@/i18n'
import type { ResolvedThemeMode, ThemeMode } from '@/preferences/app'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  mode: ThemeMode
  resolvedMode: ResolvedThemeMode
  testId?: string
}>()

const emit = defineEmits<{
  change: [mode: ThemeMode]
}>()

const triggerLabel = computed(() => t('shell.themeMenuLabel', {
  mode: t(`shell.preferences.theme${props.mode === 'system' ? 'System' : props.mode === 'dark' ? 'Dark' : 'Light'}`),
}))

const triggerIcon = computed(() => {
  if (props.mode === 'system') {
    return DesktopOutlined
  }
  return props.resolvedMode === 'dark' ? BulbFilled : BulbOutlined
})

const options: Array<{ icon: typeof DesktopOutlined; label: string; value: ThemeMode }> = [
  { icon: DesktopOutlined, label: t('shell.preferences.themeSystem'), value: 'system' },
  { icon: BulbOutlined, label: t('shell.preferences.themeLight'), value: 'light' },
  { icon: BulbFilled, label: t('shell.preferences.themeDark'), value: 'dark' },
]

function handleSelect(key: string | number) {
  if (key === 'system' || key === 'light' || key === 'dark') {
    emit('change', key)
  }
}
</script>

<template>
  <a-dropdown :trigger="['click']" placement="bottomRight">
    <a-button
      v-bind="$attrs"
      type="text"
      :aria-label="triggerLabel"
      :data-testid="testId"
    >
      <template #icon>
        <component :is="triggerIcon" />
      </template>
    </a-button>

    <template #overlay>
      <a-menu
        class="theme-mode-menu"
        :selected-keys="[mode]"
        @click="handleSelect($event.key)"
      >
        <a-menu-item v-for="option in options" :key="option.value">
          <span class="theme-mode-menu__item">
            <component :is="option.icon" />
            <span>{{ option.label }}</span>
            <CheckOutlined v-if="mode === option.value" class="theme-mode-menu__check" />
          </span>
        </a-menu-item>
      </a-menu>
    </template>
  </a-dropdown>
</template>

<style scoped lang="scss">
.theme-mode-menu {
  min-width: 176px;
  padding: 6px;
}

.theme-mode-menu__item {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) 18px;
  align-items: center;
  gap: 10px;
  min-height: 30px;
}

.theme-mode-menu__check {
  color: var(--accent);
}
</style>
