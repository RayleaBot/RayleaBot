<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'

import AuthTextField from '@/components/auth/AuthTextField.vue'
import { t } from '@/i18n'

const props = defineProps<{
  title: string
  subtitle: string
  submitLabel: string
  pending: boolean
  secretAutocomplete: 'current-password' | 'new-password'
}>()

const emit = defineEmits<{
  submit: [payload: { identifier: string, secret: string }]
}>()

const identifier = ref('admin')
const secret = ref('')
const errors = reactive<{ identifier: string | null, secret: string | null }>({
  identifier: null,
  secret: null,
})

const cardRef = ref<HTMLElement | null>(null)
const identifierField = ref<InstanceType<typeof AuthTextField> | null>(null)
const secretField = ref<InstanceType<typeof AuthTextField> | null>(null)
const isShaking = ref(false)
const pointerIn = ref(false)

let shakeTimer: ReturnType<typeof setTimeout> | null = null
let glowFrame: number | null = null
let glowX = 0
let glowY = 0

watch(identifier, (value) => {
  if (value.trim()) {
    errors.identifier = null
  }
})

watch(secret, (value) => {
  if (value) {
    errors.secret = null
  }
})

function shake() {
  isShaking.value = true
  if (shakeTimer) {
    clearTimeout(shakeTimer)
  }
  shakeTimer = setTimeout(() => {
    isShaking.value = false
  }, 400)
}

function handleSubmit() {
  if (props.pending) {
    return
  }
  errors.identifier = identifier.value.trim() ? null : t('auth.validation.identifierRequired')
  errors.secret = secret.value ? null : t('auth.validation.secretRequired')
  if (errors.identifier || errors.secret) {
    shake()
    if (errors.identifier) {
      identifierField.value?.focus()
    } else {
      secretField.value?.focus()
    }
    return
  }
  emit('submit', { identifier: identifier.value, secret: secret.value })
}

function handlePointerMove(event: PointerEvent) {
  const card = cardRef.value
  if (!card) {
    return
  }
  const rect = card.getBoundingClientRect()
  glowX = event.clientX - rect.left
  glowY = event.clientY - rect.top
  if (glowFrame !== null) {
    return
  }
  glowFrame = requestAnimationFrame(() => {
    glowFrame = null
    cardRef.value?.style.setProperty('--mx', `${glowX}px`)
    cardRef.value?.style.setProperty('--my', `${glowY}px`)
  })
}

onBeforeUnmount(() => {
  if (shakeTimer) {
    clearTimeout(shakeTimer)
  }
  if (glowFrame !== null) {
    cancelAnimationFrame(glowFrame)
  }
})

defineExpose({ shake })
</script>

<template>
  <section
    ref="cardRef"
    class="auth-panel-card"
    :class="{ 'is-shaking': isShaking, 'is-pointer-in': pointerIn }"
    @pointermove="handlePointerMove"
    @pointerenter="pointerIn = true"
    @pointerleave="pointerIn = false"
  >
    <header class="auth-panel-card__header">
      <span class="auth-panel-card__badge" aria-hidden="true">R</span>
      <p class="auth-panel-card__eyebrow">{{ t('app.brand') }} · {{ t('auth.surface') }}</p>
      <h1 class="auth-panel-card__title">{{ title }}</h1>
      <p class="auth-panel-card__subtitle">{{ subtitle }}</p>
    </header>

    <form
      class="auth-panel-card__form"
      novalidate
      @submit.prevent="handleSubmit"
    >
      <AuthTextField
        ref="identifierField"
        v-model="identifier"
        name="identifier"
        :label="t('auth.identifier')"
        autocomplete="username"
        :error="errors.identifier"
      />
      <AuthTextField
        ref="secretField"
        v-model="secret"
        name="secret"
        type="password"
        :label="t('auth.secret')"
        :autocomplete="secretAutocomplete"
        :error="errors.secret"
      />
      <button
        type="submit"
        class="auth-submit"
        :disabled="pending"
        :aria-busy="pending || undefined"
        @click.prevent="handleSubmit"
      >
        <span
          v-if="pending"
          class="auth-submit__spinner"
          aria-hidden="true"
        />
        <span class="auth-submit__label">{{ submitLabel }}</span>
      </button>
    </form>
  </section>
</template>

