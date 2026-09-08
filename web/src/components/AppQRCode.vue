<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheckIcon, RotateCwIcon } from '@lucide/vue'
import { encodeQRCode } from '@/lib/qrcode'
import AppButton from './AppButton.vue'
import AppSpinner from './AppSpinner.vue'
const props = withDefaults(defineProps<{
  value: string; status?: 'active' | 'expired' | 'scanned' | 'loading'; size?: number
}>(), { status: 'active', size: 168 })
defineEmits<{ refresh: [] }>()
const source = computed(() => {
  try { return props.value ? encodeQRCode(props.value) : '' } catch { return '' }
})
</script>
<template>
  <div class="app-qrcode" :style="{ width: size + 'px', height: size + 'px' }">
    <img v-if="source" :src="source" :width="size" :height="size" alt="登录二维码" />
    <div v-if="status !== 'active' || !source" class="app-qrcode__status" role="status">
      <AppSpinner v-if="status === 'loading'" />
      <template v-else-if="status === 'scanned'"><CircleCheckIcon :size="28" /><span>已扫码</span></template>
      <template v-else><span>{{ status === 'expired' ? '二维码已过期' : '二维码不可用' }}</span><AppButton size="sm" @click="$emit('refresh')"><RotateCwIcon :size="14" />刷新</AppButton></template>
    </div>
  </div>
</template>
<style scoped>
.app-qrcode { position: relative; flex: none; overflow: hidden; border: 1px solid var(--border); border-radius: 10px; }
.app-qrcode img { display: block; width: 100%; height: 100%; image-rendering: pixelated; }
.app-qrcode__status { position: absolute; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 10px; padding: 12px; background: color-mix(in srgb, var(--surface-strong) 95%, transparent); color: var(--text); font-size: 13px; text-align: center; }
</style>
