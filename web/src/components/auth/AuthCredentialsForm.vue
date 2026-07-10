<script setup lang="ts">
import { reactive, ref, watch } from 'vue'

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

const identifierField = ref<InstanceType<typeof AuthTextField> | null>(null)
const secretField = ref<InstanceType<typeof AuthTextField> | null>(null)

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

function handleSubmit() {
  if (props.pending) {
    return
  }
  errors.identifier = identifier.value.trim() ? null : t('auth.validation.identifierRequired')
  errors.secret = secret.value ? null : t('auth.validation.secretRequired')
  if (errors.identifier || errors.secret) {
    if (errors.identifier) {
      identifierField.value?.focus()
    } else {
      secretField.value?.focus()
    }
    return
  }
  emit('submit', { identifier: identifier.value, secret: secret.value })
}
</script>

<template>
  <section class="auth-panel-card">
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
  position: relative;
  z-index: 1;
  width: min(420px, calc(100vw - 32px));
  padding: var(--auth-card-padding);
  border: 1px solid var(--border);
  border-radius: var(--auth-card-radius);
  background: var(--surface);
  box-shadow: 0 16px 40px rgba(2, 6, 23, 0.14);
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
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.25);
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
  color: #fff;
  font-family: inherit;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.08em;
  cursor: pointer;
  transition: background-color 120ms ease, filter 120ms ease;

  &:hover:not(:disabled) {
    filter: brightness(1.06);
  }

  &:active:not(:disabled) {
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

@media (prefers-reduced-motion: reduce) {
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

@keyframes authSpin {
  to {
    transform: rotate(360deg);
  }
}
</style>
