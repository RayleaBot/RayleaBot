import { Button, Text } from "@fluentui/react-components";
import {
  ArrowClockwise20Regular,
  Dismiss20Regular,
  FolderOpen20Filled,
  Square20Regular,
  SquareMultiple20Regular,
  Subtract20Regular,
} from "@fluentui/react-icons";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

import {
  busyActionLabels,
  sectionContent,
  sections,
  serviceStateConfig,
  statusSummary,
} from "./AppShell.shared";
import type { SectionId, SectionTransitionState } from "./AppShell.shared";
import { AppShellDiagnosticsSection } from "./AppShellDiagnosticsSection";
import { AppShellEnvironmentSection } from "./AppShellEnvironmentSection";
import { AppShellSettingsSection } from "./AppShellSettingsSection";
import { AppShellStatusSection } from "./AppShellStatusSection";

export type AppShellViewProps = {
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

export function AppShellView({
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
}: AppShellViewProps) {
  const trayStatus = statusSummary(snapshot.serviceState);
  const sectionMeta = sectionContent[renderedSection];
  const hasRecentStderr = snapshot.recentStderr.length > 0;
  const busyLabel = busyAction ? (busyActionLabels[busyAction] ?? "正在执行操作") : "";
  const canRunRecoveryActions =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && !controlsDisabled;
  const canRecheckRecovery = canRunRecoveryActions && Boolean(snapshot.recoverySummary);
  const environmentChecks = snapshot.environmentChecks || [];
  const environmentReadiness =
    environmentChecks.some((item) => item.severity === "error")
      ? { label: "需要处理" }
      : environmentChecks.some((item) => item.severity === "warning")
        ? { label: "可继续，但有警告" }
        : { label: "可以启动" };

  const sectionHeaderBadges =
    renderedSection === "status" ? (
      <>
        <span className="glass-chip glass-chip--accent">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</span>
        {busyAction && <span className="glass-chip glass-chip--muted">{busyLabel}</span>}
      </>
    ) : renderedSection === "environment" ? (
      <>
        <span className="glass-chip glass-chip--accent">{environmentReadiness.label}</span>
        <span className="glass-chip glass-chip--muted">{environmentChecks.length} 项检查</span>
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
        <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={onRecoveryRecheck} disabled={!canRecheckRecovery}>重新检查</Button>
        <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行环境</Button>
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
              <AppShellStatusSection
                snapshot={snapshot}
                resolvedSettings={resolvedSettings}
                busyAction={busyAction}
                controlsDisabled={controlsDisabled}
                onStart={onStart}
                onStop={onStop}
                onOpenWeb={onOpenWeb}
                onRecoveryRecheck={onRecoveryRecheck}
                onRuntimeBootstrap={onRuntimeBootstrap}
                onOpenReleasePage={onOpenReleasePage}
                onOpenLogs={onOpenLogs}
              />
            )}

            {renderedSection === "environment" && (
              <AppShellEnvironmentSection
                snapshot={snapshot}
                platformLabel={platformLabel}
              />
            )}

            {renderedSection === "diagnostics" && (
              <AppShellDiagnosticsSection
                snapshot={snapshot}
                diagnosticsSummary={diagnosticsSummary}
              />
            )}

            {renderedSection === "settings" && (
              <AppShellSettingsSection
                snapshot={snapshot}
                settingsDraft={settingsDraft}
                resolvedSettings={resolvedSettings}
                editingSettings={editingSettings}
                busyAction={busyAction}
                controlsDisabled={controlsDisabled}
                onUpdateInstallationRoot={onUpdateInstallationRoot}
                onUpdateCloseBehavior={onUpdateCloseBehavior}
                onUpdateAdvancedOverride={onUpdateAdvancedOverride}
                onChooseInstallationRoot={onChooseInstallationRoot}
                onChooseServer={onChooseServer}
                onChooseConfig={onChooseConfig}
                onChooseWorkdir={onChooseWorkdir}
                onResetAdmin={onResetAdmin}
                onExit={onExit}
              />
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
