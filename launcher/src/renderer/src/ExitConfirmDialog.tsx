import React, { useCallback, useState } from "react";
import {
  Button,
  Checkbox,
  Dialog,
  DialogBody,
  DialogContent,
  DialogSurface,
  DialogTitle,
} from "@fluentui/react-components";
import {
  Dismiss20Regular,
  DismissSquare20Regular,
  Subtract20Regular,
  SignOut20Regular,
} from "@fluentui/react-icons";

export type ExitConfirmDialogProps = {
  open: boolean;
  onClose: () => void;
  onConfirm: (action: "hide" | "exit", setAsDefault: boolean) => void;
};

export const ExitConfirmDialog = React.memo(function ExitConfirmDialog({ open, onClose, onConfirm }: ExitConfirmDialogProps) {
  const [setAsDefault, setSetAsDefault] = useState(false);

  const handleAction = useCallback(
    (action: "hide" | "exit") => {
      onConfirm(action, setAsDefault);
      setSetAsDefault(false);
    },
    [onConfirm, setAsDefault],
  );

  const handleClose = useCallback(() => {
    setSetAsDefault(false);
    onClose();
  }, [onClose]);

  return (
    <Dialog open={open} onOpenChange={(_event, data) => {
      if (!data.open) handleClose();
    }}>
      <DialogSurface className="exit-confirm-surface">
        <DialogBody>
          <DialogTitle className="exit-confirm-title">
            <span className="exit-confirm-title__icon" aria-hidden="true">
              <DismissSquare20Regular />
            </span>
            关闭窗口
          </DialogTitle>
          <DialogContent className="exit-confirm-content">
            <p className="exit-confirm-lead">
              选择关闭后的行为
            </p>
            <p className="exit-confirm-detail">
              保留到托盘可在后台继续运行服务；完全退出将结束启动器进程。
            </p>
            <div className="exit-confirm-checkbox">
              <Checkbox
                label="将本次选择设为默认行为"
                checked={setAsDefault}
                onChange={(_event, data) => setSetAsDefault(Boolean(data.checked))}
              />
            </div>
          </DialogContent>
          <div className="exit-confirm-actions">
            <Button
              appearance="transparent"
              onClick={handleClose}
              icon={<Dismiss20Regular />}
              className="exit-confirm-btn exit-confirm-btn--ghost"
            >
              取消
            </Button>
            <div className="exit-confirm-actions__spacer" />
            <Button
              appearance="outline"
              onClick={() => handleAction("hide")}
              icon={<Subtract20Regular />}
              className="exit-confirm-btn exit-confirm-btn--accent"
            >
              隐藏到托盘
            </Button>
            <Button
              appearance="primary"
              onClick={() => handleAction("exit")}
              icon={<SignOut20Regular />}
              className="exit-confirm-btn exit-confirm-btn--danger"
            >
              完全退出
            </Button>
          </div>
        </DialogBody>
      </DialogSurface>
    </Dialog>
  );
});
