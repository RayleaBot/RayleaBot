<script setup lang="ts">
import { computed, ref } from 'vue'
import { Alert as AAlert, Button as AButton, Input as AInput, Select as ASelect, SelectOption as ASelectOption } from 'ant-design-vue'
import { usePluginHost } from '@rayleabot/plugin-ui'

import { normalizeSettings, type FortuneSettings, validateSettings } from './model'

const host = usePluginHost()
const hostErrorMessage = computed(() => host.error.value?.message ?? '')
const draft = ref<FortuneSettings>(normalizeSettings({}))
const status = ref('正在连接宿主…')
const saving = ref(false)
const errors = computed(() => validateSettings(draft.value))
const commandText = computed({
  get: () => draft.value.trigger_commands.join('\n'),
  set: (value: string) => { draft.value.trigger_commands = lines(value) },
})
const statsCommandText = computed({
  get: () => draft.value.stats_trigger_commands.join('\n'),
  set: (value: string) => { draft.value.stats_trigger_commands = lines(value) },
})
const goodText = computed({
  get: () => draft.value.good_actions.join('\n'),
  set: (value: string) => { draft.value.good_actions = lines(value) },
})
const badText = computed({
  get: () => draft.value.bad_actions.join('\n'),
  set: (value: string) => { draft.value.bad_actions = lines(value) },
})

void host.ready.then((init) => {
  draft.value = normalizeSettings(init.config)
  status.value = '配置已加载'
})

async function reload() {
  const result = await host.client.reloadSettings()
  draft.value = normalizeSettings(result.config)
  status.value = '已重新读取配置'
}

async function save() {
  if (errors.value.length > 0) return
  saving.value = true
  try {
    const result = await host.client.saveSettings(structuredClone(draft.value) as unknown as Record<string, unknown>)
    draft.value = normalizeSettings(result.config)
    status.value = '配置已保存'
  } finally {
    saving.value = false
  }
}

function addFortune() {
  draft.value.fortunes.push({ name: '大吉', stars: '★★★★★★★', sign: '', explanation: '' })
}

function addSpecialDate() {
  draft.value.special_dates.push({ date: '', fortune_name: draft.value.fortunes[0]?.name ?? '' })
}

function lines(value: string): string[] {
  return [...new Set(value.split(/\r?\n/).map((item) => item.trim()).filter(Boolean))]
}
</script>

<template>
  <main class="page-shell">
    <header class="page-header">
      <div>
        <p class="eyebrow">RAYLEABOT · PLUGIN</p>
        <h1>运势设置</h1>
        <p>管理触发词、抽签内容和特殊日期；保存后插件立即重新读取配置。</p>
      </div>
      <span class="status">{{ status }}</span>
    </header>

    <AAlert v-if="hostErrorMessage" type="error" :message="hostErrorMessage" show-icon />
    <AAlert v-if="errors.length" type="warning" :message="errors[0]" :description="errors.slice(1).join('；')" show-icon />

    <section class="panel grid two">
      <label>
        <span>运势触发词（每行一个）</span>
        <AInput.TextArea v-model:value="commandText" :rows="4" />
      </label>
      <label>
        <span>统计触发词（每行一个）</span>
        <AInput.TextArea v-model:value="statsCommandText" :rows="4" />
      </label>
      <label>
        <span>时区</span>
        <ASelect v-model:value="draft.timezone">
          <ASelectOption value="Asia/Shanghai">Asia/Shanghai</ASelectOption>
          <ASelectOption value="UTC">UTC</ASelectOption>
        </ASelect>
      </label>
    </section>

    <section class="panel">
      <div class="section-heading">
        <div><h2>运势池</h2><p>{{ draft.fortunes.length }} 条可抽取内容</p></div>
        <AButton type="primary" ghost @click="addFortune">新增运势</AButton>
      </div>
      <div class="fortune-grid">
        <article v-for="(item, index) in draft.fortunes" :key="index" class="fortune-card">
          <div class="card-topline">
            <strong>#{{ index + 1 }}</strong>
            <AButton danger type="text" @click="draft.fortunes.splice(index, 1)">删除</AButton>
          </div>
          <AInput v-model:value="item.name" placeholder="名称" />
          <AInput v-model:value="item.stars" placeholder="星级" />
          <AInput.TextArea v-model:value="item.sign" :rows="2" placeholder="签文" />
          <AInput.TextArea v-model:value="item.explanation" :rows="3" placeholder="解签" />
        </article>
      </div>
    </section>

    <section class="panel grid two">
      <label><span>今日宜（每行一个）</span><AInput.TextArea v-model:value="goodText" :rows="6" /></label>
      <label><span>今日忌（每行一个）</span><AInput.TextArea v-model:value="badText" :rows="6" /></label>
    </section>

    <section class="panel">
      <div class="section-heading">
        <div><h2>特殊日期</h2><p>支持 YYYY-MM-DD 和 MM-DD</p></div>
        <AButton @click="addSpecialDate">新增日期</AButton>
      </div>
      <div class="special-list">
        <div v-for="(item, index) in draft.special_dates" :key="index" class="special-row">
          <AInput v-model:value="item.date" placeholder="08-02" />
          <ASelect v-model:value="item.fortune_name">
            <ASelectOption v-for="fortune in draft.fortunes" :key="fortune.name" :value="fortune.name">{{ fortune.name }}</ASelectOption>
          </ASelect>
          <AButton danger type="text" @click="draft.special_dates.splice(index, 1)">删除</AButton>
        </div>
      </div>
    </section>

    <footer class="action-bar">
      <AButton @click="reload">重新读取</AButton>
      <AButton type="primary" :loading="saving" :disabled="errors.length > 0" @click="save">保存配置</AButton>
    </footer>
  </main>
</template>
