import { Button } from "@fluentui/react-components";
import { ArrowClockwise20Regular, FolderOpen20Filled } from "@fluentui/react-icons";
import { deriveLauncherPresentation, getEnvironmentSummaryLabel } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import type { ReactNode } from "react";

import { busyActionLabels, sectionContent, serviceStateConfig } from "./AppShell.shared";
import type { SectionId } from "./AppShell.shared";

type AppShellSectionHeaderProps = {
  snapshot: LauncherSnapshot;
  renderedSection: SectionId;
  busyAction: string | null;
  controlsDisabled: boolean;
  editingSettings: boolean;
  onRefresh: () => void;
  onOpenRuntimeTasks: () => void;
  onOpenLogs: () => void;
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
};

function getSectionHeaderBadges(
  renderedSection: SectionId,
  snapshot: LauncherSnapshot,
  busyAction: string | null,
  editingSettings: boolean,
  environmentLabel: string,
  hasRecentStderr: boolean,
): ReactNode {
  const presentation = deriveLauncherPresentation(snapshot);

  if (renderedSection === "status") {
    return (
      <>
        <span className="glass-chip glass-chip--accent">{serviceStateConfig[presentation.state]?.label ?? "未知"}</span>
        {busyAction && <span className="glass-chip glass-chip--muted">{busyActionLabels[busyAction] ?? "正在执行操作"}</span>}
      </>
    );
  }
  if (renderedSection === "environment") {
    return (
      <>
        <span className="glass-chip glass-chip--accent">{environmentLabel}</span>
        <span className="glass-chip glass-chip--muted">{snapshot.launcher.preflightChecks.length} 项检查</span>
      </>
    );
  }
  if (renderedSection === "diagnostics") {
    return (
      <>
        <span className={`glass-chip ${hasRecentStderr ? "glass-chip--danger" : "glass-chip--muted"}`}>
          {hasRecentStderr ? "检测到异常输出" : "当前安静"}
        </span>
        <span className="glass-chip glass-chip--muted">{snapshot.launcher.endpoint.baseUrl}</span>
      </>
    );
  }
  return <span className="glass-chip glass-chip--accent">{editingSettings ? "草稿编辑中" : "已加载当前配置"}</span>;
}

function getSectionHeaderActions(props: AppShellSectionHeaderProps, canRunRecoveryActions: boolean): ReactNode {
  if (props.renderedSection === "status") {
    return (
      <Button
        appearance="transparent"
        size="small"
        onClick={props.onRefresh}
        icon={<ArrowClockwise20Regular />}
        className="frost-button frost-button--ghost"
        disabled={props.controlsDisabled}
      >
        刷新状态
      </Button>
    );
  }
  if (props.renderedSection === "environment") {
    return (
      <>
        <Button
          appearance="transparent"
          size="small"
          className="frost-button frost-button--secondary"
          onClick={props.onRefresh}
          disabled={props.controlsDisabled}
        >
          重新检查
        </Button>
        <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={props.onOpenRuntimeTasks} disabled={!canRunRecoveryActions}>打开运行环境任务</Button>
      </>
    );
  }
  if (props.renderedSection === "diagnostics") {
    return <Button appearance="transparent" size="small" className="frost-button frost-button--secondary" onClick={props.onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>;
  }
  if (props.editingSettings) {
    return (
      <>
        <Button appearance="transparent" size="small" className="frost-button frost-button--ghost" onClick={props.onCancelEdit}>放弃</Button>
        <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={props.onSaveSettings}>保存</Button>
      </>
    );
  }
  return <Button appearance="transparent" size="small" className="frost-button frost-button--primary" onClick={props.onBeginEdit}>编辑配置</Button>;
}

export function AppShellSectionHeader(props: AppShellSectionHeaderProps) {
  const sectionMeta = sectionContent[props.renderedSection];
  const presentation = deriveLauncherPresentation(props.snapshot);
  const hasRecentStderr = props.snapshot.launcher.recentStderr.length > 0;
  const canRunRecoveryActions = presentation.canRunRecoveryActions && !props.controlsDisabled;
  const environmentLabel = getEnvironmentSummaryLabel(props.snapshot.launcher.preflightChecks);

  return (
    <header className="section-header glass-panel glass-panel--subtle">
      <div className="section-header__copy">
        <div className="section-header__eyebrow">{sectionMeta.eyebrow}</div>
        <div className="section-header__title-row">
          <h2 className="section-header__title">{sectionMeta.title}</h2>
          <div className="section-header__badges">
            {getSectionHeaderBadges(props.renderedSection, props.snapshot, props.busyAction, props.editingSettings, environmentLabel, hasRecentStderr)}
          </div>
        </div>
        <p className="section-header__detail">{sectionMeta.detail}</p>
      </div>
      <div className="section-header__actions">
        {getSectionHeaderActions(props, canRunRecoveryActions)}
      </div>
    </header>
  );
}
