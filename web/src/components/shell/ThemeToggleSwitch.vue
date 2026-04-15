<script setup lang="ts">
const props = withDefaults(defineProps<{
  checked: boolean
  label: string
  size?: 'compact' | 'default'
  testId?: string
}>(), {
  size: 'compact',
  testId: undefined,
})

const emit = defineEmits<{
  toggle: []
}>()

function handleToggle() {
  emit('toggle')
}
</script>

<template>
  <label
    :class="['theme-toggle-switch', `theme-toggle-switch--${props.size}`]"
    :aria-checked="props.checked"
    :aria-label="props.label"
    :data-testid="props.testId"
    role="switch"
    tabindex="0"
    @click.prevent="handleToggle"
    @keydown.enter.prevent="handleToggle"
    @keydown.space.prevent="handleToggle"
  >
    <input
      class="sr-only theme-toggle-switch__input"
      :checked="props.checked"
      tabindex="-1"
      type="checkbox"
      aria-hidden="true"
      @change.prevent
    />
    <span class="theme-toggle-switch__track" aria-hidden="true"></span>
  </label>
</template>

<style scoped lang="scss">
.theme-toggle-switch {
  --toggle-width: 66px;
  --toggle-height: 34px;
  --toggle-padding: 4px;
  --toggle-thumb-size: calc(var(--toggle-height) - (var(--toggle-padding) * 2));
  --toggle-emoji-size: 13px;
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  cursor: pointer;
  user-select: none;
  -webkit-tap-highlight-color: transparent;

  &--default {
    --toggle-width: 72px;
    --toggle-height: 38px;
    --toggle-emoji-size: 14px;
  }

  &:focus-visible {
    outline: none;
  }

  &:focus-visible .theme-toggle-switch__track,
  &:focus-within .theme-toggle-switch__track {
    box-shadow:
      0 0 0 3px color-mix(in srgb, var(--app-primary) 18%, transparent),
      0 8px 18px color-mix(in srgb, #94a3b8 32%, transparent);
  }

  &[aria-checked='true'] .theme-toggle-switch__track {
    background: #383838;
    box-shadow:
      0 8px 18px color-mix(in srgb, #111827 58%, transparent),
      inset 0 0 0 1px rgba(255, 255, 255, 0.04);
  }

  &[aria-checked='true'] .theme-toggle-switch__track::before {
    opacity: 0;
    transform: translate(10px, -135%) rotate(90deg);
  }

  &[aria-checked='true'] .theme-toggle-switch__track::after {
    opacity: 1;
    transform: translateY(0) rotate(180deg);
  }
}

.theme-toggle-switch__track {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: var(--toggle-width);
  height: var(--toggle-height);
  overflow: hidden;
  border-radius: 999px;
  background: #e5e7eb;
  box-shadow:
    0 8px 18px color-mix(in srgb, #94a3b8 34%, transparent),
    inset 0 0 0 1px rgba(255, 255, 255, 0.55);
  transition: background-color 0.5s ease, box-shadow 0.5s ease;

  &::before,
  &::after {
    position: absolute;
    top: var(--toggle-padding);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--toggle-thumb-size);
    height: var(--toggle-thumb-size);
    border-radius: 999px;
    font-size: var(--toggle-emoji-size);
    line-height: 1;
    transition:
      transform 0.7s cubic-bezier(0.22, 1, 0.36, 1),
      opacity 0.7s cubic-bezier(0.22, 1, 0.36, 1),
      background-color 0.5s ease,
      box-shadow 0.5s ease;
  }

  &::before {
    content: '☀️';
    left: var(--toggle-padding);
    background: #ffffff;
    box-shadow: 0 4px 12px rgba(148, 163, 184, 0.28);
  }

  &::after {
    content: '🌑';
    right: var(--toggle-padding);
    opacity: 0;
    background: #1d1d1d;
    box-shadow: 0 4px 12px rgba(15, 23, 42, 0.45);
    transform: translateY(120%) rotate(0deg);
  }
}
</style>
