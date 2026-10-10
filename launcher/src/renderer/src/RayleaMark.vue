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
    <path v-for="(part, index) in rayleaMark.paths" :key="index" :d="part.d" :fill="part.fill" :fill-rule="part.fillRule" :opacity="part.opacity" />
  </svg>
</template>
