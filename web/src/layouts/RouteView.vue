<script setup lang="ts">
import type { RouteLocationNormalizedLoaded } from 'vue-router'

defineOptions({
  name: 'RouteView',
})

defineProps<{
  displayRoute?: RouteLocationNormalizedLoaded
}>()
</script>

<template>
  <div class="route-view-shell">
    <RouterView :route="displayRoute" v-slot="{ Component }">
      <component :is="Component" v-if="Component" />
    </RouterView>
  </div>
</template>

<style scoped lang="scss">
.route-view-shell {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}
</style>
