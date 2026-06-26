import type { LauncherSnapshot } from "@shared/launcher-models";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";

import { serviceStateConfig } from "./AppShell.shared";

type DiagnosticsSectionProps = {
  snapshot: LauncherSnapshot;
  diagnosticsSummary: string;
};

export function AppShellDiagnosticsSection({
  snapshot,
  diagnosticsSummary,
}: DiagnosticsSectionProps) {
  const presentation = deriveLauncherPresentation(snapshot);
  const hasRecentStderr = snapshot.launcher.recentStderr.length > 0;
  const logAlert = hasRecentStderr ? "error" : "none";

  return (
    <article className="panel glass-panel diagnostics-panel" data-alert={logAlert}>
      <div className="diagnostics-context-grid">
        <div className="diagnostics-context-card">
          <span className="status-label">服务状态</span>
          <span className="diagnostics-context-card__value">{serviceStateConfig[presentation.state]?.label ?? "未知"}</span>
        </div>
        <div className="diagnostics-context-card">
          <span className="status-label">本地端点</span>
          <span className="diagnostics-context-card__value mono">{snapshot.launcher.endpoint.baseUrl}</span>
        </div>
        <div className={`diagnostics-context-card ${hasRecentStderr ? "diagnostics-context-card--alert" : ""}`}>
          <span className="status-label">日志状态</span>
          <span className="diagnostics-context-card__value">
            {hasRecentStderr ? "发现异常日志，请查看下方摘要。" : "未发现异常日志。"}
          </span>
        </div>
      </div>
      <pre className="log-surface diagnostics-surface">{diagnosticsSummary}</pre>
    </article>
  );
}
