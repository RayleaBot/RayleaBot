<script setup lang="ts">
import { MotionDirective as vMotion } from '@vueuse/motion'
import { reactive, ref } from 'vue'
import type { FormInstance, Rule } from 'ant-design-vue/es/form'
import { useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { toLoginErrorMessage } from '@/lib/auth-feedback'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()
const submitError = ref<string | null>(null)

const form = reactive({
  identifier: 'admin',
  secret: '',
})
const formRef = ref<FormInstance>()
const rules: Record<string, Rule[]> = {
  identifier: [{ required: true, message: t('auth.validation.identifierRequired'), trigger: 'blur' }],
  secret: [{ required: true, message: t('auth.validation.secretRequired'), trigger: 'blur' }],
}

async function submit() {
  submitError.value = null

  try {
    await formRef.value?.validate()
    await sessionStore.login(form)
    notifySuccess(t('auth.feedback.loginSuccess'))
    await router.push({ name: 'status' })
  } catch (error) {
    const message = toLoginErrorMessage(error)
    submitError.value = message
    notifyError(message)
  }
}
</script>

<template>
  <a-card
    v-motion="{
      initial: { opacity: 0, y: 12 },
      enter: { opacity: 1, y: 0, transition: { duration: 350, ease: 'easeOut', delay: 120 } },
    }"
    class="auth-panel-card"
    :bordered="false"
  >
    <div class="auth-panel-card__copy">
      <h1>{{ t('auth.loginTitle') }}</h1>
      <p>{{ t('auth.loginBody') }}</p>
    </div>

    <a-alert
      v-if="sessionStore.bootstrapError"
      :message="t('auth.alerts.bootstrapUnavailable')"
      type="warning"
      :description="sessionStore.bootstrapError"
      show-icon
      class="section-gap"
    />

    <a-alert
      v-if="sessionStore.launcherAdmissionHint"
      :message="t('auth.alerts.launcherManualLogin')"
      type="warning"
      :description="sessionStore.launcherAdmissionHint"
      show-icon
      class="section-gap"
    />

    <a-alert
      v-if="submitError"
      :message="t('auth.alerts.loginIncomplete')"
      type="error"
      :description="submitError"
      role="alert"
      aria-live="assertive"
      show-icon
      class="section-gap"
    />

    <a-form ref="formRef" layout="vertical" :model="form" :rules="rules">
      <a-form-item
        v-motion="{
          initial: { opacity: 0, y: 8 },
          enter: { opacity: 1, y: 0, transition: { duration: 300, ease: 'easeOut', delay: 200 } },
        }"
        :label="t('auth.identifier')"
        name="identifier"
      >
        <a-input v-model:value="form.identifier" autocomplete="username" :aria-label="t('auth.identifier')" />
      </a-form-item>

      <a-form-item
        v-motion="{
          initial: { opacity: 0, y: 8 },
          enter: { opacity: 1, y: 0, transition: { duration: 300, ease: 'easeOut', delay: 280 } },
        }"
        :label="t('auth.secret')"
        name="secret"
      >
        <a-input-password v-model:value="form.secret" autocomplete="current-password" :aria-label="t('auth.secret')" />
      </a-form-item>

      <a-button
        v-motion="{
          initial: { opacity: 0, y: 8 },
          enter: { opacity: 1, y: 0, transition: { duration: 300, ease: 'easeOut', delay: 360 } },
        }"
        type="primary"
        class="auth-submit"
        :aria-label="t('auth.loginSubmit')"
        :loading="sessionStore.loginPending"
        @click="submit"
      >
        {{ t('auth.loginSubmit') }}
      </a-button>
    </a-form>
  </a-card>
</template>

<style scoped lang="scss">
.auth-panel-card__copy {
  display: grid;
  gap: 8px;
  margin-bottom: 18px;
}

:deep(.ant-input),
:deep(.ant-input-password) {
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
}

:deep(.ant-input:focus),
:deep(.ant-input-password .ant-input:focus),
:deep(.ant-input-focused) {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent);
}

.auth-panel-card {
  backdrop-filter: blur(8px);
  background: color-mix(in srgb, var(--surface-strong) 92%, transparent) !important;
}
</style>
