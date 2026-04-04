import { startTransition, useCallback, useMemo, useState } from "react";
import type { LauncherAdvancedOverrides, LauncherSettings } from "@shared/launcher-models";

import { AppShell } from "./AppShell";
import { describeLauncherError, type SectionId } from "./AppState.shared";
import { useLauncherInitialization } from "./useLauncherInitialization";
import { useLauncherSectionState } from "./useLauncherSectionState";
import { useLauncherSettingsState } from "./useLauncherSettingsState";

export function App() {
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
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
          lastError: describeLauncherError(error, "启动器操作失败。"),
          serviceDetail:
            action === "restart"
              ? "重启服务失败。"
              : action === "start"
                ? "启动服务失败。"
                : prev.serviceDetail,
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
      setEditingDraft((prev) => update(prev ?? snapshot.settings));
    },
    [editingSettings, setEditingDraft, snapshot.settings],
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
      ...snapshot.settings,
      advancedOverrides: snapshot.settings.advancedOverrides
        ? { ...snapshot.settings.advancedOverrides }
        : undefined,
    });
    setEditingSettings(true);
  }, [setEditingDraft, snapshot.settings]);

  const handleCancelEdit = useCallback(() => {
    setEditingSettings(false);
    setEditingDraft(null);
  }, [setEditingDraft]);

  const handlePrimaryServiceAction = useCallback(() => {
    const isManagedRunnable =
      (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
      && snapshot.serviceOwnership === "launcher_managed";

    if (isManagedRunnable) {
      return runAction("restart", async () => {
        await window.rayleaLauncher.stop();
        await window.rayleaLauncher.start();
      });
    }
    if (snapshot.serviceState === "setup_required") {
      return runAction("open-web", () => window.rayleaLauncher.openWebUi());
    }
    return runAction("start", () => window.rayleaLauncher.start());
  }, [runAction, snapshot.serviceOwnership, snapshot.serviceState]);

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
        <div className="launcher-loading-shell__eyebrow">RayleaLauncher</div>
        <h1 className="launcher-loading-shell__title">正在准备启动器</h1>
        <p className="launcher-loading-shell__detail">正在读取安装设置并检查本地服务状态。</p>
      </div>
    );
  }

  return (
    <AppShell
      snapshot={snapshot}
      activeSection={activeSection}
      renderedSection={renderedSection}
      sectionTransitionState={sectionTransitionState}
      platformLabel={platformLabel}
      settingsDraft={settingsDraft}
      resolvedSettings={editingSettings ? previewResolvedSettings : snapshot.resolvedSettings}
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
      onRecoveryRecheck={() => runAction("recovery-recheck", () => window.rayleaLauncher.createRecoveryRecheck())}
      onRuntimeBootstrap={() => runAction("runtime-bootstrap", () => window.rayleaLauncher.createRuntimeBootstrap())}
      onOpenRecoveryPlugin={(pluginId: string) => runAction("open-plugin", () => window.rayleaLauncher.openWebUi(`/plugins/${encodeURIComponent(pluginId)}`))}
      onOpenReleasePage={() => runAction("open-release-page", () => window.rayleaLauncher.openReleasePage())}
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
  );
}
