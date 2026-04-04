import { DocumentText20Filled, FolderOpen20Filled, Globe20Filled, Status20Filled } from "@fluentui/react-icons";

import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

type AppShellStatusSummaryProps = {
  resolvedSettings: LauncherResolvedSettings;
  snapshot: LauncherSnapshot;
};

export function AppShellStatusSummary({ resolvedSettings, snapshot }: AppShellStatusSummaryProps) {
  return (
    <article className="panel glass-panel panel--interactive">
      <div className="brand-eyebrow">核心参数</div>
      <div className="status-list status-list--grid-modern">
        <div className="status-item-modern">
          <div className="status-item-modern__icon"><Status20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">进程 ID</span>
            <code className="status-value status-value--highlight">{snapshot.processId ?? "—"}</code>
          </div>
        </div>
        <div className="status-item-modern">
          <div className="status-item-modern__icon"><Globe20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">本地访问地址</span>
            <span className="status-value mono">{snapshot.endpoint.baseUrl}</span>
          </div>
        </div>
        <div className="status-item-modern status-item-modern--full">
          <div className="status-item-modern__icon"><FolderOpen20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">安装目录</span>
            <span className="status-value mono" title={snapshot.settings.installationRoot}>{snapshot.settings.installationRoot || "—"}</span>
          </div>
        </div>
        <div className="status-item-modern status-item-modern--full">
          <div className="status-item-modern__icon"><DocumentText20Filled /></div>
          <div className="status-item-modern__content">
            <span className="status-label">运行目录</span>
            <span className="status-value mono" title={resolvedSettings.workdir}>{resolvedSettings.workdir || "—"}</span>
          </div>
        </div>
      </div>
    </article>
  );
}
