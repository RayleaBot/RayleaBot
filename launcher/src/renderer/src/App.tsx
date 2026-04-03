import { startTransition, useState, useEffect, useMemo, useCallback, useDeferredValue, useRef } from "react";
import type {
  LauncherAdvancedOverrides,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";
import { AppShell } from "./AppShell";

type SectionId = "status" | "environment" | "diagnostics" | "settings";
type SectionTransitionState = "idle" | "exiting" | "entering";

const SECTION_EXIT_MS = 90;
const SECTION_ENTER_MS = 180;

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
  serviceOwnership: "none",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
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
  const [renderedSection, setRenderedSection] = useState<SectionId>("status");
  const [sectionTransitionState, setSectionTransitionState] = useState<SectionTransitionState>("idle");
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [editingSettings, setEditingSettings] = useState(false);
  const [initializing, setInitializing] = useState(true);
  const [snapshot, setSnapshot] = useState<LauncherSnapshot>(initialSnapshot);
  const [platformLabel, setPlatformLabel] = useState("");
  // Local draft: only used during settings editing; mirrors what Vue's settingsDraft ref did
  const [editingDraft, setEditingDraft] = useState<LauncherSettings | null>(null);
  const [isMaximized, setIsMaximized] = useState(false);
  const [previewResolvedSettings, setPreviewResolvedSettings] = useState(initialSnapshot.resolvedSettings);
  const sectionExitTimerRef = useRef<number | null>(null);
  const sectionEnterTimerRef = useRef<number | null>(null);

  // settingsDraft = active editing draft when editing, else current settings from snapshot
  const settingsDraft = editingDraft ?? snapshot.settings;
  const deferredSettingsDraft = useDeferredValue(settingsDraft);

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
      "恢复兼容性：",
      snapshot.recoverySummary
        ? `${snapshot.recoverySummary.status} / ${snapshot.recoverySummary.operation} / ${snapshot.recoverySummary.phase}`
        : "当前没有恢复摘要。",
      "最近错误输出：",
      recentErrors,
    ].join("\n");
  }, [snapshot]);

  const clearSectionTransitionTimers = useCallback(() => {
    if (sectionExitTimerRef.current !== null) {
      window.clearTimeout(sectionExitTimerRef.current);
      sectionExitTimerRef.current = null;
    }

    if (sectionEnterTimerRef.current !== null) {
      window.clearTimeout(sectionEnterTimerRef.current);
      sectionEnterTimerRef.current = null;
    }
  }, []);

  // Sync snapshot updates into editing draft when not actively editing
  useEffect(() => {
    if (!editingSettings && editingDraft !== null) {
      setEditingDraft(null);
    }
  }, [snapshot.settings, editingSettings, editingDraft]);

  useEffect(() => {
    if (!editingSettings) {
      setPreviewResolvedSettings(snapshot.resolvedSettings);
      return;
    }

    let cancelled = false;
    window.rayleaLauncher.previewResolvedSettings(deferredSettingsDraft)
      .then((resolvedSettings) => {
        if (!cancelled) {
          setPreviewResolvedSettings(resolvedSettings);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPreviewResolvedSettings(snapshot.resolvedSettings);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [editingSettings, deferredSettingsDraft, snapshot.resolvedSettings]);

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

  useEffect(() => clearSectionTransitionTimers, [clearSectionTransitionTimers]);

  useEffect(() => {
    if (activeSection === renderedSection) {
      return;
    }

    if (sectionExitTimerRef.current !== null) {
      window.clearTimeout(sectionExitTimerRef.current);
      sectionExitTimerRef.current = null;
    }

    setSectionTransitionState("exiting");

    sectionExitTimerRef.current = window.setTimeout(() => {
      setRenderedSection(activeSection);
      setSectionTransitionState("entering");
      sectionExitTimerRef.current = null;
    }, SECTION_EXIT_MS);

    return () => {
      if (sectionExitTimerRef.current !== null) {
        window.clearTimeout(sectionExitTimerRef.current);
        sectionExitTimerRef.current = null;
      }
    };
  }, [activeSection, renderedSection]);

  useEffect(() => {
    if (sectionTransitionState !== "entering") {
      return;
    }

    if (sectionEnterTimerRef.current !== null) {
      window.clearTimeout(sectionEnterTimerRef.current);
      sectionEnterTimerRef.current = null;
    }

    sectionEnterTimerRef.current = window.setTimeout(() => {
      setSectionTransitionState("idle");
      sectionEnterTimerRef.current = null;
    }, SECTION_ENTER_MS);

    return () => {
      if (sectionEnterTimerRef.current !== null) {
        window.clearTimeout(sectionEnterTimerRef.current);
        sectionEnterTimerRef.current = null;
      }
    };
  }, [sectionTransitionState]);

  useEffect(() => {
    let cancelled = false;
    window.rayleaLauncher
      .getPlatform()
      .then((value) => {
        if (!cancelled) {
          setPlatformLabel(value);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setPlatformLabel("");
        }
      });

    return () => {
      cancelled = true;
    };
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
    [activeSection],
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
      onRecoveryRecheck={() =>
        runAction("recovery-recheck", () => window.rayleaLauncher.createRecoveryRecheck())
      }
      onRuntimeBootstrap={() =>
        runAction("runtime-bootstrap", () => window.rayleaLauncher.createRuntimeBootstrap(["chromium"]))
      }
      onOpenRecoveryPlugin={(pluginId: string) =>
        runAction("open-plugin", () => window.rayleaLauncher.openWebUi(`/plugins/${encodeURIComponent(pluginId)}`))
      }
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
