<script setup lang="ts">
import { useRouter } from 'vue-router'

import { notifyError, notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AuthCredentialsForm from '@/components/auth/AuthCredentialsForm.vue'
import { toLoginErrorMessage } from '@/lib/auth-feedback'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

useToastFeedback(() => (
  sessionStore.bootstrapError
      ? {
          key: `bootstrap:${sessionStore.bootstrapError}`,
          level: 'warning' as const,
          message: sessionStore.bootstrapError,
        }
    : null
))

async function handleSubmit(payload: { identifier: string, secret: string }) {
  try {
    await sessionStore.login(payload)
    notifySuccess(t('auth.feedback.loginSuccess'))
    await router.push(resolvePostAuthTarget())
  } catch (error) {
    notifyError(toLoginErrorMessage(error))
  }
}

function resolvePostAuthTarget() {
  const redirect = router.currentRoute.value.query.redirect
  const candidate = Array.isArray(redirect) ? redirect[0] : redirect
  if (typeof candidate === 'string' && candidate.startsWith('/') && !candidate.startsWith('//') && !/\\/.test(candidate)) {
    return candidate
  }

  return { name: 'status' }
}
</script>

<template>
  <AuthCredentialsForm
    :title="t('auth.loginTitle')"
    :subtitle="t('auth.loginBody')"
    :submit-label="t('auth.loginSubmit')"
    :pending="sessionStore.loginPending"
    secret-autocomplete="current-password"
    @submit="handleSubmit"
  />
</template>
