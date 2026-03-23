<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { useSessionStore } from '@/stores/session'

const router = useRouter()
const sessionStore = useSessionStore()

const form = reactive({
  identifier: 'admin',
  secret: '',
})
const formRef = ref()

async function submit() {
  await formRef.value?.validate()
  await sessionStore.login(form)
  ElMessage.success('登录成功')
  await router.push({ name: 'status' })
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <div class="auth-copy">
        <div class="page-eyebrow">Session</div>
        <h1>登录管理面</h1>
        <p>当前只消费既有 session token surface，不引入额外刷新协议。</p>
      </div>

      <el-alert
        v-if="sessionStore.bootstrapError"
        title="setup 状态读取失败，仍可尝试登录"
        type="warning"
        :description="sessionStore.bootstrapError"
        show-icon
        class="section-gap"
      />

      <el-form ref="formRef" :model="form" label-position="top">
        <el-form-item label="Identifier" prop="identifier" :rules="[{ required: true, message: '请输入 identifier' }]">
          <el-input v-model="form.identifier" autocomplete="username" />
        </el-form-item>

        <el-form-item label="Secret" prop="secret" :rules="[{ required: true, message: '请输入 secret' }]">
          <el-input v-model="form.secret" type="password" show-password autocomplete="current-password" />
        </el-form-item>

        <el-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>
