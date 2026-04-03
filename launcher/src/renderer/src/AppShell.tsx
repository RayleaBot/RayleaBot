import { useEffect, useMemo, useState } from "react";
import { Radio, RadioGroup, Input, PresenceBadge, Text, Button } from "@fluentui/react-components";
import type { PresenceBadgeStatus } from "@fluentui/react-components";
import {
  Play20Filled,
  Stop20Filled,
  Globe20Filled,
  FolderOpen20Filled,
  CheckmarkCircle20Filled,
  Warning20Filled,
  DismissCircle20Filled,
  Status20Filled,
  HeartPulse20Filled,
  DocumentText20Filled,
  Settings20Filled,
  ArrowClockwise20Regular,
  Subtract20Regular,
  Square20Regular,
  SquareMultiple20Regular,
  Dismiss20Regular,
} from "@fluentui/react-icons";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
  LauncherServiceState,
} from "@shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

const serviceStateConfig: Record<LauncherServiceState, { status: PresenceBadgeStatus; label: string }> = {
  stopped: { status: "offline", label: "已停止" },
  starting: { status: "busy", label: "启动中" },
  running: { status: "available", label: "运行中" },
  degraded: { status: "busy", label: "受限运行" },
  setup_required: { status: "blocked", label: "需要设置" },
  stopping: { status: "busy", label: "停止中" },
  failed: { status: "blocked", label: "启动失败" },
};

const severityConfig = {
  error: { label: "阻塞", icon: <DismissCircle20Filled /> },
  warning: { label: "警告", icon: <Warning20Filled /> },
  ok: { label: "正常", icon: <CheckmarkCircle20Filled /> },
};

