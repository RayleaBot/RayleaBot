import { Button, Input, Radio, RadioGroup } from "@fluentui/react-components";
import {
  FolderOpen20Filled,
  Stop20Filled,
  Warning20Filled,
} from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";

import { closeBehaviorOptions } from "./AppShell.shared";

type SettingsSectionProps = {
  snapshot: LauncherSnapshot;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  busyAction: string | null;
  controlsDisabled: boolean;
  onUpdateInstallationRoot: (value: string) => void;
  onUpdateCloseBehavior: (value: LauncherSettings["closeBehavior"]) => void;
  onUpdateAdvancedOverride: (key: keyof LauncherAdvancedOverrides, value: string) => void;
  onChooseInstallationRoot: () => void;
  onChooseServer: () => void;
  onChooseConfig: () => void;
  onChooseWorkdir: () => void;
  onResetAdmin: () => void;
  onExit: () => void;
};

function displayPath(value: string) {
  return value.trim() || "未设置";
}

export function AppShellSettingsSection({
  snapshot,
  settingsDraft,
  resolvedSettings,
  editingSettings,
  busyAction,
  controlsDisabled,
  onUpdateInstallationRoot,
  onUpdateCloseBehavior,
  onUpdateAdvancedOverride,
  onChooseInstallationRoot,
  onChooseServer,
  onChooseConfig,
  onChooseWorkdir,
  onResetAdmin,
  onExit,
}: SettingsSectionProps) {
  const presentation = deriveLauncherPresentation(snapshot);
  const serverExecutablePath = settingsDraft.advancedOverrides?.serverExecutablePath || resolvedSettings.serverExecutablePath;
  const configPath = settingsDraft.advancedOverrides?.configPath || resolvedSettings.configPath;
  const workdir = settingsDraft.advancedOverrides?.workdir || resolvedSettings.workdir;
  const closeBehavior = closeBehaviorOptions.find((option) => option.value === settingsDraft.closeBehavior)
    ?? closeBehaviorOptions[0];

  return (
    <article className="settings-workspace" data-busy={busyAction ?? "idle"}>
      {editingSettings ? (
        <div className="attention-note settings-edit-notice" role="status">
          <strong>正在编辑设置</strong>
          <span>当前内容是草稿，保存后生效。</span>
        </div>
      ) : null}

      <section className="settings-section">
        <div className="settings-section__heading">
          <FolderOpen20Filled />
          <div><h3>路径设置</h3><p>启动器当前使用的目录和文件位置。</p></div>
        </div>

        {editingSettings ? (
          <div className="settings-path-fields">
            <label className="path-field">
              <span className="path-field__label">安装目录</span>
              <div className="path-control">
                <Input aria-label="安装目录" value={settingsDraft.installationRoot} className="settings-input settings-input--path" onChange={(_, data) => onUpdateInstallationRoot(data.value)} />
                <Button appearance="secondary" onClick={onChooseInstallationRoot} icon={<FolderOpen20Filled />}>浏览</Button>
              </div>
            </label>
            <label className="path-field">
              <span className="path-field__label">服务端程序</span>
              <div className="path-control">
                <Input aria-label="服务端程序" value={serverExecutablePath} className="settings-input settings-input--path" onChange={(_, data) => onUpdateAdvancedOverride("serverExecutablePath", data.value)} />
                <Button appearance="secondary" onClick={onChooseServer} icon={<FolderOpen20Filled />}>浏览</Button>
              </div>
            </label>
            <label className="path-field">
              <span className="path-field__label">配置文件</span>
              <div className="path-control">
                <Input aria-label="配置文件" value={configPath} className="settings-input settings-input--path" onChange={(_, data) => onUpdateAdvancedOverride("configPath", data.value)} />
                <Button appearance="secondary" onClick={onChooseConfig} icon={<FolderOpen20Filled />}>浏览</Button>
              </div>
            </label>
            <label className="path-field">
              <span className="path-field__label">进程工作目录</span>
              <div className="path-control">
                <Input aria-label="进程工作目录" value={workdir} className="settings-input settings-input--path" onChange={(_, data) => onUpdateAdvancedOverride("workdir", data.value)} />
                <Button appearance="secondary" onClick={onChooseWorkdir} icon={<FolderOpen20Filled />}>选择</Button>
              </div>
            </label>
          </div>
        ) : (
          <dl className="definition-list settings-read-list">
            <div className="definition-row"><dt>安装目录</dt><dd className="mono" title={settingsDraft.installationRoot}>{displayPath(settingsDraft.installationRoot)}</dd></div>
            <div className="definition-row"><dt>服务端程序</dt><dd className="mono" title={serverExecutablePath}>{displayPath(serverExecutablePath)}</dd></div>
            <div className="definition-row"><dt>配置文件</dt><dd className="mono" title={configPath}>{displayPath(configPath)}</dd></div>
            <div className="definition-row"><dt>进程工作目录</dt><dd className="mono" title={workdir}>{displayPath(workdir)}</dd></div>
          </dl>
        )}
      </section>

      <section className="settings-section">
        <div className="settings-section__heading">
          <div><h3>关闭行为</h3><p>关闭窗口时采用的默认动作，托盘模式会保留后台入口。</p></div>
        </div>

        {editingSettings ? (
          <RadioGroup value={settingsDraft.closeBehavior} onChange={(_, data) => onUpdateCloseBehavior(data.value as LauncherSettings["closeBehavior"])}>
            <div className="preference-options">
              {closeBehaviorOptions.map((option) => (
                <label key={option.value} className={`preference-option${settingsDraft.closeBehavior === option.value ? " is-selected" : ""}`}>
                  <Radio className="preference-radio" value={option.value} />
                  <span className="preference-option__body">
                    <span className="preference-option__title">{option.label}</span>
                    <span className="preference-option__detail">{option.detail}</span>
                  </span>
                </label>
              ))}
            </div>
          </RadioGroup>
        ) : (
          <div className="settings-choice-summary">
            <strong>{closeBehavior.label}</strong>
            <span>{closeBehavior.detail}</span>
          </div>
        )}
      </section>

      <section className="settings-section maintenance-section">
        <div className="settings-section__heading">
          <div><h3>维护操作</h3><p>用于重置本地凭据或结束启动器进程。</p></div>
        </div>
        <div className="maintenance-action-list">
          <div className="maintenance-row" data-tone="danger">
            <span className="maintenance-row__icon" aria-hidden="true"><Warning20Filled /></span>
            <div className="maintenance-row__copy">
              <strong>重置凭据</strong>
              <span>清除本地管理凭据，下次启动时重新完成初始化。</span>
            </div>
            <Button appearance="secondary" className="danger-button" onClick={onResetAdmin} disabled={controlsDisabled || presentation.state === "starting" || presentation.state === "stopping"}>立即重置</Button>
          </div>
          <div className="maintenance-row">
            <span className="maintenance-row__icon" aria-hidden="true"><Stop20Filled /></span>
            <div className="maintenance-row__copy">
              <strong>退出启动器</strong>
              <span>关闭窗口和托盘入口，不影响已保存配置与服务文件。</span>
            </div>
            <Button appearance="secondary" className="danger-outline-button" onClick={onExit}>退出启动器</Button>
          </div>
        </div>
      </section>
    </article>
  );
}
