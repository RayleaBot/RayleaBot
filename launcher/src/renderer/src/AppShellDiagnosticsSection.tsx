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
          <span className="status-label">{hasRecentStderr ? "异常状态" : "日志状态"}</span>
          <span className="diagnostics-context-card__value">
            {hasRecentStderr ? "最近有异常输出，请结合摘要定位问题。" : "当前没有新的异常输出。"}
          </span>
        </div>
      </div>
      <div className={`diagnostics-banner ${hasRecentStderr ? "diagnostics-banner--alert" : "diagnostics-banner--quiet"}`}>
        {hasRecentStderr ? "最近异常输出已写入诊断摘要。" : "诊断摘要已准备好，当前输出平稳。"}
      </div>
      <pre className="log-surface diagnostics-surface">{diagnosticsSummary}</pre>
    </article>
  );
}
