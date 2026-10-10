<script setup lang="ts">
import { rayleaMark } from "@shared/launcher-theme-tokens.generated";

withDefaults(defineProps<{
  tone?: string;
  variant?: "chrome" | "monochrome" | "neutral";
}>(), { variant: "monochrome" });

</script>

<template>
  <svg
    aria-hidden="true"
    :class="`raylea-mark raylea-mark--${variant}`"
    :data-tone="tone"
    focusable="false"
    :viewBox="rayleaMark.viewBox"
  >
    <g v-for="(theme, mode) in rayleaMark.themes" :key="mode" :class="`raylea-mark__theme--${mode}`">
      <path :d="theme.paths[theme.outline.pathIndex].d" fill="none" :stroke="theme.outline.color" :stroke-width="theme.outline.width" stroke-linejoin="round" />
      <path v-for="(part, index) in theme.paths" :key="index" :d="part.d" :fill="part.fill" :fill-rule="part.fillRule" :opacity="part.opacity" />
    </g>
  </svg>
</template>
