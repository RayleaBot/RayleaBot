<script setup lang="ts">
import type { LauncherDiagnosticIssue } from "@shared/launcher-models";
import { computed } from "vue";

import { formatRuntimeResource } from "./AppShell.copy";
import { severityConfig } from "./AppShell.shared";
import LauncherButton from "./LauncherButton.vue";

const props = defineProps<{
  checks: Array<{ code: string; severity: "ok" | "warning" | "error"; title: string; summary: string }>;
  runtimeResources: NonNullable<LauncherDiagnosticIssue["runtime_resources"]>;
}>();
const emit = defineEmits<{ openWeb: [] }>();

const issueTone = computed(() => props.checks.some((item) => item.severity === "error") ? "danger" : "warning");
</script>

<template>
  <aside class="status-attention-column" aria-label="需要关注的项目">
    <section v-if="checks.length > 0" class="attention-panel content-group" :data-tone="issueTone">
      <h3>环境问题</h3>
      <div class="attention-list">
        <div v-for="item in checks" :key="item.code" class="attention-list__item" :data-severity="item.severity">
          <span class="attention-list__icon"><component :is="severityConfig[item.severity].icon" /></span>
          <div>
            <strong>{{ item.title }}</strong>
            <p>{{ item.summary }}</p>
          </div>
        </div>
      </div>
    </section>

    <section v-if="runtimeResources.length > 0" class="attention-panel content-group" data-tone="attention">
      <h3>运行环境准备</h3>
      <p>以下项目可在管理界面的系统状态页准备。</p>
      <ul class="attention-panel__items">
        <li v-for="resource in runtimeResources" :key="resource">{{ formatRuntimeResource(resource) }}</li>
      </ul>
      <div class="button-row button-row--stackable">
        <LauncherButton @click="emit('openWeb')">在管理界面准备</LauncherButton>
      </div>
    </section>
  </aside>
</template>
