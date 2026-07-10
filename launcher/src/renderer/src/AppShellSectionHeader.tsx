import { Button } from "@fluentui/react-components";
import { ArrowClockwise20Regular } from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
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
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
};

function getSectionHeaderBadges(
  renderedSection: SectionId,
  snapshot: LauncherSnapshot,
  busyAction: string | null,
  editingSettings: boolean,
  hasRecentStderr: boolean,
): ReactNode {
  const presentation = deriveLauncherPresentation(snapshot);

  if (renderedSection === "status") {
    return (
      <>
        <span className="status-chip" data-tone={serviceStateConfig[presentation.state]?.tone ?? "neutral"}>
          {serviceStateConfig[presentation.state]?.label ?? "未知"}
        </span>
        {busyAction && <span className="status-chip status-chip--muted">{busyActionLabels[busyAction] ?? "正在执行操作"}</span>}
      </>
    );
  }
  if (renderedSection === "environment") {
    return null;
  }
  if (renderedSection === "diagnostics") {
    return hasRecentStderr ? <span className="status-chip" data-tone="danger">发现异常日志</span> : null;
  }
  if (renderedSection === "about") {
    return null;
  }
  return editingSettings ? <span className="status-chip" data-tone="attention">草稿编辑中</span> : null;
}

function isRuntimePreparationIssue(code: string) {
  return ["deps.", "chromium.", "python.", "nodejs.", "npm."].some((prefix) => code.startsWith(prefix));
}

function getSectionHeaderActions(props: AppShellSectionHeaderProps, canPrepareRuntime: boolean): ReactNode {
  if (props.renderedSection === "status") {
    return (
      <Button
        appearance="subtle"
        onClick={props.onRefresh}
        icon={<ArrowClockwise20Regular />}
        className="action-button action-button--ghost"
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
          appearance="secondary"
          onClick={props.onRefresh}
          disabled={props.controlsDisabled}
        >
          重新检查
        </Button>
        {canPrepareRuntime ? <Button appearance="primary" onClick={props.onOpenRuntimeTasks}>准备运行环境</Button> : null}
      </>
    );
  }
  if (props.renderedSection === "diagnostics") {
    return null;
  }
  if (props.renderedSection === "about") {
    return null;
  }
  if (props.editingSettings) {
    return (
      <>
        <Button appearance="subtle" onClick={props.onCancelEdit}>放弃</Button>
        <Button appearance="primary" onClick={props.onSaveSettings}>保存</Button>
      </>
    );
  }
  return <Button appearance="primary" onClick={props.onBeginEdit}>编辑配置</Button>;
}

export function AppShellSectionHeader(props: AppShellSectionHeaderProps) {
  const sectionMeta = sectionContent[props.renderedSection];
  const presentation = deriveLauncherPresentation(props.snapshot);
  const hasRecentStderr = props.snapshot.launcher.recentStderr.length > 0;
  const canPrepareRuntime = presentation.canRunRecoveryActions
    && !props.controlsDisabled
    && props.snapshot.launcher.preflightChecks.some((item) => item.severity !== "ok" && isRuntimePreparationIssue(item.code));

  return (
    <header className="section-header">
      <div className="section-header__copy">
        <div className="section-header__title-row">
          <h1 className="section-header__title">{sectionMeta.title}</h1>
          <div className="section-header__badges">
            {getSectionHeaderBadges(props.renderedSection, props.snapshot, props.busyAction, props.editingSettings, hasRecentStderr)}
          </div>
        </div>
      </div>
      <div className="section-header__actions">
        {getSectionHeaderActions(props, canPrepareRuntime)}
      </div>
    </header>
  );
}
