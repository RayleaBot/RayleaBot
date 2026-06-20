import { DocumentText20Filled, FolderOpen20Filled, Globe20Filled, Status20Filled } from "@fluentui/react-icons";

import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

type AppShellStatusSummaryProps = {
  resolvedSettings: LauncherResolvedSettings;
  snapshot: LauncherSnapshot;
};

function normalizeComparablePath(value: string) {
  const trimmed = value.trim();
  const withoutTrailingSlash = trimmed.replace(/[\\/]+$/, "");
  return withoutTrailingSlash || trimmed;
}

function isWindowsPath(value: string) {
  const trimmed = value.trim();
  return /^[a-z]:[\\/]?/i.test(trimmed) || trimmed.startsWith("\\\\");
}

function isSameDirectoryPath(left: string, right: string) {
  const normalizedLeft = normalizeComparablePath(left);
  const normalizedRight = normalizeComparablePath(right);
  if (!normalizedLeft || !normalizedRight) {
    return false;
  }

  if (isWindowsPath(left) || isWindowsPath(right)) {
    return normalizedLeft.toLowerCase() === normalizedRight.toLowerCase();
  }

  return normalizedLeft === normalizedRight;
}

export function AppShellStatusSummary({ resolvedSettings, snapshot }: AppShellStatusSummaryProps) {
  const installationRoot = snapshot.launcher.settings.installationRoot;
  const workdir = resolvedSettings.workdir;
  const showWorkdir = Boolean(workdir.trim()) && !isSameDirectoryPath(installationRoot, workdir);

  return (
    <article className="panel glass-panel panel--interactive">
      <div className="brand-eyebrow">核心参数</div>
      <div className="status-list status-list--grid-modern">
        <div className="status-item-modern">
          <div className="status-item-modern__icon"><Status20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">进程 ID</span>
            <code className="status-value status-value--highlight">{snapshot.launcher.processId ?? "—"}</code>
          </div>
        </div>
        <div className="status-item-modern">
          <div className="status-item-modern__icon"><Globe20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">本地访问地址</span>
            <span className="status-value mono">{snapshot.launcher.endpoint.baseUrl}</span>
          </div>
        </div>
        <div className="status-item-modern status-item-modern--full">
          <div className="status-item-modern__icon"><FolderOpen20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">安装目录</span>
            <span className="status-value mono" title={installationRoot}>{installationRoot || "—"}</span>
          </div>
        </div>
        {showWorkdir ? (
          <div className="status-item-modern status-item-modern--full">
            <div className="status-item-modern__icon"><DocumentText20Filled /></div>
            <div className="status-item-modern__content">
              <span className="status-label">进程工作目录</span>
              <span className="status-value mono" title={workdir}>{workdir}</span>
            </div>
          </div>
        ) : null}
      </div>
    </article>
  );
}
