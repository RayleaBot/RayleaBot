<script setup lang="ts">
import { rayleaMark } from "@shared/launcher-theme-tokens.generated";

withDefaults(defineProps<{
  tone?: string;
  variant?: "chrome" | "monochrome" | "neutral";
}>(), { variant: "monochrome" });

const outlinePath = rayleaMark.paths[rayleaMark.outline.pathIndex].d;
</script>

<template>
  <svg
    aria-hidden="true"
    :class="`raylea-mark raylea-mark--${variant}`"
    :data-tone="tone"
    focusable="false"
    :viewBox="rayleaMark.viewBox"
  >
    <path :d="outlinePath" fill="none" :stroke="rayleaMark.outline.color" :stroke-width="rayleaMark.outline.width" stroke-linejoin="round" />
    <path v-for="part in rayleaMark.paths" :key="part.fill" :d="part.d" :fill="part.fill" :fill-rule="part.fillRule" :opacity="part.opacity" />
  </svg>
</template>
