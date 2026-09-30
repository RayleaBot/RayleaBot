import { Button, Input, Radio, RadioGroup } from "@fluentui/react-components";
import {
  DocumentSettings20Regular,
  Folder20Regular,
  FolderOpen20Regular,
  KeyReset20Regular,
  Server20Regular,
  SignOut20Regular,
} from "@fluentui/react-icons";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";
import type { ReactNode } from "react";

import { closeBehaviorOptions } from "./AppShell.shared";
import { DetailRow } from "./AppShellDetailList";

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

type PathFieldProps = {
  icon: ReactNode;
  label: string;
  value: string;
  chooseLabel: string;
  disabled: boolean;
  onChange: (value: string) => void;
  onChoose: () => void;
};

function displayPath(value: string) {
  return value.trim() || "未设置";
}

function PathField({ icon, label, value, chooseLabel, disabled, onChange, onChoose }: PathFieldProps) {
  return (
    <div className="field-row">
      <span className="field-row__label">
        <span className="detail-list__icon" aria-hidden="true">{icon}</span>
        {label}
      </span>
      <div className="field-row__control">
        <Input aria-label={label} value={value} disabled={disabled} className="settings-input settings-input--path" onChange={(_, data) => onChange(data.value)} />
        <Button appearance="secondary" className="launcher-button" data-emphasis="regular" onClick={onChoose} disabled={disabled} icon={<FolderOpen20Regular />}>{chooseLabel}</Button>
      </div>
    </div>
  );
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
  const resetDisabled = controlsDisabled || presentation.state === "starting" || presentation.state === "stopping";

  return (
    <div className="settings-workspace" data-busy={busyAction ?? "idle"}>
      {editingSettings ? (
        <div className="attention-note" role="status">
          <strong>正在编辑设置</strong>
          <span>当前内容是草稿，保存后生效。</span>
        </div>
      ) : null}

      <section className="workspace-group" aria-labelledby="settings-paths-title">
        <div className="workspace-group__header">
          <h3 id="settings-paths-title" className="workspace-group__title">路径设置</h3>
          <p className="workspace-group__description">启动器当前使用的目录和文件位置。</p>
        </div>

        {editingSettings ? (
          <div className="field-list content-group">
            <PathField icon={<Folder20Regular />} label="安装目录" value={settingsDraft.installationRoot} chooseLabel="浏览" disabled={controlsDisabled} onChange={onUpdateInstallationRoot} onChoose={onChooseInstallationRoot} />
            <PathField icon={<Server20Regular />} label="服务端程序" value={serverExecutablePath} chooseLabel="浏览" disabled={controlsDisabled} onChange={(value) => onUpdateAdvancedOverride("serverExecutablePath", value)} onChoose={onChooseServer} />
            <PathField icon={<DocumentSettings20Regular />} label="配置文件" value={configPath} chooseLabel="浏览" disabled={controlsDisabled} onChange={(value) => onUpdateAdvancedOverride("configPath", value)} onChoose={onChooseConfig} />
            <PathField icon={<FolderOpen20Regular />} label="进程工作目录" value={workdir} chooseLabel="选择" disabled={controlsDisabled} onChange={(value) => onUpdateAdvancedOverride("workdir", value)} onChoose={onChooseWorkdir} />
          </div>
        ) : (
          <dl className="detail-list detail-list--wrap content-group">
            <DetailRow icon={<Folder20Regular />} label="安装目录" value={displayPath(settingsDraft.installationRoot)} title={settingsDraft.installationRoot || undefined} />
            <DetailRow icon={<Server20Regular />} label="服务端程序" value={displayPath(serverExecutablePath)} title={serverExecutablePath || undefined} />
            <DetailRow icon={<DocumentSettings20Regular />} label="配置文件" value={displayPath(configPath)} title={configPath || undefined} />
            <DetailRow icon={<FolderOpen20Regular />} label="进程工作目录" value={displayPath(workdir)} title={workdir || undefined} />
          </dl>
        )}
      </section>

      <section className="workspace-group" aria-labelledby="settings-close-title">
        <div className="workspace-group__header">
          <h3 id="settings-close-title" className="workspace-group__title">关闭行为</h3>
          <p className="workspace-group__description">关闭窗口时采用的默认动作，托盘模式会保留后台入口。</p>
        </div>

        {editingSettings ? (
          <RadioGroup
            className="choice-group"
            value={settingsDraft.closeBehavior}
            disabled={controlsDisabled}
            aria-labelledby="settings-close-title"
            onChange={(_, data) => onUpdateCloseBehavior(data.value as LauncherSettings["closeBehavior"])}
          >
            <div className="choice-list content-group">
              {closeBehaviorOptions.map((option) => (
                <label key={option.value} className="choice-row" data-selected={settingsDraft.closeBehavior === option.value}>
                  <Radio className="choice-row__radio" value={option.value} />
                  <span className="choice-row__body">
                    <span className="choice-row__title">{option.label}</span>
                    <span className="choice-row__detail">{option.detail}</span>
                  </span>
                </label>
              ))}
            </div>
          </RadioGroup>
        ) : (
          <div className="choice-summary content-group">
            <strong>{closeBehavior.label}</strong>
            <span>{closeBehavior.detail}</span>
          </div>
        )}
      </section>

      <section className="workspace-group" aria-labelledby="settings-maintenance-title">
        <div className="workspace-group__header">
          <h3 id="settings-maintenance-title" className="workspace-group__title">维护操作</h3>
          <p className="workspace-group__description">用于重置本地凭据或结束启动器进程。</p>
        </div>

        <div className="action-list content-group">
          <div className="action-row" data-tone="danger">
            <span className="action-row__icon" aria-hidden="true"><KeyReset20Regular /></span>
            <div className="action-row__copy">
              <strong>重置凭据</strong>
              <span>清除本地管理凭据，下次启动时重新完成初始化。</span>
            </div>
            <Button appearance="secondary" className="launcher-button launcher-button--danger" data-emphasis="regular" onClick={onResetAdmin} disabled={resetDisabled}>立即重置</Button>
          </div>
          <div className="action-row">
            <span className="action-row__icon" aria-hidden="true"><SignOut20Regular /></span>
            <div className="action-row__copy">
              <strong>退出启动器</strong>
              <span>关闭启动器窗口和托盘入口。</span>
            </div>
            <Button appearance="secondary" className="launcher-button launcher-button--danger" data-emphasis="regular" onClick={onExit} disabled={controlsDisabled}>退出启动器</Button>
          </div>
        </div>
      </section>
    </div>
  );
}
