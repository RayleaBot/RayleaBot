<script setup lang="ts">
import { useId } from 'vue'
import { ChevronRightIcon } from '@lucide/vue'
import type { RouteLocationRaw } from 'vue-router'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'

defineProps<{ title: string; meta?: string; to?: RouteLocationRaw; linkLabel?: string }>()
const headingId = useId()
</script>

<template>
  <section class="hub-section" :aria-labelledby="headingId">
    <header class="hub-section__header">
      <h2 :id="headingId">{{ title }}</h2>
      <span v-if="meta" class="hub-section__meta">{{ meta }}</span>
      <MotionRouterLink v-if="to && linkLabel" :to="to" class="hub-section__link">
        {{ linkLabel }}<ChevronRightIcon aria-hidden="true" />
      </MotionRouterLink>
    </header>
    <div class="hub-section__grid">
      <slot />
    </div>
  </section>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.hub-section {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.hub-section__header {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.hub-section__header h2 {
  margin: 0;
  color: var(--text);
  font-size: 18px;
  font-weight: 700;
  line-height: 1.4;
}

.hub-section__meta {
  color: var(--muted);
  font-size: 13.5px;
}

.hub-section__link {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-inline-start: auto;
  padding: 2px 4px 2px 10px;
  border-radius: 999px;
  color: var(--muted);
  font-size: 13.5px;
  font-weight: 600;
  text-decoration: none;
}

.hub-section__link:hover {
  color: var(--text);
  background: var(--nav-hover);
}

.hub-section__link svg {
  width: 16px;
  height: 16px;
}

// Auto-fill keeps tiles near 170px wide, so a 2K screen shows 10 to 15 objects per section.
.hub-section__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 12px;
}

@media (max-width: #{bp.$phone - 1px}) {
  .hub-section__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
}
</style>
