import { Button } from "@fluentui/react-components";
import {
  ArrowClockwise20Regular,
  Dismiss20Regular,
  Edit20Regular,
  Globe20Regular,
  Save20Regular,
} from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import type { ReactNode } from "react";

import { busyActionLabels, sectionContent } from "./AppShell.shared";
import type { SectionId } from "./AppShell.shared";

type AppShellSectionHeaderProps = {
  snapshot: LauncherSnapshot;
  renderedSection: SectionId;
  busyAction: string | null;
  controlsDisabled: boolean;
  editingSettings: boolean;
  onRefresh: () => void;
  onOpenWeb: () => void;
  onBeginEdit: () => void;
  onCancelEdit: () => void;
  onSaveSettings: () => void;
};

function getSectionHeaderBadges(
  renderedSection: SectionId,
  busyAction: string | null,
  editingSettings: boolean,
  hasRecentStderr: boolean,
): ReactNode {
  if (renderedSection === "status") {
    // The service pane names the state; the header only reports an operation in progress.
    return busyAction ? <span className="status-chip status-chip--muted">{busyActionLabels[busyAction] ?? "正在执行操作"}</span> : null;
  }
  if (renderedSection === "environment") {
    return null;
  }
  if (renderedSection === "diagnostics") {
    return hasRecentStderr ? <span className="status-chip" data-tone="danger">发现异常输出</span> : null;
  }
  if (renderedSection === "about") {
    return null;
  }
  return editingSettings ? <span className="status-chip" data-tone="attention">草稿编辑中</span> : null;
}

function getSectionHeaderActions(props: AppShellSectionHeaderProps, canPrepareRuntime: boolean): ReactNode {
  if (props.renderedSection === "status") {
    return (
      <Button
        appearance="secondary"
        onClick={props.onRefresh}
        icon={<ArrowClockwise20Regular />}
        className="launcher-button"
        data-emphasis="regular"
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
          icon={<ArrowClockwise20Regular />}
          className="launcher-button"
          data-emphasis="regular"
          disabled={props.controlsDisabled}
        >
          重新检查
        </Button>
        {canPrepareRuntime ? (
          <Button
            appearance="primary"
            onClick={props.onOpenWeb}
            icon={<Globe20Regular />}
            className="launcher-button"
            data-emphasis="prominent"
          >
            在管理界面准备
          </Button>
        ) : null}
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
        <Button
          appearance="secondary"
          onClick={props.onCancelEdit}
          icon={<Dismiss20Regular />}
          className="launcher-button"
          data-emphasis="regular"
          disabled={props.controlsDisabled}
        >
          放弃
        </Button>
        <Button
          appearance="primary"
          onClick={props.onSaveSettings}
          icon={<Save20Regular />}
          className="launcher-button"
          data-emphasis={props.controlsDisabled ? "regular" : "prominent"}
          disabled={props.controlsDisabled}
        >
          保存
        </Button>
      </>
    );
  }
  return (
    <Button
      appearance="secondary"
      onClick={props.onBeginEdit}
      icon={<Edit20Regular />}
      className="launcher-button"
      data-emphasis="regular"
      disabled={props.controlsDisabled}
    >
      编辑设置
    </Button>
  );
}

export function AppShellSectionHeader(props: AppShellSectionHeaderProps) {
  const sectionMeta = sectionContent[props.renderedSection];
  const presentation = deriveLauncherPresentation(props.snapshot);
  const hasRecentStderr = props.snapshot.launcher.recentStderr.length > 0;
  const canPrepareRuntime = presentation.preparableRuntimeResources.length > 0 && !props.controlsDisabled;

  return (
    <header className="section-header">
      <div className="section-header__copy">
        <div className="section-header__title-row">
          <h1 className="section-header__title">{sectionMeta.title}</h1>
          <div className="section-header__badges">
            {getSectionHeaderBadges(props.renderedSection, props.busyAction, props.editingSettings, hasRecentStderr)}
          </div>
        </div>
      </div>
      <div className="section-header__actions">
        {getSectionHeaderActions(props, canPrepareRuntime)}
      </div>
    </header>
  );
}
