<script setup lang="ts">
import { useId } from 'vue'
import AppButton from './AppButton.vue'
import AppDialog from './AppDialog.vue'
withDefaults(defineProps<{
  open: boolean; title: string; description: string; confirmText?: string; cancelText?: string; busy?: boolean; danger?: boolean
}>(), { confirmText: '确认', cancelText: '取消' })
defineEmits<{ confirm: []; cancel: []; afterClose: [] }>()
const cancelId = `confirm-cancel-${useId()}`
</script>
<template>
  <AppDialog :open="open" :title="title" :description="description" :width="440" :busy="busy" role="alertdialog" :initial-focus="`[id='${cancelId}']`" @close="$emit('cancel')" @after-close="$emit('afterClose')">
    <slot />
    <template #footer>
      <div class="flex justify-end gap-3">
        <AppButton :id="cancelId" :disabled="busy" @click="$emit('cancel')">{{ cancelText }}</AppButton>
        <AppButton :variant="danger ? 'destructive' : 'default'" :loading="busy" @click="$emit('confirm')">{{ confirmText }}</AppButton>
      </div>
    </template>
  </AppDialog>
</template>
