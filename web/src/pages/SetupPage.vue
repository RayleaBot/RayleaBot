<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

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
const formRef = ref()

async function submit() {
  submitError.value = null

  try {
    await formRef.value?.validate()
    await sessionStore.setupAdmin(form)
    ElMessage.success('管理员账号已创建')
    await router.push({ name: 'status' })
  } catch (error) {
    const message = toSetupErrorMessage(error)
    submitError.value = message
    ElMessage.error(message)
  }
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <div class="auth-copy">
        <div class="page-eyebrow">{{ t('auth.surface') }}</div>
        <h1>{{ t('auth.setupTitle') }}</h1>
        <p>{{ t('auth.setupBody') }}</p>
      </div>

      <el-alert
        v-if="submitError"
        title="创建管理员账号未完成"
        type="error"
        :description="submitError"
        show-icon
        class="section-gap"
      />

      <el-form ref="formRef" :model="form" label-position="top">
        <el-form-item :label="t('auth.identifier')" prop="identifier" :rules="[{ required: true, message: '请输入管理员账号' }]">
          <el-input v-model="form.identifier" autocomplete="username" />
        </el-form-item>

        <el-form-item :label="t('auth.secret')" prop="secret" :rules="[{ required: true, message: '请输入管理员密钥' }]">
          <el-input v-model="form.secret" type="password" show-password autocomplete="new-password" />
        </el-form-item>

        <el-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
          {{ t('auth.setupSubmit') }}
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>
