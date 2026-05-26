<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  checked: boolean
  loading?: boolean
  disabled?: boolean
  compact?: boolean
  dataTestid?: string
  checkedLabel?: string
  uncheckedLabel?: string
}>(), {
  loading: false,
  disabled: false,
  compact: false,
  checkedLabel: '启动',
  uncheckedLabel: '停用',
})

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const currentLabel = computed(() => (props.checked ? props.checkedLabel : props.uncheckedLabel))
const actionLabel = computed(() => (props.checked ? props.uncheckedLabel : props.checkedLabel))
const ariaLabel = computed(() => {
  if (props.loading) {
    return `${currentLabel.value}处理中`
  }

  return `当前${currentLabel.value}，点击切换为${actionLabel.value}`
})

function handleClick(event: MouseEvent) {
  if (props.disabled || props.loading) {
    event.preventDefault()
    return
  }

  emit('click', event)
}
</script>

<template>
  <button
    type="button"
    role="switch"
    class="plugin-holo-button"
    :class="[
      compact && 'plugin-holo-button--compact',
      checked && 'is-checked',
      loading && 'is-loading',
      disabled && 'is-disabled',
    ]"
    :disabled="disabled || loading"
    :data-testid="dataTestid"
    :aria-busy="loading ? 'true' : undefined"
    :aria-checked="checked ? 'true' : 'false'"
    :aria-label="ariaLabel"
    @click="handleClick"
  >
    <span class="plugin-holo-button__track" aria-hidden="true">
      <span class="plugin-holo-button__track-lines">
        <span class="plugin-holo-button__track-line" />
      </span>

      <span class="plugin-holo-button__thumb">
        <span class="plugin-holo-button__thumb-core" />
        <span class="plugin-holo-button__thumb-inner" />
        <span class="plugin-holo-button__thumb-scan" />
        <span class="plugin-holo-button__thumb-particles">
          <span v-for="index in 5" :key="index" class="plugin-holo-button__thumb-particle" />
        </span>
      </span>

      <span class="plugin-holo-button__data">
        <span class="plugin-holo-button__text plugin-holo-button__text--off">{{ uncheckedLabel }}</span>
        <span class="plugin-holo-button__text plugin-holo-button__text--on">{{ checkedLabel }}</span>
        <span class="plugin-holo-button__status plugin-holo-button__status--off" />
        <span class="plugin-holo-button__status plugin-holo-button__status--on" />
      </span>

      <span class="plugin-holo-button__energy">
        <span v-for="index in 3" :key="index" class="plugin-holo-button__energy-ring" />
      </span>

      <span class="plugin-holo-button__interface">
        <span v-for="index in 6" :key="index" class="plugin-holo-button__interface-line" />
      </span>

      <span class="plugin-holo-button__reflection" />
      <span class="plugin-holo-button__glow" />
    </span>
  </button>
</template>

<style scoped lang="scss">
.plugin-holo-button {
  --button-width: 92px;
  --button-height: 34px;
  --track-radius: 99px;
  --thumb-size: 28px;
  --thumb-offset: 3px;
  --off-track: color-mix(in srgb, var(--text) 8%, var(--surface-soft));
  --off-track-border: color-mix(in srgb, var(--text) 16%, var(--surface-soft));
  --off-text: color-mix(in srgb, var(--text) 65%, transparent);
  --thumb-bg: #ffffff;
  --on-track: var(--accent);
  --on-text: #ffffff;
  position: relative;
  display: inline-flex;
  width: var(--button-width);
  min-width: var(--button-width);
  height: var(--button-height);
  padding: 0;
  border: 0;
  background: transparent;
  cursor: pointer;
  appearance: none;
  transition: transform 0.25s cubic-bezier(0.25, 0.8, 0.25, 1), opacity 0.2s ease;
  user-select: none;
}

.plugin-holo-button--compact {
  --button-width: 78px;
  --button-height: 28px;
  --thumb-size: 22px;
}

.plugin-holo-button:hover:not(:disabled) {
  transform: translateY(-1px);
}

.plugin-holo-button:hover:not(:disabled):not(.is-checked) .plugin-holo-button__track {
  background: color-mix(in srgb, var(--text) 12%, var(--surface-soft));
  border-color: color-mix(in srgb, var(--text) 24%, var(--surface-soft));
}

