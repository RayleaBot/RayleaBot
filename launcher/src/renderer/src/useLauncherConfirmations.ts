import { onScopeDispose, ref } from "vue";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { describeLauncherError } from "./AppState.shared";

export type ConfirmedLauncherAction = "reset-admin" | "stop-external" | "apply-update";

type RunAction = (action: string, task: () => Promise<void>) => Promise<void>;
type ReportState = (patch: Partial<LauncherSnapshot["launcher"]>) => void;

export function useLauncherConfirmations(reportState: ReportState, runAction: RunAction) {
  const exitConfirmOpen = ref(false);
  const confirmedAction = ref<ConfirmedLauncherAction | null>(null);
  let externalStopConfirmPending = false;
  let active = true;

  const respondToExitConfirm = async (action: "hide" | "exit" | "cancel", setAsDefault: boolean) => {
    exitConfirmOpen.value = false;
    try {
      await window.rayleaLauncher.closeConfirmResponse({ action, setAsDefault });
    } catch (error) {
      exitConfirmOpen.value = true;
      reportState({
        statusHint: "无法提交关闭确认。",
        lastLocalError: describeLauncherError(error, "关闭确认提交失败。"),
      });
    }
  };

  const handleExitConfirm = (action: "hide" | "exit", setAsDefault: boolean) => {
    void respondToExitConfirm(action, setAsDefault);
  };

  const handleExitConfirmClose = () => {
    void respondToExitConfirm("cancel", false);
  };

  const respondToExternalStopConfirm = (confirmed: boolean) => {
    if (!externalStopConfirmPending) {
      return;
    }
    externalStopConfirmPending = false;
    void window.rayleaLauncher.externalStopConfirmResponse(confirmed).catch((error) => {
      reportState({
        statusHint: "无法提交现有服务的停止确认。",
        lastLocalError: describeLauncherError(error, "停止确认提交失败。"),
      });
    });
  };

  const handleConfirmedAction = (action: ConfirmedLauncherAction) => {
    confirmedAction.value = null;
    if (action === "stop-external") {
      respondToExternalStopConfirm(true);
      return;
    }
    if (action === "apply-update") {
      void runAction(action, () => window.rayleaLauncher.applyUpdate());
      return;
    }
    void runAction(action, () => window.rayleaLauncher.resetAdmin());
  };

  const handleConfirmedActionCancel = () => {
    if (confirmedAction.value === "stop-external") {
      respondToExternalStopConfirm(false);
    }
    confirmedAction.value = null;
  };

  // A prompt the host raised before the renderer subscribed is still pending; one that the event already
  // showed is not shown again when the lookup arrives late.
  let exitConfirmNotified = false;
  const showExitConfirm = () => {
    if (!active) return;
    exitConfirmNotified = true;
    exitConfirmOpen.value = true;
  };
  const unsubscribeExitConfirm = window.rayleaLauncher.onShowExitConfirm(showExitConfirm);
  void window.rayleaLauncher.hasPendingCloseConfirm()
    .then((pending) => {
      if (pending && !exitConfirmNotified) {
        showExitConfirm();
      }
    })
    .catch((error) => {
      if (!active) return;
      reportState({ lastLocalError: describeLauncherError(error, "无法读取关闭确认状态。") });
    });

  let externalStopNotified = false;
  const showExternalStopConfirm = () => {
    if (!active) return;
    externalStopNotified = true;
    externalStopConfirmPending = true;
    confirmedAction.value = "stop-external";
  };
  const unsubscribeExternalStop = window.rayleaLauncher.onShowExternalStopConfirm(showExternalStopConfirm);
  void window.rayleaLauncher.hasPendingExternalStopConfirm()
    .then((pending) => {
      if (pending && !externalStopNotified) {
        showExternalStopConfirm();
      }
    })
    .catch((error) => {
      if (!active) return;
      reportState({ lastLocalError: describeLauncherError(error, "无法读取现有服务停止确认状态。") });
    });

  onScopeDispose(() => {
    active = false;
    unsubscribeExitConfirm();
    unsubscribeExternalStop();
  });

  return {
    confirmedAction,
    exitConfirmOpen,
    handleConfirmedAction,
    handleConfirmedActionCancel,
    handleExitConfirm,
    handleExitConfirmClose,
  };
}
