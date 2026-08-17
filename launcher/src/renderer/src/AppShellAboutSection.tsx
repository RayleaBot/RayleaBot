import { Button } from "@fluentui/react-components";
import { ArrowClockwise20Regular, Open20Regular } from "@fluentui/react-icons";
import { MessageBar, MessageBarBody, MessageBarTitle } from "@fluentui/react-message-bar";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { formatByteCount, formatReleaseVersion } from "./AppShell.shared";
import { RayleaMark } from "./RayleaMark";

type AppShellAboutSectionProps = {
  snapshot: LauncherSnapshot;
  controlsDisabled: boolean;
  onCheckForUpdates: () => void;
  onDownloadUpdate: () => void;
  onInstallDownloadedUpdate: () => void;
  onOpenReleasePage: () => void;
  onOpenRepositoryPage: () => void;
};

function buildVersionHint(releaseCheck: LauncherSnapshot["launcher"]["releaseCheck"], progressLabel: string) {
  const latestVersion = releaseCheck.latestVersion.trim();
  switch (releaseCheck.status) {
    case "checking":
      return "正在检查更新";
    case "update_available":
      return latestVersion ? `有新版本 ${latestVersion}` : "有新版本";
    case "downloading":
      return progressLabel ? `下载中 ${progressLabel}` : "正在下载更新";
    case "ready_to_install":
      return latestVersion ? `已验证 ${latestVersion}` : "更新已准备安装";
    case "installing":
      return "正在安装更新";
    case "failed":
    case "rollback_failed":
      return releaseCheck.summary || releaseCheck.errorCode || releaseCheck.detail || "更新检查没有返回错误信息";
    case "rolled_back":
      return "新版启动失败，已恢复上一版本";
    default:
      return "";
  }
}

export function AppShellAboutSection({
  snapshot,
  controlsDisabled,
  onCheckForUpdates,
  onDownloadUpdate,
  onInstallDownloadedUpdate,
  onOpenReleasePage,
  onOpenRepositoryPage,
}: AppShellAboutSectionProps) {
  const releaseCheck = snapshot.launcher.releaseCheck;
  const currentVersion = formatReleaseVersion(releaseCheck.currentVersion);
  const progressLabel =
    releaseCheck.downloadedBytes && releaseCheck.totalBytes
      ? `${formatByteCount(releaseCheck.downloadedBytes)} / ${formatByteCount(releaseCheck.totalBytes)}`
      : "";
  const versionHint = buildVersionHint(releaseCheck, progressLabel);
  const guidedRelease = Boolean(releaseCheck.releasePageUrl)
    && !releaseCheck.canDownload
    && !releaseCheck.canInstall
    && (
      releaseCheck.updateAvailable
      || (releaseCheck.status === "disabled" && !releaseCheck.canCheck)
    );
  const updateButtonLabel =
    guidedRelease
      ? "打开发布页"
      : releaseCheck.status === "ready_to_install"
      ? "确认安装"
      : releaseCheck.status === "downloading"
        ? "下载中"
        : releaseCheck.status === "checking"
          ? "检查中"
          : releaseCheck.status === "installing"
            ? "安装中"
            : releaseCheck.canDownload
              ? "下载更新"
              : "检查更新";
  const updateDisabled =
    controlsDisabled
    || releaseCheck.status === "checking"
    || releaseCheck.status === "downloading"
    || releaseCheck.status === "installing"
    || (!guidedRelease && !releaseCheck.canCheck && !releaseCheck.canDownload && !releaseCheck.canInstall);
  const updateInProgress = releaseCheck.status === "checking"
    || releaseCheck.status === "downloading"
    || releaseCheck.status === "installing";
  const showUpdateAction = releaseCheck.canCheck
    || releaseCheck.canDownload
    || releaseCheck.canInstall
    || guidedRelease
    || updateInProgress;
  const showUpdateError = Boolean(releaseCheck.errorCode)
    || releaseCheck.status === "failed"
    || releaseCheck.status === "rollback_failed";
  const onUpdateAction =
    guidedRelease
      ? onOpenReleasePage
      : releaseCheck.canInstall
      ? onInstallDownloadedUpdate
      : releaseCheck.canDownload
        ? onDownloadUpdate
        : onCheckForUpdates;

  return (
    <article className="about-workspace">
      <section className="about-panel">
        <div className="about-panel__header">
          <div className="about-panel__identity">
            <span className="about-panel__mark" aria-hidden="true"><RayleaMark variant="neutral" /></span>
            <div>
              <div className="section-kicker">RayleaBot</div>
              <h2>RayleaBot 启动器</h2>
              <p>检查本地环境、管理服务并定位运行问题。</p>
            </div>
          </div>
          <div className="about-panel__actions">
            {showUpdateAction ? (
              <Button
                appearance={releaseCheck.canInstall ? "primary" : "secondary"}
                className={releaseCheck.canInstall ? "attention-button" : undefined}
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
