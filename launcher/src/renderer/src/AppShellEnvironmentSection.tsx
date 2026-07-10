import { getEnvironmentSummaryLabel, resolveRecoverySummary } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";

import { formatRecoverySummary } from "./AppShell.copy";
import { severityConfig, sortChecks } from "./AppShell.shared";

type EnvironmentSectionProps = {
  snapshot: LauncherSnapshot;
  platformLabel: string;
};

export function AppShellEnvironmentSection({
  snapshot,
  platformLabel,
}: EnvironmentSectionProps) {
  const checks = sortChecks(snapshot.launcher.preflightChecks || []);
  const groupedChecks = {
    blocking: checks.filter((item) => item.severity === "error"),
    warnings: checks.filter((item) => item.severity === "warning"),
    ready: checks.filter((item) => item.severity === "ok"),
  };
  const categorizedChecks = (() => {
    const corePrefixes = ["server.", "config.", "workdir."];
    const runtimePrefixes = ["deps.", "chromium.", "python.", "nodejs.", "npm."];
    return {
      core: sortChecks(checks.filter((item) => corePrefixes.some((prefix) => item.code.startsWith(prefix)))),
      runtimes: sortChecks(checks.filter((item) => runtimePrefixes.some((prefix) => item.code.startsWith(prefix)))),
      others: sortChecks(
        checks.filter(
          (item) =>
            !corePrefixes.some((prefix) => item.code.startsWith(prefix))
            && !runtimePrefixes.some((prefix) => item.code.startsWith(prefix)),
        ),
      ),
    };
  })();
  const environmentSummaryLabel = getEnvironmentSummaryLabel(snapshot.launcher.preflightChecks);
  const environmentReadiness =
    environmentSummaryLabel === "需要处理"
      ? { label: environmentSummaryLabel, detail: "存在阻塞项，启动前需要先解决。" }
      : environmentSummaryLabel === "可继续，但有警告"
        ? { label: environmentSummaryLabel, detail: "核心能力可用，建议先检查告警项。" }
        : { label: environmentSummaryLabel, detail: "当前未发现阻塞或告警项。" };
  const recoverySummary = resolveRecoverySummary(snapshot);
  const recoveryStatusSummary = recoverySummary ? formatRecoverySummary(recoverySummary) : "";
  const categories = [
    { key: "core", title: "系统核心", data: categorizedChecks.core },
    { key: "runtimes", title: "运行环境", data: categorizedChecks.runtimes },
    { key: "others", title: "环境特性", data: categorizedChecks.others },
  ].filter((section) => section.data.length > 0);
  const totalChecks = checks.length;
  const allChecksReady = groupedChecks.blocking.length === 0 && groupedChecks.warnings.length === 0;

  return (
    <div className="environment-workspace">
      <section className="environment-summary" aria-labelledby="environment-summary-title">
        <div>
          <div className="section-kicker">启动前检查</div>
          <h2 id="environment-summary-title">{environmentReadiness.label}</h2>
          <p>{environmentReadiness.detail}</p>
        </div>
        <div className="count-badges" aria-label="检查计数">
          {groupedChecks.blocking.length > 0 ? <span data-state="danger">阻塞 {groupedChecks.blocking.length}</span> : null}
          {groupedChecks.warnings.length > 0 ? <span data-state="warning">警告 {groupedChecks.warnings.length}</span> : null}
          {totalChecks > 0 ? (
            <span data-state={allChecksReady ? "neutral" : "success"}>{groupedChecks.ready.length}/{totalChecks} 正常</span>
          ) : (
            <span>暂无检查项</span>
          )}
          <span>平台 {platformLabel || "—"}</span>
        </div>
      </section>

      <dl className="definition-list environment-meta">
        {snapshot.launcher.releaseCheck.currentVersion ? (
          <div className="definition-row"><dt>核心版本</dt><dd>{snapshot.launcher.releaseCheck.currentVersion}</dd></div>
        ) : null}
        {snapshot.launcher.settings.installationRoot ? (
          <div className="definition-row"><dt>安装路径</dt><dd className="mono">{snapshot.launcher.settings.installationRoot}</dd></div>
        ) : null}
        {recoverySummary ? (
          <div className="definition-row"><dt>恢复兼容性</dt><dd>{recoveryStatusSummary}</dd></div>
        ) : null}
        <div className="definition-row"><dt>服务地址</dt><dd className="mono">{snapshot.launcher.endpoint.baseUrl}</dd></div>
      </dl>

      {categories.map((section) => {
        const issues = section.data.filter((item) => item.severity !== "ok");
        const healthy = section.data.filter((item) => item.severity === "ok");
        return (
          <section key={section.key} className="check-group" aria-labelledby={`check-group-${section.key}`}>
            <div className="check-group__heading">
              <h3 id={`check-group-${section.key}`}>{section.title}</h3>
              {issues.length > 0 ? <span>{issues.length} 项需要处理</span> : null}
            </div>

            {issues.length > 0 ? (
              <div className="check-list check-list--issues">
                {issues.map((item) => (
                  <div key={item.code} className="check-row" data-severity={item.severity}>
                    <span className="check-row__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</span>
                    <div className="check-row__copy">
                      <div className="check-row__heading">
                        <strong>{item.title}</strong>
                        <span className="status-label" data-state={item.severity}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                      </div>
                      <p>{item.summary}</p>
                      {item.detail && item.detail !== item.summary ? <p className="muted">{item.detail}</p> : null}
                      {item.remediation ? (
                        <div className="check-row__remediation">
                          <strong>处理方式</strong>
                          <span>{item.remediation}</span>
                        </div>
                      ) : null}
                    </div>
                  </div>
                ))}
              </div>
            ) : null}

            {healthy.length > 0 ? (
              <details className="healthy-disclosure">
                <summary>
                  <span>{issues.length > 0 ? "查看正常项" : "检查全部正常"}</span>
                  <span>{healthy.length} 项通过</span>
                </summary>
                <div className="check-list check-list--healthy">
                  {healthy.map((item) => (
                    <div key={item.code} className="check-row check-row--healthy" data-severity="ok">
                      <span className="check-row__icon">{severityConfig.ok.icon}</span>
                      <div className="check-row__copy">
                        <strong>{item.title}</strong>
                        <p>{item.summary}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </details>
            ) : null}
          </section>
        );
      })}
    </div>
  );
}
