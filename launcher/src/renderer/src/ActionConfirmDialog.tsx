import React, { useState } from "react";
import {
  Button,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogTitle,
} from "@fluentui/react-components";
import { ArrowDownload24Filled, Delete24Filled, Stop24Filled } from "@fluentui/react-icons";

import { LauncherDialog } from "./LauncherDialog";
import { StatusLens } from "./StatusLens";

export type ConfirmedLauncherAction = "reset-admin" | "stop-external" | "apply-update";

type ActionConfirmDialogProps = {
  action: ConfirmedLauncherAction | null;
  onCancel: () => void;
  onConfirm: (action: ConfirmedLauncherAction) => void;
};

const actionCopy = {
  "reset-admin": {
    title: "重置管理员凭据",
    lead: "确认清除本地管理员凭据和现有会话？",
    detail: "服务会停止并重置管理员状态，随后回到首次设置流程。配置、数据和已安装插件不会被删除。",
    confirm: "确认重置",
    icon: <Delete24Filled />,
    tone: "danger",
  },
  "stop-external": {
    title: "停止现有服务",
    lead: "确认停止当前检测到的本机 RayleaBot 服务？",
    detail: "该服务由其他进程启动。确认后，启动器会请求它停止运行。",
    confirm: "停止服务",
    icon: <Stop24Filled />,
    tone: "attention",
  },
  "apply-update": {
    title: "安装更新",
    lead: "确认下载并安装新版本？",
    detail: "启动器会停止服务，覆盖安装目录中的程序文件后重新打开；配置、数据和已安装插件保持不变。建议先创建备份。",
    confirm: "安装更新",
    icon: <ArrowDownload24Filled />,
    tone: "attention",
  },
} satisfies Record<ConfirmedLauncherAction, {
  title: string;
  lead: string;
  detail: string;
  confirm: string;
  icon: React.ReactNode;
  tone: "danger" | "attention";
}>;

export const ActionConfirmDialog = React.memo(function ActionConfirmDialog({
  action,
  onCancel,
  onConfirm,
}: ActionConfirmDialogProps) {
  // The dialog keeps the last action's copy while it animates out.
  const [shownAction, setShownAction] = useState<ConfirmedLauncherAction>(action ?? "reset-admin");
  if (action !== null && action !== shownAction) {
    setShownAction(action);
  }
  const copy = actionCopy[action ?? shownAction];
  return (
    <LauncherDialog open={action !== null} onDismiss={onCancel} tone={copy.tone}>
      <DialogBody className="launcher-dialog__body">
        <DialogTitle className="launcher-dialog__title" action={null}>
          <StatusLens tone={copy.tone} size="small" icon={copy.icon} />
          <span className="launcher-dialog__heading">
            <strong>{copy.title}</strong>
          </span>
        </DialogTitle>
        <DialogContent className="launcher-dialog__content">
          <p className="launcher-dialog__lead">{copy.lead}</p>
          <p className="launcher-dialog__detail">{copy.detail}</p>
        </DialogContent>
        <DialogActions className="launcher-dialog__actions">
          <Button appearance="secondary" autoFocus className="launcher-button launcher-dialog__button" onClick={onCancel}>取消</Button>
          <Button
            appearance="primary"
            className="launcher-button launcher-dialog__button launcher-dialog__button--confirm"
            data-tone={copy.tone}
            onClick={() => {
              if (action) onConfirm(action);
            }}
          >
            {copy.confirm}
          </Button>
        </DialogActions>
      </DialogBody>
    </LauncherDialog>
  );
});
