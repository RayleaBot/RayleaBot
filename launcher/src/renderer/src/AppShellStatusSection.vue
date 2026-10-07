<script setup lang="ts">
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";
import { computed } from "vue";

import { formatDiagnosticCheckName, formatDiagnosticCheckValue } from "./AppShell.copy";
import { busyActionLabels, sortChecks } from "./AppShell.shared";
import AppShellRuntimePreparePanel from "./AppShellRuntimePreparePanel.vue";
import AppShellServiceControl from "./AppShellServiceControl.vue";
import AppShellStatusLogs from "./AppShellStatusLogs.vue";
import AppShellStatusRail from "./AppShellStatusRail.vue";
import AppShellStatusSummary from "./AppShellStatusSummary.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  resolvedSettings: LauncherResolvedSettings;
  busyAction: string | null;
  controlsDisabled: boolean;
}>();
const emit = defineEmits<{
  start: [];
  stop: [];
  openWeb: [];
  openLogs: [];
}>();

const presentation = computed(() => deriveLauncherPresentation(props.snapshot));
const runtimePrepare = computed(() => props.snapshot.launcher.runtimePrepare ?? null);
const readiness = computed(() => props.snapshot.server.readiness ?? null);
const setupRequired = computed(() => readiness.value?.status === "setup_required");
const nonOkChecks = computed(() => sortChecks(props.snapshot.launcher.preflightChecks || []).filter((item) => item.severity !== "ok"));
const readinessIssues = computed(() => setupRequired.value ? [] : readiness.value?.issues ?? []);
const readinessReason = computed(() => setupRequired.value ? "" : readiness.value?.reason?.trim() ?? "");
const nonOkReadinessChecks = computed(() => setupRequired.value
  ? []
  : Object.entries(readiness.value?.checks ?? {}).filter(([, value]) => value && value !== "ok"));
const remediationIssues = computed(() => readinessIssues.value.filter((issue) => issue.remediation).slice(0, 3));
const primaryReadinessIssue = computed(() => readinessIssues.value[0] ?? null);
const localError = computed(() => props.snapshot.launcher.lastLocalError.trim());
const statusHint = computed(() => props.snapshot.launcher.statusHint.trim());
const serviceFault = computed(() => presentation.value.state === "failed" || presentation.value.state === "unhealthy");
const statusAlert = computed(() => {
  if (localError.value) return "error";
  if (primaryReadinessIssue.value) return primaryReadinessIssue.value.severity === "error" ? "error" : "warning";
  if (readinessReason.value) return serviceFault.value ? "error" : "warning";
  return nonOkChecks.value.length > 0 ? "warning" : "none";
});
const statusReasonText = computed(() => readinessReason.value || primaryReadinessIssue.value?.summary || presentation.value.detail);
// A local error with a hint describes a failed Launcher operation, so the hint replaces the state description.
// An error without one, such as a system status the Launcher may not read, keeps the description and is shown
// as the cause below.
const serviceControlDetail = computed(() => {
  if (runtimePrepare.value?.active) return "运行环境准备中。";
  if (localError.value && statusHint.value) return statusHint.value;
  if (presentation.value.state === "degraded") return "服务可以使用，但部分运行条件未满足。";
  if (presentation.value.state === "unhealthy" && readiness.value) return "服务未通过就绪检查。";
  return presentation.value.detail;
});
const hasReadinessDiagnostics = computed(() =>
  !runtimePrepare.value?.active && (remediationIssues.value.length > 0 || nonOkReadinessChecks.value.length > 0));
