<script setup lang="ts">
import { ChevronRightIcon } from "@lucide/vue";
import { motion } from "motion-v";
import { computed, ref } from "vue";

import { launcherMotion, overlayExit, useLauncherReducedMotion } from "./launcherMotion";

defineProps<{
  title: string;
  meta: string;
}>();

const reducedMotion = useLauncherReducedMotion();
const open = ref(false);
const expanded = ref(false);
const transition = computed(() => reducedMotion.value
  ? { duration: 0 }
  : expanded.value
    ? { duration: launcherMotion.overlay / 1000, ease: launcherMotion.ease }
    : overlayExit);

function toggle() {
  if (expanded.value) {
    expanded.value = false;
    if (reducedMotion.value) open.value = false;
    return;
  }
  open.value = true;
  expanded.value = true;
}

function finishAnimation() {
  if (!expanded.value) open.value = false;
}
</script>

<!--
  A native details element whose content expands and collapses with Motion. The element stays open until the
  collapse finishes, and its content stays in the document while collapsed, as with a plain details element.
-->
<template>
  <details :open="open" :data-expanded="expanded ? 'true' : 'false'">
    <summary @click.prevent="toggle">
      <ChevronRightIcon class="disclosure__chevron" aria-hidden="true" />
      <span>{{ title }}</span>
      <span>{{ meta }}</span>
    </summary>
    <motion.div
      class="disclosure__content"
      :initial="false"
      :animate="expanded ? { height: 'auto', opacity: 1 } : { height: 0, opacity: 0 }"
      :transition="transition"
      @animation-complete="finishAnimation"
    >
      <slot />
    </motion.div>
  </details>
</template>
