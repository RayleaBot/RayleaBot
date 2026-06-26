import { Button, Input, Radio, RadioGroup, Text } from "@fluentui/react-components";
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
  const pathSurfaceTag = editingSettings ? "可编辑" : "当前生效";
  const serverExecutablePath = settingsDraft.advancedOverrides?.serverExecutablePath || resolvedSettings.serverExecutablePath;
  const configPath = settingsDraft.advancedOverrides?.configPath || resolvedSettings.configPath;
  const workdir = settingsDraft.advancedOverrides?.workdir || resolvedSettings.workdir;

  return (
    <article className="panel glass-panel settings-panel" data-busy={busyAction ?? "idle"}>
      {editingSettings && (
        <div className="settings-edit-bar glass-panel glass-panel--subtle">
          <div className="settings-edit-status">
            <span className="settings-edit-status__dot" aria-hidden="true"></span>
            <div className="settings-edit-status__copy">
              <div className="settings-edit-status__title">编辑中</div>
              <Text size={200} className="settings-edit-status__detail">当前显示草稿路径与预览结果，保存后才会切换为生效值。</Text>
            </div>
          </div>
        </div>
      )}

      <div className="settings-layout">
        <div className="settings-column settings-column--primary">
          <section className="settings-section settings-paths-panel glass-panel glass-panel--subtle">
            <div className="settings-section__header">
              <FolderOpen20Filled className="settings-section__icon" />
              <div className="panel-copy">
                <div className="brand-eyebrow brand-eyebrow--tight">路径设置</div>
                <Text size={200} className="panel-muted">当前使用的目录和文件路径。</Text>
              </div>
              <span className="settings-surface-tag">{pathSurfaceTag}</span>
            </div>
            <div className="settings-path-fields">
              <label className="path-field">
                <span className="path-field__label">安装目录</span>
                <div className="path-control">
                  <Input aria-label="安装目录" value={settingsDraft.installationRoot} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateInstallationRoot(data.value)} />
                  <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseInstallationRoot} icon={<FolderOpen20Filled />}>浏览</Button>
                </div>
              </label>
              <label className="path-field">
                <span className="path-field__label">服务端程序</span>
                <div className="path-control">
                  <Input aria-label="服务端程序" value={serverExecutablePath} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("serverExecutablePath", data.value)} />
                  <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseServer} icon={<FolderOpen20Filled />}>浏览</Button>
                </div>
              </label>
              <label className="path-field">
                <span className="path-field__label">配置文件</span>
                <div className="path-control">
                  <Input aria-label="配置文件" value={configPath} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("configPath", data.value)} />
                  <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseConfig} icon={<FolderOpen20Filled />}>浏览</Button>
                </div>
              </label>
              <label className="path-field">
                <span className="path-field__label">进程工作目录</span>
                <div className="path-control">
                  <Input aria-label="进程工作目录" value={workdir} readOnly={!editingSettings} className="frost-input frost-input--path" onChange={(_, data) => onUpdateAdvancedOverride("workdir", data.value)} />
                  <Button appearance="transparent" disabled={!editingSettings} size="small" className="frost-button frost-button--secondary frost-button--compact" onClick={onChooseWorkdir} icon={<FolderOpen20Filled />}>选择</Button>
                </div>
              </label>
            </div>
          </section>
        </div>

        <div className="settings-column settings-column--secondary">
          <section className="preferences-panel glass-panel glass-panel--subtle">
            <div className="panel-copy">
              <div className="brand-eyebrow brand-eyebrow--tight">退出行为偏好</div>
              <Text size={200} className="panel-muted">关闭窗口时采用的默认动作。托盘模式会保留后台入口。</Text>
            </div>
            <RadioGroup value={settingsDraft.closeBehavior} onChange={(_, data) => onUpdateCloseBehavior(data.value as LauncherSettings["closeBehavior"])}>
              <div className="preference-options">
                {closeBehaviorOptions.map((option) => (
                  <label key={option.value} className={`preference-option${settingsDraft.closeBehavior === option.value ? " is-selected" : ""}${!editingSettings ? " is-disabled" : ""}`}>
                    <Radio className="preference-radio" value={option.value} disabled={!editingSettings} />
                    <span className="preference-option__body">
                      <span className="preference-option__title">{option.label}</span>
                      <span className="preference-option__detail">{option.detail}</span>
                    </span>
                  </label>
                ))}
              </div>
            </RadioGroup>
          </section>

          <section className="maintenance-panel glass-panel glass-panel--subtle">
            <div className="panel-copy">
              <div className="brand-eyebrow brand-eyebrow--tight">维护操作</div>
              <Text size={200} className="panel-muted">用于重置本地凭据或直接结束启动器进程。</Text>
            </div>
            <div className="maintenance-action-list">
              <div className="maintenance-action-card maintenance-action-card--danger">
                <div className="maintenance-action-card__lead">
                  <span className="maintenance-action-card__badge" aria-hidden="true"><Warning20Filled /></span>
                  <div className="maintenance-action-card__copy">
                    <div className="maintenance-action-card__title">重置凭据</div>
                    <Text size={200} className="maintenance-action-card__detail">清除本地管理凭据，下次启动时重新完成初始化。</Text>
                  </div>
                </div>
                <Button appearance="transparent" size="small" className="frost-button frost-button--danger maintenance-action-card__button" onClick={onResetAdmin} disabled={controlsDisabled || presentation.state === "starting" || presentation.state === "stopping"}>立即重置</Button>
              </div>
              <div className="maintenance-action-card maintenance-action-card--soft">
                <div className="maintenance-action-card__lead">
                  <span className="maintenance-action-card__badge" aria-hidden="true"><Stop20Filled /></span>
                  <div className="maintenance-action-card__copy">
                    <div className="maintenance-action-card__title">退出启动器</div>
                    <Text size={200} className="maintenance-action-card__detail">关闭窗口和托盘入口，不影响已保存配置与服务文件。</Text>
                  </div>
                </div>
                <Button appearance="transparent" size="small" className="frost-button frost-button--secondary maintenance-action-card__button" onClick={onExit}>退出启动器</Button>
              </div>
            </div>
          </section>
        </div>
      </div>
    </article>
  );
}
