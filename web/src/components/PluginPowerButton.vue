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
  --button-width: 148px;
  --button-height: 58px;
  --track-radius: 999px;
  --thumb-size: 52px;
  --thumb-offset: 3px;
  --off-track: rgba(8, 28, 60, 0.84);
  --off-track-border: rgba(0, 164, 255, 0.34);
  --off-track-shadow: rgba(0, 98, 255, 0.24);
  --off-text: rgba(122, 210, 255, 0.72);
  --off-line: rgba(0, 164, 255, 0.34);
  --off-glow: rgba(0, 162, 255, 0.28);
  --off-thumb: rgba(7, 42, 88, 0.94);
  --off-thumb-border: rgba(88, 205, 255, 0.6);
  --off-thumb-core: rgba(0, 189, 255, 0.56);
  --off-thumb-inner: rgba(255, 255, 255, 0.86);
  --on-track: rgba(6, 52, 31, 0.84);
  --on-track-border: rgba(0, 255, 166, 0.32);
  --on-track-shadow: rgba(0, 255, 166, 0.22);
  --on-text: rgba(112, 255, 200, 0.74);
  --on-line: rgba(0, 255, 166, 0.34);
  --on-glow: rgba(0, 255, 166, 0.3);
  --on-thumb: rgba(8, 82, 43, 0.94);
  --on-thumb-border: rgba(112, 255, 200, 0.56);
  --on-thumb-core: rgba(0, 255, 179, 0.52);
  --on-thumb-inner: rgba(255, 255, 255, 0.86);
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
  perspective: 800px;
  transition: transform 0.18s ease, opacity 0.24s ease;
}

.plugin-holo-button--compact {
  --button-width: 118px;
  --button-height: 44px;
  --thumb-size: 38px;
}

.plugin-holo-button:hover:not(:disabled) {
  transform: translateY(-1px);
}

.plugin-holo-button:active:not(:disabled) {
  transform: scale(0.985);
}

.plugin-holo-button:disabled {
  cursor: not-allowed;
}

.plugin-holo-button.is-disabled {
  opacity: 0.52;
}

.plugin-holo-button.is-loading {
  cursor: progress;
}

