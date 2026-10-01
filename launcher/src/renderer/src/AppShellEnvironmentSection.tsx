import {
  CheckmarkCircle20Regular,
  Desktop20Regular,
  ErrorCircle20Regular,
  Folder20Regular,
  Globe20Regular,
  QuestionCircle20Regular,
  Tag20Regular,
  Warning20Regular,
} from "@fluentui/react-icons";
import { getEnvironmentSummaryLabel, isBlockingEnvironmentIssue } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import type { ReactNode } from "react";

import { isRuntimePreparationIssue, severityConfig, sortChecks } from "./AppShell.shared";
import { DetailRow } from "./AppShellDetailList";
import { Disclosure } from "./Disclosure";

type EnvironmentSectionProps = {
  snapshot: LauncherSnapshot;
  platformLabel: string;
};

type ReadinessTone = "neutral" | "success" | "warning" | "danger";

const readinessIcons: Record<ReadinessTone, ReactNode> = {
  neutral: <QuestionCircle20Regular />,
  success: <CheckmarkCircle20Regular />,
  warning: <Warning20Regular />,
  danger: <ErrorCircle20Regular />,
};

export function AppShellEnvironmentSection({
  snapshot,
  platformLabel,
}: EnvironmentSectionProps) {
  const checks = sortChecks(snapshot.launcher.preflightChecks || []);
  const groupedChecks: Record<"blocking" | "warnings" | "ready", typeof checks> = {
    blocking: [],
    warnings: [],
    ready: [],
  };
  const categorizedChecks: Record<"installation" | "runtimes", typeof checks> = {
    installation: [],
    runtimes: [],
  };
  for (const item of checks) {
    groupedChecks[item.severity === "error" ? "blocking" : item.severity === "warning" ? "warnings" : "ready"].push(item);
    categorizedChecks[isRuntimePreparationIssue(item.code) ? "runtimes" : "installation"].push(item);
  }
  const checksUnavailable = snapshot.launcher.preflightChecks.length === 0;
  const summaryLabel = getEnvironmentSummaryLabel(snapshot.launcher.preflightChecks);
  const readiness: { tone: ReadinessTone; label: string; detail: string } = checksUnavailable
    ? snapshot.launcher.lastLocalError
      ? { tone: "danger", label: "检查结果不可用", detail: "未能获取环境检查结果，请重新检查。" }
      : { tone: "neutral", label: summaryLabel, detail: "完成环境检查后，才能确认服务是否具备启动条件。" }
    : checks.some(isBlockingEnvironmentIssue)
      ? { tone: "danger", label: summaryLabel, detail: "存在阻塞项，启动前需要先解决。" }
      : groupedChecks.warnings.length > 0
        ? { tone: "warning", label: summaryLabel, detail: "核心能力可用，建议先检查警告项。" }
        : { tone: "success", label: summaryLabel, detail: "当前未发现阻塞或警告项。" };
  const categories = [
    { key: "installation", title: "安装与配置", data: categorizedChecks.installation },
    { key: "runtimes", title: "运行环境", data: categorizedChecks.runtimes },
  ].filter((section) => section.data.length > 0);
  const totalChecks = checks.length;
  const allChecksReady = groupedChecks.blocking.length === 0 && groupedChecks.warnings.length === 0;
  const { endpoint, releaseCheck, settings } = snapshot.launcher;

  return (
    <div className="environment-workspace">
      <section className="environment-summary" data-tone={readiness.tone} aria-labelledby="environment-summary-title">
        <span className="environment-summary__icon" aria-hidden="true">{readinessIcons[readiness.tone]}</span>
        <div className="environment-summary__copy">
          <h2 id="environment-summary-title">{readiness.label}</h2>
          <p>{readiness.detail}</p>
        </div>
        <div className="count-badges" aria-label="检查计数">
          {groupedChecks.blocking.length > 0 ? <span data-state="danger">阻塞 {groupedChecks.blocking.length}</span> : null}
          {groupedChecks.warnings.length > 0 ? <span data-state="warning">警告 {groupedChecks.warnings.length}</span> : null}
          {totalChecks > 0 ? (
            <span data-state={allChecksReady ? "neutral" : "success"}>{groupedChecks.ready.length}/{totalChecks} 正常</span>
          ) : (
            <span>暂无检查项</span>
          )}
        </div>
      </section>

      {categories.map((section) => {
        const issues = section.data.filter((item) => item.severity !== "ok");
        const healthy = section.data.filter((item) => item.severity === "ok");
        return (
          <section key={section.key} className="workspace-group" aria-labelledby={`check-group-${section.key}`}>
            <div className="workspace-group__header">
              <h3 id={`check-group-${section.key}`} className="workspace-group__title">{section.title}</h3>
              {issues.length > 0 ? <span className="workspace-group__meta">{issues.length} 项需要处理</span> : null}
            </div>

            <div className="check-list content-group">
              {issues.map((item) => (
                <div key={item.code} className="check-row" data-severity={item.severity}>
                  <span className="check-row__icon" aria-hidden="true">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</span>
                  <div className="check-row__copy">
                    <div className="check-row__heading">
                      <strong>{item.title}</strong>
                      <span className="status-label" data-state={item.severity}>{severityConfig[item.severity as keyof typeof severityConfig]?.label}</span>
                    </div>
                    <p>{item.summary}</p>
                    {item.detail && item.detail !== item.summary ? <p>{item.detail}</p> : null}
                    {item.remediation ? (
                      <div className="check-row__remediation">
                        <strong>处理方式</strong>
                        <span>{item.remediation}</span>
                      </div>
                    ) : null}
                  </div>
                </div>
              ))}

              {healthy.length > 0 ? (
                <Disclosure
                  className="disclosure check-disclosure"
                  title={issues.length > 0 ? "查看正常项" : "检查全部正常"}
                  meta={`${healthy.length} 项通过`}
                >
                  <div className="check-disclosure__list">
                    {healthy.map((item) => (
                      <div key={item.code} className="check-row check-row--healthy" data-severity="ok">
                        <span className="check-row__icon" aria-hidden="true">{severityConfig.ok.icon}</span>
                        <div className="check-row__copy">
                          <strong>{item.title}</strong>
                          <p>{item.summary}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </Disclosure>
              ) : null}
            </div>
          </section>
        );
      })}

      <section className="workspace-group" aria-labelledby="environment-info-title">
        <h3 id="environment-info-title" className="workspace-group__title">环境信息</h3>
        <dl className="detail-list detail-list--wrap content-group">
          <DetailRow icon={<Desktop20Regular />} label="平台" value={platformLabel || "—"} />
          {releaseCheck.currentVersion ? (
            <DetailRow icon={<Tag20Regular />} label="RayleaBot 版本" value={releaseCheck.currentVersion} mono={false} />
          ) : null}
          {settings.installationRoot ? (
            <DetailRow icon={<Folder20Regular />} label="安装目录" value={settings.installationRoot} />
          ) : null}
          <DetailRow icon={<Globe20Regular />} label="管理界面地址" value={endpoint.baseUrl} />
        </dl>
      </section>
    </div>
  );
}
