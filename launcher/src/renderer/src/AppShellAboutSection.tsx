import { Button } from "@fluentui/react-components";
import {
  ArrowClockwise20Regular,
  Code20Filled,
  Info20Filled,
  Open20Regular,
} from "@fluentui/react-icons";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { formatReleaseVersion } from "./AppShell.shared";

type AppShellAboutSectionProps = {
  snapshot: LauncherSnapshot;
  controlsDisabled: boolean;
  onCheckForUpdates: () => void;
  onDownloadUpdate: () => void;
  onInstallDownloadedUpdate: () => void;
  onOpenRepositoryPage: () => void;
};

function formatBytes(value: number | null) {
  if (!value || value <= 0) {
    return "";
  }
  if (value >= 1024 * 1024) {
    return `${(value / 1024 / 1024).toFixed(1)} MB`;
  }
  if (value >= 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${value} B`;
}

function buildVersionHint(releaseCheck: LauncherSnapshot["launcher"]["releaseCheck"], progressLabel: string) {
  const latestVersion = releaseCheck.latestVersion.trim();
  switch (releaseCheck.status) {
    case "checking":
      return "正在检查更新";
    case "update_available":
      return latestVersion ? `有新版本 ${latestVersion}` : "有新版本";
    case "downloading":
      return progressLabel ? `下载中 ${progressLabel}` : "正在下载更新";
    case "downloaded":
      return latestVersion ? `已下载 ${latestVersion}` : "更新已下载";
    case "installing":
      return "正在安装更新";
    case "error":
      return releaseCheck.summary || "更新检查失败";
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
  onOpenRepositoryPage,
}: AppShellAboutSectionProps) {
  const releaseCheck = snapshot.launcher.releaseCheck;
  const currentVersion = formatReleaseVersion(releaseCheck.currentVersion);
  const progressLabel =
    releaseCheck.downloadedBytes && releaseCheck.totalBytes
      ? `${formatBytes(releaseCheck.downloadedBytes)} / ${formatBytes(releaseCheck.totalBytes)}`
      : "";
  const versionHint = buildVersionHint(releaseCheck, progressLabel);
  const updateButtonLabel =
    releaseCheck.status === "downloaded"
      ? "重启安装"
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
    || (!releaseCheck.canCheck && !releaseCheck.canDownload && !releaseCheck.canInstall);
  const onUpdateAction =
    releaseCheck.canInstall
      ? onInstallDownloadedUpdate
      : releaseCheck.canDownload
        ? onDownloadUpdate
        : onCheckForUpdates;

  return (
    <article className="about-section">
      <section className="about-hero glass-panel">
        <div className="about-hero__mark" aria-hidden="true">
          <Code20Filled />
        </div>
        <div className="about-hero__copy">
          <div className="brand-eyebrow brand-eyebrow--tight">RayleaBot</div>
          <h3 className="about-hero__title">RayleaBot 启动器</h3>
        </div>
        <div className="about-hero__actions">
          <Button
            appearance="transparent"
            className="frost-button frost-button--secondary"
            icon={<ArrowClockwise20Regular />}
            disabled={updateDisabled}
            onClick={onUpdateAction}
          >
            {updateButtonLabel}
          </Button>
          <Button
            appearance="transparent"
            className="frost-button frost-button--primary"
            icon={<Open20Regular />}
            onClick={onOpenRepositoryPage}
          >
            GitHub
          </Button>
        </div>
      </section>

      <section className="about-info-panel glass-panel glass-panel--subtle">
        <div className="about-panel-heading">
          <Info20Filled className="about-panel-heading__icon" />
          <div className="brand-eyebrow brand-eyebrow--tight">应用信息</div>
        </div>
        <div className="about-info-list">
          <div className="about-info-row">
            <span className="about-info-row__label">应用</span>
            <span className="about-info-row__value">RayleaBot</span>
          </div>
          <div className="about-info-row">
            <span className="about-info-row__label">启动器</span>
            <span className="about-info-row__value">RayleaLauncher</span>
          </div>
          <div className="about-info-row">
            <span className="about-info-row__label">版本</span>
            <span className="about-info-row__value about-version-value">
              <span>{currentVersion}</span>
              {versionHint ? <span className="about-version-hint">{versionHint}</span> : null}
            </span>
          </div>
          <div className="about-info-row">
            <span className="about-info-row__label">许可证</span>
            <span className="about-info-row__value">AGPL-3.0</span>
          </div>
        </div>
      </section>
    </article>
  );
}
