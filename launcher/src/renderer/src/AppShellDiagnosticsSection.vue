<script setup lang="ts">
import {
  ActivityIcon,
  CircleCheckIcon,
  FileTextIcon,
  FolderOpenIcon,
  GlobeIcon,
  InfoIcon,
  TriangleAlertIcon,
} from "@lucide/vue";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import { computed } from "vue";

import { uncapturedOutputText } from "./AppShell.copy";
import { serviceStateConfig } from "./AppShell.shared";
import Disclosure from "./Disclosure.vue";
import LauncherButton from "./LauncherButton.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  diagnosticsSummary: string;
}>();
const emit = defineEmits<{ openLogs: [] }>();

const serviceState = computed(() => serviceStateConfig[deriveLauncherPresentation(props.snapshot).state]);
const hasRecentStderr = computed(() => props.snapshot.launcher.recentStderr.length > 0);
const outputUncaptured = computed(() => !hasRecentStderr.value && props.snapshot.launcher.processOwnership === "external");
const baseUrl = computed(() => props.snapshot.launcher.endpoint.baseUrl);
</script>

<template>
  <div class="diagnostics-workspace" :data-alert="hasRecentStderr ? 'error' : 'none'">
    <dl class="overview-strip content-group">
      <div class="overview-strip__item">
        <dt><ActivityIcon aria-hidden="true" />服务状态</dt>
        <dd>
          <span :class="`status-indicator status-indicator--${serviceState.tone}`" aria-hidden="true" />
          {{ serviceState.label }}
        </dd>
      </div>
      <div class="overview-strip__item" :data-state="hasRecentStderr ? 'danger' : outputUncaptured ? undefined : 'success'">
        <dt><FileTextIcon aria-hidden="true" />日志状态</dt>
        <dd>{{ hasRecentStderr ? "发现异常输出" : outputUncaptured ? "未捕获服务输出" : "未发现异常输出" }}</dd>
      </div>
      <div class="overview-strip__item">
        <dt><GlobeIcon aria-hidden="true" />管理界面地址</dt>
        <dd><code :title="baseUrl">{{ baseUrl }}</code></dd>
      </div>
    </dl>

    <section v-if="hasRecentStderr" class="diagnostics-log content-group" aria-labelledby="diagnostics-log-title">
      <div class="diagnostics-log__heading">
        <div class="diagnostics-log__title">
          <TriangleAlertIcon aria-hidden="true" />
          <h3 id="diagnostics-log-title">异常输出</h3>
          <span class="status-label" data-state="danger">需要检查</span>
        </div>
        <LauncherButton :icon="FolderOpenIcon" @click="emit('openLogs')">打开日志目录</LauncherButton>
      </div>
      <pre class="log-surface diagnostics-log__surface">{{ snapshot.launcher.recentStderr.join("\n") }}</pre>
    </section>
    <div v-else class="diagnostics-empty content-group" :data-alert="outputUncaptured ? 'unknown' : 'none'">
      <div class="diagnostics-empty__status" role="status">
        <span class="diagnostics-empty__icon" aria-hidden="true">
          <InfoIcon v-if="outputUncaptured" />
          <CircleCheckIcon v-else />
        </span>
        <div v-if="outputUncaptured">
          <strong>未捕获服务输出</strong>
          <span>{{ uncapturedOutputText }}</span>
        </div>
        <div v-else>
          <strong>当前没有新的异常输出</strong>
          <span>需要完整上下文时可以打开日志目录，或展开下方技术详情。</span>
        </div>
      </div>
      <LauncherButton :icon="FolderOpenIcon" @click="emit('openLogs')">打开日志目录</LauncherButton>
    </div>

    <Disclosure class="disclosure technical-disclosure content-group" title="技术详情" meta="系统状态、路径与检查快照">
      <pre class="technical-disclosure__surface" aria-label="诊断技术详情">{{ diagnosticsSummary }}</pre>
    </Disclosure>
  </div>
</template>
