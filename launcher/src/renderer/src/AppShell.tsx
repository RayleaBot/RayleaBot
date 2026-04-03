import { Button, Input, PresenceBadge, Radio, RadioGroup, Text } from "@fluentui/react-components";
import type { PresenceBadgeStatus } from "@fluentui/react-components";
import {
  ArrowClockwise20Regular,
  CheckmarkCircle20Filled,
  Dismiss20Regular,
  DismissCircle20Filled,
  DocumentText20Filled,
  FolderOpen20Filled,
  Globe20Filled,
  HeartPulse20Filled,
  Play20Filled,
  Settings20Filled,
  Square20Regular,
  SquareMultiple20Regular,
  Status20Filled,
  Stop20Filled,
  Subtract20Regular,
  Warning20Filled,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useRef, useState } from "react";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherServiceState,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";
type SectionTransitionState = "idle" | "exiting" | "entering";

const serviceStateConfig: Record<LauncherServiceState, { status: PresenceBadgeStatus; label: string }> = {
  stopped: { status: "offline", label: "已停止" },
  starting: { status: "busy", label: "启动中" },
  running: { status: "available", label: "运行中" },
  degraded: { status: "busy", label: "运行条件受限" },
  setup_required: { status: "blocked", label: "需要设置" },
  stopping: { status: "busy", label: "停止中" },
  failed: { status: "blocked", label: "启动失败" },
};

const severityConfig = {
  error: { label: "阻塞", icon: <DismissCircle20Filled /> },
  warning: { label: "警告", icon: <Warning20Filled /> },
  ok: { label: "正常", icon: <CheckmarkCircle20Filled /> },
};

const sections = [
  { id: "status" as SectionId, title: "运行状态", icon: <Status20Filled /> },
  { id: "environment" as SectionId, title: "环境检查", icon: <HeartPulse20Filled /> },
  { id: "diagnostics" as SectionId, title: "日志诊断", icon: <DocumentText20Filled /> },
  { id: "settings" as SectionId, title: "偏好设置", icon: <Settings20Filled /> },
];

const sectionContent = {
  status: {
    eyebrow: "Service Console",
    title: "运行状态",
    detail: "查看当前服务状态，并直接处理启动、停止、管理和恢复动作。",
  },
  environment: {
    eyebrow: "Environment Review",
    title: "环境检查",
    detail: "汇总本地运行条件、恢复兼容性和受控运行时准备情况。",
  },
  diagnostics: {
    eyebrow: "Diagnostics",
    title: "日志诊断",
    detail: "集中查看系统状态摘要与最近异常输出。",
  },
  settings: {
    eyebrow: "Launcher Settings",
    title: "偏好设置",
    detail: "管理安装路径、关闭行为和本地维护操作。",
  },
} satisfies Record<SectionId, { eyebrow: string; title: string; detail: string }>;

const severityOrder = {
  error: 0,
  warning: 1,
  ok: 2,
} satisfies Record<"error" | "warning" | "ok", number>;

const busyActionLabels: Record<string, string> = {
  initialize: "正在准备启动器",
  refresh: "正在刷新状态",
  start: "正在启动服务",
  stop: "正在停止服务",
  restart: "正在重启服务",
  save: "正在保存设置",
  "open-web": "正在打开管理面板",
  "open-release-page": "正在打开版本页面",
  "open-logs": "正在打开日志目录",
  "reset-admin": "正在重置本地凭据",
  "recovery-recheck": "正在复核恢复兼容性",
  "runtime-bootstrap": "正在准备受控运行时",
  "open-plugin": "正在打开插件详情",
};

const closeBehaviorOptions: Array<{
  value: LauncherSettings["closeBehavior"];
  label: string;
  detail: string;
}> = [
  { value: "ask_every_time", label: "每次询问", detail: "每次关闭窗口时都显示确认选项。" },
  { value: "hide_to_tray", label: "系统托盘", detail: "关闭主窗口后保留托盘入口和后台状态。" },
  { value: "exit_application", label: "完全退出", detail: "直接结束启动器窗口与托盘进程。" },
];

type AppShellProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
  renderedSection: SectionId;
  sectionTransitionState: SectionTransitionState;
  platformLabel?: string;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  diagnosticsSummary: string;
  busyAction: string | null;
  controlsDisabled: boolean;
  isMaximized: boolean;
  onNavigate: (section: SectionId) => void;
  onRefresh: () => void;
  onStart: () => void;
  onStop: () => void;
  onOpenWeb: () => void;
  onRecoveryRecheck: () => void;
  onRuntimeBootstrap: () => void;
  onOpenRecoveryPlugin: (pluginId: string) => void;
  onOpenReleasePage: () => void;
  onOpenLogs: () => void;
  onResetAdmin: () => void;
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
  onUpdateInstallationRoot: (value: string) => void;
  onUpdateCloseBehavior: (value: LauncherSettings["closeBehavior"]) => void;
  onUpdateAdvancedOverride: (key: keyof LauncherAdvancedOverrides, value: string) => void;
  onChooseInstallationRoot: () => void;
  onChooseServer: () => void;
  onChooseConfig: () => void;
  onChooseWorkdir: () => void;
  onExit: () => void;
};

