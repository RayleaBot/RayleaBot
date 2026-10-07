<script setup lang="ts">
import { MotionConfig } from "motion-v";
import { onErrorCaptured, ref } from "vue";

import App from "./App.vue";
import { useLauncherReducedMotion } from "./launcherMotion";
import { provideTheme } from "./useTheme";

provideTheme();
// Motion follows the launcher's reduced-motion preference, which also covers forced colors.
const reducedMotion = useLauncherReducedMotion();

const failed = ref(false);
onErrorCaptured(() => {
  failed.value = true;
  return false;
});
</script>

<template>
  <MotionConfig :reduced-motion="reducedMotion ? 'always' : 'never'">
    <div class="launcher-theme">
      <div v-if="failed" class="launcher-loading-shell launcher-loading-shell--error">
        <h1 class="launcher-loading-shell__title">启动器界面暂时不可用</h1>
        <p class="launcher-loading-shell__detail">请关闭后重新打开 RayleaBot 启动器。</p>
      </div>
      <App v-else />
    </div>
  </MotionConfig>
</template>
