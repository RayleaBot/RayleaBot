import { Text } from "@fluentui/react-components";
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
  const recoveryStatusSummary = formatRecoverySummary(recoverySummary);

  return (
    <div className="env-details-flow">
      <article className="panel glass-panel env-overview-card">
        <div className="brand-eyebrow">启动前检查</div>
        <div className="env-overview-strip">
          <div className="env-overview-card__lead">
            <span className="env-overview-card__label">当前结论</span>
            <strong className="env-overview-card__title">{environmentReadiness.label}</strong>
            <Text size={200} className="panel-muted">{environmentReadiness.detail}</Text>
          </div>
          <div className="env-overview-metrics">
            <div className="metric-card metric-card--error"><Text size={100} block className="metric-label">阻塞项</Text><Text size={600} weight="bold">{groupedChecks.blocking.length}</Text></div>
            <div className="metric-card metric-card--warning"><Text size={100} block className="metric-label">警告项</Text><Text size={600} weight="bold">{groupedChecks.warnings.length}</Text></div>
            <div className="metric-card metric-card--ok"><Text size={100} block className="metric-label">正常项</Text><Text size={600} weight="bold">{groupedChecks.ready.length}</Text></div>
            <div className="metric-card"><Text size={100} block className="metric-label">平台架构</Text><Text size={300} weight="bold">{platformLabel || "—"}</Text></div>
          </div>
        </div>
        <div className="status-list env-status-grid">
          <div className="status-item"><span className="status-label">核心版本</span><span className="status-value">{snapshot.launcher.releaseCheck.currentVersion || "—"}</span></div>
          <div className="status-item"><span className="status-label">安装路径</span><span className="status-value mono">{snapshot.launcher.settings.installationRoot || "—"}</span></div>
          <div className="status-item"><span className="status-label">恢复兼容性</span><span className="status-value">{recoveryStatusSummary}</span></div>
          <div className="status-item"><span className="status-label">服务地址</span><span className="status-value mono">{snapshot.launcher.endpoint.baseUrl}</span></div>
        </div>
      </article>

      {[{ title: "系统核心", data: categorizedChecks.core }, { title: "运行环境", data: categorizedChecks.runtimes }, { title: "环境特性", data: categorizedChecks.others }]
        .filter((section) => section.data.length > 0)
        .map((section) => (
          <section key={section.title} className="env-section">
            <div className="brand-eyebrow brand-eyebrow--section">{section.title}</div>
            <div className="checks-stack checks-stack--grid">
              {section.data.map((item) => (
                <div key={item.code} className={`check-item glass-panel glass-panel--subtle check-item--${item.severity}`}>
                  <div className="check-item__lead">
                    <div className="check-item__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
                    <div className="check-item__copy">
                      <div className="check-item__headline">
                        <Text weight="bold" size={200}>{item.title}</Text>
                        <span className={`status-pill status-pill--${item.severity}`}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                      </div>
                      <Text size={100} className="check-item__summary">{item.summary}</Text>
                      {item.detail && item.detail !== item.summary && <Text size={100} className="check-item__detail">{item.detail}</Text>}
                      {item.remediation && (
                        <div className="check-item__remediation">
                          <span className="check-item__remediation-label">{item.severity === "ok" ? "离线准备" : "处理方式"}</span>
                          <Text size={100} className="check-item__remediation-text">{item.remediation}</Text>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ))}
    </div>
  );
}