function statusSummary(state: LauncherServiceState): string {
  switch (state) {
    case "stopped":
      return "已停止";
    case "starting":
      return "正在启动";
    case "running":
      return "正在运行";
    case "degraded":
      return "运行条件受限";
    case "setup_required":
      return "需要设置";
    case "stopping":
      return "正在停止";
    case "failed":
      return "启动失败";
    default:
      return "未知状态";
  }
}

function sortChecks<T extends { severity: "ok" | "warning" | "error"; title: string }>(items: T[]): T[] {
  return [...items].sort((left, right) => {
    const severityGap = severityOrder[left.severity] - severityOrder[right.severity];
    if (severityGap !== 0) {
      return severityGap;
    }

    return left.title.localeCompare(right.title, "zh-CN");
  });
}

export function AppShell({
  snapshot,
  activeSection,
  renderedSection,
  sectionTransitionState,
  platformLabel = "",
  settingsDraft,
  resolvedSettings,
  editingSettings,
  diagnosticsSummary,
  busyAction,
  controlsDisabled,
  isMaximized,
  onNavigate,
  onRefresh,
  onStart,
  onStop,
  onOpenWeb,
  onRecoveryRecheck,
  onRuntimeBootstrap,
  onOpenReleasePage,
  onOpenLogs,
  onResetAdmin,
  onBeginEdit,
  onCancelEdit,
  onSaveSettings,
  onUpdateInstallationRoot,
  onUpdateCloseBehavior,
  onUpdateAdvancedOverride,
  onChooseInstallationRoot,
  onChooseServer,
  onChooseConfig,
  onChooseWorkdir,
  onExit,
}: AppShellProps) {
  const [showAdvancedOverrides, setShowAdvancedOverrides] = useState(false);
  const [statusHighlight, setStatusHighlight] = useState<"none" | "signal" | "alert">("none");
  const [logHighlight, setLogHighlight] = useState<"none" | "fresh">("none");

  const checks = useMemo(() => sortChecks(snapshot.environmentChecks || []), [snapshot.environmentChecks]);
  const groupedChecks = useMemo(
    () => ({
      blocking: checks.filter((item) => item.severity === "error"),
      warnings: checks.filter((item) => item.severity === "warning"),
      ready: checks.filter((item) => item.severity === "ok"),
    }),
    [checks],
  );
  const categorizedChecks = useMemo(() => {
    const corePrefixes = ["server.", "config.", "workdir."];
    const runtimePrefixes = ["deps.", "chromium.", "python.", "nodejs.", "npm."];
    return {
      core: sortChecks(checks.filter((item) => corePrefixes.some((prefix) => item.code.startsWith(prefix)))),
      runtimes: sortChecks(checks.filter((item) => runtimePrefixes.some((prefix) => item.code.startsWith(prefix)))),
      others: sortChecks(
        checks.filter(
          (item) =>
            !corePrefixes.some((prefix) => item.code.startsWith(prefix))
            && !runtimePrefixes.some((prefix) => item.code.startsWith(prefix)),
        ),
      ),
    };
  }, [checks]);

  const hasAdvancedOverrides = Boolean(
    settingsDraft.advancedOverrides?.serverExecutablePath
      || settingsDraft.advancedOverrides?.configPath
      || settingsDraft.advancedOverrides?.workdir,
  );

  useEffect(() => {
    if (hasAdvancedOverrides) {
      setShowAdvancedOverrides(true);
    }
  }, [hasAdvancedOverrides]);

  const previousStatusRef = useRef({
    serviceState: snapshot.serviceState,
    busyAction,
    lastError: snapshot.lastError,
  });
  const previousLogsRef = useRef(snapshot.recentStderr.join("\n"));

  useEffect(() => {
    const previous = previousStatusRef.current;
    const serviceStateChanged = previous.serviceState !== snapshot.serviceState;
    const actionChanged = previous.busyAction !== busyAction && busyAction !== null;
    const errorChanged = previous.lastError !== snapshot.lastError && Boolean(snapshot.lastError);

    previousStatusRef.current = {
      serviceState: snapshot.serviceState,
      busyAction,
      lastError: snapshot.lastError,
    };

    if (!(serviceStateChanged || actionChanged || errorChanged)) {
      return;
    }

    setStatusHighlight(errorChanged ? "alert" : "signal");
    const timeoutId = window.setTimeout(() => {
      setStatusHighlight("none");
    }, 1200);

    return () => window.clearTimeout(timeoutId);
  }, [snapshot.serviceState, snapshot.lastError, busyAction]);

  useEffect(() => {
    const nextLogState = snapshot.recentStderr.join("\n");
    const hadLogs = previousLogsRef.current.length > 0;
    const hasLogsNow = nextLogState.length > 0;
    previousLogsRef.current = nextLogState;

    if (!hasLogsNow || hadLogs === hasLogsNow) {
      return;
    }

    setLogHighlight("fresh");
    const timeoutId = window.setTimeout(() => {
      setLogHighlight("none");
    }, 1600);

    return () => window.clearTimeout(timeoutId);
  }, [snapshot.recentStderr]);

  const trayStatus = useMemo(() => statusSummary(snapshot.serviceState), [snapshot.serviceState]);
  const isManagedRunnable =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && snapshot.serviceOwnership === "launcher_managed";
  const isExternalRunnable =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && snapshot.serviceOwnership === "external";
  const canOpenWebUi =
    snapshot.serviceState === "running"
    || snapshot.serviceState === "degraded"
    || snapshot.serviceState === "setup_required";
  const canRunRecoveryActions =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && !controlsDisabled;
  const primaryActionLabel =
    isExternalRunnable
      ? "检测到现有服务"
      : isManagedRunnable
        ? "重启服务"
        : snapshot.serviceState === "setup_required"
          ? "打开初始化"
          : "启动 RayleaBot";
  const startDisabled =
    controlsDisabled
    || busyAction === "start"
    || busyAction === "restart"
    || busyAction === "stop"
    || busyAction === "open-web"
    || isExternalRunnable
    || snapshot.serviceState === "starting"
    || snapshot.serviceState === "stopping";
  const stopDisabled =
    controlsDisabled
    || busyAction === "restart"
    || busyAction === "stop"
    || snapshot.serviceState === "starting"
    || snapshot.serviceState === "stopping"
    || snapshot.serviceOwnership === "none";
  const nonOkChecks = useMemo(() => checks.filter((item) => item.severity !== "ok"), [checks]);
  const primaryEnvironmentIssue = nonOkChecks[0] ?? null;
  const recoveryStatusSummary = snapshot.recoverySummary
    ? `${snapshot.recoverySummary.status} · ${snapshot.recoverySummary.operation}`
    : "当前没有恢复摘要。";
  const hasRecentStderr = snapshot.recentStderr.length > 0;
  const sectionMeta = sectionContent[renderedSection];
  const busyLabel = busyAction ? (busyActionLabels[busyAction] ?? "正在执行操作") : "";
  const statusAlert = snapshot.lastError ? "error" : nonOkChecks.length > 0 ? "warning" : "none";
  const logAlert = hasRecentStderr ? "error" : "none";
  const statusReasonLabel =
    snapshot.serviceState === "degraded" || snapshot.serviceState === "setup_required"
      ? "当前限制"
      : "运行说明";
  const statusReasonText =
    snapshot.serviceState === "degraded" || snapshot.serviceState === "setup_required"
      ? snapshot.serviceDetail
      : primaryEnvironmentIssue
        ? `${primaryEnvironmentIssue.title}：${primaryEnvironmentIssue.summary}`
        : snapshot.serviceDetail;
  const statusGuidanceLabel = snapshot.lastError ? "异常提示" : primaryEnvironmentIssue ? "处理提示" : "异常提示";
  const statusGuidanceText =
    snapshot.lastError
    || primaryEnvironmentIssue?.remediation
    || primaryEnvironmentIssue?.detail
    || "当前没有阻塞异常。";
  const environmentReadiness = groupedChecks.blocking.length > 0
    ? { label: "需要处理", detail: "存在阻塞项，启动前需要先解决。" }
    : groupedChecks.warnings.length > 0
      ? { label: "可继续，但有警告", detail: "核心能力可用，建议先检查告警项。" }
      : { label: "可以启动", detail: "当前未发现阻塞或告警项。" };
  const settingsSurfaceTag = editingSettings ? "当前草稿" : "当前值";
  const sectionHeaderBadges =
    renderedSection === "status" ? (
      <>
        <span className="glass-chip glass-chip--accent">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</span>
        {busyAction && <span className="glass-chip glass-chip--muted">{busyLabel}</span>}
      </>
    ) : renderedSection === "environment" ? (
      <>
        <span className="glass-chip glass-chip--accent">{environmentReadiness.label}</span>
        <span className="glass-chip glass-chip--muted">{checks.length} 项检查</span>
      </>
    ) : renderedSection === "diagnostics" ? (
      <>
        <span className={`glass-chip ${hasRecentStderr ? "glass-chip--danger" : "glass-chip--muted"}`}>
          {hasRecentStderr ? "检测到异常输出" : "当前安静"}
        </span>
        <span className="glass-chip glass-chip--muted">{snapshot.endpoint.baseUrl}</span>
      </>
    ) : (
      <span className="glass-chip glass-chip--accent">{editingSettings ? "草稿编辑中" : "已加载当前配置"}</span>
    );
  const sectionHeaderActions =
    renderedSection === "status" ? (
      <Button
        appearance="transparent"
        size="small"
        onClick={onRefresh}
        icon={<ArrowClockwise20Regular />}
        className="frost-button frost-button--ghost"
        disabled={controlsDisabled}
      >
        刷新状态
      </Button>
    ) : renderedSection === "environment" ? (
      <>
        <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onRecoveryRecheck} disabled={!canRunRecoveryActions}>重新检查</Button>
        <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行时</Button>
      </>
    ) : renderedSection === "diagnostics" ? (
      <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>
    ) : editingSettings ? (
      <>
        <Button appearance="transparent" size="small" className="frost-button frost-button--ghost" onClick={onCancelEdit}>放弃</Button>
        <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onSaveSettings}>保存</Button>
      </>
    ) : (
      <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onBeginEdit}>编辑配置</Button>
    );

  return (
    <div className="app-shell">
      <div className="window-drag-handle">
        <div className="window-title">RAYLEALAUNCHER</div>
        <div className="window-controls">
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.minimize()} title="最小化"><Subtract20Regular /></button>
          <button className="window-control-btn" onClick={() => window.rayleaLauncher.maximize()} title={isMaximized ? "还原" : "最大化"}>{isMaximized ? <SquareMultiple20Regular /> : <Square20Regular />}</button>
          <button className="window-control-btn danger" onClick={() => window.rayleaLauncher.close()} title="关闭"><Dismiss20Regular /></button>
        </div>
      </div>

      <aside className="shell-sidebar">
        <div className="brand-card glass-panel">
          <div className="brand-eyebrow">RayleaBot</div>
          <div className="brand-headline">
            <h1>RayleaLauncher</h1>
            {snapshot.releaseCheck.currentVersion && <span className="glass-chip">v{snapshot.releaseCheck.currentVersion}</span>}
          </div>
        </div>

        <nav className="section-nav">
          {sections.map((section) => (
            <button
              key={section.id}
              className={`nav-item${activeSection === section.id ? " active" : ""}`}
              onClick={() => onNavigate(section.id)}
              aria-current={activeSection === section.id ? "page" : undefined}
            >
              <span className="nav-item__icon">{section.icon}</span>
              <span className="nav-item__label">{section.title}</span>
            </button>
          ))}
        </nav>

        <div className="sidebar-footer glass-panel glass-panel--subtle">
          <div className="sidebar-footer__group">
            <Text size={100} className="eyebrow-text">LAUNCHER STATUS</Text>
            <Text weight="bold" className="sidebar-footer__status">{trayStatus.toUpperCase()}</Text>
          </div>
          <div className="sidebar-footer__group">
            <Text size={100} className="eyebrow-text">API ENDPOINT</Text>
            <Text size={100} className="sidebar-footer__endpoint">{snapshot.endpoint.baseUrl}</Text>
          </div>
          <Button appearance="transparent" size="small" onClick={onRefresh} icon={<ArrowClockwise20Regular />} className="frost-button frost-button--ghost frost-button--inline">刷新状态</Button>
        </div>
      </aside>

      <main className={`shell-main ${renderedSection === "environment" ? "active-environment" : ""}`} data-active-section={activeSection} data-rendered-section={renderedSection} data-transition={sectionTransitionState}>
        <div className="section-shell" data-section={renderedSection} data-transition={sectionTransitionState}>
          <header className="section-header glass-panel glass-panel--subtle">
            <div className="section-header__copy">
              <div className="section-header__eyebrow">{sectionMeta.eyebrow}</div>
              <div className="section-header__title-row">
                <h2 className="section-header__title">{sectionMeta.title}</h2>
                <div className="section-header__badges">{sectionHeaderBadges}</div>
              </div>
              <p className="section-header__detail">{sectionMeta.detail}</p>
            </div>
            <div className="section-header__actions">{sectionHeaderActions}</div>
          </header>

          <div className="section-shell__content">
            {renderedSection === "status" && (
              <div className="status-homepage status-view-flow" data-state={snapshot.serviceState} data-busy={busyAction ?? "idle"} data-alert={statusAlert}>
                <section className="status-hero glass-panel hero-card hero-card--fancy" data-highlight={statusHighlight}>
                  <div className="status-hero__body hero-copy">
                    <div className="brand-eyebrow brand-eyebrow--faded">Service Control</div>
                    <div className="hero-status-row hero-status-row--main">
                      <div className="hero-status-indicator">
                        <PresenceBadge status={serviceStateConfig[snapshot.serviceState]?.status ?? "unknown"} size="extra-large" />
                        <div className={`hero-status-glow hero-status-glow--${serviceStateConfig[snapshot.serviceState]?.status}`} />
                      </div>
                      <div className="hero-status-content">
                        <Text weight="bold" size={800} className="hero-status-text hero-status-text--huge">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</Text>
                        <Text size={300} className="hero-detail hero-detail--bright">{snapshot.serviceDetail}</Text>
                      </div>
                    </div>

                    <div className="hero-context-grid">
                      <div className="hero-context-card">
                        <span className="hero-context-card__label">当前状态</span>
                        <span className="hero-context-card__value">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</span>
                      </div>
                      <div className="hero-context-card">
                        <span className="hero-context-card__label">{statusReasonLabel}</span>
                        <span className="hero-context-card__value">{statusReasonText}</span>
                      </div>
                      <div className={`hero-context-card ${snapshot.lastError || primaryEnvironmentIssue ? "hero-context-card--alert" : ""}`}>
                        <span className="hero-context-card__label">{statusGuidanceLabel}</span>
                        <span className="hero-context-card__value">{statusGuidanceText}</span>
                      </div>
                    </div>
                  </div>

                  <div className="status-hero__actions hero-actions hero-actions--premium">
                    <div className="status-hero__primary-action">
                      <Button appearance="transparent" size="large" className="frost-button frost-button--primary status-action status-action--primary" onClick={onStart} disabled={startDisabled} icon={<Play20Filled />}>
                        <span className="button-text-large">{primaryActionLabel}</span>
                      </Button>
                    </div>
                    <div className="status-hero__secondary-actions hero-actions-row">
                      <Button appearance="transparent" size="large" className="frost-button frost-button--secondary status-action" onClick={onStop} disabled={stopDisabled} icon={<Stop20Filled />}>停止服务</Button>
                      <Button appearance="transparent" size="large" className="frost-button frost-button--secondary status-action" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Filled />}>管理面板</Button>
                    </div>
                    <div className="status-action-feedback" data-busy={busyAction ? "true" : "false"}>
                      <span className="status-action-feedback__dot" aria-hidden="true"></span>
                      <span>{busyLabel || "当前没有进行中的操作。"}</span>
                    </div>
                  </div>
                </section>

                <div className="status-summary-grid status-grid">
                  <div className="status-summary-main status-main-column">
                    <article className="panel glass-panel panel--interactive">
                      <div className="brand-eyebrow">核心参数</div>
                      <div className="status-list status-list--grid-modern">
                        <div className="status-item-modern">
                          <div className="status-item-modern__icon"><Status20Filled /></div>
                          <div className="status-item-modern__content">
                            <span className="status-label">进程 ID</span>
                            <code className="status-value status-value--highlight">{snapshot.processId ?? "—"}</code>
                          </div>
                        </div>
                        <div className="status-item-modern">
                          <div className="status-item-modern__icon"><Globe20Filled /></div>
                          <div className="status-item-modern__content">
                            <span className="status-label">本地端点</span>
                            <span className="status-value mono">{snapshot.endpoint.baseUrl}</span>
                          </div>
                        </div>
                        <div className="status-item-modern status-item-modern--full">
                          <div className="status-item-modern__icon"><FolderOpen20Filled /></div>
                          <div className="status-item-modern__content">
                            <span className="status-label">安装目录</span>
                            <span className="status-value mono" title={snapshot.settings.installationRoot}>{snapshot.settings.installationRoot || "—"}</span>
                          </div>
                        </div>
                        <div className="status-item-modern status-item-modern--full">
                          <div className="status-item-modern__icon"><DocumentText20Filled /></div>
                          <div className="status-item-modern__content">
                            <span className="status-label">运行目录</span>
                            <span className="status-value mono" title={resolvedSettings.workdir}>{resolvedSettings.workdir || "—"}</span>
                          </div>
                        </div>
                      </div>
                    </article>
                  </div>

                  <aside className="status-summary-rail status-side-column">
                    {nonOkChecks.length > 0 && (
                      <div className="checks-stack checks-stack--side panel glass-panel glass-panel--subtle panel--side">
                        <div className="brand-eyebrow brand-eyebrow--tight">环境预警</div>
                        {nonOkChecks.map((item) => (
                          <div key={item.code} className={`check-item-mini check-item-mini--${item.severity}`}>
                            <div className="check-item-mini__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
                            <div className="check-item-mini__content">
                              <Text weight="bold" size={200}>{item.title}</Text>
                              <Text size={100} className="panel-muted">
                                {item.code === "os.long_paths_unknown" && item.severity === "warning"
                                  ? "无法确认长路径支持状态。若资源展开遇到限制，请手动检查系统长路径设置。"
                                  : item.summary}
                              </Text>
                              <Text size={100} className="panel-muted">{severityConfig[item.severity as keyof typeof severityConfig]?.label}</Text>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}

                    <article className="panel glass-panel glass-panel--subtle panel--side">
                      <div className="brand-eyebrow brand-eyebrow--tight">恢复兼容性</div>
                      <Text size={200} className="panel-muted">{recoveryStatusSummary}</Text>
                      <div className="side-actions-stack">
                        <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRecoveryRecheck} disabled={!canRunRecoveryActions}>重新检查</Button>
                        <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行时</Button>
                      </div>
                    </article>

                    <article className="panel glass-panel glass-panel--subtle panel--side">
                      <div className="brand-eyebrow brand-eyebrow--tight">版本监控</div>
                      <div className="version-status">
                        <Text size={200} className="panel-muted">{snapshot.releaseCheck.summary}</Text>
                      </div>
                      <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block frost-button--outline" onClick={onOpenReleasePage}>检查更新</Button>
                    </article>
                  </aside>
                </div>

                <article className="status-log-panel panel glass-panel" data-alert={logAlert} data-highlight={logHighlight}>
                  <div className="panel-header-row">
                    <div className="brand-eyebrow">异常输出监控</div>
                    <span className={`status-log-indicator status-log-indicator--${hasRecentStderr ? "alert" : "quiet"}`}>
                      {hasRecentStderr ? "已检测到异常输出" : "当前无新异常"}
                    </span>
                  </div>
                  {hasRecentStderr ? (
                    <pre className="log-surface status-log-surface--modern">{snapshot.recentStderr.join("\n")}</pre>
                  ) : (
                    <div className="log-empty-state">
                      <div className="log-empty-state__title">当前没有新的异常日志</div>
                      <Text size={200} className="panel-muted">服务输出保持安静，完整日志仍可随时打开。</Text>
                    </div>
                  )}
                  <div className="panel-footer-actions">
                    <Button appearance="transparent" size="small" className="frost-button frost-button--ghost-bright" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>
                  </div>
                </article>
              </div>
            )}

            {renderedSection === "environment" && (
              <div className="env-details-flow">
                <article className="panel glass-panel env-overview-card">
                  <div className="brand-eyebrow">运行环境概览</div>
                  <div className="env-overview-strip">
                    <div className="env-overview-card__lead">
                      <span className="env-overview-card__label">当前结论</span>
                      <strong className="env-overview-card__title">{environmentReadiness.label}</strong>
                      <Text size={200} className="panel-muted">{environmentReadiness.detail}</Text>
                    </div>
                    <div className="env-overview-metrics">
                      <div className="metric-card metric-card--error"><Text size={100} block className="metric-label">阻塞项</Text><Text size={600} weight="bold">{groupedChecks.blocking.length}</Text></div>
                      <div className="metric-card metric-card--warning"><Text size={100} block className="metric-label">警告项</Text><Text size={600} weight="bold">{groupedChecks.warnings.length}</Text></div>
                      <div className="metric-card metric-card--ok"><Text size={100} block className="metric-label">正常项</Text><Text size={600} weight="bold">{groupedChecks.ready.length}</Text></div>
                      <div className="metric-card"><Text size={100} block className="metric-label">平台架构</Text><Text size={300} weight="bold">{platformLabel || "—"}</Text></div>
                    </div>
                  </div>
                  <div className="status-list env-status-grid">
                    <div className="status-item"><span className="status-label">核心版本</span><span className="status-value">{snapshot.releaseCheck.currentVersion || "—"}</span></div>
                    <div className="status-item"><span className="status-label">安装路径</span><span className="status-value mono">{snapshot.settings.installationRoot || "—"}</span></div>
                    <div className="status-item"><span className="status-label">恢复兼容性</span><span className="status-value">{recoveryStatusSummary}</span></div>
                    <div className="status-item"><span className="status-label">本地端点</span><span className="status-value mono">{snapshot.endpoint.baseUrl}</span></div>
                  </div>
                </article>

                {[{ title: "系统核心", data: categorizedChecks.core }, { title: "受控运行时", data: categorizedChecks.runtimes }, { title: "环境特性", data: categorizedChecks.others }]
                  .filter((section) => section.data.length > 0)
                  .map((section) => (
                    <section key={section.title} className="env-section">
                      <div className="brand-eyebrow brand-eyebrow--section">{section.title}</div>
                      <div className="checks-stack checks-stack--grid">
                        {section.data.map((item) => (
                          <div key={item.code} className={`check-item glass-panel glass-panel--subtle check-item--${item.severity}`}>
                            <div className="check-item__lead">
                              <div className="check-item__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
                              <div className="check-item__copy">
                                <div className="check-item__headline">
                                  <Text weight="bold" size={200}>{item.title}</Text>
                                  <span className={`status-pill status-pill--${item.severity}`}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                                </div>
                                <Text size={100} className="check-item__summary">{item.summary}</Text>
                                {item.detail && item.detail !== item.summary && <Text size={100} className="check-item__detail">{item.detail}</Text>}
                                {item.remediation && (
                                  <div className="check-item__remediation">
                                    <span className="check-item__remediation-label">{item.severity === "ok" ? "离线准备" : "处理方式"}</span>
                                    <Text size={100} className="check-item__remediation-text">{item.remediation}</Text>
                                  </div>
                                )}
                              </div>
                            </div>
                          </div>
                        ))}
                      </div>
                    </section>
                  ))}
              </div>
            )}

            {renderedSection === "diagnostics" && (
              <article className="panel glass-panel diagnostics-panel" data-alert={logAlert}>
                <div className="diagnostics-context-grid">
                  <div className="diagnostics-context-card">
                    <span className="status-label">服务状态</span>
                    <span className="diagnostics-context-card__value">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</span>
                  </div>
                  <div className="diagnostics-context-card">
                    <span className="status-label">本地端点</span>
                    <span className="diagnostics-context-card__value mono">{snapshot.endpoint.baseUrl}</span>
                  </div>
                  <div className={`diagnostics-context-card ${hasRecentStderr ? "diagnostics-context-card--alert" : ""}`}>
                    <span className="status-label">{hasRecentStderr ? "异常状态" : "日志状态"}</span>
                    <span className="diagnostics-context-card__value">
                      {hasRecentStderr ? "最近有异常输出，请结合摘要定位问题。" : "当前没有新的异常输出。"}
                    </span>
                  </div>
                </div>
                <div className={`diagnostics-banner ${hasRecentStderr ? "diagnostics-banner--alert" : "diagnostics-banner--quiet"}`}>
                  {hasRecentStderr ? "最近异常输出已写入诊断摘要。" : "诊断摘要已准备好，当前输出平稳。"}
                </div>
                <pre className="log-surface diagnostics-surface">{diagnosticsSummary}</pre>
              </article>
            )}

            {renderedSection === "settings" && (
              <article className="panel glass-panel settings-panel" data-busy={busyAction ?? "idle"}>
                {editingSettings && (
                  <div className="settings-edit-bar glass-panel glass-panel--subtle">
                    <div className="settings-edit-status">
                      <span className="settings-edit-status__dot" aria-hidden="true"></span>
                      <div className="settings-edit-status__copy">
                        <div className="settings-edit-status__title">编辑中</div>
                        <Text size={200} className="settings-edit-status__detail">当前显示草稿路径与预览结果，保存后才会切换为生效值。</Text>
                      </div>
                    </div>
                  </div>
                )}

                <div className="settings-compare-strip">
                  <div className="settings-compare-card">
                    <span className="settings-surface-tag">{settingsSurfaceTag}</span>
                    <span className="settings-compare-card__label">安装目录</span>
                    <span className="settings-compare-card__value" title={settingsDraft.installationRoot}>{settingsDraft.installationRoot || "—"}</span>
                  </div>
                  <div className="settings-compare-card settings-compare-card--resolved">
                    <span className="settings-surface-tag settings-surface-tag--resolved">当前生效</span>
                    <span className="settings-compare-card__label">运行目录</span>
                    <span className="settings-compare-card__value" title={resolvedSettings.workdir}>{resolvedSettings.workdir || "—"}</span>
                  </div>
                </div>

                <div className="settings-layout">
                  <div className="settings-column settings-column--primary">
                    <section className="settings-section glass-panel glass-panel--subtle">
                      <div className="settings-section__header">
                        <FolderOpen20Filled className="settings-section__icon" />
                        <div className="panel-copy">
                          <div className="brand-eyebrow brand-eyebrow--tight">安装目录</div>
                          <Text size={200} className="panel-muted">启动器和工作服务的根目录位置</Text>
                        </div>
                        <span className="settings-surface-tag">{settingsSurfaceTag}</span>
                      </div>
                      <div className="path-field">
                        <div className="path-control">
                          <Input aria-label="安装目录" value={settingsDraft.installationRoot} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateInstallationRoot(data.value)} />
                          <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseInstallationRoot} icon={<FolderOpen20Filled />}>浏览</Button>
                        </div>
                      </div>
                    </section>

                    <section className={`settings-section glass-panel glass-panel--subtle ${showAdvancedOverrides ? "is-expanded" : ""}`}>
                      <button type="button" className="settings-section__toggle" aria-expanded={showAdvancedOverrides} aria-label={showAdvancedOverrides ? "收起高级覆盖" : "展开高级覆盖"} onClick={() => setShowAdvancedOverrides((current) => !current)}>
                        <div className="settings-section__header">
                          <DocumentText20Filled className="settings-section__icon" />
                          <div className="panel-copy">
                            <div className="brand-eyebrow brand-eyebrow--tight">高级覆盖</div>
                            <Text size={200} className="panel-muted">使用显式路径覆盖自动推导结果</Text>
                          </div>
                          <span className="settings-surface-tag">{settingsSurfaceTag}</span>
                        </div>
                        <span className="settings-section__chevron" aria-hidden="true">{showAdvancedOverrides ? "收起高级覆盖" : "展开高级覆盖"}</span>
                      </button>

                      {showAdvancedOverrides && (
                        <div className="settings-advanced-fields">
                          <label className="path-field"><span className="path-field__label">服务端覆盖</span><div className="path-control"><Input aria-label="服务端覆盖" value={settingsDraft.advancedOverrides?.serverExecutablePath ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.serverExecutablePath} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("serverExecutablePath", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseServer} icon={<FolderOpen20Filled />}>浏览</Button></div></label>
                          <label className="path-field"><span className="path-field__label">配置覆盖</span><div className="path-control"><Input aria-label="配置覆盖" value={settingsDraft.advancedOverrides?.configPath ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.configPath} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("configPath", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseConfig} icon={<FolderOpen20Filled />}>浏览</Button></div></label>
                          <label className="path-field"><span className="path-field__label">运行目录覆盖</span><div className="path-control"><Input aria-label="运行目录覆盖" value={settingsDraft.advancedOverrides?.workdir ?? ""} readOnly={!editingSettings} placeholder={resolvedSettings.workdir} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("workdir", data.value)} /><Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseWorkdir} icon={<FolderOpen20Filled />}>选择</Button></div></label>
                        </div>
                      )}

                      <div className="settings-resolution-panel">
                        <div className="settings-resolution-panel__header">
                          <Status20Filled className="settings-resolution-panel__icon" />
                          <div className="panel-copy">
                            <div className="brand-eyebrow brand-eyebrow--tight">当前解析结果</div>
                            <Text size={200} className="panel-muted">当前生效的服务端、配置与工作目录路径。</Text>
                          </div>
                          <span className="settings-surface-tag settings-surface-tag--resolved">当前生效</span>
                        </div>
                        <div className="settings-info-list">
                          <div className="settings-info-item">
                            <span className="settings-info-item__label">服务端</span>
                            <span className="settings-info-item__value" title={resolvedSettings.serverExecutablePath}>{resolvedSettings.serverExecutablePath}</span>
                          </div>
                          <div className="settings-info-item">
                            <span className="settings-info-item__label">配置</span>
                            <span className="settings-info-item__value" title={resolvedSettings.configPath}>{resolvedSettings.configPath}</span>
                          </div>
                          <div className="settings-info-item">
                            <span className="settings-info-item__label">工作目录</span>
                            <span className="settings-info-item__value" title={resolvedSettings.workdir}>{resolvedSettings.workdir}</span>
                          </div>
                        </div>
                      </div>
                    </section>
                  </div>

                  <div className="settings-column settings-column--secondary">
                    <section className="preferences-panel glass-panel glass-panel--subtle">
                      <div className="panel-copy">
                        <div className="brand-eyebrow brand-eyebrow--tight">退出行为偏好</div>
                        <Text size={200} className="panel-muted">关闭窗口时采用的默认动作。托盘模式会保留后台入口。</Text>
                      </div>
                      <RadioGroup value={settingsDraft.closeBehavior} onChange={(_, data) => onUpdateCloseBehavior(data.value as LauncherSettings["closeBehavior"])}>
                        <div className="preference-options">
                          {closeBehaviorOptions.map((option) => (
                            <label key={option.value} className={`preference-option${settingsDraft.closeBehavior === option.value ? " is-selected" : ""}${!editingSettings ? " is-disabled" : ""}`}>
                              <Radio className="preference-radio" value={option.value} disabled={!editingSettings} />
                              <span className="preference-option__body">
                                <span className="preference-option__title">{option.label}</span>
                                <span className="preference-option__detail">{option.detail}</span>
                              </span>
                            </label>
                          ))}
                        </div>
                      </RadioGroup>
                    </section>

                    <section className="maintenance-panel glass-panel glass-panel--subtle">
                      <div className="panel-copy">
                        <div className="brand-eyebrow brand-eyebrow--tight">维护操作</div>
                        <Text size={200} className="panel-muted">用于重置本地凭据或直接结束启动器进程。</Text>
                      </div>
                      <div className="maintenance-action-list">
                        <div className="maintenance-action-card maintenance-action-card--danger">
                          <div className="maintenance-action-card__lead">
                            <span className="maintenance-action-card__badge" aria-hidden="true"><Warning20Filled /></span>
                            <div className="maintenance-action-card__copy">
                              <div className="maintenance-action-card__title">重置凭据</div>
                              <Text size={200} className="maintenance-action-card__detail">清除本地管理凭据，下次启动时重新完成初始化。</Text>
                            </div>
                          </div>
                          <Button appearance="transparent" size="small" className="frost-button frost-button--danger maintenance-action-card__button" onClick={onResetAdmin} disabled={controlsDisabled || snapshot.serviceState === "starting" || snapshot.serviceState === "stopping"}>立即重置</Button>
                        </div>
                        <div className="maintenance-action-card maintenance-action-card--soft">
                          <div className="maintenance-action-card__lead">
                            <span className="maintenance-action-card__badge" aria-hidden="true"><Stop20Filled /></span>
                            <div className="maintenance-action-card__copy">
                              <div className="maintenance-action-card__title">退出启动器</div>
                              <Text size={200} className="maintenance-action-card__detail">关闭窗口和托盘入口，不影响已保存配置与服务文件。</Text>
                            </div>
                          </div>
                          <Button appearance="transparent" size="small" className="frost-button frost-button--secondary maintenance-action-card__button" onClick={onExit}>退出启动器</Button>
                        </div>
                      </div>
                    </section>
                  </div>
                </div>
              </article>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
