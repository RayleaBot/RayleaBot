<script setup lang="ts">
import { SearchIcon } from '@lucide/vue'
import AppButton from './AppButton.vue'
import AppInput from './AppInput.vue'
defineProps<{ placeholder?: string; loading?: boolean; label?: string }>()
const model = defineModel<string>({ default: '' })
defineEmits<{ search: [value: string] }>()
</script>
<template>
  <form class="app-search-input" role="search" @submit.prevent="$emit('search', model)">
    <AppInput v-model="model" :placeholder="placeholder" :aria-label="label || placeholder || '搜索'" allow-clear />
    <AppButton type="submit" :aria-label="label || '搜索'" :loading="loading" size="icon"><SearchIcon /></AppButton>
  </form>
</template>
<style scoped>
.app-search-input { display: flex; gap: 8px; align-items: center; min-width: 0; }
</style>
