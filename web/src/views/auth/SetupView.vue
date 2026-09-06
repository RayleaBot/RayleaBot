<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AuthCredentialsForm from '@/components/auth/AuthCredentialsForm.vue'
import { toSetupErrorMessage } from '@/lib/auth-feedback'
import { readInternalRedirectTarget } from '@/lib/route-redirect'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()
const submissionError = ref<string | null>(null)

const formFeedback = computed(() => {
  if (submissionError.value) {
    return { level: 'error' as const, message: submissionError.value }
  }
  if (sessionStore.bootstrapError) {
    return { level: 'warning' as const, message: sessionStore.bootstrapError }
  }
  return null
})

async function handleSubmit(payload: { identifier: string, secret: string }) {
  submissionError.value = null
  try {
    await sessionStore.setupAdmin(payload)
    notifySuccess(t('auth.feedback.setupSuccess'))
    await router.push(readInternalRedirectTarget(router.currentRoute.value.query.redirect) ?? { name: 'status' })
  } catch (error) {
    submissionError.value = toSetupErrorMessage(error)
  }
}
</script>

<template>
  <AuthCredentialsForm
    :feedback="formFeedback"
    :title="t('auth.setupTitle')"
    :subtitle="t('auth.setupBody')"
    :submit-label="t('auth.setupSubmit')"
    :pending="sessionStore.loginPending"
    secret-autocomplete="new-password"
    @change="submissionError = null"
    @submit="handleSubmit"
  >
    <template #footer><p>{{ t('auth.setupHint') }}</p></template>
  </AuthCredentialsForm>
</template>
