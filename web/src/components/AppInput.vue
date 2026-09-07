<script setup lang="ts">
import { ref, useAttrs } from 'vue'
import { EyeIcon, EyeOffIcon } from '@lucide/vue'
import { Input } from '@/components/ui/input'
import { useFieldContext } from './form-context'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ type?: string }>(), { type: 'text' })
const model = defineModel<string>({ default: '' })
const attrs = useAttrs()
const field = useFieldContext()
const revealed = ref(false)
</script>

<template>
  <div class="app-input-wrap">
    <Input
      :id="field?.id" :aria-invalid="Boolean(field?.error) || undefined"
      :aria-describedby="field?.error ? field.descriptionId : undefined"
      :aria-required="field?.required || undefined"
      v-bind="attrs"
      :model-value="model"
      :type="props.type === 'password' && revealed ? 'text' : props.type"
      class="app-input" :class="{ 'pr-11': props.type === 'password' }"
      @update:model-value="model = String($event)"
    />
    <button v-if="type === 'password'" type="button" class="app-input-reveal" :aria-label="revealed ? '隐藏密码' : '显示密码'" :aria-pressed="revealed" :disabled="Boolean(attrs.disabled)" @click="revealed = !revealed">
      <EyeOffIcon v-if="revealed" :size="17" /><EyeIcon v-else :size="17" />
    </button>
  </div>
</template>

<style scoped>
.app-input-wrap { position: relative; min-width: 0; width: 100%; }
.app-input { height: 40px; background: var(--surface-strong); color: var(--text); }
.app-input-reveal { position: absolute; right: 0; top: 0; display: grid; place-items: center; width: 40px; height: 40px; color: var(--muted); border-radius: 8px; }
.app-input-reveal:hover { color: var(--text); }
@media (pointer: coarse) { .app-input, .app-input-reveal { min-height: 44px; } }
</style>
