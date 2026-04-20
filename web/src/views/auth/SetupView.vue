<script setup lang="ts">
import { MotionDirective as vMotion } from '@vueuse/motion'
import { reactive, ref } from 'vue'
import type { FormInstance, Rule } from 'ant-design-vue/es/form'
import { useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import { toSetupErrorMessage } from '@/lib/auth-feedback'
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
    await sessionStore.setupAdmin(form)
    notifySuccess('管理员账号已创建')
    await router.push({ name: 'status' })
  } catch (error) {
    const message = toSetupErrorMessage(error)
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
      <h1>{{ t('auth.setupTitle') }}</h1>
      <p>{{ t('auth.setupBody') }}</p>
    </div>

    <a-alert
      v-if="submitError"
      message="创建管理员账号未完成"
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
        <a-input-password v-model:value="form.secret" autocomplete="new-password" />
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
        {{ t('auth.setupSubmit') }}
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
</style>
