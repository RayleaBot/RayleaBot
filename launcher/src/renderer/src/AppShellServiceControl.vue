<script setup lang="ts">
import { CircleStopIcon, GlobeIcon, PlayIcon, RefreshCwIcon } from "@lucide/vue";
import type { LauncherPresentationState } from "@shared/launcher-presentation";
import { AnimatePresence, motion } from "motion-v";
import { computed, useId } from "vue";

import { serviceStateConfig, serviceStateGlyphs } from "./AppShell.shared";
import LauncherButton from "./LauncherButton.vue";
import { launcherMotion, useLauncherReducedMotion } from "./launcherMotion";
import StatusLens from "./StatusLens.vue";

const props = defineProps<{
  attention: {
    label: string;
    text: string;
    tone: "attention" | "warning" | "danger";
  } | null;
  busyLabel: string;
  canOpenWebUi: boolean;
  controlsDisabled: boolean;
  externalService: boolean;
  serviceDetail: string;
  serviceState: LauncherPresentationState;
  showRunningActions: boolean;
  startDisabled: boolean;
  stopDisabled: boolean;
  stopOpensWeb: boolean;
}>();
const emit = defineEmits<{
  openWeb: [];
  start: [];
  stop: [];
}>();

const noteId = useId();
const reducedMotion = useLauncherReducedMotion();
const stateConfig = computed(() => serviceStateConfig[props.serviceState]);
const tone = computed(() => stateConfig.value.tone);
const stateLabel = computed(() => stateConfig.value.label);
const primaryDisabled = computed(() => props.showRunningActions ? !props.canOpenWebUi || props.controlsDisabled : props.startDisabled);
const note = computed(() => props.busyLabel
  || (!props.showRunningActions
    ? "服务启动后可进入管理界面"
    : props.externalService
      ? "服务由其他进程启动，无法在启动器中重启"
      : ""));

// State text fades in as it replaces the previous copy. Assistive technology reads the plain copy beside it.
const copyMotion = computed(() => ({
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -4, transition: { duration: reducedMotion.value ? 0 : 0.12, ease: launcherMotion.ease } },
  transition: { duration: reducedMotion.value ? 0 : launcherMotion.content / 1000, ease: launcherMotion.ease },
}));
</script>

<template>
  <section class="service-control" :data-tone="tone" aria-labelledby="service-control-title">
    <div class="service-control__summary">
      <div class="service-control__state" aria-live="polite">
        <StatusLens
          :tone="tone"
          :icon="serviceStateGlyphs[serviceState]"
          :icon-key="serviceState"
          :spinning="serviceState === 'starting' || serviceState === 'stopping'"
        />
        <div class="service-control__state-copy">
          <h2 id="service-control-title" class="service-control__state-value">
            <span class="visually-hidden">服务控制：{{ stateLabel }}</span>
            <span class="presence-stack" aria-hidden="true">
              <AnimatePresence :initial="false">
                <motion.span :key="stateLabel" v-bind="copyMotion">{{ stateLabel }}</motion.span>
              </AnimatePresence>
            </span>
          </h2>
          <p class="visually-hidden">{{ serviceDetail }}</p>
          <div class="presence-stack" aria-hidden="true">
            <AnimatePresence :initial="false">
              <motion.p :key="serviceDetail" class="service-control__detail" v-bind="copyMotion">{{ serviceDetail }}</motion.p>
            </AnimatePresence>
          </div>
        </div>
      </div>
      <div v-if="attention" class="attention-note" :data-severity="attention.tone">
        <span class="attention-note__label">{{ attention.label }}</span>
        <span>{{ attention.text }}</span>
      </div>
    </div>

    <div class="service-control__actions">
      <!-- A service started elsewhere can only be stopped from its own management console. -->
      <LauncherButton v-if="stopOpensWeb" :icon="GlobeIcon" :disabled="controlsDisabled || !canOpenWebUi" @click="emit('openWeb')">
        在管理界面停止
      </LauncherButton>
      <LauncherButton v-else class="launcher-button--danger" :icon="CircleStopIcon" :disabled="stopDisabled" @click="emit('stop')">
        停止服务
      </LauncherButton>
      <LauncherButton
        v-if="showRunningActions"
        :icon="RefreshCwIcon"
        :disabled="startDisabled"
        :aria-describedby="externalService ? noteId : undefined"
        @click="emit('start')"
      >
        重启服务
      </LauncherButton>
      <LauncherButton v-else :icon="GlobeIcon" disabled>管理界面</LauncherButton>
      <LauncherButton
        class="service-control__primary"
        :icon="showRunningActions ? GlobeIcon : PlayIcon"
        :emphasis="primaryDisabled ? 'regular' : 'prominent'"
        :disabled="primaryDisabled"
        @click="showRunningActions ? emit('openWeb') : emit('start')"
      >
        {{ showRunningActions ? "管理界面" : "启动服务" }}
      </LauncherButton>
      <p v-if="note" :id="noteId" class="operation-status" aria-live="polite">{{ note }}</p>
    </div>
  </section>
</template>
