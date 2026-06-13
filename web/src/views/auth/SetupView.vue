<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AuthCredentialsForm from '@/components/auth/AuthCredentialsForm.vue'
import { toSetupErrorMessage } from '@/lib/auth-feedback'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()
const formRef = ref<InstanceType<typeof AuthCredentialsForm> | null>(null)

async function handleSubmit(payload: { identifier: string, secret: string }) {
  try {
    await sessionStore.setupAdmin(payload)
    notifySuccess(t('auth.feedback.setupSuccess'))
    await router.push(resolvePostAuthTarget())
  } catch (error) {
    notifyError(toSetupErrorMessage(error))
    formRef.value?.shake()
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
    ref="formRef"
    :title="t('auth.setupTitle')"
    :subtitle="t('auth.setupBody')"
    :submit-label="t('auth.setupSubmit')"
    :pending="sessionStore.loginPending"
    secret-autocomplete="new-password"
    @submit="handleSubmit"
  />
</template>
