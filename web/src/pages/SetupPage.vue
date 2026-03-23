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
  ElMessage.success('初始化成功')
  await router.push({ name: 'status' })
}
</script>

<template>
  <div class="auth-page">
    <el-card class="auth-card">
      <div class="auth-copy">
        <div class="page-eyebrow">Setup</div>
        <h1>初始化管理账号</h1>
        <p>服务端尚未完成首次 bootstrap。完成后会直接建立管理会话。</p>
      </div>

      <el-form ref="formRef" :model="form" label-position="top">
        <el-form-item label="Identifier" prop="identifier" :rules="[{ required: true, message: '请输入 identifier' }]">
          <el-input v-model="form.identifier" autocomplete="username" />
        </el-form-item>

        <el-form-item label="Secret" prop="secret" :rules="[{ required: true, message: '请输入 secret' }]">
          <el-input v-model="form.secret" type="password" show-password autocomplete="new-password" />
        </el-form-item>

        <el-button type="primary" class="auth-submit" :loading="sessionStore.loginPending" @click="submit">
          初始化并登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>
