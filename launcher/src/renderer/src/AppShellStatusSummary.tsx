import {
  FolderOpen20Regular,
  Globe20Regular,
  NumberSymbol20Regular,
} from "@fluentui/react-icons";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { DetailRow } from "./AppShellDetailList";

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

// The installation directory is a fixed setting shown in preferences. A work directory elsewhere is repeated here
// because the service's logs are written there.
export function AppShellStatusSummary({ resolvedSettings, snapshot }: AppShellStatusSummaryProps) {
  const installationRoot = snapshot.launcher.settings.installationRoot;
  const workdir = resolvedSettings.workdir;
  const showWorkdir = Boolean(workdir.trim()) && !isSameDirectoryPath(installationRoot, workdir);
  const baseUrl = snapshot.launcher.endpoint.baseUrl;

  return (
    <section className="workspace-group" aria-labelledby="status-details-title">
      <h3 id="status-details-title" className="workspace-group__title">运行详情</h3>
      <dl className="detail-list content-group">
        <DetailRow icon={<NumberSymbol20Regular />} label="进程 ID" value={String(snapshot.launcher.processId ?? "—")} />
        <DetailRow icon={<Globe20Regular />} label="管理界面地址" value={baseUrl} title={baseUrl} />
        {showWorkdir ? (
          <DetailRow icon={<FolderOpen20Regular />} label="工作目录" value={workdir} title={workdir} />
        ) : null}
      </dl>
    </section>
  );
}
