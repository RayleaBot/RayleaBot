<script setup lang="ts">
import { FolderOpenIcon, GlobeIcon, HashIcon } from "@lucide/vue";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";
import { computed } from "vue";

import DetailRow from "./DetailRow.vue";

const props = defineProps<{
  resolvedSettings: LauncherResolvedSettings;
  snapshot: LauncherSnapshot;
}>();

function normalizeComparablePath(value: string) {
  const trimmed = value.trim();
  const withoutTrailingSlash = trimmed.replace(/[\\/]+$/, "");
  return withoutTrailingSlash || trimmed;
}

function isWindowsPath(value: string) {
  const trimmed = value.trim();
  return /^[a-z]:[\\/]?/i.test(trimmed) || trimmed.startsWith("\\\\");
}

function isSameDirectoryPath(left: string, right: string) {
  const normalizedLeft = normalizeComparablePath(left);
  const normalizedRight = normalizeComparablePath(right);
  if (!normalizedLeft || !normalizedRight) {
    return false;
  }

  if (isWindowsPath(left) || isWindowsPath(right)) {
    return normalizedLeft.toLowerCase() === normalizedRight.toLowerCase();
  }

  return normalizedLeft === normalizedRight;
}

// The installation directory is a fixed setting shown in preferences. A work directory elsewhere is repeated here
// because the service's logs are written there.
const workdir = computed(() => props.resolvedSettings.workdir);
const showWorkdir = computed(() =>
  Boolean(workdir.value.trim()) && !isSameDirectoryPath(props.snapshot.launcher.settings.installationRoot, workdir.value));
const baseUrl = computed(() => props.snapshot.launcher.endpoint.baseUrl);
</script>

<template>
  <section class="workspace-group" aria-labelledby="status-details-title">
    <h3 id="status-details-title" class="workspace-group__title">运行详情</h3>
    <dl class="detail-list content-group">
      <DetailRow :icon="HashIcon" label="进程 ID" :value="String(snapshot.launcher.processId ?? '—')" />
      <DetailRow :icon="GlobeIcon" label="管理界面地址" :value="baseUrl" :title="baseUrl" />
      <DetailRow v-if="showWorkdir" :icon="FolderOpenIcon" label="工作目录" :value="workdir" :title="workdir" />
    </dl>
  </section>
</template>
