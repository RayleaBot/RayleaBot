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
import { ArrowSync20Regular, Delete20Regular } from "@fluentui/react-icons";

export type ConfirmedLauncherAction = "install-update" | "reset-admin";

type ActionConfirmDialogProps = {
  action: ConfirmedLauncherAction | null;
  onCancel: () => void;
  onConfirm: (action: ConfirmedLauncherAction) => void;
};

const actionCopy = {
  "install-update": {
    title: "安装更新",
    lead: "确认重启并安装已验证的更新？",
    detail: "Launcher 将先停止服务并创建离线备份，再执行事务式替换。启动后检查失败时会自动恢复上一版本。",
    confirm: "确认安装",
    icon: <ArrowSync20Regular />,
  },
  "reset-admin": {
    title: "重置管理员凭据",
    lead: "确认清除本地管理员凭据和现有会话？",
    detail: "服务会停止并重置管理员状态，随后回到首次设置流程。配置、数据和已安装插件不会被删除。",
    confirm: "确认重置",
    icon: <Delete20Regular />,
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
  const copy = action ? actionCopy[action] : actionCopy["install-update"];
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
