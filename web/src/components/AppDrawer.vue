<script setup lang="ts">
import AppDialog from './AppDialog.vue'
withDefaults(defineProps<{ open: boolean; title: string; width?: number; placement?: 'left' | 'right'; busy?: boolean }>(), { width: 380, placement: 'right' })
defineEmits<{ close: []; afterClose: [] }>()
</script>
<template><AppDialog v-bind="$attrs" :open="open" :title="title" :width="width" :placement="placement" :busy="busy" @close="$emit('close')" @after-close="$emit('afterClose')"><slot /><template v-if="$slots.footer" #footer><slot name="footer" /></template></AppDialog></template>
