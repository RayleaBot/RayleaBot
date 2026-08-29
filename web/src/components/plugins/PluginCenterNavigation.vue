<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { pluginCenterPages } from '@/access/plugin-center'
import { resolveMenuIcon } from '@/access/icons'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { t } from '@/i18n'

const route = useRoute()
const navigation = ref<HTMLElement | null>(null)

function revealCurrentPage() {
  const container = navigation.value
  const current = container?.querySelector<HTMLElement>('[aria-current="page"]')
  if (!container || !current) return
  const bounds = container.getBoundingClientRect()
  const item = current.getBoundingClientRect()
  if (item.left < bounds.left) container.scrollLeft += item.left - bounds.left - 4
  else if (item.right > bounds.right) container.scrollLeft += item.right - bounds.right + 4
}

watch(() => route.name, () => { void nextTick(revealCurrentPage) })
onMounted(() => {
  revealCurrentPage()
  window.addEventListener('resize', revealCurrentPage)
})
onBeforeUnmount(() => window.removeEventListener('resize', revealCurrentPage))
</script>

<template>
  <nav ref="navigation" class="plugin-center-navigation" :aria-label="t('routes.pluginCenter')" data-testid="plugin-center-navigation">
    <MotionRouterLink
      v-for="page in pluginCenterPages"
      :key="page.name"
      :to="route.name === page.name ? route.fullPath : page.path"
      class="plugin-center-navigation__item"
      :aria-current="route.name === page.name ? 'page' : undefined"
    >
      <component :is="resolveMenuIcon(page.icon)" aria-hidden="true" />
      <span>{{ t(page.titleKey) }}</span>
    </MotionRouterLink>
  </nav>
</template>

<style scoped>
.plugin-center-navigation {
  display: flex;
  flex: none;
  gap: 4px;
  min-width: 0;
  max-width: 100%;
  padding: 4px;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  scrollbar-width: thin;
}
.plugin-center-navigation__item {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 36px;
  padding: 6px 12px;
  border-radius: var(--radius-md);
  color: var(--muted);
  font-size: 14px;
  line-height: 20px;
  white-space: nowrap;
  text-decoration: none;
  transition: color 140ms ease, background-color 140ms ease;
}
.plugin-center-navigation__item:hover { color: var(--text); background: var(--surface-accent); }
.plugin-center-navigation__item[aria-current="page"] { color: var(--brand-foreground); background: var(--surface-accent); }
.plugin-center-navigation__item:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
@media (max-width: 639px), (pointer: coarse) { .plugin-center-navigation__item { min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .plugin-center-navigation__item { transition: none; } }
@media (forced-colors: active) { .plugin-center-navigation__item[aria-current="page"] { outline: 1px solid Highlight; outline-offset: -2px; color: Highlight; } }
</style>
