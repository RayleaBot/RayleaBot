<script setup lang="ts">
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
  <a-card class="auth-panel-card" :bordered="false">
    <div class="auth-panel-card__copy">
      <div class="page-eyebrow">{{ t('auth.surface') }}</div>
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
      <a-form-item :label="t('auth.identifier')" name="identifier">
        <a-input v-model:value="form.identifier" autocomplete="username" />
      </a-form-item>

      <a-form-item :label="t('auth.secret')" name="secret">
        <a-input-password v-model:value="form.secret" autocomplete="current-password" />
      </a-form-item>

      <a-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
        {{ t('auth.loginSubmit') }}
      </a-button>
    </a-form>
  </a-card>
</template>

<style scoped lang="scss">
.auth-panel-card__copy {
  display: grid;
  gap: 12px;
  margin-bottom: 20px;
}
</style>
