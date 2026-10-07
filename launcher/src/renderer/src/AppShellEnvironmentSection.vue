<script setup lang="ts">
import {
  CircleAlertIcon,
  CircleCheckIcon,
  CircleHelpIcon,
  FolderIcon,
  GlobeIcon,
  MonitorIcon,
  TagIcon,
  TriangleAlertIcon,
} from "@lucide/vue";
import { getEnvironmentSummaryLabel, isBlockingEnvironmentIssue } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { computed, type Component } from "vue";

import { isRuntimePreparationIssue, severityConfig, sortChecks } from "./AppShell.shared";
import DetailRow from "./DetailRow.vue";
import Disclosure from "./Disclosure.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  platformLabel: string;
}>();

type ReadinessTone = "neutral" | "success" | "warning" | "danger";

const readinessIcons: Record<ReadinessTone, Component> = {
  neutral: CircleHelpIcon,
  success: CircleCheckIcon,
  warning: TriangleAlertIcon,
  danger: CircleAlertIcon,
};

const checks = computed(() => sortChecks(props.snapshot.launcher.preflightChecks || []));
const blocking = computed(() => checks.value.filter((item) => item.severity === "error"));
const warnings = computed(() => checks.value.filter((item) => item.severity === "warning"));
const ready = computed(() => checks.value.filter((item) => item.severity === "ok"));
const readiness = computed((): { tone: ReadinessTone; label: string; detail: string } => {
  const summaryLabel = getEnvironmentSummaryLabel(props.snapshot.launcher.preflightChecks);
  if (props.snapshot.launcher.preflightChecks.length === 0) {
    return props.snapshot.launcher.lastLocalError
      ? { tone: "danger", label: "检查结果不可用", detail: "未能获取环境检查结果，请重新检查。" }
      : { tone: "neutral", label: summaryLabel, detail: "完成环境检查后，才能确认服务是否具备启动条件。" };
  }
  if (checks.value.some(isBlockingEnvironmentIssue)) {
    return { tone: "danger", label: summaryLabel, detail: "存在阻塞项，启动前需要先解决。" };
  }
  if (warnings.value.length > 0) {
    return { tone: "warning", label: summaryLabel, detail: "核心能力可用，建议先检查警告项。" };
  }
  return { tone: "success", label: summaryLabel, detail: "当前未发现阻塞或警告项。" };
});
const categories = computed(() => [
  { key: "installation", title: "安装与配置", data: checks.value.filter((item) => !isRuntimePreparationIssue(item.code)) },
  { key: "runtimes", title: "运行环境", data: checks.value.filter((item) => isRuntimePreparationIssue(item.code)) },
]
  .filter((section) => section.data.length > 0)
  .map((section) => ({
    ...section,
    issues: section.data.filter((item) => item.severity !== "ok"),
    healthy: section.data.filter((item) => item.severity === "ok"),
  })));
const allChecksReady = computed(() => blocking.value.length === 0 && warnings.value.length === 0);
</script>

<template>
  <div class="environment-workspace">
    <section class="environment-summary" :data-tone="readiness.tone" aria-labelledby="environment-summary-title">
      <span class="environment-summary__icon" aria-hidden="true"><component :is="readinessIcons[readiness.tone]" /></span>
      <div class="environment-summary__copy">
        <h2 id="environment-summary-title">{{ readiness.label }}</h2>
        <p>{{ readiness.detail }}</p>
      </div>
      <div class="count-badges" aria-label="检查计数">
        <span v-if="blocking.length > 0" data-state="danger">阻塞 {{ blocking.length }}</span>
        <span v-if="warnings.length > 0" data-state="warning">警告 {{ warnings.length }}</span>
        <span v-if="checks.length > 0" :data-state="allChecksReady ? 'neutral' : 'success'">{{ ready.length }}/{{ checks.length }} 正常</span>
        <span v-else>暂无检查项</span>
      </div>
    </section>

    <section
      v-for="section in categories"
      :key="section.key"
      class="workspace-group"
      :aria-labelledby="`check-group-${section.key}`"
    >
      <div class="workspace-group__header">
        <h3 :id="`check-group-${section.key}`" class="workspace-group__title">{{ section.title }}</h3>
        <span v-if="section.issues.length > 0" class="workspace-group__meta">{{ section.issues.length }} 项需要处理</span>
      </div>

      <div class="check-list content-group">
        <div v-for="item in section.issues" :key="item.code" class="check-row" :data-severity="item.severity">
          <span class="check-row__icon" aria-hidden="true"><component :is="severityConfig[item.severity].icon" /></span>
          <div class="check-row__copy">
            <div class="check-row__heading">
              <strong>{{ item.title }}</strong>
              <span class="status-label" :data-state="item.severity">{{ severityConfig[item.severity].label }}</span>
            </div>
            <p>{{ item.summary }}</p>
            <p v-if="item.detail && item.detail !== item.summary">{{ item.detail }}</p>
            <div v-if="item.remediation" class="check-row__remediation">
              <strong>处理方式</strong>
              <span>{{ item.remediation }}</span>
            </div>
          </div>
        </div>

        <Disclosure
          v-if="section.healthy.length > 0"
          class="disclosure check-disclosure"
          :title="section.issues.length > 0 ? '查看正常项' : '检查全部正常'"
          :meta="`${section.healthy.length} 项通过`"
        >
          <div class="check-disclosure__list">
            <div v-for="item in section.healthy" :key="item.code" class="check-row check-row--healthy" data-severity="ok">
              <span class="check-row__icon" aria-hidden="true"><component :is="severityConfig.ok.icon" /></span>
              <div class="check-row__copy">
                <strong>{{ item.title }}</strong>
                <p>{{ item.summary }}</p>
              </div>
            </div>
          </div>
        </Disclosure>
      </div>
    </section>

    <section class="workspace-group" aria-labelledby="environment-info-title">
      <h3 id="environment-info-title" class="workspace-group__title">环境信息</h3>
      <dl class="detail-list detail-list--wrap content-group">
        <DetailRow :icon="MonitorIcon" label="平台" :value="platformLabel || '—'" />
        <DetailRow
          v-if="snapshot.launcher.releaseCheck.currentVersion"
          :icon="TagIcon"
          label="RayleaBot 版本"
          :value="snapshot.launcher.releaseCheck.currentVersion"
          :mono="false"
        />
        <DetailRow
          v-if="snapshot.launcher.settings.installationRoot"
          :icon="FolderIcon"
          label="安装目录"
          :value="snapshot.launcher.settings.installationRoot"
        />
        <DetailRow :icon="GlobeIcon" label="管理界面地址" :value="snapshot.launcher.endpoint.baseUrl" />
      </dl>
    </section>
  </div>
</template>
