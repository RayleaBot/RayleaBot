<script setup lang="ts">
import type { RuntimePrepareResourceProgress, RuntimePrepareSnapshot } from "@shared/launcher-models";
import { ProgressIndicator, ProgressRoot } from "reka-ui";
import { computed } from "vue";

import { formatByteCount } from "./AppShell.shared";

const props = defineProps<{
  runtimePrepare: RuntimePrepareSnapshot | null;
}>();

const statusLabels: Record<string, string> = {
  pending: "待处理",
  running: "进行中",
  succeeded: "已完成",
  failed: "失败",
};

const stageLabels: Record<string, string> = {
  inspect: "检查",
  lock: "等待其他准备完成",
  probe: "选择下载源",
  download: "下载",
  verify: "校验",
  cleanup: "清理",
  extract: "解压",
  activate: "启用",
  complete: "完成",
  manifest: "读取资源清单",
  entrypoint: "检查程序文件",
};

function progressText(item: RuntimePrepareResourceProgress) {
  if (item.stage === "download") {
    const downloaded = formatByteCount(item.downloadedBytes);
    const total = formatByteCount(item.totalBytes);
    if (downloaded && total) {
      return `${downloaded} / ${total}`;
    }
    if (downloaded) {
      return downloaded;
    }
  }
  if (item.stage === "extract" && item.extractedEntries !== null && item.totalEntries !== null && item.totalEntries > 0) {
    return `${item.extractedEntries} / ${item.totalEntries} 个文件`;
  }
  if (item.progress !== null) {
    return `${item.progress}%`;
  }
  return "正在处理";
}

function statusLabel(status: string) {
  return statusLabels[status] ?? status;
}

function resourceKey(item: RuntimePrepareResourceProgress) {
  return item.kind || item.resourceId || item.label;
}

const visible = computed(() => props.runtimePrepare !== null && (props.runtimePrepare.active || props.runtimePrepare.resources.length > 0));
const current = computed(() => {
  const prepare = props.runtimePrepare;
  if (!prepare) return null;
  return prepare.resources.find((item) => item.kind === prepare.currentKind) ?? prepare.resources.at(-1) ?? null;
});
</script>

<template>
  <article v-if="visible && runtimePrepare" class="panel surface-panel surface-panel--subtle runtime-prepare-panel content-group">
    <div class="runtime-prepare-panel__header">
      <span class="runtime-prepare-panel__summary">{{ runtimePrepare.summary || current?.summary || "正在准备运行环境" }}</span>
      <span v-if="current" :class="`runtime-prepare-status runtime-prepare-status--${current.status}`">
        {{ statusLabel(current.status) }}
      </span>
    </div>
    <div class="runtime-prepare-list">
      <div v-for="item in runtimePrepare.resources" :key="resourceKey(item)" :class="`runtime-prepare-item runtime-prepare-item--${item.status}`">
        <div class="runtime-prepare-item__header">
          <div class="runtime-prepare-item__title">
            <strong>{{ item.label }}</strong>
            <span class="runtime-prepare-item__stage">{{ stageLabels[item.stage] ?? item.stage }}</span>
          </div>
          <span :class="`runtime-prepare-status runtime-prepare-status--${item.status}`">{{ statusLabel(item.status) }}</span>
        </div>
        <div class="runtime-prepare-progress-row">
          <ProgressRoot
            class="runtime-prepare-progress"
            :model-value="item.progress"
            :aria-label="`${item.label}准备进度`"
            :get-value-label="() => `${statusLabel(item.status)}，${progressText(item)}`"
          >
            <ProgressIndicator
              class="runtime-prepare-progress__bar"
              :style="item.progress === null ? undefined : { transform: `translateX(-${100 - item.progress}%)` }"
            />
          </ProgressRoot>
          <span class="runtime-prepare-progress-text">{{ progressText(item) }}</span>
        </div>
        <div class="runtime-prepare-details">
          <div v-if="item.sourceUrl">
            <span>下载来源</span><code :title="item.sourceUrl">{{ item.sourceLabel ? `${item.sourceLabel} · ${item.sourceUrl}` : item.sourceUrl }}</code>
          </div>
          <div v-if="item.archivePath"><span>下载位置</span><code :title="item.archivePath">{{ item.archivePath }}</code></div>
          <div v-if="item.storeRoot"><span>解压位置</span><code :title="item.storeRoot">{{ item.storeRoot }}</code></div>
          <div v-if="item.error"><span>错误</span><code :title="item.error">{{ item.error }}</code></div>
        </div>
      </div>
    </div>
  </article>
</template>
