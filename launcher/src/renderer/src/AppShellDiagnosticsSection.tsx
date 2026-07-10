import { Button } from "@fluentui/react-components";
import { CheckmarkCircle20Regular, FolderOpen20Filled, Warning20Filled } from "@fluentui/react-icons";
import type { LauncherSnapshot } from "@shared/launcher-models";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";

import { serviceStateConfig } from "./AppShell.shared";

type DiagnosticsSectionProps = {
  snapshot: LauncherSnapshot;
  diagnosticsSummary: string;
  onOpenLogs: () => void;
};

export function AppShellDiagnosticsSection({
  snapshot,
  diagnosticsSummary,
  onOpenLogs,
}: DiagnosticsSectionProps) {
  const presentation = deriveLauncherPresentation(snapshot);
  const hasRecentStderr = snapshot.launcher.recentStderr.length > 0;
  const logAlert = hasRecentStderr ? "error" : "none";

  return (
    <section className="diagnostics-workspace" data-alert={logAlert}>
      <dl className="definition-list diagnostics-meta">
        <div className="definition-row">
          <dt>服务状态</dt>
          <dd>{serviceStateConfig[presentation.state]?.label ?? "未知"}</dd>
        </div>
        <div className="definition-row" data-state={hasRecentStderr ? "danger" : "success"}>
          <dt>日志状态</dt>
          <dd>{hasRecentStderr ? "发现异常日志" : "未发现异常日志"}</dd>
        </div>
        <div className="definition-row">
          <dt>本地端点</dt>
          <dd className="mono">{snapshot.launcher.endpoint.baseUrl}</dd>
        </div>
      </dl>

      <div className="diagnostics-body">
        {hasRecentStderr ? (
          <section className="diagnostics-log" aria-labelledby="diagnostics-log-title">
            <div className="workspace-heading">
              <div className="diagnostics-log__title">
                <Warning20Filled aria-hidden="true" />
                <h3 id="diagnostics-log-title">最近异常输出</h3>
              </div>
              <span className="status-label" data-state="danger">需要检查</span>
            </div>
            <pre className="log-surface diagnostics-log-surface">{snapshot.launcher.recentStderr.join("\n")}</pre>
          </section>
        ) : (
          <div className="diagnostics-empty-state" role="status">
            <CheckmarkCircle20Regular aria-hidden="true" />
            <div>
              <strong>当前没有新的异常日志</strong>
              <span>需要完整上下文时可以打开日志目录，或展开下方技术详情。</span>
            </div>
          </div>
        )}

        <details className="technical-disclosure">
          <summary>
            <span>技术详情</span>
            <span>系统状态、路径与检查快照</span>
          </summary>
          <pre className="diagnostics-technical-surface" aria-label="诊断技术详情">{diagnosticsSummary}</pre>
        </details>
      </div>

      <div className="workspace-footer diagnostics-footer">
        <Button appearance="subtle" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>打开完整日志</Button>
      </div>
    </section>
  );
}
