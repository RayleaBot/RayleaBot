<script setup lang="ts">
import { nextTick, reactive, ref, watch } from 'vue'
import { ArrowRightIcon } from '@lucide/vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'

import AuthPanel from '@/components/auth/AuthPanel.vue'
import { t } from '@/i18n'

type AuthFormFeedback = {
  level: 'error' | 'warning'
  message: string
}

type FocusableInput = {
  focus: () => void
}

const props = withDefaults(defineProps<{
  feedback?: AuthFormFeedback | null
  pending: boolean
  secretAutocomplete: 'current-password' | 'new-password'
  submitLabel: string
  subtitle: string
  title: string
}>(), {
  feedback: null,
})

const emit = defineEmits<{
  change: []
  submit: [payload: { identifier: string, secret: string }]
}>()

const credentials = reactive({
  identifier: 'admin',
  secret: '',
})
const errors = reactive<{ identifier: string | null, secret: string | null }>({
  identifier: null,
  secret: null,
})
const identifierField = ref<FocusableInput | null>(null)
const secretField = ref<FocusableInput | null>(null)

watch(() => credentials.identifier, (value) => {
  if (value.trim()) {
    errors.identifier = null
  }
  emit('change')
})

watch(() => credentials.secret, (value) => {
  if (value) {
    errors.secret = null
  }
  emit('change')
})

async function handleSubmit() {
  if (props.pending) {
    return
  }

  errors.identifier = credentials.identifier.trim() ? null : t('auth.validation.identifierRequired')
  errors.secret = credentials.secret ? null : t('auth.validation.secretRequired')

  if (errors.identifier || errors.secret) {
    await nextTick()
    if (errors.identifier) {
      identifierField.value?.focus()
    } else {
      secretField.value?.focus()
    }
    return
  }

  emit('submit', {
    identifier: credentials.identifier,
    secret: credentials.secret,
  })
}
</script>

<template>
  <AuthPanel :title="title" :subtitle="subtitle">
    <form class="auth-form" novalidate @submit.prevent="handleSubmit">
      <AppField for="auth-identifier" :label="t('auth.identifier')" :error="errors.identifier || undefined" required>
        <AppInput ref="identifierField" v-model="credentials.identifier" class="auth-form__control" autocomplete="username" :disabled="pending" name="identifier" />
      </AppField>
      <AppField for="auth-secret" :label="t('auth.secret')" :error="errors.secret || undefined" required>
        <AppInput ref="secretField" v-model="credentials.secret" class="auth-form__control" type="password" :autocomplete="secretAutocomplete" :disabled="pending" :show-secret-label="t('auth.showSecret')" :hide-secret-label="t('auth.hideSecret')" :placeholder="t(secretAutocomplete === 'new-password' ? 'auth.newSecretPlaceholder' : 'auth.secretPlaceholder')" name="secret" />
      </AppField>
      <AppAlert v-if="feedback" class="auth-form__feedback" :title="feedback.message" :tone="feedback.level === 'error' ? 'danger' : 'warning'" />
      <AppButton class="auth-form__submit" type="submit" variant="default" :loading="pending">
        <span>{{ submitLabel }}</span><ArrowRightIcon v-if="!pending" aria-hidden="true" />
      </AppButton>
    </form>
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </AuthPanel>
</template>
<style scoped>
.auth-form { display: grid; gap: 22px; margin-top: 32px; }
.auth-form :deep(.app-field) { margin: 0; gap: 6px; }
.auth-form :deep(.app-field__label) { color: var(--auth-text); font-size: 13px; font-weight: 600; line-height: 1.4; }
.auth-form :deep(.auth-form__control) { height: 50px; min-height: 50px; padding-inline: 14px 48px; color: var(--auth-text); font-size: 16px; border: 1px solid var(--auth-border-control); border-radius: 16px; background: var(--auth-glass-control, var(--auth-control)); box-shadow: none; transition: color var(--motion-fast), background-color var(--motion-fast), border-color var(--motion-fast), box-shadow var(--motion-fast); }
.auth-form :deep(.auth-form__control:hover), .auth-form :deep(.auth-form__control:focus) { border-color: var(--auth-brand-stroke, var(--auth-brand-foreground)); background: var(--auth-glass-control-hover, var(--auth-control-hover)); }
.auth-form :deep(.auth-form__control:focus) { outline: none; box-shadow: inset 0 0 0 1px var(--auth-brand-stroke, var(--auth-brand-foreground)); }
.auth-form :deep(.auth-form__control[aria-invalid=true]:focus) { box-shadow: inset 0 0 0 1px var(--auth-danger); }
.auth-form :deep(.auth-form__control[aria-invalid=true]) { border-color: var(--auth-danger); }
.auth-form :deep(.auth-form__control:disabled) { color: var(--auth-text-muted); border-color: var(--auth-border); background: color-mix(in srgb, var(--auth-control) 72%, var(--auth-canvas)); }
.auth-form :deep(input:-webkit-autofill) { box-shadow: 0 0 0 1000px var(--auth-control) inset; -webkit-text-fill-color: var(--auth-text); caret-color: var(--auth-text); }
.auth-form :deep(.app-input-reveal) { top: 3px; right: 3px; width: 44px; height: 44px; color: var(--auth-text-muted); border-radius: 12px; }
.auth-form :deep(.app-input-reveal:hover), .auth-form :deep(.app-input-reveal:focus-visible) { color: var(--auth-brand-foreground); background: var(--auth-brand-soft); }
.auth-form :deep(.app-field__description) { color: var(--auth-danger); }
.auth-form__feedback { border-radius: 16px; }
.auth-form__submit { height: 50px; min-height: 50px; margin-top: 6px; color: var(--auth-on-brand); border-color: var(--auth-brand-fill); border-radius: 16px; background: var(--auth-brand-fill); box-shadow: 0 6px 16px -6px color-mix(in srgb, var(--auth-brand-fill) 50%, transparent), inset 0 1px 0 color-mix(in srgb, var(--auth-glass-edge) 35%, transparent); font-weight: 600; }
.auth-form__submit:not(:disabled):hover { background: var(--auth-brand-fill-hover); }
.auth-form__submit:not(:disabled):active { background: var(--auth-brand-fill-pressed); box-shadow: none; }
.auth-form__submit:not(:disabled):focus-visible { outline: 2px solid var(--auth-on-brand); outline-offset: var(--focus-outline-offset); }
@media (max-width: 600px) { .auth-form { gap: 16px; } }
@media (prefers-reduced-motion: reduce) { .auth-form :deep(.auth-form__control) { transition: none; } }
@media (forced-colors: active) {
  .auth-form :deep(.auth-form__control) { border-color: CanvasText; }
  .auth-form :deep(.auth-form__control:focus), .auth-form__submit:not(:disabled):focus-visible { outline: 2px solid Highlight; outline-offset: var(--focus-outline-offset); box-shadow: none; }
  .auth-form :deep(.auth-form__control[aria-invalid=true]) { border-color: Mark; }
}
</style>
