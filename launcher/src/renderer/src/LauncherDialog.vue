<script setup lang="ts">
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot } from "reka-ui";
import { motion } from "motion-v";
import { computed, ref, watch } from "vue";

import { overlayEnter, overlayExit, useLauncherReducedMotion } from "./launcherMotion";

const props = defineProps<{
  open: boolean;
  tone?: "danger" | "attention";
}>();
const emit = defineEmits<{ dismiss: [] }>();

// The dialog, its content and its focus trap stay mounted until the exit animation ends, so the surface does
// not vanish mid-fade.
const present = ref(props.open);
watch(() => props.open, (open) => {
  if (open) present.value = true;
});

const reducedMotion = useLauncherReducedMotion();
const transition = computed(() => reducedMotion.value ? { duration: 0 } : props.open ? overlayEnter : overlayExit);

function dismiss() {
  if (props.open) emit("dismiss");
}

function finishExit() {
  if (!props.open) present.value = false;
}
</script>

<!-- A Reka dialog whose surface and scrim are animated by Motion. -->
<template>
  <DialogRoot :open="present" @update:open="dismiss">
    <DialogPortal force-mount>
      <DialogOverlay v-if="present" force-mount as-child>
        <motion.div
          class="launcher-dialog-scrim"
          :initial="reducedMotion ? false : { opacity: 0 }"
          :animate="{ opacity: open ? 1 : 0 }"
          :transition="transition"
        />
      </DialogOverlay>
      <DialogContent
        v-if="present"
        force-mount
        as-child
        @escape-key-down.prevent="dismiss"
        @pointer-down-outside.prevent="dismiss"
        @interact-outside.prevent
      >
        <motion.section
          class="launcher-dialog"
          :data-tone="tone"
          :data-state="open ? 'open' : 'closing'"
          :initial="reducedMotion ? false : { opacity: 0, scale: 0.96 }"
          :animate="open ? { opacity: 1, scale: 1 } : { opacity: 0, scale: 0.96 }"
          :transition="transition"
          @animation-complete="finishExit"
        >
          <slot />
        </motion.section>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
