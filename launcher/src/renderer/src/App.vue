<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import AppShell from "./AppShell.vue";
import type { LauncherSettings, LauncherSnapshot } from "../../shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

const activeSection = ref<SectionId>("status");
const busyAction = ref<string | null>(null);
const editingSettings = ref(false);
const initializing = ref(true);
const desktopApi = window.rayleaLauncher;
const snapshot = ref<LauncherSnapshot>({
  settings: {
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
    closeBehavior: "ask_every_time",
  },
  endpoint: {
    host: "127.0.0.1",
    port: 8080,
    baseUrl: "http://127.0.0.1:8080/",
  },
  environmentChecks: [],
  recentStderr: [],
  processId: null,
  serviceState: "stopped",
  shutdownRequested: false,
  serviceDetail: "正在加载启动器设置...",
  lastError: "",
  releaseCheck: {
    status: "unavailable",
    currentVersion: "",
    latestVersion: "",
    summary: "版本信息不可用",
    detail: "",
    releasePageUrl: "",
    updateAvailable: false,
  },
});
const settingsDraft = ref<LauncherSettings>({ ...snapshot.value.settings });
const controlsDisabled = computed(() => initializing.value || busyAction.value === "initialize");

const diagnosticsSummary = computed(() => {
  const checks = snapshot.value.environmentChecks
    .map((item) => `- ${item.title}：${item.summary}（${item.detail}${item.remediation ? `；${item.remediation}` : ""}）`)
    .join("\n");
  const recentErrors = snapshot.value.recentStderr.length ? snapshot.value.recentStderr.join("\n") : "当前没有新的错误输出。";
  return [`服务状态：${snapshot.value.serviceDetail}`, `服务入口：${snapshot.value.endpoint.baseUrl}`, "环境检查：", checks || "- 当前没有检查项。", "最近错误输出：", recentErrors].join("\n");
});

let unsubscribe: (() => void) | null = null;

function syncDraft(next: LauncherSnapshot) {
  snapshot.value = next;
  if (!editingSettings.value) {
    settingsDraft.value = { ...next.settings };
  }
}

function describeError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

async function runAction(action: string, task: () => Promise<void>) {
  busyAction.value = action;
  try {
    await task();
  } catch (error) {
    snapshot.value = {
      ...snapshot.value,
      lastError: describeError(error, "启动器操作失败。"),
      serviceDetail: action === "start" ? "启动服务失败。" : snapshot.value.serviceDetail,
    };
  } finally {
    busyAction.value = null;
  }
}

onMounted(async () => {
  unsubscribe = desktopApi.onSnapshot(syncDraft);
  busyAction.value = "initialize";
  try {
    await desktopApi.initialize();
    syncDraft(await desktopApi.getSnapshot());
  } catch (error) {
    snapshot.value = {
      ...snapshot.value,
      lastError: describeError(error, "启动器初始化失败。"),
      serviceDetail: "启动器初始化失败。",
    };
  } finally {
    busyAction.value = null;
    initializing.value = false;
  }
});

onBeforeUnmount(() => {
  unsubscribe?.();
});

async function saveSettings() {
  await runAction("save", async () => {
    await desktopApi.saveSettings(settingsDraft.value);
    editingSettings.value = false;
  });
}
</script>

<template>
  <AppShell
    :snapshot="snapshot"
    :active-section="activeSection"
    :settings-draft="settingsDraft"
    :editing-settings="editingSettings"
    :diagnostics-summary="diagnosticsSummary"
    :busy-action="busyAction"
    :controls-disabled="controlsDisabled"
    @navigate="activeSection = $event"
    @refresh="runAction('refresh', () => desktopApi.refresh())"
    @start="runAction('start', () => desktopApi.start())"
    @stop="runAction('stop', () => desktopApi.stop())"
    @open-web="runAction('open-web', () => desktopApi.openWebUi())"
    @open-release-page="runAction('open-release-page', () => desktopApi.openReleasePage())"
    @open-logs="runAction('open-logs', () => desktopApi.openLogsDirectory())"
    @begin-edit="editingSettings = true"
    @cancel-edit="
      editingSettings = false;
      settingsDraft = { ...snapshot.settings };
    "
    @save-settings="saveSettings"
    @update-settings="settingsDraft = $event"
    @choose-server="
      desktopApi.chooseServerExecutable().then((value: string | null) => {
        if (value) settingsDraft = { ...settingsDraft, serverExecutablePath: value };
      })
    "
    @choose-config="
      desktopApi.chooseConfigFile().then((value: string | null) => {
        if (value) settingsDraft = { ...settingsDraft, configPath: value };
      })
    "
    @choose-workdir="
      desktopApi.chooseWorkdir().then((value: string | null) => {
        if (value) settingsDraft = { ...settingsDraft, workdir: value };
      })
    "
    @exit="desktopApi.exitApplication()"
  />
</template>
