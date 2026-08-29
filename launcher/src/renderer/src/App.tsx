import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherAdvancedOverrides, LauncherSettings } from "@shared/launcher-models";

import { AppShellView } from "./AppShellView";
import { describeLauncherError } from "./AppState.shared";
import { ExitConfirmDialog } from "./ExitConfirmDialog";
import { ActionConfirmDialog, type ConfirmedLauncherAction } from "./ActionConfirmDialog";
import { useLauncherInitialization } from "./useLauncherInitialization";
import { useLauncherSectionState } from "./useLauncherSectionState";
import { useLauncherSettingsState } from "./useLauncherSettingsState";

export function App() {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
  const [exitConfirmOpen, setExitConfirmOpen] = useState(false);
  const [confirmedAction, setConfirmedAction] = useState<ConfirmedLauncherAction | null>(null);
  const externalStopConfirmPending = useRef(false);
  const {
    activeSection,
    setActiveSection,
  } = useLauncherSectionState();
  const {
    initializing,
    isMaximized,
    platformLabel,
    setSnapshot,
    snapshot,
  } = useLauncherInitialization();
  const {
    diagnosticsSummary,
    editingDraft,
    previewResolvedSettings, 
    setEditingDraft,
    settingsDraft,
  } = useLauncherSettingsState(snapshot, editingSettings);
  const presentation = useMemo(() => deriveLauncherPresentation(snapshot), [snapshot]);

  const controlsDisabled = busyAction !== null;

  const runAction = useCallback(
    async (action: string, task: () => Promise<void>) => {
      setBusyAction(action);
      try {
        await task();
      } catch (error) {
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "启动器操作失败。"),
            statusHint:
              action === "restart"
                ? "重启服务失败。"
                : action === "start"
                  ? "启动服务失败。"
                  : prev.launcher.statusHint,
          },
        }));
      } finally {
        setBusyAction(null);
      }
    },
    [setSnapshot],
  );

  const handleUpdateSettings = useCallback(
    (update: (current: LauncherSettings) => LauncherSettings) => {
      if (!editingSettings) return;
      setEditingDraft((prev) => update(prev ?? snapshot.launcher.settings));
    },
    [editingSettings, setEditingDraft, snapshot.launcher.settings],
  );

  const handleUpdateInstallationRoot = useCallback(
    (installationRoot: string) => {
      handleUpdateSettings((current) => ({
        ...current,
        installationRoot,
      }));
    },
    [handleUpdateSettings],
  );

  const handleUpdateCloseBehavior = useCallback(
    (closeBehavior: LauncherSettings["closeBehavior"]) => {
      handleUpdateSettings((current) => ({
        ...current,
        closeBehavior,
      }));
    },
    [handleUpdateSettings],
  );

  const handleUpdateAdvancedOverride = useCallback(
    (key: keyof LauncherAdvancedOverrides, value: string) => {
      handleUpdateSettings((current) => {
        const nextOverrides = {
          ...(current.advancedOverrides ?? {}),
          [key]: value,
        } satisfies LauncherAdvancedOverrides;
        const hasOverrides = Boolean(
          nextOverrides.serverExecutablePath
          || nextOverrides.configPath
          || nextOverrides.workdir,
        );
        return {
          ...current,
          advancedOverrides: hasOverrides ? nextOverrides : undefined,
        };
      });
    },
    [handleUpdateSettings],
  );

  const handleSaveSettings = useCallback(async () => {
    if (!editingDraft) return;
    await runAction("save", async () => {
      await window.rayleaLauncher.saveSettings(editingDraft);
      setEditingSettings(false);
      setEditingDraft(null);
    });
  }, [editingDraft, runAction, setEditingDraft]);

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
    if (action === "install-update") {
      void runAction(action, () => window.rayleaLauncher.installDownloadedUpdate());
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
    const showExitConfirm = () => {
      setExitConfirmOpen(true);
    };
    const unsubscribe = window.rayleaLauncher.onShowExitConfirm(showExitConfirm);
    void window.rayleaLauncher.hasPendingCloseConfirm()
      .then((pending) => {
        if (pending) {
          showExitConfirm();
        }
      })
      .catch((error) => {
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "无法读取关闭确认状态。"),
          },
        }));
      });
    return unsubscribe;
  }, [setSnapshot]);

  useEffect(() => {
    const showExternalStopConfirm = () => {
      externalStopConfirmPending.current = true;
      setConfirmedAction("stop-external");
    };
    const unsubscribe = window.rayleaLauncher.onShowExternalStopConfirm(showExternalStopConfirm);
    void window.rayleaLauncher.hasPendingExternalStopConfirm()
      .then((pending) => {
        if (pending) {
          showExternalStopConfirm();
        }
      })
      .catch((error) => {
        setSnapshot((prev) => ({
          ...prev,
          launcher: {
            ...prev.launcher,
            lastLocalError: describeLauncherError(error, "无法读取现有服务停止确认状态。"),
          },
        }));
      });
    return unsubscribe;
  }, [setSnapshot]);

  const handleBeginEdit = useCallback(() => {
    setEditingDraft({
      ...snapshot.launcher.settings,
      advancedOverrides: snapshot.launcher.settings.advancedOverrides
        ? { ...snapshot.launcher.settings.advancedOverrides }
        : undefined,
    });
    setEditingSettings(true);
  }, [setEditingDraft, snapshot.launcher.settings]);

  const handleCancelEdit = useCallback(() => {
    setEditingSettings(false);
    setEditingDraft(null);
  }, [setEditingDraft]);

  const handlePrimaryServiceAction = useCallback(() => {
    if (
      (presentation.state === "running" || presentation.state === "degraded")
      && snapshot.launcher.processOwnership === "launcher_managed"
    ) {
      return runAction("restart", async () => {
        await window.rayleaLauncher.stop();
        await window.rayleaLauncher.start();
      });
    }
    return runAction("start", () => window.rayleaLauncher.start());
  }, [presentation.state, runAction, snapshot.launcher.processOwnership]);

  const handleChoosePath = useCallback(
    (choose: () => Promise<string | null>, apply: (value: string) => void) => {
      void runAction("choose-path", async () => {
        const value = await choose();
        if (value) {
          apply(value);
        }
      });
    },
    [runAction],
  );

  if (initializing) {
    return (
      <div className="launcher-loading-shell">
        <h1 className="launcher-loading-shell__title">正在准备启动器</h1>
        <p className="launcher-loading-shell__detail">正在读取安装设置并检查本地服务状态。</p>
      </div>
    );
  }

  return (
    <>
    <AppShellView
      snapshot={snapshot}
      activeSection={activeSection}
      platformLabel={platformLabel}
      settingsDraft={settingsDraft}
      resolvedSettings={editingSettings ? previewResolvedSettings : snapshot.launcher.resolvedSettings}
      editingSettings={editingSettings}
      diagnosticsSummary={diagnosticsSummary}
      busyAction={busyAction}
      controlsDisabled={controlsDisabled}
      isMaximized={isMaximized}
      onNavigate={setActiveSection}
      onRefresh={() => runAction("refresh", () => window.rayleaLauncher.refresh())}
      onStart={handlePrimaryServiceAction}
      onStop={() => runAction("stop", () => window.rayleaLauncher.stop())}
      onOpenWeb={() => runAction("open-web", () => window.rayleaLauncher.openWebUi())}
      onOpenTasks={() => runAction("open-web", () => window.rayleaLauncher.openWebUi("/logs?source=tasks"))}
      onCheckForUpdates={() => runAction("check-updates", () => window.rayleaLauncher.checkForUpdates())}
      onDownloadUpdate={() => runAction("download-update", () => window.rayleaLauncher.downloadUpdate())}
      onInstallDownloadedUpdate={() => setConfirmedAction("install-update")}
      onOpenReleasePage={() => runAction("open-release-page", () => window.rayleaLauncher.openReleasePage())}
      onOpenRepositoryPage={() => runAction("open-repository-page", () => window.rayleaLauncher.openRepositoryPage())}
      onOpenLogs={() => runAction("open-logs", () => window.rayleaLauncher.openLogsDirectory())}
      onResetAdmin={() => setConfirmedAction("reset-admin")}
      onBeginEdit={handleBeginEdit}
      onCancelEdit={handleCancelEdit}
      onSaveSettings={handleSaveSettings}
      onUpdateInstallationRoot={handleUpdateInstallationRoot}
      onUpdateCloseBehavior={handleUpdateCloseBehavior}
      onUpdateAdvancedOverride={handleUpdateAdvancedOverride}
      onChooseInstallationRoot={() => handleChoosePath(
        () => window.rayleaLauncher.chooseInstallationRoot(),
        handleUpdateInstallationRoot,
      )}
      onChooseServer={() => handleChoosePath(
        () => window.rayleaLauncher.chooseServerExecutable(),
        (value) => handleUpdateAdvancedOverride("serverExecutablePath", value),
      )}
      onChooseConfig={() => handleChoosePath(
        () => window.rayleaLauncher.chooseConfigFile(),
        (value) => handleUpdateAdvancedOverride("configPath", value),
      )}
      onChooseWorkdir={() => handleChoosePath(
        () => window.rayleaLauncher.chooseWorkdir(),
        (value) => handleUpdateAdvancedOverride("workdir", value),
      )}
      onExit={() => window.rayleaLauncher.exitApplication()}
    />
    <ExitConfirmDialog
      open={exitConfirmOpen}
      onClose={handleExitConfirmClose}
      onConfirm={handleExitConfirm}
    />
    <ActionConfirmDialog
      action={confirmedAction}
      onCancel={handleConfirmedActionCancel}
      onConfirm={handleConfirmedAction}
    />
  </>);
}
