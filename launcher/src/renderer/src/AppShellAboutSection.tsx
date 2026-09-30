import { Button, MessageBar, MessageBarBody, MessageBarTitle } from "@fluentui/react-components";
import {
  AppGeneric20Regular,
  ArrowClockwise20Regular,
  ArrowDownload20Regular,
  Certificate20Regular,
  Open20Regular,
  Tag20Regular,
  TextFont20Regular,
} from "@fluentui/react-icons";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { formatReleaseVersion } from "./AppShell.shared";
import { DetailRow } from "./AppShellDetailList";
import { RayleaMark } from "./RayleaMark";

type AppShellAboutSectionProps = {
  snapshot: LauncherSnapshot;
  controlsDisabled: boolean;
  onApplyUpdate: () => void;
  onCheckForUpdates: () => void;
  onOpenReleasePage: () => void;
  onOpenRepositoryPage: () => void;
};

function buildVersionHint(releaseCheck: LauncherSnapshot["launcher"]["releaseCheck"]) {
  const latestVersion = releaseCheck.latestVersion.trim();
  switch (releaseCheck.status) {
    case "checking":
      return "正在检查更新";
    case "updating":
      return releaseCheck.summary || "正在更新";
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
  onApplyUpdate,
  onCheckForUpdates,
  onOpenReleasePage,
  onOpenRepositoryPage,
}: AppShellAboutSectionProps) {
  const releaseCheck = snapshot.launcher.releaseCheck;
  const currentVersion = formatReleaseVersion(releaseCheck.currentVersion);
  const versionHint = buildVersionHint(releaseCheck);
  const updating = releaseCheck.status === "updating";
  const canApplyUpdate = releaseCheck.updateAvailable && !updating && releaseCheck.status !== "checking";
  const guidedRelease = Boolean(releaseCheck.releasePageUrl) && !releaseCheck.canCheck;
  const updateButtonLabel = guidedRelease ? "打开发布页" : releaseCheck.status === "checking" ? "检查中" : "检查更新";
  const updateDisabled = controlsDisabled || releaseCheck.status === "checking" || (!guidedRelease && !releaseCheck.canCheck);
  const showUpdateAction = releaseCheck.canCheck || guidedRelease || releaseCheck.status === "checking";
  const showUpdateError = Boolean(releaseCheck.errorCode) || releaseCheck.status === "failed";
  const onUpdateAction = guidedRelease ? onOpenReleasePage : onCheckForUpdates;

  return (
    <div className="about-workspace">
      <section className="about-identity" aria-labelledby="about-title">
        <span className="about-identity__mark" aria-hidden="true"><RayleaMark variant="neutral" /></span>
        <div className="about-identity__copy">
          <h2 id="about-title">RayleaBot 启动器</h2>
          <p>检查本地环境、管理服务并定位运行问题。</p>
        </div>
        <div className="about-identity__actions">
          {updating ? (
            <Button appearance="primary" className="launcher-button" data-emphasis="regular" icon={<ArrowDownload20Regular />} disabled>
              正在更新
            </Button>
          ) : canApplyUpdate ? (
            <>
              <Button
                appearance="primary"
                className="launcher-button"
                data-emphasis={controlsDisabled ? "regular" : "prominent"}
                icon={<ArrowDownload20Regular />}
                disabled={controlsDisabled}
                onClick={onApplyUpdate}
              >
                立即更新
              </Button>
              <Button appearance="secondary" className="launcher-button" data-emphasis="regular" icon={<Open20Regular />} onClick={onOpenReleasePage}>发布页</Button>
            </>
          ) : showUpdateAction ? (
            <Button
              appearance="secondary"
              className="launcher-button"
              data-emphasis="regular"
              icon={guidedRelease ? <Open20Regular /> : <ArrowClockwise20Regular />}
              disabled={updateDisabled}
              onClick={onUpdateAction}
            >
              {updateButtonLabel}
            </Button>
          ) : (
            <span className="update-unavailable">当前构建不提供更新检查</span>
          )}
          <Button appearance="secondary" className="launcher-button" data-emphasis="regular" icon={<Open20Regular />} onClick={onOpenRepositoryPage}>GitHub</Button>
        </div>
      </section>

      <dl className="detail-list detail-list--wrap content-group">
        <DetailRow icon={<AppGeneric20Regular />} label="程序" value="RayleaLauncher" mono={false} />
        <DetailRow icon={<Tag20Regular />} label="版本">
          <span className="version-value" data-status={releaseCheck.status}>
            <span>{currentVersion}</span>
            {versionHint ? <span>{versionHint}</span> : null}
          </span>
        </DetailRow>
        <DetailRow icon={<Certificate20Regular />} label="许可证" value="AGPL-3.0" mono={false} />
        {/* HarmonyOS Sans requires a notice in the software that the fonts are used. */}
        <DetailRow icon={<TextFont20Regular />} label="界面字体" value="HarmonyOS Sans SC" mono={false} />
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
    </div>
  );
}
