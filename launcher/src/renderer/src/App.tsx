import { useState, useEffect, useMemo, useCallback } from "react";
import type {
  LauncherAdvancedOverrides,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";
import { AppShell } from "./AppShell";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

const initialSnapshot: LauncherSnapshot = {
  settings: {
    installationRoot: "",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "",
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
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
};

export function App() {
  const [activeSection, setActiveSection] = useState<SectionId>("status");
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
  const [initializing, setInitializing] = useState(true);
  const [snapshot, setSnapshot] = useState<LauncherSnapshot>(initialSnapshot);
  // Local draft: only used during settings editing; mirrors what Vue's settingsDraft ref did
  const [editingDraft, setEditingDraft] = useState<LauncherSettings | null>(null);
  const [isMaximized, setIsMaximized] = useState(false);

  // settingsDraft = active editing draft when editing, else current settings from snapshot
  const settingsDraft = editingDraft ?? snapshot.settings;

  const controlsDisabled = useMemo(
    () => initializing || busyAction === "initialize",
    [initializing, busyAction],
  );

  const diagnosticsSummary = useMemo(() => {
    const checks = snapshot.environmentChecks
      .map(
        (item) =>
          `- ${item.title}：${item.summary}（${item.detail}${item.remediation ? `；${item.remediation}` : ""}）`,
      )
      .join("\n");
    const recentErrors =
      snapshot.recentStderr.length
        ? snapshot.recentStderr.join("\n")
        : "当前没有新的错误输出。";
    return [
      `服务状态：${snapshot.serviceDetail}`,
      `服务入口：${snapshot.endpoint.baseUrl}`,
      `安装目录：${snapshot.settings.installationRoot || "未设置"}`,
      `服务端：${snapshot.resolvedSettings.serverExecutablePath || "未设置"}`,
      `配置文件：${snapshot.resolvedSettings.configPath || "未设置"}`,
      `运行目录：${snapshot.resolvedSettings.workdir || "未设置"}`,
      "环境检查：",
      checks || "- 当前没有检查项。",
      "最近错误输出：",
      recentErrors,
    ].join("\n");
  }, [snapshot]);

  // Sync snapshot updates into editing draft when not actively editing
  useEffect(() => {
    if (!editingSettings && editingDraft !== null) {
      setEditingDraft(null);
    }
  }, [snapshot.settings, editingSettings, editingDraft]);

  useEffect(() => {
    const unsub = window.rayleaLauncher.onSnapshot((next) => {
      setSnapshot(next);
    });
    setBusyAction("initialize");
    window.rayleaLauncher
      .initialize()
      .then(async () => {
        const snap = await window.rayleaLauncher.getSnapshot();
        setSnapshot(snap);
      })
      .catch((error: unknown) => {
        setSnapshot((prev) => ({
          ...prev,
          lastError:
            error instanceof Error && error.message
              ? error.message
              : "启动器初始化失败。",
          serviceDetail: "启动器初始化失败。",
        }));
      })
      .finally(() => {
        setBusyAction(null);
        setInitializing(false);
      });

    return unsub;
  }, []);

  useEffect(() => {
    window.rayleaLauncher.isMaximized().then(setIsMaximized);
    const unsub = window.rayleaLauncher.onMaximizedChange(setIsMaximized);
    return unsub;
  }, []);

  const describeError = useCallback((error: unknown, fallback: string) => {
    if (error instanceof Error && error.message) {
      return error.message;
    }
    return fallback;
  }, []);

  const runAction = useCallback(
    async (action: string, task: () => Promise<void>) => {
      setBusyAction(action);
      try {
        await task();
      } catch (error) {
        setSnapshot((prev) => ({
          ...prev,
          lastError: describeError(error, "启动器操作失败。"),
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
    [describeError],
  );

  const handleUpdateSettings = useCallback(
    (update: (current: LauncherSettings) => LauncherSettings) => {
      if (!editingSettings) return;
      setEditingDraft((prev) => update(prev ?? snapshot.settings));
    },
    [editingSettings, snapshot.settings],
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
  }, [runAction, editingDraft]);

  const handleBeginEdit = useCallback(() => {
    setEditingDraft({
      ...snapshot.settings,
      advancedOverrides: snapshot.settings.advancedOverrides
        ? { ...snapshot.settings.advancedOverrides }
        : undefined,
    });
    setEditingSettings(true);
  }, [snapshot.settings]);

  const handleCancelEdit = useCallback(() => {
    setEditingSettings(false);
    setEditingDraft(null);
  }, []);

  const handlePrimaryServiceAction = useCallback(() => {
    if (snapshot.serviceState === "ready") {
      return runAction("restart", async () => {
        await window.rayleaLauncher.stop();
        await window.rayleaLauncher.start();
      });
    }
    return runAction("start", () => window.rayleaLauncher.start());
  }, [runAction, snapshot.serviceState]);

  return (
    <AppShell
      snapshot={snapshot}
      activeSection={activeSection}
      settingsDraft={settingsDraft}
      resolvedSettings={snapshot.resolvedSettings}
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
      onOpenReleasePage={() =>
        runAction("open-release-page", () =>
          window.rayleaLauncher.openReleasePage(),
        )
      }
      onOpenLogs={() =>
        runAction("open-logs", () =>
          window.rayleaLauncher.openLogsDirectory(),
        )
      }
      onResetAdmin={() =>
        runAction("reset-admin", () =>
          window.rayleaLauncher.resetAdmin(),
        )
      }
      onBeginEdit={handleBeginEdit}
      onCancelEdit={handleCancelEdit}
      onSaveSettings={handleSaveSettings}
      onUpdateInstallationRoot={handleUpdateInstallationRoot}
      onUpdateCloseBehavior={handleUpdateCloseBehavior}
      onUpdateAdvancedOverride={handleUpdateAdvancedOverride}
      onChooseInstallationRoot={() => {
        window.rayleaLauncher
          .chooseInstallationRoot()
          .then((value: string | null) => {
            if (value) {
              handleUpdateInstallationRoot(value);
            }
          });
      }}
      onChooseServer={() => {
        window.rayleaLauncher
          .chooseServerExecutable()
          .then((value: string | null) => {
            if (value) {
              handleUpdateAdvancedOverride("serverExecutablePath", value);
            }
          });
      }}
      onChooseConfig={() => {
        window.rayleaLauncher
          .chooseConfigFile()
          .then((value: string | null) => {
            if (value) {
              handleUpdateAdvancedOverride("configPath", value);
            }
          });
      }}
      onChooseWorkdir={() => {
        window.rayleaLauncher
          .chooseWorkdir()
          .then((value: string | null) => {
            if (value) {
              handleUpdateAdvancedOverride("workdir", value);
            }
          });
      }}
      onExit={() => window.rayleaLauncher.exitApplication()}
    />
  );
}
