<script setup lang="ts">
import { DownloadIcon, ExternalLinkIcon, RotateCwIcon, ScrollTextIcon, TagIcon, TypeIcon } from "@lucide/vue";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { computed } from "vue";

import { formatReleaseVersion } from "./AppShell.shared";
import DetailRow from "./DetailRow.vue";
import LauncherButton from "./LauncherButton.vue";
import RayleaMark from "./RayleaMark.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  controlsDisabled: boolean;
}>();
const emit = defineEmits<{
  applyUpdate: [];
  checkForUpdates: [];
  openReleasePage: [];
  openRepositoryPage: [];
}>();

// Update failures share the failed status; the error code tells a failed check from a failed installation.
const checkFailureCodes = new Set(["launcher.update_check_failed", "launcher.update_response_invalid"]);

const releaseCheck = computed(() => props.snapshot.launcher.releaseCheck);
const currentVersion = computed(() => formatReleaseVersion(releaseCheck.value));
// The version line names the state only; the error bar below carries the failure details.
const versionHint = computed(() => {
  const check = releaseCheck.value;
  const latestVersion = check.latestVersion.trim();
  switch (check.status) {
    case "checking":
      return "正在检查更新";
    case "updating":
      return check.summary || "正在更新";
    case "up_to_date":
      return "已是最新";
    case "update_available":
      return latestVersion ? `有新版本 ${latestVersion}` : "有新版本";
    case "cancelled":
      return "已取消更新";
    case "failed":
      if (check.errorCode === "launcher.update_relaunch_failed") {
        return "待重新打开";
      }
      return checkFailureCodes.has(check.errorCode) ? "检查失败" : "更新失败";
    default:
      return "";
  }
});
const updating = computed(() => releaseCheck.value.status === "updating");
const canApplyUpdate = computed(() => releaseCheck.value.updateAvailable && !updating.value && releaseCheck.value.status !== "checking");
const guidedRelease = computed(() => Boolean(releaseCheck.value.releasePageUrl) && !releaseCheck.value.canCheck);
const updateButtonLabel = computed(() => guidedRelease.value ? "打开发布页" : releaseCheck.value.status === "checking" ? "检查中" : "检查更新");
const updateDisabled = computed(() =>
  props.controlsDisabled || releaseCheck.value.status === "checking" || (!guidedRelease.value && !releaseCheck.value.canCheck));
const showUpdateAction = computed(() => releaseCheck.value.canCheck || guidedRelease.value || releaseCheck.value.status === "checking");
// A cancelled update is the user's choice, not a failure, so it only shows in the version line.
const showUpdateError = computed(() =>
  releaseCheck.value.status !== "cancelled" && (Boolean(releaseCheck.value.errorCode) || releaseCheck.value.status === "failed"));

function runUpdateAction() {
  if (guidedRelease.value) {
    emit("openReleasePage");
  } else {
    emit("checkForUpdates");
  }
}
</script>

<template>
  <div class="about-workspace">
    <section class="about-identity" aria-labelledby="about-title">
      <span class="about-identity__mark" aria-hidden="true"><RayleaMark variant="neutral" /></span>
      <div class="about-identity__copy">
        <h2 id="about-title">RayleaBot 启动器</h2>
        <p>检查本地环境、管理服务并定位运行问题。</p>
      </div>
      <div class="about-identity__actions">
        <LauncherButton v-if="updating" :icon="DownloadIcon" disabled>正在更新</LauncherButton>
        <template v-else-if="canApplyUpdate">
          <LauncherButton
            :icon="DownloadIcon"
            :emphasis="controlsDisabled ? 'regular' : 'prominent'"
            :disabled="controlsDisabled"
            @click="emit('applyUpdate')"
          >
            立即更新
          </LauncherButton>
          <LauncherButton :icon="ExternalLinkIcon" @click="emit('openReleasePage')">发布页</LauncherButton>
        </template>
        <LauncherButton
          v-else-if="showUpdateAction"
          :icon="guidedRelease ? ExternalLinkIcon : RotateCwIcon"
          :disabled="updateDisabled"
          @click="runUpdateAction"
        >
          {{ updateButtonLabel }}
        </LauncherButton>
        <span v-else class="update-unavailable">当前构建不提供更新检查</span>
        <LauncherButton :icon="ExternalLinkIcon" @click="emit('openRepositoryPage')">GitHub</LauncherButton>
      </div>
    </section>

    <dl class="detail-list detail-list--wrap content-group">
      <DetailRow :icon="TagIcon" label="RayleaBot 版本">
        <span class="version-value" :data-status="releaseCheck.status">
          <span>{{ currentVersion }}</span>
          <span v-if="versionHint">{{ versionHint }}</span>
        </span>
      </DetailRow>
      <DetailRow :icon="ScrollTextIcon" label="许可证" value="AGPL-3.0" :mono="false" />
      <!-- HarmonyOS Sans requires a notice in the software that the fonts are used. -->
      <DetailRow :icon="TypeIcon" label="界面字体" value="HarmonyOS Sans SC" :mono="false" />
    </dl>

    <div v-if="showUpdateError" class="update-error-message">
      <strong class="update-error-message__title">{{ releaseCheck.summary || "更新检查没有返回错误摘要" }}</strong>
      <div v-if="releaseCheck.errorCode" class="update-error-code">
        <span>错误代码</span>
        <code>{{ releaseCheck.errorCode }}</code>
      </div>
      <p class="update-error-detail">{{ releaseCheck.detail || "更新检查没有返回错误原因。" }}</p>
    </div>
  </div>
</template>
