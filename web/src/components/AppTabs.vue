<script setup lang="ts" generic="T extends string">
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'reka-ui'
defineProps<{ items: { value: T; label: string; disabled?: boolean }[]; label?: string; keepAlive?: boolean }>()
const model = defineModel<T>({ required: true })
</script>
<template>
  <TabsRoot v-model="model" class="app-tabs" activation-mode="automatic">
    <div class="app-tabs__header">
      <TabsList class="app-tabs__list" :aria-label="label"><TabsTrigger v-for="item in items" :key="item.value" :value="item.value" :disabled="item.disabled" class="app-tabs__trigger"><slot name="tab" :item="item">{{ item.label }}</slot></TabsTrigger></TabsList>
      <div v-if="$slots.extra" class="app-tabs__extra"><slot name="extra" /></div>
    </div>
    <TabsContent v-for="item in items" :key="item.value" v-show="item.value === model" :value="item.value" :force-mount="keepAlive" class="app-tabs__content"><slot :name="item.value" /></TabsContent>
  </TabsRoot>
</template>
<style>
.app-tabs { min-width: 0; }
.app-tabs__header { display: flex; align-items: center; justify-content: space-between; gap: 16px; border-bottom: 1px solid var(--border); margin-bottom: 24px; }
.app-tabs__list { display: flex; gap: 20px; overflow-x: auto; min-width: 0; }
.app-tabs__extra { flex-shrink: 0; }
.app-tabs__trigger { flex-shrink: 0; min-height: 44px; padding: 8px 2px; border-bottom: 2px solid transparent; color: var(--muted); font-size: 14px; cursor: pointer; }
.app-tabs__trigger[data-state=active] { border-bottom-color: var(--brand-foreground); color: var(--brand-foreground); font-weight: 600; }
.app-tabs__trigger:focus-visible, .app-tabs__content:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; border-radius: 4px; }
.app-tabs__trigger:disabled { opacity: .5; cursor: not-allowed; }
.app-tabs__content { min-width: 0; }
</style>
