<script setup lang="ts">
import { AnimatePresence, motion } from "motion-v";
import { computed, type Component } from "vue";

import type { LauncherVisualTone } from "./AppShell.shared";
import { launcherMotion, useLauncherReducedMotion } from "./launcherMotion";

const props = withDefaults(defineProps<{
  tone: LauncherVisualTone;
  icon: Component;
  /** Regular and small lenses set the status dot in a raised ring; the compact lens is the dot alone. */
  size?: "regular" | "small" | "compact";
  /** Changing the key swaps the glyph with a short scale and fade. */
  iconKey?: string;
  /** Turns the glyph slowly while an operation is in progress. */
  spinning?: boolean;
}>(), { size: "regular", iconKey: "glyph", spinning: false });

const sizeClassNames = {
  regular: "status-lens",
  small: "status-lens status-lens--small",
  compact: "status-lens status-lens--compact",
} as const;

const reducedMotion = useLauncherReducedMotion();
const turning = computed(() => props.spinning && !reducedMotion.value);
const transition = computed(() => reducedMotion.value
  ? { duration: 0 }
  : {
    duration: launcherMotion.content / 1000,
    ease: launcherMotion.ease,
    rotate: turning.value ? { duration: 1.4, ease: "linear" as const, repeat: Infinity } : { duration: 0 },
  });
</script>

<!-- A solid status dot in the tone's color with the state glyph, so the state never depends on color alone. -->
<template>
  <span :class="sizeClassNames[size]" :data-tone="tone" aria-hidden="true">
    <span class="status-lens__core">
      <AnimatePresence :initial="false">
        <motion.span
          :key="iconKey"
          class="status-lens__glyph"
          :initial="{ opacity: 0, scale: 0.6 }"
          :animate="{ opacity: 1, scale: 1, rotate: turning ? 360 : 0 }"
          :exit="{ opacity: 0, scale: 0.6 }"
          :transition="transition"
        >
          <component :is="icon" />
        </motion.span>
      </AnimatePresence>
    </span>
  </span>
</template>
