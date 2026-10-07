<script setup lang="ts">
import { motion } from "motion-v";
import { computed, ref, shallowRef, watch } from "vue";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherAdvancedOverrides, LauncherSettings, LauncherSnapshot } from "@shared/launcher-models";

import ActionConfirmDialog from "./ActionConfirmDialog.vue";
import AppShellAboutSection from "./AppShellAboutSection.vue";
import AppShellChrome from "./AppShellChrome.vue";
import AppShellDiagnosticsSection from "./AppShellDiagnosticsSection.vue";
import AppShellEnvironmentSection from "./AppShellEnvironmentSection.vue";
import AppShellSectionHeader from "./AppShellSectionHeader.vue";
import AppShellSettingsSection from "./AppShellSettingsSection.vue";
import AppShellStatusSection from "./AppShellStatusSection.vue";
import { describeLauncherError } from "./AppState.shared";
import ExitConfirmDialog from "./ExitConfirmDialog.vue";
import { workspaceOpacity } from "./launcherMotion";
import { useLauncherConfirmations } from "./useLauncherConfirmations";
import { patchLauncherState, useLauncherInitialization } from "./useLauncherInitialization";
import { useLauncherSectionState } from "./useLauncherSectionState";
import { useLauncherSettingsState } from "./useLauncherSettingsState";

const busyAction = ref<string | null>(null);
const editingSettings = ref(false);
const { activeSection, setActiveSection } = useLauncherSectionState();
const { initializing, isMaximized, platformLabel, snapshot } = useLauncherInitialization();
const { diagnosticsSummary, editingDraft, previewResolvedSettings, settingsDraft } = useLauncherSettingsState(snapshot, editingSettings);
const presentation = computed(() => deriveLauncherPresentation(snapshot.value));
const controlsDisabled = computed(() => busyAction.value !== null);
const resolvedSettings = computed(() => editingSettings.value ? previewResolvedSettings.value : snapshot.value.launcher.resolvedSettings);

function reportState(patch: Partial<LauncherSnapshot["launcher"]>) {
  snapshot.value = patchLauncherState(snapshot.value, patch);
}

async function runAction(action: string, task: () => Promise<void>) {
  busyAction.value = action;
  try {
    await task();
  } catch (error) {
    const statusHint = action === "restart"
      ? "重启服务失败。"
      : action === "start"
        ? "启动服务失败。"
        : snapshot.value.launcher.statusHint;
    reportState({ lastLocalError: describeLauncherError(error, "启动器操作失败。"), statusHint });
  } finally {
    busyAction.value = null;
  }
}

const {
  confirmedAction,
  exitConfirmOpen,
  handleConfirmedAction,
  handleConfirmedActionCancel,
  handleExitConfirm,
  handleExitConfirmClose,
} = useLauncherConfirmations(reportState, runAction);

function updateSettings(update: (current: LauncherSettings) => LauncherSettings) {
  if (!editingSettings.value) return;
  editingDraft.value = update(editingDraft.value ?? snapshot.value.launcher.settings);
}

function updateInstallationRoot(installationRoot: string) {
  updateSettings((current) => ({ ...current, installationRoot }));
}

// A chosen close behavior shows at once and saves on its own, without the path draft; the next snapshot that
// changes the saved behavior replaces the choice, since the call result and the snapshot arrive in either order.
const pendingCloseBehavior = shallowRef<LauncherSettings["closeBehavior"] | null>(null);
const closeBehaviorError = ref("");
const closeBehavior = computed(() => pendingCloseBehavior.value ?? snapshot.value.launcher.settings.closeBehavior);
watch(() => snapshot.value.launcher.settings.closeBehavior, () => {
  pendingCloseBehavior.value = null;
  closeBehaviorError.value = "";
});

async function saveCloseBehavior(next: LauncherSettings["closeBehavior"]) {
  if (busyAction.value !== null || next === closeBehavior.value) return;
  pendingCloseBehavior.value = next;
  closeBehaviorError.value = "";
  busyAction.value = "save-close-behavior";
  try {
    await window.rayleaLauncher.saveSettings({ ...snapshot.value.launcher.settings, closeBehavior: next });
  } catch (error) {
    pendingCloseBehavior.value = null;
    closeBehaviorError.value = describeLauncherError(error, "请稍后重试。");
  } finally {
    busyAction.value = null;
  }
}

function updateAdvancedOverride(key: keyof LauncherAdvancedOverrides, value: string) {
  updateSettings((current) => {
    const nextOverrides = {
      ...(current.advancedOverrides ?? {}),
      [key]: value,
    } satisfies LauncherAdvancedOverrides;
    const hasOverrides = Boolean(nextOverrides.serverExecutablePath || nextOverrides.configPath || nextOverrides.workdir);
    return {
      ...current,
      advancedOverrides: hasOverrides ? nextOverrides : undefined,
    };
  });
}

async function saveSettings() {
  const draft = editingDraft.value;
  if (!draft) return;
  await runAction("save", async () => {
    await window.rayleaLauncher.saveSettings({ ...draft, closeBehavior: closeBehavior.value });
    editingSettings.value = false;
    editingDraft.value = null;
  });
}