.plugin-holo-button:active:not(:disabled) {
  transform: translateY(0) scale(0.97);
}

.plugin-holo-button:disabled {
  cursor: not-allowed;
}

.plugin-holo-button.is-disabled {
  opacity: 0.5;
}

.plugin-holo-button.is-loading {
  cursor: progress;
}

.plugin-holo-button__track {
  position: absolute;
  inset: 0;
  border-radius: var(--track-radius);
  border: 1px solid var(--off-track-border);
  background: var(--off-track);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.05);
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.25s cubic-bezier(0.4, 0, 0.2, 1), border-color 0.25s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.25s cubic-bezier(0.4, 0, 0.2, 1), color 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
}

.plugin-holo-button__thumb {
  position: absolute;
  top: var(--thumb-offset);
  left: var(--thumb-offset);
  width: var(--thumb-size);
  height: var(--thumb-size);
  z-index: 2;
  border-radius: 50%;
  background: var(--thumb-bg);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1), 0 1px 2px rgba(0, 0, 0, 0.06);
  transition: left 0.25s cubic-bezier(0.4, 0, 0.2, 1), background-color 0.25s ease, box-shadow 0.25s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.plugin-holo-button__thumb-inner {
  display: none;
  width: 14px;
  height: 14px;
  border: 2px solid var(--accent);
  border-top-color: transparent;
  border-radius: 50%;
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease, background-color 0.25s ease, color 0.25s ease;
}

.plugin-holo-button--compact .plugin-holo-button__thumb-inner {
  width: 11px;
  height: 11px;
  border-width: 1.5px;
}

.plugin-holo-button.is-loading .plugin-holo-button__thumb-inner {
  display: block;
  animation: spinner 0.6s linear infinite;
}

.plugin-holo-button.is-checked.is-loading .plugin-holo-button__thumb-inner {
  border-color: var(--accent);
  border-top-color: transparent;
}

.plugin-holo-button__data {
  position: absolute;
  inset: 0;
  z-index: 1;
  pointer-events: none;
  display: flex;
  align-items: center;
}

.plugin-holo-button__text {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  letter-spacing: 0.04em;
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.plugin-holo-button--compact .plugin-holo-button__text {
  font-size: 11px;
}

.plugin-holo-button__text--off {
  right: 14px;
  color: var(--off-text);
  opacity: 1;
}

.plugin-holo-button--compact .plugin-holo-button__text--off {
  right: 11px;
}

.plugin-holo-button__text--on {
  left: 14px;
  color: var(--on-text);
  opacity: 0;
  transform: translateY(-50%) scale(0.85);
}

.plugin-holo-button--compact .plugin-holo-button__text--on {
  left: 11px;
}

.plugin-holo-button.is-checked .plugin-holo-button__track {
  background: var(--on-track);
  border-color: transparent;
  box-shadow: 0 2px 8px color-mix(in srgb, var(--on-track) 25%, transparent);
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb {
  left: calc(100% - var(--thumb-size) - var(--thumb-offset));
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15), 0 1px 2px rgba(0, 0, 0, 0.1);
}

.plugin-holo-button.is-checked .plugin-holo-button__text--off {
  opacity: 0;
  transform: translateY(-50%) scale(0.85);
}

.plugin-holo-button.is-checked .plugin-holo-button__text--on {
  opacity: 1;
  transform: translateY(-50%) scale(1);
}

.plugin-holo-button__track-lines,
.plugin-holo-button__track-line,
.plugin-holo-button__thumb-core,
.plugin-holo-button__thumb-scan,
.plugin-holo-button__thumb-particles,
.plugin-holo-button__thumb-particle,
.plugin-holo-button__energy,
.plugin-holo-button__energy-ring,
.plugin-holo-button__interface,
.plugin-holo-button__interface-line,
.plugin-holo-button__reflection,
.plugin-holo-button__glow,
.plugin-holo-button__status {
  display: none !important;
}

@keyframes spinner {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .plugin-holo-button,
  .plugin-holo-button__track,
  .plugin-holo-button__thumb,
  .plugin-holo-button__text {
    transition: none !important;
  }

  .plugin-holo-button.is-loading .plugin-holo-button__thumb-inner {
    animation: none;
  }
}
</style>
