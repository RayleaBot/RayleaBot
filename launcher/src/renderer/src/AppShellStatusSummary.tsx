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
    <section className="data-section" aria-labelledby="status-details-title">
      <div className="data-section__heading">
        <h3 id="status-details-title">运行详情</h3>
        <span className="data-section__hint">本地服务关键信息</span>
      </div>
      <dl className="definition-list detail-grid">
        <div className="definition-row detail-card">
          <dt>进程 ID</dt>
          <dd><code className="detail-card__value">{snapshot.launcher.processId ?? "—"}</code></dd>
        </div>
        <div className="definition-row detail-card">
          <dt>服务地址</dt>
          <dd className="mono detail-card__value" title={snapshot.launcher.endpoint.baseUrl}>
            {snapshot.launcher.endpoint.baseUrl}
          </dd>
        </div>
        <div className="definition-row detail-card detail-card--wide">
          <dt>安装目录</dt>
          <dd className="mono detail-card__value" title={installationRoot}>{installationRoot || "—"}</dd>
        </div>
        {showWorkdir ? (
          <div className="definition-row detail-card detail-card--wide">
            <dt>工作目录</dt>
            <dd className="mono detail-card__value" title={workdir}>{workdir}</dd>
          </div>
        ) : null}
      </dl>
    </section>
  );
}
