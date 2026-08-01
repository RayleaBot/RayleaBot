<script setup lang="ts">
import { ref } from 'vue'
import { Button as AButton, Input as AInput, Select as ASelect, SelectOption as ASelectOption } from 'ant-design-vue'
import { usePluginHost } from '@rayleabot/plugin-ui'

import { normalizeSettings } from './model'

const host = usePluginHost()
const draft = ref(normalizeSettings({}))
const status = ref('正在连接宿主…')
const secretDraft = ref('')
const secretConfigured = ref(false)
const secretStatus = ref('正在读取密钥状态…')

void host.ready.then((init) => {
  draft.value = normalizeSettings(init.config)
  secretConfigured.value = Boolean(init.secrets_configured.api_key)
  status.value = '配置已加载'
  secretStatus.value = secretConfigured.value ? 'API 密钥已配置' : 'API 密钥未配置'
})

async function reload() {
  const response = await host.client.reloadSettings()
  draft.value = normalizeSettings(response.config)
  status.value = '已重新读取配置'
}

async function save() {
  const response = await host.client.saveSettings({ ...draft.value })
  draft.value = normalizeSettings(response.config)
  status.value = '配置已保存'
}

async function saveSecret() {
  const response = await host.client.setSecrets({ api_key: secretDraft.value })
  secretConfigured.value = Boolean(response.configured.api_key)
  secretDraft.value = ''
  secretStatus.value = 'API 密钥已覆盖'
}

async function deleteSecret() {
  const response = await host.client.deleteSecrets(['api_key'])
  secretConfigured.value = Boolean(response.configured.api_key)
  secretDraft.value = ''
  secretStatus.value = 'API 密钥已删除'
}
</script>

<template>
  <main>
    <p class="eyebrow">GO + VUE PLUGIN EXAMPLE</p>
    <h1>配置面板示例</h1>
    <p class="intro">页面运行于插件独立域，只能通过 MessageChannel bridge v2 读写自身配置。</p>
    <section>
      <label><span>默认城市</span><AInput v-model:value="draft.default_city" data-testid="default-city-input" /></label>
      <label><span>温度单位</span><ASelect v-model:value="draft.unit" data-testid="unit-select"><ASelectOption value="celsius">摄氏度</ASelectOption><ASelectOption value="fahrenheit">华氏度</ASelectOption></ASelect></label>
      <pre data-testid="settings-preview">{{ draft }}</pre>
      <footer><span data-testid="settings-status">{{ status }}</span><div><AButton @click="reload">重新读取</AButton><AButton type="primary" :disabled="!draft.default_city.trim()" data-testid="save-settings" @click="save">保存</AButton></div></footer>
    </section>
    <section>
      <h2>敏感配置</h2>
      <p data-testid="secret-status">{{ secretStatus }}</p>
      <label><span>API 密钥</span><AInput v-model:value="secretDraft" type="password" autocomplete="new-password" data-testid="secret-input" /></label>
      <footer>
        <span>{{ secretConfigured ? '当前已配置' : '当前未配置' }}</span>
        <div><AButton danger :disabled="!secretConfigured" data-testid="delete-secret" @click="deleteSecret">删除</AButton><AButton type="primary" :disabled="!secretDraft" data-testid="save-secret" @click="saveSecret">覆盖保存</AButton></div>
      </footer>
    </section>
  </main>
</template>