const preparableResources = computed(() => props.controlsDisabled ? [] : presentation.value.preparableRuntimeResources);
const showStatusRail = computed(() => nonOkChecks.value.length > 0 || preparableResources.value.length > 0);
const externalService = computed(() => props.snapshot.launcher.processOwnership === "external");
const startDisabled = computed(() => {
  const state = presentation.value.state;
  return props.controlsDisabled
    || ((state === "running" || state === "setup_required" || state === "degraded") && externalService.value)
    || state === "starting"
    || state === "stopping";
});
const stopDisabled = computed(() => {
  const state = presentation.value.state;
  return props.controlsDisabled || state === "starting" || state === "stopping" || props.snapshot.launcher.processOwnership === "none";
});
const busyLabel = computed(() => props.busyAction ? (busyActionLabels[props.busyAction] ?? "正在执行操作") : "");
// A restart keeps the running actions in place while the service stops and starts again.
const showRunningActions = computed(() => presentation.value.canOpenWebUi || props.busyAction === "restart");
const serviceAttention = computed(() => {
  if (runtimePrepare.value?.active) {
    return { label: "准备进度", text: runtimePrepare.value.summary || "正在准备运行环境。", tone: "attention" as const };
  }
  const note = localError.value
    ? { label: "失败原因", text: localError.value, tone: "danger" as const }
    : serviceFault.value
      ? { label: "失败原因", text: statusReasonText.value, tone: "danger" as const }
      : presentation.value.state === "degraded"
        ? { label: "当前限制", text: statusReasonText.value, tone: "warning" as const }
        : null;
  return note && note.text !== serviceControlDetail.value ? note : null;
});
</script>

<template>
  <div
    class="status-homepage status-view-flow"
    :data-state="presentation.state"
    :data-busy="busyAction ?? 'idle'"
    :data-alert="statusAlert"
  >
    <AppShellServiceControl
      :attention="serviceAttention"
      :busy-label="busyLabel"
      :can-open-web-ui="presentation.canOpenWebUi"
      :controls-disabled="controlsDisabled"
      :external-service="externalService"
      :service-detail="serviceControlDetail"
      :service-state="presentation.state"
      :show-running-actions="showRunningActions"
      :start-disabled="startDisabled"
      :stop-disabled="stopDisabled"
      :stop-opens-web="snapshot.launcher.controlCapability === 'open_web'"
      @open-web="emit('openWeb')"
      @start="emit('start')"
      @stop="emit('stop')"
    />

    <div class="status-layout" :data-has-rail="showStatusRail ? 'true' : 'false'">
      <div class="status-main-column">
        <AppShellRuntimePreparePanel :runtime-prepare="runtimePrepare" />

        <section v-if="hasReadinessDiagnostics" class="service-diagnostics content-group">
          <h3>服务诊断</h3>

          <div v-if="remediationIssues.length > 0" class="status-diagnostics-block">
            <span class="status-label">问题与处理方式</span>
            <div class="status-diagnostics-list">
              <div
                v-for="issue in remediationIssues"
                :key="`${issue.code}-${issue.summary}`"
                :class="`status-diagnostics-item status-diagnostics-item--${issue.severity}`"
              >
                <div class="status-diagnostics-item__header">
                  <span class="status-diagnostics-item__summary">{{ issue.summary }}</span>
                  <span :class="`status-pill status-pill--${issue.severity === 'error' ? 'error' : 'warning'}`">
                    {{ issue.severity === "error" ? "阻塞" : "警告" }}
                  </span>
                </div>
                <p class="status-diagnostics-item__remedy">处理方式：{{ issue.remediation }}</p>
              </div>
            </div>
          </div>

          <div v-if="nonOkReadinessChecks.length > 0" class="status-diagnostics-block">
            <span class="status-label">检查项</span>
            <div class="status-diagnostics-checks">
              <div v-for="[name, value] in nonOkReadinessChecks" :key="`${name}-${value}`" class="status-diagnostics-check">
                <span class="status-diagnostics-check__name">{{ formatDiagnosticCheckName(name) }}</span>
                <span class="status-diagnostics-check__value">{{ formatDiagnosticCheckValue(value) }}</span>
              </div>
            </div>
          </div>
        </section>

        <AppShellStatusSummary :snapshot="snapshot" :resolved-settings="resolvedSettings" />
      </div>

      <AppShellStatusRail
        v-if="showStatusRail"
        :checks="nonOkChecks"
        :runtime-resources="preparableResources"
        @open-web="emit('openWeb')"
      />
    </div>

    <AppShellStatusLogs
      :external-service="externalService"
      :logs="snapshot.launcher.recentStderr"
      @open-logs="emit('openLogs')"
    />
  </div>
</template>
