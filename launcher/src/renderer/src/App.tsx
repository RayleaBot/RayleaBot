import { startTransition, useCallback, useEffect, useMemo, useState } from "react";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherAdvancedOverrides, LauncherSettings } from "@shared/launcher-models";

import { AppShell } from "./AppShell";
import { describeLauncherError, type SectionId } from "./AppState.shared";
import { ExitConfirmDialog } from "./ExitConfirmDialog";
import { useLauncherInitialization } from "./useLauncherInitialization";
import { useLauncherSectionState } from "./useLauncherSectionState";
import { useLauncherSettingsState } from "./useLauncherSettingsState";

export function App() {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
  const [exitConfirmOpen, setExitConfirmOpen] = useState(false);
  const {
    activeSection,
    renderedSection,
    sectionTransitionState,
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

  const controlsDisabled = useMemo(
    () => initializing || busyAction === "initialize",
    [initializing, busyAction],
  );

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

  const handleExitConfirm = useCallback(
    (action: "hide" | "exit", setAsDefault: boolean) => {
      setExitConfirmOpen(false);
      void window.rayleaLauncher.closeConfirmResponse({ action, setAsDefault });
    },
    [],
  );

  const handleExitConfirmClose = useCallback(() => {
    setExitConfirmOpen(false);
  }, []);

  useEffect(() => {
    const unsubscribe = window.rayleaLauncher.onShowExitConfirm(() => {
      setExitConfirmOpen(true);
    });
    return unsubscribe;
  }, []);

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
    if (presentation.state === "setup_required") {
      return runAction("open-web", () => window.rayleaLauncher.openWebUi());
    }
    return runAction("start", () => window.rayleaLauncher.start());
  }, [presentation.state, runAction, snapshot.launcher.processOwnership]);

  const handleNavigate = useCallback(
    (section: SectionId) => {
      if (section === activeSection) {
        return;
      }
      startTransition(() => {
        setActiveSection(section);
      });
    },
    [activeSection, setActiveSection],
  );

  if (initializing) {
    return (
      <div className="launcher-loading-shell">
        <div className="launcher-loading-shell__eyebrow">Raylea 启动器</div>
        <h1 className="launcher-loading-shell__title">正在准备启动器</h1>
        <p className="launcher-loading-shell__detail">正在读取安装设置并检查本地服务状态。</p>
      </div>
    );
  }

  return (
    <>
    <AppShell
      snapshot={snapshot}
      activeSection={activeSection}
      renderedSection={renderedSection}
      sectionTransitionState={sectionTransitionState}
      platformLabel={platformLabel}
      settingsDraft={settingsDraft}
      resolvedSettings={editingSettings ? previewResolvedSettings : snapshot.launcher.resolvedSettings}
      editingSettings={editingSettings}
      diagnosticsSummary={diagnosticsSummary}
      busyAction={busyAction}
      controlsDisabled={controlsDisabled}
      isMaximized={isMaximized}
      onNavigate={handleNavigate}
      onRefresh={() => runAction("refresh", () => window.rayleaLauncher.refresh())}
      onStart={handlePrimaryServiceAction}
      onStop={() => runAction("stop", () => window.rayleaLauncher.stop())}
      onOpenWeb={() => runAction("open-web", () => window.rayleaLauncher.openWebUi())}
      onOpenRecoveryTasks={() => runAction("open-web", () => window.rayleaLauncher.openWebUi("/tasks?task_type=recovery.recheck"))}
      onOpenRuntimeTasks={() => runAction("open-web", () => window.rayleaLauncher.openWebUi("/tasks?task_type=runtime.bootstrap"))}
      onOpenRecoveryPlugin={(pluginId: string) => runAction("open-plugin", () => window.rayleaLauncher.openWebUi(`/plugins/${encodeURIComponent(pluginId)}`))}
      onCheckForUpdates={() => runAction("check-updates", () => window.rayleaLauncher.checkForUpdates())}
      onDownloadUpdate={() => runAction("download-update", () => window.rayleaLauncher.downloadUpdate())}
      onInstallDownloadedUpdate={() => runAction("install-update", () => window.rayleaLauncher.installDownloadedUpdate())}
      onOpenRepositoryPage={() => runAction("open-repository-page", () => window.rayleaLauncher.openRepositoryPage())}
      onOpenLogs={() => runAction("open-logs", () => window.rayleaLauncher.openLogsDirectory())}
      onResetAdmin={() => runAction("reset-admin", () => window.rayleaLauncher.resetAdmin())}
      onBeginEdit={handleBeginEdit}
      onCancelEdit={handleCancelEdit}
      onSaveSettings={handleSaveSettings}
      onUpdateInstallationRoot={handleUpdateInstallationRoot}
      onUpdateCloseBehavior={handleUpdateCloseBehavior}
      onUpdateAdvancedOverride={handleUpdateAdvancedOverride}
      onChooseInstallationRoot={() => {
        window.rayleaLauncher.chooseInstallationRoot().then((value: string | null) => {
          if (value) handleUpdateInstallationRoot(value);
        });
      }}
      onChooseServer={() => {
        window.rayleaLauncher.chooseServerExecutable().then((value: string | null) => {
          if (value) handleUpdateAdvancedOverride("serverExecutablePath", value);
        });
      }}
      onChooseConfig={() => {
        window.rayleaLauncher.chooseConfigFile().then((value: string | null) => {
          if (value) handleUpdateAdvancedOverride("configPath", value);
        });
      }}
      onChooseWorkdir={() => {
        window.rayleaLauncher.chooseWorkdir().then((value: string | null) => {
          if (value) handleUpdateAdvancedOverride("workdir", value);
        });
      }}
      onExit={() => window.rayleaLauncher.exitApplication()}
    />
    <ExitConfirmDialog
      open={exitConfirmOpen}
      onClose={handleExitConfirmClose}
      onConfirm={handleExitConfirm}
    />
  </>);
}
