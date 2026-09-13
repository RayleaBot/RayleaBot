import { Button, MessageBar, MessageBarBody, MessageBarTitle } from "@fluentui/react-components";
import { ArrowClockwise20Regular, Open20Regular } from "@fluentui/react-icons";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { formatReleaseVersion } from "./AppShell.shared";
import { RayleaMark } from "./RayleaMark";

type AppShellAboutSectionProps = {
  snapshot: LauncherSnapshot;
  controlsDisabled: boolean;
  onCheckForUpdates: () => void;
  onOpenReleasePage: () => void;
  onOpenRepositoryPage: () => void;
};

function buildVersionHint(releaseCheck: LauncherSnapshot["launcher"]["releaseCheck"]) {
  const latestVersion = releaseCheck.latestVersion.trim();
  switch (releaseCheck.status) {
    case "checking":
      return "正在检查更新";
    case "update_available":
      return latestVersion ? `有新版本 ${latestVersion}` : "有新版本";
    case "failed":
      return releaseCheck.summary || releaseCheck.errorCode || releaseCheck.detail || "更新检查没有返回错误信息";
    default:
      return "";
  }
}

export function AppShellAboutSection({
  snapshot,
  controlsDisabled,
  onCheckForUpdates,
  onOpenReleasePage,
  onOpenRepositoryPage,
}: AppShellAboutSectionProps) {
  const releaseCheck = snapshot.launcher.releaseCheck;
  const currentVersion = formatReleaseVersion(releaseCheck.currentVersion);
  const versionHint = buildVersionHint(releaseCheck);
  const guidedRelease = Boolean(releaseCheck.releasePageUrl) && (releaseCheck.updateAvailable || !releaseCheck.canCheck);
  const updateButtonLabel = guidedRelease ? "打开发布页" : releaseCheck.status === "checking" ? "检查中" : "检查更新";
  const updateDisabled = controlsDisabled || releaseCheck.status === "checking" || (!guidedRelease && !releaseCheck.canCheck);
  const showUpdateAction = releaseCheck.canCheck || guidedRelease || releaseCheck.status === "checking";
  const showUpdateError = Boolean(releaseCheck.errorCode) || releaseCheck.status === "failed";
  const onUpdateAction = guidedRelease ? onOpenReleasePage : onCheckForUpdates;

  return (
    <article className="about-workspace">
      <section className="about-panel">
        <div className="about-panel__header">
          <div className="about-panel__identity">
            <span className="about-panel__mark" aria-hidden="true"><RayleaMark variant="neutral" /></span>
            <div>
              <h2>RayleaBot 启动器</h2>
              <p>检查本地环境、管理服务并定位运行问题。</p>
            </div>
          </div>
          <div className="about-panel__actions">
            {showUpdateAction ? (
              <Button
                appearance="secondary"
                icon={guidedRelease ? <Open20Regular /> : <ArrowClockwise20Regular />}
                disabled={updateDisabled}
                onClick={onUpdateAction}
              >
                {updateButtonLabel}
              </Button>
            ) : (
              <span className="update-unavailable">当前构建不提供更新检查</span>
            )}
            <Button appearance="subtle" icon={<Open20Regular />} onClick={onOpenRepositoryPage}>GitHub</Button>
          </div>
        </div>

        <dl className="definition-list about-information">
          <div className="definition-row">
            <dt>程序</dt>
            <dd>RayleaLauncher</dd>
          </div>
          <div className="definition-row">
            <dt>版本</dt>
            <dd className="version-value" data-status={releaseCheck.status}>
              <span>{currentVersion}</span>
              {versionHint ? <span>{versionHint}</span> : null}
            </dd>
          </div>
          <div className="definition-row">
            <dt>许可证</dt>
            <dd>AGPL-3.0</dd>
          </div>
        </dl>
        {showUpdateError ? (
          <MessageBar className="update-error-message" intent="error" layout="multiline">
            <MessageBarBody>
              <MessageBarTitle>{releaseCheck.summary || "更新检查没有返回错误摘要"}</MessageBarTitle>
              {releaseCheck.errorCode ? (
                <div className="update-error-code">
                  <span>错误代码</span>
                  <code>{releaseCheck.errorCode}</code>
                </div>
              ) : null}
              <p className="update-error-detail">
                {releaseCheck.detail || "更新检查没有返回错误原因。"}
              </p>
            </MessageBarBody>
          </MessageBar>
        ) : null}
      </section>
    </article>
  );
}
