<script setup lang="ts">
import type { Component } from "vue";

withDefaults(defineProps<{
  icon?: Component;
  /** The one prominent action of a view is filled with the brand color. */
  emphasis?: "regular" | "prominent";
}>(), { emphasis: "regular" });
</script>

<template>
  <button type="button" class="launcher-button" :data-emphasis="emphasis">
    <component :is="icon" v-if="icon" class="launcher-button__icon" aria-hidden="true" />
    <slot />
  </button>
</template>
