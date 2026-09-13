import React from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogSurface,
  DialogTitle,
} from "@fluentui/react-components";
import { ArrowDownload20Regular, Delete20Regular, Stop20Regular } from "@fluentui/react-icons";

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
    icon: <Delete20Regular />,
  },
  "stop-external": {
    title: "停止现有服务",
    lead: "确认停止当前检测到的本机 RayleaBot 服务？",
    detail: "该服务由其他进程启动。确认后，启动器会请求它停止运行。",
    confirm: "停止服务",
    icon: <Stop20Regular />,
  },
  "apply-update": {
    title: "安装更新",
    lead: "确认下载并安装新版本？",
    detail: "启动器会停止服务，覆盖安装目录中的程序文件后重新打开；配置、数据和已安装插件保持不变。建议先创建备份。",
    confirm: "安装更新",
    icon: <ArrowDownload20Regular />,
  },
} satisfies Record<ConfirmedLauncherAction, {
  title: string;
  lead: string;
  detail: string;
  confirm: string;
  icon: React.ReactNode;
}>;

export const ActionConfirmDialog = React.memo(function ActionConfirmDialog({
  action,
  onCancel,
  onConfirm,
}: ActionConfirmDialogProps) {
  const copy = action ? actionCopy[action] : actionCopy["reset-admin"];
  return (
    <Dialog
      open={action !== null}
      onOpenChange={(_event, data) => {
        if (!data.open) onCancel();
      }}
    >
      <DialogSurface className="confirmation-surface" data-tone={action === "reset-admin" ? "danger" : "attention"}>
        <DialogBody>
          <DialogTitle action={null}>
            <span aria-hidden="true">{copy.icon}</span>
            {copy.title}
          </DialogTitle>
          <DialogContent>
            <p>{copy.lead}</p>
            <p>{copy.detail}</p>
          </DialogContent>
          <DialogActions>
            <Button appearance="secondary" autoFocus onClick={onCancel}>取消</Button>
            <Button
              appearance="primary"
              className={action === "reset-admin" ? "danger-button" : "attention-button"}
              onClick={() => {
                if (action) onConfirm(action);
              }}
            >
              {copy.confirm}
            </Button>
          </DialogActions>
        </DialogBody>
      </DialogSurface>
    </Dialog>
  );
});
