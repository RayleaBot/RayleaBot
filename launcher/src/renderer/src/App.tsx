import { useCallback, useMemo, useState } from "react";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherAdvancedOverrides, LauncherSettings } from "@shared/launcher-models";

import { AppShellView } from "./AppShellView";
import { describeLauncherError } from "./AppState.shared";
import { ExitConfirmDialog } from "./ExitConfirmDialog";
import { ActionConfirmDialog } from "./ActionConfirmDialog";
import { useLauncherConfirmations } from "./useLauncherConfirmations";
import { useLauncherInitialization } from "./useLauncherInitialization";
import { useLauncherSectionState } from "./useLauncherSectionState";
import { useLauncherSettingsState } from "./useLauncherSettingsState";

export function App() {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
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

  const {
    confirmedAction,
    exitConfirmOpen,
    handleConfirmedAction,
    handleConfirmedActionCancel,
    handleExitConfirm,
    handleExitConfirmClose,
    setConfirmedAction,
  } = useLauncherConfirmations(setSnapshot, runAction);

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
      (presentation.state === "running" || presentation.state === "setup_required" || presentation.state === "degraded")
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
      onApplyUpdate={() => setConfirmedAction("apply-update")}
      onCheckForUpdates={() => runAction("check-updates", () => window.rayleaLauncher.checkForUpdates())}
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
