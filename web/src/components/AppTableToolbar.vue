<script setup lang="ts">
defineProps<{
  title?: string
}>()
</script>

<template>
  <div class="app-table-toolbar">
    <div class="app-table-toolbar__left">
      <span v-if="title" class="app-table-toolbar__title">{{ title }}</span>
      <slot name="left" />
    </div>
    <div class="app-table-toolbar__right">
      <slot name="right" />
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border);
}

.app-table-toolbar__left,
.app-table-toolbar__right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.app-table-toolbar__title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text);
}
</style>