type AppShellProps = {
  snapshot: LauncherSnapshot;
  activeSection: SectionId;
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

const sections = [
  { id: "status" as SectionId, title: "运行状态", icon: <Status20Filled /> },
  { id: "environment" as SectionId, title: "环境检查", icon: <HeartPulse20Filled /> },
  { id: "diagnostics" as SectionId, title: "日志诊断", icon: <DocumentText20Filled /> },
  { id: "settings" as SectionId, title: "偏好设置", icon: <Settings20Filled /> },
];

const closeBehaviorOptions: Array<{
  value: LauncherSettings["closeBehavior"];
  label: string;
  detail: string;
}> = [
  { value: "ask_every_time", label: "每次询问", detail: "每次关闭窗口时都显示确认选项。" },
  { value: "hide_to_tray", label: "系统托盘", detail: "关闭主窗口后保留托盘入口和后台状态。" },
  { value: "exit_application", label: "完全退出", detail: "直接结束启动器窗口与托盘进程。" },
];

function statusSummary(state: LauncherServiceState): string {
  switch (state) {
    case "stopped":
      return "已停止";
    case "starting":
      return "正在启动";
    case "running":
      return "正在运行";
    case "degraded":
      return "受限运行";
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

export function AppShell({
  snapshot,
  activeSection,
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
  const checks = useMemo(() => snapshot.environmentChecks || [], [snapshot.environmentChecks]);
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
      core: checks.filter((item) => corePrefixes.some((prefix) => item.code.startsWith(prefix))),
      runtimes: checks.filter((item) => runtimePrefixes.some((prefix) => item.code.startsWith(prefix))),
      others: checks.filter(
        (item) =>
          !corePrefixes.some((prefix) => item.code.startsWith(prefix))
          && !runtimePrefixes.some((prefix) => item.code.startsWith(prefix)),
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
            <button key={section.id} className={`nav-item${activeSection === section.id ? " active" : ""}`} onClick={() => onNavigate(section.id)}>
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

      <main className={`shell-main ${activeSection === "environment" ? "active-environment" : ""}`}>
        {activeSection === "status" && (
          <>
            <section className="hero-card glass-panel">
              <div className="hero-copy">
                <div className="brand-eyebrow">Service Control</div>
                <div className="hero-status-row">
                  <PresenceBadge status={serviceStateConfig[snapshot.serviceState]?.status ?? "unknown"} size="large" />
                  <Text weight="bold" size={500} className="hero-status-text">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</Text>
                </div>
                <Text size={200} className="hero-detail">{snapshot.serviceDetail}</Text>
                {snapshot.lastError && (
                  <div className="inline-alert inline-alert--error">
                    <span className="inline-alert__icon"><DismissCircle20Filled /></span>
                    <span className="inline-alert__text">{snapshot.lastError}</span>
                  </div>
                )}
              </div>

              <div className="hero-actions">
                <Button appearance="transparent" className="frost-button frost-button--primary frost-button--block" onClick={onStart} disabled={startDisabled} icon={<Play20Filled />}>{primaryActionLabel}</Button>
                <Button appearance="transparent" className="frost-button frost-button--secondary frost-button--block" onClick={onStop} disabled={stopDisabled} icon={<Stop20Filled />}>停止服务</Button>
                <Button appearance="transparent" className="frost-button frost-button--secondary frost-button--block" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Filled />}>管理面板</Button>
              </div>
            </section>

            {checks.length > 0 && checks.some((item) => item.severity !== "ok") && (
              <div className="checks-stack">
                {checks.filter((item) => item.severity !== "ok").map((item) => (
                  <div key={item.code} className={`check-item glass-panel glass-panel--subtle check-item--${item.severity}`}>
                    <div className="check-item__lead">
                      <div className="check-item__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
                      <div className="check-item__copy">
                        <Text weight="bold" size={300}>{item.title}</Text>
                        <Text size={200} className="check-item__summary">
                          {item.code === "os.long_paths_unknown" && item.severity === "warning"
                            ? "无法确认长路径支持状态。若资源展开遇到限制，请手动检查系统长路径设置。"
                            : item.summary}
                        </Text>
                      </div>
                    </div>
                    <span className={`status-pill status-pill--${item.severity}`}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                  </div>
                ))}
              </div>
            )}

            <article className="panel glass-panel">
              <div className="brand-eyebrow">核心参数</div>
              <div className="status-list">
                <div className="status-item"><span className="status-label">进程 ID</span><code className="status-value">{snapshot.processId ?? "—"}</code></div>
                <div className="status-item"><span className="status-label">本地端点</span><span className="status-value mono">{snapshot.endpoint.baseUrl}</span></div>
                <div className="status-item"><span className="status-label">安装目录</span><span className="status-value mono">{snapshot.settings.installationRoot || "—"}</span></div>
                <div className="status-item"><span className="status-label">运行目录</span><span className="status-value mono">{resolvedSettings.workdir || "—"}</span></div>
              </div>
            </article>

            <article className="panel glass-panel glass-panel--subtle panel-row">
              <div className="panel-copy">
                <div className="brand-eyebrow brand-eyebrow--tight">版本监控</div>
                <Text size={200} className="panel-muted">{snapshot.releaseCheck.summary}</Text>
              </div>
              <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onOpenReleasePage}>检查更新</Button>
            </article>

            <article className="panel glass-panel glass-panel--subtle panel-row">
              <div className="panel-copy">
                <div className="brand-eyebrow brand-eyebrow--tight">恢复兼容性</div>
                <Text size={200} className="panel-muted">{snapshot.recoverySummary ? `${snapshot.recoverySummary.status} · ${snapshot.recoverySummary.operation}` : "当前没有恢复摘要。"}</Text>
              </div>
              <div className="panel-actions panel-actions--stack">
                <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onRecoveryRecheck} disabled={!canRunRecoveryActions}>重新检查</Button>
                <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行时</Button>
              </div>
            </article>

            <article className="panel glass-panel">
              <div className="brand-eyebrow">异常输出监控</div>
              <pre className="log-surface">{snapshot.recentStderr.join("\n") || "当前无异常日志。"}</pre>
              <Button appearance="transparent" size="small" className="frost-button frost-button--ghost" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>
            </article>
          </>
        )}

        {activeSection === "environment" && (
          <div className="env-details-flow">
            <article className="panel glass-panel">
              <div className="brand-eyebrow">运行环境概览</div>
              <div className="status-list env-status-grid">
                <div className="status-item"><span className="status-label">平台架构</span><span className="status-value">{platformLabel || "—"}</span></div>
                <div className="status-item"><span className="status-label">核心版本</span><span className="status-value">{snapshot.releaseCheck.currentVersion || "—"}</span></div>
                <div className="status-item"><span className="status-label">安装路径</span><span className="status-value mono">{snapshot.settings.installationRoot || "—"}</span></div>
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
                            <Text weight="bold" size={200}>{item.title}</Text>
                            <Text size={100} className="check-item__summary">{item.summary}</Text>
                          </div>
                        </div>
                        <span className={`status-pill status-pill--${item.severity}`}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                      </div>
                    ))}
                  </div>
                </section>
              ))}

            <div className="metric-panel-container">
              <article className="panel glass-panel metric-panel">
                <div className="brand-eyebrow">环境评分摘要</div>
                <div className="metric-grid">
                  <div className="metric-card metric-card--error"><Text size={100} block className="metric-label">阻塞项</Text><Text size={600} weight="bold">{groupedChecks.blocking.length}</Text></div>
                  <div className="metric-card metric-card--warning"><Text size={100} block className="metric-label">警告项</Text><Text size={600} weight="bold">{groupedChecks.warnings.length}</Text></div>
                  <div className="metric-card metric-card--ok"><Text size={100} block className="metric-label">正常项</Text><Text size={600} weight="bold">{groupedChecks.ready.length}</Text></div>
                </div>
              </article>
            </div>
          </div>
        )}

        {activeSection === "diagnostics" && (
          <article className="panel glass-panel diagnostics-panel">
            <div className="brand-eyebrow">系统诊断快照</div>
            <pre className="log-surface diagnostics-surface">{diagnosticsSummary}</pre>
          </article>
        )}

        {activeSection === "settings" && (
          <article className="panel glass-panel settings-panel">
            <div className="panel-toolbar">
              <div className="panel-copy">
                <div className="brand-eyebrow brand-eyebrow--tight">设置</div>
              </div>
              <div className="settings-actions">
                {!editingSettings && (
                  <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onBeginEdit}>编辑配置</Button>
                )}
              </div>
            </div>

            {editingSettings && (
              <div className="settings-edit-bar glass-panel glass-panel--subtle">
                <div className="settings-edit-status">
                  <span className="settings-edit-status__dot" aria-hidden="true"></span>
                  <div className="settings-edit-status__copy">
                    <div className="settings-edit-status__title">编辑中</div>
                    <Text size={200} className="settings-edit-status__detail">路径变更尚未保存，当前显示的是预览结果。</Text>
                  </div>
                </div>
                <div className="settings-edit-actions">
                  <Button appearance="transparent" size="small" className="frost-button frost-button--ghost" onClick={onCancelEdit}>放弃</Button>
                  <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onSaveSettings}>保存</Button>
                </div>
              </div>
            )}

            <div className="settings-layout">
              <div className="settings-column settings-column--primary">
                <section className="settings-section glass-panel glass-panel--subtle">
                  <div className="settings-section__header">
                    <FolderOpen20Filled className="settings-section__icon" />
                    <div className="panel-copy">
                      <div className="brand-eyebrow brand-eyebrow--tight">安装目录</div>
                      <Text size={200} className="panel-muted">启动器和工作服务的根目录位置</Text>
                    </div>
                  </div>
                  <div className="path-field">
                    <div className="path-control">
                      <Input aria-label="安装目录" value={settingsDraft.installationRoot} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateInstallationRoot(data.value)} />
                      <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseInstallationRoot} icon={<FolderOpen20Filled />}>浏览</Button>
                    </div>
                  </div>
                </section>

                <section className={`settings-section glass-panel glass-panel--subtle ${showAdvancedOverrides ? "is-expanded" : ""}`}>
                  <button
                    type="button"
                    className="settings-section__toggle"
                    aria-expanded={showAdvancedOverrides}
                    aria-label={showAdvancedOverrides ? "收起高级覆盖" : "展开高级覆盖"}
                    onClick={() => setShowAdvancedOverrides((current) => !current)}
                  >
                    <div className="settings-section__header">
                      <DocumentText20Filled className="settings-section__icon" />
                      <div className="panel-copy">
                        <div className="brand-eyebrow brand-eyebrow--tight">高级覆盖</div>
                        <Text size={200} className="panel-muted">使用显式路径覆盖自动推导结果</Text>
                      </div>
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
      </main>
    </div>
  );
}
