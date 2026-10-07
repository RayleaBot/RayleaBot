<script setup lang="ts">
import { GlobeIcon, RotateCwIcon } from "@lucide/vue";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { computed } from "vue";

import { busyActionLabels, sectionContent, type SectionId } from "./AppShell.shared";
import LauncherButton from "./LauncherButton.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  renderedSection: SectionId;
  busyAction: string | null;
  controlsDisabled: boolean;
  editingSettings: boolean;
}>();
const emit = defineEmits<{
  refresh: [];
  openWeb: [];
}>();

const sectionMeta = computed(() => sectionContent[props.renderedSection]);
const hasRecentStderr = computed(() => props.snapshot.launcher.recentStderr.length > 0);
const canPrepareRuntime = computed(() =>
  deriveLauncherPresentation(props.snapshot).preparableRuntimeResources.length > 0 && !props.controlsDisabled);
</script>

<template>
  <header class="section-header">
    <div class="section-header__copy">
      <div class="section-header__title-row">
        <h1 class="section-header__title">{{ sectionMeta.title }}</h1>
        <div class="section-header__badges">
          <!-- The service pane names the state; the header only reports an operation in progress. -->
          <template v-if="renderedSection === 'status'">
            <span v-if="busyAction" class="status-chip status-chip--muted">{{ busyActionLabels[busyAction] ?? "正在执行操作" }}</span>
          </template>
          <template v-else-if="renderedSection === 'diagnostics'">
            <span v-if="hasRecentStderr" class="status-chip" data-tone="danger">发现异常输出</span>
          </template>
          <template v-else-if="renderedSection === 'settings'">
            <span v-if="editingSettings" class="status-chip" data-tone="attention">草稿编辑中</span>
          </template>
        </div>
      </div>
    </div>
    <div class="section-header__actions">
      <LauncherButton v-if="renderedSection === 'status'" :icon="RotateCwIcon" :disabled="controlsDisabled" @click="emit('refresh')">
        刷新状态
      </LauncherButton>
      <template v-else-if="renderedSection === 'environment'">
        <LauncherButton :icon="RotateCwIcon" :disabled="controlsDisabled" @click="emit('refresh')">重新检查</LauncherButton>
        <LauncherButton v-if="canPrepareRuntime" :icon="GlobeIcon" emphasis="prominent" @click="emit('openWeb')">
          在管理界面准备
        </LauncherButton>
      </template>
    </div>
  </header>
</template>
