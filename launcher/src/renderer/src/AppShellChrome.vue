<script setup lang="ts">
import { CopyIcon, MinusIcon, SquareIcon, XIcon } from "@lucide/vue";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { motion } from "motion-v";
import { computed, watch } from "vue";

import { sections, serviceStateConfig, serviceStateGlyphs, type SectionId } from "./AppShell.shared";
import { useLauncherReducedMotion } from "./launcherMotion";
import RayleaMark from "./RayleaMark.vue";
import StatusLens from "./StatusLens.vue";
import ThemeModeMenu from "./ThemeModeMenu.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  isMaximized: boolean;
}>();
const emit = defineEmits<{ navigate: [section: SectionId] }>();

const selectionTransition = { type: "spring", visualDuration: 0.32, bounce: 0.18 } as const;

const presentation = computed(() => deriveLauncherPresentation(props.snapshot));
const stateConfig = computed(() => serviceStateConfig[presentation.value.state]);
const reducedMotion = useLauncherReducedMotion();

// Where the new selection starts, relative to the clicked item. It is read at click time, while the layout is
// still clean, instead of measuring the page on every sidebar render.
let selectionStart: { section: SectionId; offset: number } | null = null;
watch(() => props.activeSection, () => {
  selectionStart = null;
}, { flush: "post" });

function selectionInitial(section: SectionId) {
  return !reducedMotion.value && selectionStart?.section === section ? { y: selectionStart.offset } : false;
}

const minimize = () => window.rayleaLauncher.minimize();
const maximize = () => window.rayleaLauncher.maximize();
const close = () => window.rayleaLauncher.close();

function navigate(event: MouseEvent, section: SectionId) {
  const item = event.currentTarget as HTMLElement;
  const previous = item.parentElement?.querySelector(".nav-item__selection")?.getBoundingClientRect();
  if (previous && section !== props.activeSection) {
    selectionStart = { section, offset: previous.top - item.getBoundingClientRect().top };
  }
  emit("navigate", section);
}
</script>

<template>
  <div class="window-drag-handle">
    <div class="window-title"><RayleaMark variant="chrome" />RayleaBot 启动器</div>
    <div class="window-controls">
      <button type="button" class="window-control-btn" title="最小化" aria-label="最小化" @click="minimize"><MinusIcon /></button>
      <button
        type="button"
        class="window-control-btn"
        :title="isMaximized ? '还原' : '最大化'"
        :aria-label="isMaximized ? '还原' : '最大化'"
        @click="maximize"
      >
        <CopyIcon v-if="isMaximized" />
        <SquareIcon v-else />
      </button>
      <button type="button" class="window-control-btn danger" title="关闭" aria-label="关闭" @click="close"><XIcon /></button>
    </div>
  </div>

  <aside class="shell-sidebar">
    <nav class="section-nav">
      <button
        v-for="section in sections"
        :key="section.id"
        type="button"
        :class="['nav-item', { active: activeSection === section.id }]"
        :aria-current="activeSection === section.id ? 'page' : undefined"
        :title="section.title"
        @click="navigate($event, section.id)"
      >
        <motion.span
          v-if="activeSection === section.id"
          class="nav-item__selection"
          :initial="selectionInitial(section.id)"
          :animate="{ y: 0 }"
          :transition="selectionTransition"
        />
        <span class="nav-item__icon"><component :is="section.icon" /></span>
        <span class="nav-item__label">{{ section.title }}</span>
      </button>
    </nav>

    <div class="sidebar-footer--compact">
      <div
        class="sidebar-footer__status"
        role="img"
        :aria-label="`运行状态：${stateConfig.label}`"
        :title="`运行状态：${stateConfig.label}`"
      >
        <StatusLens
          :tone="stateConfig.tone"
          size="compact"
          :icon="serviceStateGlyphs[presentation.state]"
          :icon-key="presentation.state"
        />
      </div>
      <ThemeModeMenu />
    </div>
  </aside>
</template>