.plugin-holo-button__track {
  position: absolute;
  inset: 0;
  overflow: hidden;
  border-radius: var(--track-radius);
  border: 1px solid var(--off-track-border);
  background: var(--off-track);
  box-shadow:
    0 0 16px var(--off-track-shadow),
    inset 0 0 10px rgba(0, 0, 0, 0.72);
  backdrop-filter: blur(8px);
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__track::before {
  content: '';
  position: absolute;
  inset: 0;
  background:
    radial-gradient(ellipse at center, rgba(0, 110, 255, 0.14) 0%, transparent 70%),
    linear-gradient(90deg, rgba(0, 60, 120, 0.12) 0%, rgba(0, 30, 60, 0.2) 100%);
  opacity: 0.72;
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__track::after {
  content: '';
  position: absolute;
  inset: 2px 2px auto;
  height: 9px;
  border-radius: 999px 999px 0 0;
  background: linear-gradient(90deg, rgba(0, 170, 255, 0.28), rgba(0, 80, 255, 0.08));
  opacity: 0.78;
  filter: blur(1px);
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__track-lines {
  position: absolute;
  inset: 50% 0 auto;
  height: 1px;
  transform: translateY(-50%);
  overflow: hidden;
}

.plugin-holo-button__track-line {
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    90deg,
    var(--off-line) 0 5px,
    transparent 5px 15px
  );
  animation: plugin-holo-track-line 3s linear infinite;
  transition: background 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__thumb,
.plugin-holo-button__energy {
  position: absolute;
  top: var(--thumb-offset);
  left: var(--thumb-offset);
  width: var(--thumb-size);
  height: var(--thumb-size);
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__thumb {
  z-index: 2;
  overflow: hidden;
  border-radius: 50%;
  border: 1px solid var(--off-thumb-border);
  background: radial-gradient(circle, rgba(10, 40, 90, 0.9) 0%, var(--off-thumb) 100%);
  box-shadow:
    0 2px 15px rgba(0, 0, 0, 0.5),
    inset 0 0 15px rgba(0, 150, 255, 0.46);
}

.plugin-holo-button__thumb-core,
.plugin-holo-button__thumb-inner {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__thumb-core {
  width: calc(var(--thumb-size) * 0.72);
  height: calc(var(--thumb-size) * 0.72);
  background: radial-gradient(circle, var(--off-thumb-core) 0%, rgba(0, 50, 120, 0.18) 100%);
  box-shadow: 0 0 20px rgba(0, 150, 255, 0.44);
  opacity: 0.9;
}

.plugin-holo-button__thumb-inner {
  width: calc(var(--thumb-size) * 0.46);
  height: calc(var(--thumb-size) * 0.46);
  background: radial-gradient(circle, var(--off-thumb-inner) 0%, rgba(100, 200, 255, 0.46) 100%);
  box-shadow: 0 0 10px rgba(100, 200, 255, 0.62);
  opacity: 0.74;
  animation: plugin-holo-pulse 2s infinite alternate;
}

.plugin-holo-button__thumb-scan {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 5px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(0, 150, 255, 0.48) 20%,
    rgba(255, 255, 255, 0.82) 50%,
    rgba(0, 150, 255, 0.48) 80%,
    transparent 100%
  );
  filter: blur(1px);
  opacity: 0.72;
  animation: plugin-holo-thumb-scan 2s linear infinite;
  transition: background 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__thumb-particles {
  position: absolute;
  inset: 0;
}

.plugin-holo-button__thumb-particle {
  position: absolute;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: rgba(100, 200, 255, 0.82);
  box-shadow: 0 0 5px rgba(100, 200, 255, 0.72);
  animation: plugin-holo-thumb-particle 3s infinite ease-out;
  opacity: 0;
  transition: background 0.45s cubic-bezier(0.23, 1, 0.32, 1), box-shadow 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__thumb-particle:nth-child(1) {
  top: 70%;
  left: 30%;
  animation-delay: 0.2s;
}

.plugin-holo-button__thumb-particle:nth-child(2) {
  top: 60%;
  left: 60%;
  animation-delay: 0.6s;
}

.plugin-holo-button__thumb-particle:nth-child(3) {
  top: 50%;
  left: 40%;
  animation-delay: 1s;
}

.plugin-holo-button__thumb-particle:nth-child(4) {
  top: 40%;
  left: 70%;
  animation-delay: 1.4s;
}

.plugin-holo-button__thumb-particle:nth-child(5) {
  top: 80%;
  left: 50%;
  animation-delay: 1.8s;
}

.plugin-holo-button__data {
  position: absolute;
  inset: 0;
  z-index: 1;
}

.plugin-holo-button__text {
  position: absolute;
  top: 50%;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.12em;
  line-height: 1;
  white-space: nowrap;
  text-transform: uppercase;
  transform: translateY(-50%);
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button--compact .plugin-holo-button__text {
  font-size: 10px;
  letter-spacing: 0.08em;
}

.plugin-holo-button__text--off {
  right: 13px;
  color: var(--off-text);
  text-shadow: 0 0 5px rgba(0, 100, 255, 0.32);
  opacity: 1;
}

.plugin-holo-button__text--on {
  left: 15px;
  color: var(--on-text);
  text-shadow: 0 0 5px rgba(0, 255, 100, 0.28);
  opacity: 0;
}

.plugin-holo-button__status {
  position: absolute;
  top: 50%;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  transform: translateY(-50%);
  animation: plugin-holo-blink 2s infinite alternate;
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button--compact .plugin-holo-button__status {
  width: 8px;
  height: 8px;
}

.plugin-holo-button__status--off {
  right: 14px;
  background: radial-gradient(circle, rgba(0, 180, 255, 0.82) 0%, rgba(0, 80, 200, 0.36) 100%);
  box-shadow: 0 0 10px rgba(0, 150, 255, 0.42);
  opacity: 1;
}

.plugin-holo-button__status--on {
  left: 14px;
  background: radial-gradient(circle, rgba(0, 255, 150, 0.82) 0%, rgba(0, 200, 80, 0.36) 100%);
  box-shadow: 0 0 10px rgba(0, 255, 150, 0.4);
  opacity: 0;
}

.plugin-holo-button__energy {
  z-index: 1;
  pointer-events: none;
}

.plugin-holo-button__energy-ring {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 2px solid transparent;
  border-radius: 50%;
  transform: translate(-50%, -50%);
  opacity: 0;
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__energy-ring:nth-child(1) {
  width: calc(var(--thumb-size) - 4px);
  height: calc(var(--thumb-size) - 4px);
  border-top-color: rgba(0, 150, 255, 0.5);
  border-right-color: rgba(0, 150, 255, 0.3);
  animation: plugin-holo-spin 3s linear infinite;
}

.plugin-holo-button__energy-ring:nth-child(2) {
  width: calc(var(--thumb-size) - 14px);
  height: calc(var(--thumb-size) - 14px);
  border-bottom-color: rgba(0, 150, 255, 0.5);
  border-left-color: rgba(0, 150, 255, 0.3);
  animation: plugin-holo-spin 2s linear infinite reverse;
}

.plugin-holo-button__energy-ring:nth-child(3) {
  width: calc(var(--thumb-size) - 24px);
  height: calc(var(--thumb-size) - 24px);
  border-left-color: rgba(0, 150, 255, 0.5);
  border-top-color: rgba(0, 150, 255, 0.3);
  animation: plugin-holo-spin 1.5s linear infinite;
}

.plugin-holo-button__interface {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.plugin-holo-button__interface-line {
  position: absolute;
  background: var(--off-line);
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button__interface-line:nth-child(1) {
  width: 15px;
  height: 1px;
  bottom: -5px;
  left: 20px;
}

.plugin-holo-button__interface-line:nth-child(2) {
  width: 1px;
  height: 8px;
  bottom: -12px;
  left: 35px;
}

.plugin-holo-button__interface-line:nth-child(3) {
  width: 25px;
  height: 1px;
  bottom: -12px;
  left: 35px;
}

.plugin-holo-button__interface-line:nth-child(4) {
  width: 15px;
  height: 1px;
  bottom: -5px;
  right: 20px;
}

.plugin-holo-button__interface-line:nth-child(5) {
  width: 1px;
  height: 8px;
  bottom: -12px;
  right: 35px;
}

.plugin-holo-button__interface-line:nth-child(6) {
  width: 25px;
  height: 1px;
  bottom: -12px;
  right: 10px;
}

.plugin-holo-button__reflection {
  position: absolute;
  inset: 0;
  border-radius: var(--track-radius);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.1) 0%, transparent 40%);
  pointer-events: none;
}

.plugin-holo-button__glow {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: var(--track-radius);
  background: radial-gradient(ellipse at center, rgba(0, 150, 255, 0.18) 0%, transparent 70%);
  filter: blur(10px);
  opacity: 0.5;
  transition: all 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.plugin-holo-button.is-checked .plugin-holo-button__track {
  border-color: var(--on-track-border);
  background: var(--on-track);
  box-shadow:
    0 0 15px var(--on-track-shadow),
    inset 0 0 10px rgba(0, 0, 0, 0.78);
}

.plugin-holo-button.is-checked .plugin-holo-button__track::before {
  background:
    radial-gradient(ellipse at center, rgba(0, 255, 150, 0.12) 0%, transparent 70%),
    linear-gradient(90deg, rgba(0, 120, 60, 0.12) 0%, rgba(0, 60, 30, 0.2) 100%);
}

.plugin-holo-button.is-checked .plugin-holo-button__track::after {
  background: linear-gradient(90deg, rgba(0, 255, 150, 0.28) 0%, rgba(0, 160, 80, 0.08) 100%);
}

.plugin-holo-button.is-checked .plugin-holo-button__track-line {
  background: repeating-linear-gradient(
    90deg,
    var(--on-line) 0 5px,
    transparent 5px 15px
  );
  animation-direction: reverse;
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb {
  left: calc(100% - var(--thumb-size) - var(--thumb-offset));
  background: radial-gradient(circle, rgba(10, 90, 40, 0.9) 0%, var(--on-thumb) 100%);
  border-color: var(--on-thumb-border);
  box-shadow:
    0 2px 15px rgba(0, 0, 0, 0.5),
    inset 0 0 15px rgba(0, 255, 150, 0.48);
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb-core {
  background: radial-gradient(circle, var(--on-thumb-core) 0%, rgba(0, 120, 50, 0.18) 100%);
  box-shadow: 0 0 20px rgba(0, 255, 150, 0.44);
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb-inner {
  background: radial-gradient(circle, var(--on-thumb-inner) 0%, rgba(100, 255, 200, 0.42) 100%);
  box-shadow: 0 0 10px rgba(100, 255, 200, 0.64);
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb-scan {
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(0, 255, 150, 0.48) 20%,
    rgba(255, 255, 255, 0.82) 50%,
    rgba(0, 255, 150, 0.48) 80%,
    transparent 100%
  );
}

.plugin-holo-button.is-checked .plugin-holo-button__thumb-particle {
  background: rgba(100, 255, 200, 0.82);
  box-shadow: 0 0 5px rgba(100, 255, 200, 0.76);
}

.plugin-holo-button.is-checked .plugin-holo-button__text--off {
  opacity: 0;
}

.plugin-holo-button.is-checked .plugin-holo-button__text--on {
  opacity: 1;
}

.plugin-holo-button.is-checked .plugin-holo-button__status--off {
  opacity: 0;
}

.plugin-holo-button.is-checked .plugin-holo-button__status--on {
  opacity: 1;
}

.plugin-holo-button.is-checked .plugin-holo-button__energy {
  left: calc(100% - var(--thumb-size) - var(--thumb-offset));
}

.plugin-holo-button.is-checked .plugin-holo-button__energy-ring {
  opacity: 1;
}

.plugin-holo-button.is-checked .plugin-holo-button__energy-ring:nth-child(1) {
  border-top-color: rgba(0, 255, 150, 0.5);
  border-right-color: rgba(0, 255, 150, 0.3);
}

.plugin-holo-button.is-checked .plugin-holo-button__energy-ring:nth-child(2) {
  border-bottom-color: rgba(0, 255, 150, 0.5);
  border-left-color: rgba(0, 255, 150, 0.3);
}

.plugin-holo-button.is-checked .plugin-holo-button__energy-ring:nth-child(3) {
  border-left-color: rgba(0, 255, 150, 0.5);
  border-top-color: rgba(0, 255, 150, 0.3);
}

.plugin-holo-button.is-checked .plugin-holo-button__interface-line {
  background: var(--on-line);
}

.plugin-holo-button.is-checked .plugin-holo-button__glow {
  background: radial-gradient(ellipse at center, rgba(0, 255, 150, 0.18) 0%, transparent 70%);
}

.plugin-holo-button:hover:not(:disabled) .plugin-holo-button__track {
  box-shadow:
    0 0 20px rgba(0, 150, 255, 0.3),
    inset 0 0 10px rgba(0, 0, 0, 0.8);
}

.plugin-holo-button.is-checked:hover:not(:disabled) .plugin-holo-button__track {
  box-shadow:
    0 0 20px rgba(0, 255, 150, 0.3),
    inset 0 0 10px rgba(0, 0, 0, 0.8);
}

.plugin-holo-button.is-loading .plugin-holo-button__track-line {
  animation-duration: 0.8s;
}

.plugin-holo-button.is-loading .plugin-holo-button__thumb-scan {
  animation-duration: 0.9s;
}

.plugin-holo-button.is-loading .plugin-holo-button__thumb-inner {
  animation-duration: 0.75s;
}

.plugin-holo-button.is-disabled .plugin-holo-button__track-line,
.plugin-holo-button.is-disabled .plugin-holo-button__thumb-inner,
.plugin-holo-button.is-disabled .plugin-holo-button__thumb-scan,
.plugin-holo-button.is-disabled .plugin-holo-button__thumb-particle,
.plugin-holo-button.is-disabled .plugin-holo-button__energy-ring,
.plugin-holo-button.is-disabled .plugin-holo-button__status {
  animation-play-state: paused;
}

@keyframes plugin-holo-track-line {
  0% {
    transform: translateX(0);
  }

  100% {
    transform: translateX(20px);
  }
}

@keyframes plugin-holo-pulse {
  0% {
    opacity: 0.5;
    transform: translate(-50%, -50%) scale(0.9);
  }

  100% {
    opacity: 0.8;
    transform: translate(-50%, -50%) scale(1.1);
  }
}

@keyframes plugin-holo-thumb-scan {
  0% {
    top: -5px;
    opacity: 0;
  }

  20% {
    opacity: 0.7;
  }

  80% {
    opacity: 0.7;
  }

  100% {
    top: calc(var(--thumb-size) + 1px);
    opacity: 0;
  }
}

@keyframes plugin-holo-thumb-particle {
  0% {
    transform: translateY(0) scale(1);
    opacity: 0;
  }

  20% {
    opacity: 0.8;
  }

  100% {
    transform: translateY(-30px) scale(0);
    opacity: 0;
  }
}

@keyframes plugin-holo-blink {
  0%,
  100% {
    opacity: 0.5;
    transform: translateY(-50%) scale(0.9);
  }

  50% {
    opacity: 1;
    transform: translateY(-50%) scale(1.1);
  }
}

@keyframes plugin-holo-spin {
  0% {
    transform: translate(-50%, -50%) rotate(0deg);
  }

  100% {
    transform: translate(-50%, -50%) rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .plugin-holo-button,
  .plugin-holo-button__track,
  .plugin-holo-button__thumb,
  .plugin-holo-button__energy,
  .plugin-holo-button__text,
  .plugin-holo-button__status,
  .plugin-holo-button__interface-line,
  .plugin-holo-button__glow {
    transition: none;
  }

  .plugin-holo-button__track-line,
  .plugin-holo-button__thumb-inner,
  .plugin-holo-button__thumb-scan,
  .plugin-holo-button__thumb-particle,
  .plugin-holo-button__energy-ring,
  .plugin-holo-button__status {
    animation: none;
  }
}
</style>