function beginEdit() {
  const settings = snapshot.value.launcher.settings;
  editingDraft.value = {
    ...settings,
    advancedOverrides: settings.advancedOverrides ? { ...settings.advancedOverrides } : undefined,
  };
  editingSettings.value = true;
}

function cancelEdit() {
  editingSettings.value = false;
  editingDraft.value = null;
}

function runPrimaryServiceAction() {
  const state = presentation.value.state;
  if (
    (state === "running" || state === "setup_required" || state === "degraded")
    && snapshot.value.launcher.processOwnership === "launcher_managed"
  ) {
    // One restart call, so the service is told it is restarting and the Web console keeps reconnecting.
    return runAction("restart", () => window.rayleaLauncher.restart());
  }
  return runAction("start", () => window.rayleaLauncher.start());
}

function choosePath(choose: () => Promise<string | null>, apply: (value: string) => void) {
  void runAction("choose-path", async () => {
    const value = await choose();
    if (value) {
      apply(value);
    }
  });
}

const desktop = () => window.rayleaLauncher;
</script>

<template>
  <div v-if="initializing" class="launcher-loading-shell">
    <h1 class="launcher-loading-shell__title">正在准备启动器</h1>
    <p class="launcher-loading-shell__detail">正在读取安装设置并检查本地服务状态。</p>
  </div>
  <template v-else>
    <div class="app-shell">
      <AppShellChrome
        :snapshot="snapshot"
        :active-section="activeSection"
        :is-maximized="isMaximized"
        @navigate="setActiveSection"
      />

      <motion.main
        :class="`shell-main active-${activeSection}`"
        :data-active-section="activeSection"
        :style="{ opacity: workspaceOpacity }"
      >
        <div class="section-shell" :data-section="activeSection">
          <AppShellSectionHeader
            :snapshot="snapshot"
            :rendered-section="activeSection"
            :busy-action="busyAction"
            :controls-disabled="controlsDisabled"
            :editing-settings="editingSettings"
            @refresh="runAction('refresh', () => desktop().refresh())"
            @open-web="runAction('open-web', () => desktop().openWebUi())"
          />

          <div :key="activeSection" class="section-shell__content">
            <AppShellStatusSection
              v-if="activeSection === 'status'"
              :snapshot="snapshot"
              :resolved-settings="resolvedSettings"
              :busy-action="busyAction"
              :controls-disabled="controlsDisabled"
              @start="runPrimaryServiceAction"
              @stop="runAction('stop', () => desktop().stop())"
              @open-web="runAction('open-web', () => desktop().openWebUi())"
              @open-logs="runAction('open-logs', () => desktop().openLogsDirectory())"
            />
            <AppShellEnvironmentSection
              v-else-if="activeSection === 'environment'"
              :snapshot="snapshot"
              :platform-label="platformLabel"
            />
            <AppShellDiagnosticsSection
              v-else-if="activeSection === 'diagnostics'"
              :snapshot="snapshot"
              :diagnostics-summary="diagnosticsSummary"
              @open-logs="runAction('open-logs', () => desktop().openLogsDirectory())"
            />
            <AppShellSettingsSection
              v-else-if="activeSection === 'settings'"
              :snapshot="snapshot"
              :settings-draft="settingsDraft"
              :resolved-settings="resolvedSettings"
              :editing-settings="editingSettings"
              :close-behavior="closeBehavior"
              :close-behavior-error="closeBehaviorError"
              :busy-action="busyAction"
              :controls-disabled="controlsDisabled"
              @begin-edit="beginEdit"
              @cancel-edit="cancelEdit"
              @save-settings="saveSettings"
              @update-installation-root="updateInstallationRoot"
              @update-advanced-override="updateAdvancedOverride"
              @choose-installation-root="choosePath(() => desktop().chooseInstallationRoot(), updateInstallationRoot)"
              @choose-server="choosePath(() => desktop().chooseServerExecutable(), (value) => updateAdvancedOverride('serverExecutablePath', value))"
              @choose-config="choosePath(() => desktop().chooseConfigFile(), (value) => updateAdvancedOverride('configPath', value))"
              @choose-workdir="choosePath(() => desktop().chooseWorkdir(), (value) => updateAdvancedOverride('workdir', value))"
              @select-close-behavior="saveCloseBehavior"
              @reset-admin="confirmedAction = 'reset-admin'"
              @exit="desktop().exitApplication()"
            />
            <AppShellAboutSection
              v-else
              :snapshot="snapshot"
              :controls-disabled="controlsDisabled"
              @apply-update="confirmedAction = 'apply-update'"
              @check-for-updates="runAction('check-updates', () => desktop().checkForUpdates())"
              @open-release-page="runAction('open-release-page', () => desktop().openReleasePage())"
              @open-repository-page="runAction('open-repository-page', () => desktop().openRepositoryPage())"
            />
          </div>
        </div>
      </motion.main>
    </div>
    <ExitConfirmDialog :open="exitConfirmOpen" @close="handleExitConfirmClose" @confirm="handleExitConfirm" />
    <ActionConfirmDialog :action="confirmedAction" @cancel="handleConfirmedActionCancel" @confirm="handleConfirmedAction" />
  </template>
</template>
