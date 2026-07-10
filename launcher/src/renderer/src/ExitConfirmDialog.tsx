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
        <DialogBody className="exit-confirm-body">
          <DialogTitle className="exit-confirm-title">
            <span className="exit-confirm-title__icon" aria-hidden="true">
              <DismissSquare20Regular />
            </span>
            <span className="exit-confirm-title__copy">
              <strong>关闭启动器</strong>
              <span>选择窗口关闭后的运行方式</span>
            </span>
          </DialogTitle>
          <DialogContent className="exit-confirm-content">
            <p className="exit-confirm-detail">
              服务可以留在系统托盘中继续运行，也可以结束启动器窗口与托盘进程。
            </p>
            <div className="exit-confirm-checkbox">
              <Checkbox
                label="记住本次选择，后续关闭窗口时直接执行"
                checked={setAsDefault}
                onChange={(_event, data) => setSetAsDefault(Boolean(data.checked))}
              />
            </div>
            <div className="exit-confirm-choices">
              <Button
                appearance="secondary"
                onClick={() => handleAction("hide")}
                className="exit-confirm-choice exit-confirm-choice--tray"
              >
                <span className="exit-confirm-choice__layout">
                  <span className="exit-confirm-choice__icon" aria-hidden="true"><Subtract20Regular /></span>
                  <span className="exit-confirm-choice__copy">
                    <strong>隐藏到托盘</strong>
                    <span>关闭主窗口，保留后台服务和托盘入口。</span>
                  </span>
                </span>
              </Button>
              <Button
                appearance="secondary"
                onClick={() => handleAction("exit")}
                className="exit-confirm-choice exit-confirm-choice--exit"
              >
                <span className="exit-confirm-choice__layout">
                  <span className="exit-confirm-choice__icon" aria-hidden="true"><SignOut20Regular /></span>
                  <span className="exit-confirm-choice__copy">
                    <strong>完全退出</strong>
                    <span>结束窗口与托盘进程，保留配置和服务文件。</span>
                  </span>
                </span>
              </Button>
            </div>
          </DialogContent>
          <div className="exit-confirm-actions">
            <Button
              appearance="subtle"
              onClick={handleClose}
              className="exit-confirm-btn exit-confirm-btn--ghost"
            >
              取消
            </Button>
          </div>
        </DialogBody>
      </DialogSurface>
    </Dialog>
  );
});
