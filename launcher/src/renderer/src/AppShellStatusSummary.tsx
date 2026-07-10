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
      <h3 id="status-details-title">运行详情</h3>
      <dl className="definition-list">
        <div className="definition-row">
          <dt>进程 ID</dt>
          <dd><code>{snapshot.launcher.processId ?? "—"}</code></dd>
        </div>
        <div className="definition-row">
          <dt>服务地址</dt>
          <dd className="mono">{snapshot.launcher.endpoint.baseUrl}</dd>
        </div>
        <div className="definition-row">
          <dt>安装目录</dt>
          <dd className="mono" title={installationRoot}>{installationRoot || "—"}</dd>
        </div>
        {showWorkdir ? (
          <div className="definition-row">
            <dt>工作目录</dt>
            <dd className="mono" title={workdir}>{workdir}</dd>
          </div>
        ) : null}
      </dl>
    </section>
  );
}
