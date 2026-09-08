<script setup lang="ts">
import { ref } from 'vue'
import AppButton from '@/components/AppButton.vue'
import AppInput from '@/components/AppInput.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppField from '@/components/AppField.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppCheckbox from '@/components/AppCheckbox.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppAlert from '@/components/AppAlert.vue'
import AppBadge from '@/components/AppBadge.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import { Skeleton } from '@/components/ui/skeleton'

const value = ref('')
const enabled = ref(true)
const count = ref(10)
const selected = ref('onebot11')
const options = [{ value: 'onebot11', label: 'OneBot11' }, { value: 'qqofficial', label: 'QQ 官方机器人' }]
const open = ref(false)
const confirmation = ref(false)
const typedValue = ref<string | number | boolean>(false)
const typedOptions: Array<{ value: string | number | boolean; label: string }> = [
  { value: false, label: '关闭（布尔）' }, { value: true, label: '开启（布尔）' },
  { value: 'true', label: '文本 true' }, { value: 1, label: '数字 1' }, { value: '1', label: '文本 1' },
]
const tags = ref(['help'])
const optionalNumber = ref<number | null>(7)
</script>
<template>
  <main class="component-showcase">
    <h1>组件与交互状态</h1>
    <p>开发预览：在主题、窄屏、键盘和减少动态效果环境中检查控件。</p>
    <section>
      <h2>操作与反馈</h2>
      <div class="flex flex-wrap gap-3">
        <AppButton variant="default" @click="open = true">打开弹窗</AppButton>
        <AppButton>次要操作</AppButton><AppButton variant="destructive">删除</AppButton>
        <AppButton disabled>不可用</AppButton><AppButton loading>正在保存</AppButton>
      </div>
      <div class="my-6 flex flex-wrap gap-3"><AppBadge>未连接</AppBadge><AppBadge tone="success">已连接</AppBadge><AppBadge tone="warning">重连中</AppBadge><AppBadge tone="danger">鉴权失败</AppBadge></div>
      <AppAlert title="更改已保存" description="连接将在服务重启后加载。" />
    </section>
    <section class="showcase-fields">
      <div>
        <h2>表单</h2>
        <AppField label="连接名称" required><AppInput v-model="value" placeholder="填写连接名称" /></AppField>
        <AppField label="访问令牌"><AppInput v-model="value" type="password" /></AppField>
        <AppField label="接入协议"><AppSelect v-model="selected" :options="options" /></AppField>
        <AppField label="连接超时（秒）"><AppNumberInput v-model="count" :min="1" /></AppField>
        <AppField label="备注"><AppTextarea v-model="value" /></AppField>
      </div>
      <div>
        <h2>边界状态</h2>
        <AppField label="无效地址" error="请输入 ws:// 或 wss:// 开头的地址。"><AppInput v-model="value" /></AppField>
        <AppField label="只读连接标识"><AppInput v-model="value" disabled placeholder="连接标识不可修改" /></AppField>
        <AppCheckbox v-model="enabled">接收群聊与单聊消息</AppCheckbox>
        <div class="my-4 flex items-center justify-between"><label for="showcase-enabled">启用连接</label><AppSwitch id="showcase-enabled" v-model="enabled" /></div>
        <Skeleton class="h-20 w-full" />
      </div>
    </section>
    <section class="showcase-fields">
      <div>
        <h2>选项值与集合</h2>
        <AppField label="保留原始类型"><AppSelect v-model="typedValue" :options="typedOptions" /></AppField>
        <output data-testid="showcase-typed-value">{{ JSON.stringify(typedValue) }}</output>
        <AppField label="可留空数值"><AppNumberInput v-model="optionalNumber" nullable /></AppField>
        <output data-testid="showcase-optional-number">{{ JSON.stringify(optionalNumber) }}</output>
      </div>
      <div>
        <AppField label="自由输入列表" hint="按 Enter 添加；逗号可作为单独的值。"><AppTagsInput v-model="tags" /></AppField>
        <output data-testid="showcase-tags-value">{{ JSON.stringify(tags) }}</output>
      </div>
    </section>
    <AppDialog :open="open" title="配置连接" description="更改保存在当前草稿中。" @close="confirmation = true">
      <AppField label="弹窗内的协议"><AppSelect v-model="selected" :options="options" /></AppField>
      <AppField label="弹窗内的名称"><AppInput v-model="value" /></AppField>
      <template #footer><div class="flex justify-end gap-3"><AppButton @click="confirmation = true">取消</AppButton><AppButton variant="default" @click="open = false">完成</AppButton></div></template>
    </AppDialog>
    <AppConfirmDialog :open="confirmation" title="放弃修改？" description="当前草稿尚未保存。" confirm-text="放弃修改" cancel-text="继续编辑" danger @cancel="confirmation = false" @confirm="confirmation = false; open = false" />
  </main>
</template>
<style scoped>
.component-showcase { max-width: 1040px; margin: 0 auto; padding: 40px 24px; color: var(--text); }
.component-showcase > h1 { font-size: 28px; font-weight: 600; }
.component-showcase > p { margin: 12px 0 40px; color: var(--muted); }
.component-showcase section { margin-top: 32px; }
.component-showcase h2 { margin: 0 0 20px; font-size: 18px; font-weight: 600; }
.showcase-fields { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; }
@media (max-width: 639px) { .showcase-fields { grid-template-columns: 1fr; gap: 24px; } }
</style>
