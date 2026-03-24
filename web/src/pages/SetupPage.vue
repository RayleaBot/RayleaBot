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
  await sessionStore.setupAdmin(form)
  ElMessage.success('管理员账号已创建')
  await router.push({ name: 'status' })
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <div class="auth-copy">
        <div class="page-eyebrow">管理界面</div>
        <h1>创建管理员账号</h1>
        <p>首次使用时，请先创建管理员账号。</p>
      </div>

      <el-form ref="formRef" :model="form" label-position="top">
        <el-form-item label="管理员账号" prop="identifier" :rules="[{ required: true, message: '请输入管理员账号' }]">
          <el-input v-model="form.identifier" autocomplete="username" />
        </el-form-item>

        <el-form-item label="管理员密钥" prop="secret" :rules="[{ required: true, message: '请输入管理员密钥' }]">
          <el-input v-model="form.secret" type="password" show-password autocomplete="new-password" />
        </el-form-item>

        <el-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
          创建并进入管理界面
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>