<style scoped lang="scss">
.auth-panel-card {
  --mx: 50%;
  --my: 0%;
  position: relative;
  z-index: 1;
  width: min(420px, calc(100vw - 32px));
  padding: var(--auth-card-padding);
  border: 1px solid var(--auth-glass-border);
  border-radius: var(--auth-card-radius);
  background: var(--auth-glass-bg);
  backdrop-filter: blur(22px) saturate(150%);
  -webkit-backdrop-filter: blur(22px) saturate(150%);
  box-shadow:
    0 24px 60px rgba(2, 6, 23, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .auth-panel-card {
    background: color-mix(in srgb, var(--surface) 94%, transparent);
  }
}

.auth-panel-card::before,
.auth-panel-card::after {
  content: '';
  position: absolute;
  border-radius: inherit;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.4s ease;
}

.auth-panel-card::before {
  inset: -1px;
  padding: 1px;
  background: radial-gradient(
    240px circle at var(--mx) var(--my),
    color-mix(in srgb, var(--auth-accent) 85%, white),
    transparent 70%
  );
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask-composite: exclude;
}

.auth-panel-card::after {
  inset: 0;
  background: radial-gradient(
    360px circle at var(--mx) var(--my),
    color-mix(in srgb, var(--auth-accent) 10%, transparent),
    transparent 65%
  );
}

.auth-panel-card.is-pointer-in::before,
.auth-panel-card.is-pointer-in::after {
  opacity: 1;
}

.auth-panel-card.is-shaking {
  animation: cardShake 0.4s ease;
}

.auth-panel-card__header {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  text-align: center;
}

.auth-panel-card__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: linear-gradient(135deg, var(--auth-accent-deep), var(--auth-accent));
  box-shadow:
    0 8px 24px color-mix(in srgb, var(--auth-accent) 45%, transparent),
    inset 0 0 0 1px rgba(255, 255, 255, 0.25);
  color: #fff;
  font-size: 22px;
  font-weight: 700;
}

.auth-panel-card__eyebrow {
  margin: 2px 0 0;
  color: var(--muted);
  font-size: 12px;
  letter-spacing: 0.12em;
}

.auth-panel-card__title {
  margin: 0;
  color: var(--text);
  font-size: 1.6rem;
  font-weight: 650;
  line-height: 1.25;
}

.auth-panel-card__subtitle {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  line-height: 1.6;
}

.auth-panel-card__form {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 18px;
  margin-top: 28px;
}

.auth-submit {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  height: 48px;
  margin-top: 6px;
  border: 0;
  border-radius: 14px;
  background: linear-gradient(135deg, var(--auth-accent-deep), var(--auth-accent));
  box-shadow: 0 10px 24px color-mix(in srgb, var(--auth-accent-deep) 35%, transparent);
  color: #fff;
  font-family: inherit;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.08em;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease, filter 0.2s ease;

  &:hover:not(:disabled) {
    transform: translateY(-1px);
    box-shadow: 0 14px 30px color-mix(in srgb, var(--auth-accent-deep) 45%, transparent);
    filter: brightness(1.06);
  }

  &:active:not(:disabled) {
    transform: translateY(0) scale(0.99);
    filter: brightness(0.98);
  }

  &:disabled {
    opacity: 0.75;
    cursor: default;
  }
}

.auth-submit__spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.45);
  border-top-color: #fff;
  border-radius: 50%;
  animation: authSpin 0.8s linear infinite;
}

@media (prefers-reduced-motion: no-preference) {
  .auth-panel-card {
    animation: authCardIn 0.55s cubic-bezier(0.22, 1, 0.36, 1) both;
  }

  .auth-panel-card__header {
    animation: authRiseIn 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
    animation-delay: 0.05s;
  }

  .auth-panel-card__form > * {
    animation: authRiseIn 0.5s cubic-bezier(0.22, 1, 0.36, 1) both;
  }

  .auth-panel-card__form > :nth-child(1) {
    animation-delay: 0.12s;
  }

  .auth-panel-card__form > :nth-child(2) {
    animation-delay: 0.18s;
  }

  .auth-panel-card__form > :nth-child(3) {
    animation-delay: 0.24s;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-panel-card::before,
  .auth-panel-card::after {
    display: none;
  }

  .auth-panel-card.is-shaking {
    animation: none;
  }

  .auth-submit {
    transition: none;
  }
}

@media (max-width: 480px) {
  .auth-panel-card {
    padding: 28px 22px;
    border-radius: 16px;
  }
}

@keyframes cardShake {
  0%,
  100% {
    transform: translateX(0);
  }

  20% {
    transform: translateX(-10px);
  }

  40% {
    transform: translateX(8px);
  }

  60% {
    transform: translateX(-6px);
  }

  80% {
    transform: translateX(4px);
  }
}

@keyframes authCardIn {
  from {
    opacity: 0;
    transform: translateY(16px) scale(0.985);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@keyframes authRiseIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@keyframes authSpin {
  to {
    transform: rotate(360deg);
  }
}
</style>
