import { Button } from "@fluentui/react-components";
import {
  CheckmarkCircle20Regular,
  ChevronRight16Regular,
  DocumentText20Regular,
  FolderOpen20Regular,
  Globe20Regular,
  Status20Regular,
  Warning20Regular,
} from "@fluentui/react-icons";
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
  const serviceState = serviceStateConfig[presentation.state];
  const hasRecentStderr = snapshot.launcher.recentStderr.length > 0;
  const baseUrl = snapshot.launcher.endpoint.baseUrl;
  const openLogs = (
    <Button appearance="secondary" className="glass glass-button" data-glass="regular" onClick={onOpenLogs} icon={<FolderOpen20Regular />}>打开完整日志</Button>
  );

  return (
    <div className="diagnostics-workspace" data-alert={hasRecentStderr ? "error" : "none"}>
      <dl className="overview-strip content-group">
        <div className="overview-strip__item">
          <dt><Status20Regular aria-hidden="true" />服务状态</dt>
          <dd>
            <span className={`status-indicator status-indicator--${serviceState?.tone ?? "neutral"}`} aria-hidden="true" />
            {serviceState?.label ?? "未知"}
          </dd>
        </div>
        <div className="overview-strip__item" data-state={hasRecentStderr ? "danger" : "success"}>
          <dt><DocumentText20Regular aria-hidden="true" />日志状态</dt>
          <dd>{hasRecentStderr ? "发现异常日志" : "未发现异常日志"}</dd>
        </div>
        <div className="overview-strip__item">
          <dt><Globe20Regular aria-hidden="true" />本地端点</dt>
          <dd><code title={baseUrl}>{baseUrl}</code></dd>
        </div>
      </dl>

      {hasRecentStderr ? (
        <section className="diagnostics-log content-group" aria-labelledby="diagnostics-log-title">
          <div className="diagnostics-log__heading">
            <div className="diagnostics-log__title">
              <Warning20Regular aria-hidden="true" />
              <h3 id="diagnostics-log-title">最近异常输出</h3>
              <span className="status-label" data-state="danger">需要检查</span>
            </div>
            {openLogs}
          </div>
          <pre className="log-surface diagnostics-log__surface">{snapshot.launcher.recentStderr.join("\n")}</pre>
        </section>
      ) : (
        <div className="diagnostics-empty content-group">
          <div className="diagnostics-empty__status" role="status">
            <span className="diagnostics-empty__icon" aria-hidden="true"><CheckmarkCircle20Regular /></span>
            <div>
              <strong>当前没有新的异常日志</strong>
              <span>需要完整上下文时可以打开日志目录，或展开下方技术详情。</span>
            </div>
          </div>
          {openLogs}
        </div>
      )}

      <details className="disclosure technical-disclosure content-group">
        <summary>
          <ChevronRight16Regular className="disclosure__chevron" aria-hidden="true" />
          <span>技术详情</span>
          <span>系统状态、路径与检查快照</span>
        </summary>
        <pre className="technical-disclosure__surface" aria-label="诊断技术详情">{diagnosticsSummary}</pre>
      </details>
    </div>
  );
}
