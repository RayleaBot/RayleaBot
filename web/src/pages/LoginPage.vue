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
  ElMessage.success('已登录')
  await router.push({ name: 'status' })
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <div class="auth-copy">
        <div class="page-eyebrow">管理界面</div>
        <h1>登录</h1>
        <p>输入管理员账号和密钥后进入管理界面。</p>
      </div>

      <el-alert
        v-if="sessionStore.bootstrapError"
        title="暂时无法确认当前状态"
        type="warning"
        :description="sessionStore.bootstrapError"
        show-icon
        class="section-gap"
      />

      <el-alert
        v-if="sessionStore.launcherAdmissionHint"
        title="请手动登录"
        type="warning"
        :description="sessionStore.launcherAdmissionHint"
        show-icon
        class="section-gap"
      />

      <el-form ref="formRef" :model="form" label-position="top">
        <el-form-item label="管理员账号" prop="identifier" :rules="[{ required: true, message: '请输入管理员账号' }]">
          <el-input v-model="form.identifier" autocomplete="username" />
        </el-form-item>

        <el-form-item label="管理员密钥" prop="secret" :rules="[{ required: true, message: '请输入管理员密钥' }]">
          <el-input v-model="form.secret" type="password" show-password autocomplete="current-password" />
        </el-form-item>

        <el-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>
