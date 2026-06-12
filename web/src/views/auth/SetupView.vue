<script setup lang="ts">
import { reactive, ref } from 'vue'
import type { FormInstance, Rule } from 'ant-design-vue/es/form'
import { useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { toSetupErrorMessage } from '@/lib/auth-feedback'
import { t } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const form = reactive({
  identifier: 'admin',
  secret: '',
})
const formRef = ref<FormInstance>()
const isShaking = ref(false)
const rules: Record<string, Rule[]> = {
  identifier: [{ required: true, message: t('auth.validation.identifierRequired'), trigger: 'blur' }],
  secret: [{ required: true, message: t('auth.validation.secretRequired'), trigger: 'blur' }],
}

async function submit() {
  try {
    await formRef.value?.validate()
    await sessionStore.setupAdmin(form)
    notifySuccess(t('auth.feedback.setupSuccess'))
    await router.push(resolvePostAuthTarget())
  } catch (error) {
    const message = toSetupErrorMessage(error)
    notifyError(message)
    triggerShake()
  }
}

function triggerShake() {
  isShaking.value = true
  setTimeout(() => {
    isShaking.value = false
  }, 400)
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
  <a-card
    class="auth-panel-card"
    :class="{ 'is-shaking': isShaking }"
    :bordered="false"
  >
    <div class="auth-card-header">
      <span class="auth-brand-badge" aria-hidden="true">R</span>
      <div class="auth-card-header__title">
        <h1>{{ t('auth.setupTitle') }}</h1>
        <p>{{ t('auth.setupBody') }}</p>
      </div>
    </div>

    <a-form ref="formRef" layout="vertical" :model="form" :rules="rules">
      <a-form-item
        :label="t('auth.identifier')"
        name="identifier"
      >
        <a-input v-model:value="form.identifier" autocomplete="username" :aria-label="t('auth.identifier')" />
      </a-form-item>

      <a-form-item
        :label="t('auth.secret')"
        name="secret"
      >
        <a-input-password v-model:value="form.secret" autocomplete="new-password" :aria-label="t('auth.secret')" />
      </a-form-item>

      <a-button
        type="primary"
        class="auth-submit"
        :aria-label="t('auth.setupSubmit')"
        :loading="sessionStore.loginPending"
        @click="submit"
      >
        {{ t('auth.setupSubmit') }}
      </a-button>
    </a-form>
  </a-card>
</template>
