<script setup lang="ts">
import { computed } from 'vue'

import { t } from '@/i18n'
import type { ConfigApplyEffects } from '@/types/api'

const props = defineProps<{
  effects?: ConfigApplyEffects | null
}>()

const sections = computed(() => {
  if (!props.effects) {
    return []
  }

  return [
    {
      key: 'applied',
      tone: 'success',
      title: t('config.applyEffects.appliedNow'),
      items: props.effects.applied_now ?? [],
    },
    {
      key: 'reloaded',
      tone: 'processing',
      title: t('config.applyEffects.reloadedNow'),
      items: props.effects.reloaded_now ?? [],
    },
    {
      key: 'restart',
      tone: 'warning',
      title: t('config.applyEffects.restartRequiredFields'),
      items: props.effects.restart_required_fields ?? [],
    },
  ].filter((section) => section.items.length > 0)
})
</script>

<template>
  <div class="config-apply-effects">
    <template v-if="sections.length > 0">
      <section v-for="section in sections" :key="section.key" class="config-apply-effects__section">
        <div class="config-apply-effects__heading">
          <span>{{ section.title }}</span>
          <a-tag :color="section.tone">{{ section.items.length }}</a-tag>
        </div>
        <div class="config-apply-effects__items">
          <code v-for="item in section.items" :key="item" class="config-apply-effects__item">{{ item }}</code>
        </div>
      </section>
    </template>
    <p v-else class="config-apply-effects__empty">{{ t('config.applyEffects.empty') }}</p>
  </div>
</template>

<style scoped lang="scss">
.config-apply-effects {
  display: grid;
  gap: 12px;
}

.config-apply-effects__section {
  display: grid;
  gap: 8px;
}

.config-apply-effects__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-weight: 600;
  color: var(--text);
}

.config-apply-effects__items {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.config-apply-effects__item {
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--border) 88%, white);
  background: color-mix(in srgb, var(--surface-soft) 92%, white);
  color: var(--text);
  padding: 6px 10px;
  font-size: 12px;
  line-height: 1.4;
  white-space: nowrap;
}

.config-apply-effects__empty {
  margin: 0;
  color: var(--text-secondary);
}
</style>
