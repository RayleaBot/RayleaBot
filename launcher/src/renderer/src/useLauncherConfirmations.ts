import { useCallback, useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import type { LauncherSnapshot } from "@shared/launcher-models";

import type { ConfirmedLauncherAction } from "./ActionConfirmDialog";
import { describeLauncherError } from "./AppState.shared";

type RunAction = (action: string, task: () => Promise<void>) => Promise<void>;

export function useLauncherConfirmations(setSnapshot: Dispatch<SetStateAction<LauncherSnapshot>>, runAction: RunAction) {
  const [exitConfirmOpen, setExitConfirmOpen] = useState(false);
  const [confirmedAction, setConfirmedAction] = useState<ConfirmedLauncherAction | null>(null);
  const externalStopConfirmPending = useRef(false);

  const respondToExitConfirm = useCallback(
    async (action: "hide" | "exit" | "cancel", setAsDefault: boolean) => {
      setExitConfirmOpen(false);
      try {
        await window.rayleaLauncher.closeConfirmResponse({ action, setAsDefault });
      } catch (error) {
        setExitConfirmOpen(true);
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            statusHint: "无法提交关闭确认。",
            lastLocalError: describeLauncherError(error, "关闭确认提交失败。"),
          },
        }));
      }
    },
    [setSnapshot],
  );

  const handleExitConfirm = useCallback(
    (action: "hide" | "exit", setAsDefault: boolean) => {
      void respondToExitConfirm(action, setAsDefault);
    },
    [respondToExitConfirm],
  );

  const handleExitConfirmClose = useCallback(() => {
    void respondToExitConfirm("cancel", false);
  }, [respondToExitConfirm]);

  const respondToExternalStopConfirm = useCallback((confirmed: boolean) => {
    if (!externalStopConfirmPending.current) {
      return;
    }
    externalStopConfirmPending.current = false;
    void window.rayleaLauncher.externalStopConfirmResponse(confirmed).catch((error) => {
      setSnapshot((prev) => ({
        ...prev,
        launcher: {
          ...prev.launcher,
          statusHint: "无法提交现有服务的停止确认。",
          lastLocalError: describeLauncherError(error, "停止确认提交失败。"),
        },
      }));
    });
  }, [setSnapshot]);

  const handleConfirmedAction = useCallback((action: ConfirmedLauncherAction) => {
    setConfirmedAction(null);
    if (action === "stop-external") {
      respondToExternalStopConfirm(true);
      return;
    }
    if (action === "apply-update") {
      void runAction(action, () => window.rayleaLauncher.applyUpdate());
      return;
    }
    void runAction(action, () => window.rayleaLauncher.resetAdmin());
  }, [respondToExternalStopConfirm, runAction]);

  const handleConfirmedActionCancel = useCallback(() => {
    if (confirmedAction === "stop-external") {
      respondToExternalStopConfirm(false);
    }
    setConfirmedAction(null);
  }, [confirmedAction, respondToExternalStopConfirm]);

  useEffect(() => {
    let active = true;
    let notified = false;
    const showExitConfirm = () => {
      if (!active) return;
      notified = true;
      setExitConfirmOpen(true);
    };
    const unsubscribe = window.rayleaLauncher.onShowExitConfirm(showExitConfirm);
    void window.rayleaLauncher.hasPendingCloseConfirm()
      .then((pending) => {
        if (pending && active && !notified) {
          showExitConfirm();
        }
      })
      .catch((error) => {
        if (!active) return;
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "无法读取关闭确认状态。"),
          },
        }));
      });
    return () => {
      active = false;
      unsubscribe();
    };
  }, [setSnapshot]);

  useEffect(() => {
    let active = true;
    let notified = false;
    const showExternalStopConfirm = () => {
      if (!active) return;
      notified = true;
      externalStopConfirmPending.current = true;
      setConfirmedAction("stop-external");
    };
    const unsubscribe = window.rayleaLauncher.onShowExternalStopConfirm(showExternalStopConfirm);
    void window.rayleaLauncher.hasPendingExternalStopConfirm()
      .then((pending) => {
        if (pending && active && !notified) {
          showExternalStopConfirm();
        }
      })
      .catch((error) => {
        if (!active) return;
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "无法读取现有服务停止确认状态。"),
          },
        }));
      });
    return () => {
      active = false;
      unsubscribe();
    };
  }, [setSnapshot]);

  return {
    confirmedAction,
    exitConfirmOpen,
    handleConfirmedAction,
    handleConfirmedActionCancel,
    handleExitConfirm,
    handleExitConfirmClose,
    setConfirmedAction,
  };
}
