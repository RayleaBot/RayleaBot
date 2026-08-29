<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
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

const open = ref(false)
const trigger = ref<{ $el: HTMLButtonElement }>()
const menu = ref<{ $el: HTMLElement }>()
const focusMenuOnOpen = ref(false)

async function closeMenu() {
  focusMenuOnOpen.value = false
  open.value = false
  await nextTick()
  trigger.value?.$el.focus()
}

function openMenu() {
  focusMenuOnOpen.value = true
  open.value = true
}

watch([open, menu, focusMenuOnOpen], (_, __, onCleanup) => {
  const popup = menu.value?.$el.parentElement
  if (!open.value || !focusMenuOnOpen.value || !popup) return
  const focusFirstItem = () => {
    if (!open.value || !focusMenuOnOpen.value) return
    const item = menu.value?.$el.querySelector<HTMLElement>('[role="menuitem"]')
    if (!item?.getClientRects().length) return
    item.focus()
    if (document.activeElement === item) focusMenuOnOpen.value = false
  }
  const observer = new MutationObserver(focusFirstItem)
  observer.observe(popup, { attributes: true, attributeFilter: ['style', 'class'] })
  onCleanup(() => observer.disconnect())
  focusFirstItem()
}, { flush: 'post' })

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
    void closeMenu()
    emit('change', key)
  }
}
</script>

<template>
  <a-dropdown v-model:open="open" :trigger="['click']" placement="bottomRight">
    <a-button
      ref="trigger"
      v-bind="$attrs"
      type="text"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-label="triggerLabel"
      :data-testid="testId"
      @keydown.esc.stop.prevent="closeMenu"
      @keydown.down.prevent="openMenu"
    >
      <template #icon>
        <component :is="triggerIcon" />
      </template>
    </a-button>

    <template #overlay>
      <a-menu
        ref="menu"
        class="theme-mode-menu"
        :selected-keys="[mode]"
        @click="handleSelect($event.key)"
        @keydown.esc.stop.prevent="closeMenu"
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
  color: var(--brand-foreground);
}
</style>
