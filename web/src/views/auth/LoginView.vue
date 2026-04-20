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
  identifier: [{ required: true, message: '请输入管理员账号', trigger: 'blur' }],
  secret: [{ required: true, message: '请输入管理员密钥', trigger: 'blur' }],
}

async function submit() {
  submitError.value = null

  try {
    await formRef.value?.validate()
    await sessionStore.login(form)
    notifySuccess('已登录')
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
      message="暂时无法进入管理界面"
      type="warning"
      :description="sessionStore.bootstrapError"
      show-icon
      class="section-gap"
    />

    <a-alert
      v-if="sessionStore.launcherAdmissionHint"
      message="请手动登录"
      type="warning"
      :description="sessionStore.launcherAdmissionHint"
      show-icon
      class="section-gap"
    />

    <a-alert
      v-if="submitError"
      message="登录未完成"
      type="error"
      :description="submitError"
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
        <a-input v-model:value="form.identifier" autocomplete="username" />
      </a-form-item>

      <a-form-item
        v-motion="{
          initial: { opacity: 0, y: 8 },
          enter: { opacity: 1, y: 0, transition: { duration: 300, ease: 'easeOut', delay: 280 } },
        }"
        :label="t('auth.secret')"
        name="secret"
      >
        <a-input-password v-model:value="form.secret" autocomplete="current-password" />
      </a-form-item>

      <a-button
        v-motion="{
          initial: { opacity: 0, y: 8 },
          enter: { opacity: 1, y: 0, transition: { duration: 300, ease: 'easeOut', delay: 360 } },
        }"
        type="primary"
        class="auth-submit"
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
