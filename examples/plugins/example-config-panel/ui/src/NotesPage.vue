<script setup lang="ts">
import { computed, ref } from 'vue'
import type { usePluginHost } from '@rayleabot/plugin-ui'

import { accountSummary, normalizeNotes } from './model'

const props = defineProps<{ host: ReturnType<typeof usePluginHost> }>()
const notes = ref<string[]>([])
const draft = ref('')
const status = ref('正在读取备注…')
const busy = ref(false)
const account = computed(() => props.host.bot.value)

// Every action of this bot_account page carries the selected account, so the backend
// reads and writes only that account's notes.
async function run(action: string, payload: Record<string, unknown> = {}) {
  busy.value = true
  try {
    const result = await props.host.client.invokeAction(action, payload)
    notes.value = normalizeNotes(result.notes)
    status.value = notes.value.length ? `共 ${notes.value.length} 条备注` : '这个账号还没有备注'
  } catch (cause) {
    status.value = cause instanceof Error ? cause.message : '操作失败'
  } finally {
    busy.value = false
  }
}

void props.host.ready.then(() => run('notes.list')).catch((cause: unknown) => {
  status.value = cause instanceof Error ? cause.message : '页面初始化失败'
})

async function add() {
  await run('notes.add', { text: draft.value.trim() })
  draft.value = ''
}
</script>

<template>
  <main>
    <h1>账号备注示例</h1>
    <p class="intro">备注按机器人账号保存。宿主在页面上方切换账号，页面通过 invokeAction 自动带上所选账号。</p>
    <section>
      <p data-testid="notes-account">{{ account ? accountSummary(account) : '正在读取账号…' }}</p>
      <ul v-if="notes.length" class="notes" data-testid="notes-list">
        <li v-for="(note, index) in notes" :key="`${index}:${note}`">
          <span>{{ note }}</span>
          <button type="button" class="danger" :disabled="busy" @click="run('notes.remove', { index })">删除</button>
        </li>
      </ul>
      <label><span>新备注</span><input v-model="draft" maxlength="200" data-testid="note-input" /></label>
      <footer>
        <span role="status" data-testid="notes-status">{{ status }}</span>
        <div><button type="button" class="primary" :disabled="busy || !draft.trim()" data-testid="add-note" @click="add">添加</button></div>
      </footer>
    </section>
  </main>
</template>
