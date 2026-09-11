<script setup lang="ts">
import { t } from '@/i18n'
import { ref, useAttrs, type HTMLAttributes } from 'vue'
import { EyeIcon, EyeOffIcon, XIcon } from '@lucide/vue'
import { Input } from '@/components/ui/input'
import { useFieldContext } from './form-context'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ type?: string; allowClear?: boolean; showSecretLabel?: string; hideSecretLabel?: string; wrapperClass?: HTMLAttributes['class'] }>(), { type: 'text' })
const model = defineModel<string>({ default: '' })
const attrs = useAttrs()
const field = useFieldContext()
const revealed = ref(false)
const wrapper = ref<HTMLElement | null>(null)
defineExpose({ focus: () => wrapper.value?.querySelector('input')?.focus() })
</script>

<template>
  <div ref="wrapper" class="app-input-wrap" :class="[wrapperClass, { 'app-input-wrap--floating': field?.floating }]">
    <Input
      :id="field?.id" :aria-invalid="Boolean(field?.error) || undefined"
      :aria-describedby="field?.error || field?.hint ? field.descriptionId : undefined"
      :aria-required="field?.required || undefined"
      v-bind="attrs"
      :placeholder="field?.floating ? (attrs.placeholder as string) || ' ' : attrs.placeholder as string | undefined"
      :model-value="model"
      :type="props.type === 'password' && revealed ? 'text' : props.type"
      class="app-input" :class="{ 'app-input--floating': field?.floating, 'pr-11': props.type === 'password' || allowClear, 'pl-10': Boolean($slots.prefix) }"
      @update:model-value="model = String($event)"
    />
    <span v-if="$slots.prefix" class="app-input-prefix" aria-hidden="true"><slot name="prefix" /></span>
    <button v-if="type === 'password'" type="button" class="app-input-reveal" :aria-label="revealed ? (hideSecretLabel ?? t('ui.hidePassword')) : (showSecretLabel ?? t('ui.showPassword'))" :aria-pressed="revealed" :disabled="Boolean(attrs.disabled)" @click="revealed = !revealed">
      <EyeOffIcon v-if="revealed" :size="17" /><EyeIcon v-else :size="17" />
    </button>
    <button v-else-if="allowClear && model && !attrs.disabled" type="button" class="app-input-reveal" :aria-label="t('ui.clearInput')" @click="model = ''; wrapper?.querySelector('input')?.focus()"><XIcon :size="16" /></button>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.app-input-wrap { position: relative; min-width: 0; width: 100%; }
.app-input { height: 40px; background: var(--surface-strong); color: var(--text); }
.app-input--floating { height: 56px; min-height: 56px; padding-top: 24px; padding-bottom: 8px; line-height: 22px; }
.app-input-wrap--floating .app-input-prefix, .app-input-wrap--floating .app-input-reveal { height: 56px; }
.app-input-prefix { position: absolute; left: 0; top: 0; display: grid; place-items: center; width: 40px; height: 40px; color: var(--muted); pointer-events: none; }
.app-input-reveal { position: absolute; right: 0; top: 0; display: grid; place-items: center; width: 40px; height: 40px; color: var(--muted); border-radius: 8px; }
.app-input-reveal:hover { color: var(--text); }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-input, .app-input-reveal { min-height: 44px; } .app-input-reveal { width: 44px; } }
</style>
