<script setup lang="ts">
defineOptions({
  name: 'RouteView',
})
</script>

<template>
  <div class="route-view-shell">
    <RouterView v-slot="{ Component }">
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
