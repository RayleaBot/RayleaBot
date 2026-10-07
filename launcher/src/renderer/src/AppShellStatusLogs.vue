<script setup lang="ts">
import { CircleCheckIcon, FolderOpenIcon, InfoIcon } from "@lucide/vue";

import { uncapturedOutputText } from "./AppShell.copy";
import LauncherButton from "./LauncherButton.vue";

defineProps<{
  externalService: boolean;
  logs: string[];
}>();
const emit = defineEmits<{ openLogs: [] }>();
</script>

<template>
  <section
    v-if="logs.length === 0"
    class="status-log-row content-group"
    :data-alert="externalService ? 'unknown' : 'none'"
    aria-labelledby="status-log-title"
  >
    <div class="status-log-row__status" role="status">
      <span class="status-log-row__icon" aria-hidden="true">
        <InfoIcon v-if="externalService" />
        <CircleCheckIcon v-else />
      </span>
      <div>
        <h3 id="status-log-title">异常输出</h3>
        <span>{{ externalService ? uncapturedOutputText : "当前没有新的异常输出。" }}</span>
      </div>
    </div>
    <LauncherButton :icon="FolderOpenIcon" @click="emit('openLogs')">打开日志目录</LauncherButton>
  </section>

  <section v-else class="status-log-panel content-group" data-alert="error" aria-labelledby="status-log-title">
    <div class="status-log-panel__heading">
      <h3 id="status-log-title">异常输出</h3>
      <span class="status-label" data-state="danger">已检测到异常输出</span>
    </div>
    <pre class="log-surface status-log-panel__surface">{{ logs.join("\n") }}</pre>
    <div class="status-log-panel__footer">
      <LauncherButton :icon="FolderOpenIcon" @click="emit('openLogs')">打开日志目录</LauncherButton>
    </div>
  </section>
</template>
