import React, { useCallback, useState } from "react";
import {
  Button,
  Checkbox,
  DialogActions,
  DialogBody,
  DialogContent,
  DialogTitle,
} from "@fluentui/react-components";
import {
  ChevronRight16Regular,
  Dismiss24Filled,
  SignOut20Regular,
  Subtract20Regular,
} from "@fluentui/react-icons";

import { LauncherDialog } from "./LauncherDialog";
import { StatusLens } from "./StatusLens";

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
    <LauncherDialog open={open} onDismiss={handleClose}>
      <DialogBody className="launcher-dialog__body">
        <DialogTitle className="launcher-dialog__title">
          <StatusLens tone="attention" size="small" icon={<Dismiss24Filled />} />
          <span className="launcher-dialog__heading">
            <strong>关闭启动器</strong>
            <span>选择窗口关闭后的运行方式</span>
          </span>
        </DialogTitle>
        <DialogContent className="launcher-dialog__content">
          <p className="launcher-dialog__detail">
            启动器可以留在托盘，服务继续运行；完全退出时，由启动器启动的服务会一并停止。
          </p>
          <Checkbox
            className="launcher-dialog__remember"
            label="记住本次选择，后续关闭窗口时直接执行"
            checked={setAsDefault}
            onChange={(_event, data) => setSetAsDefault(Boolean(data.checked))}
          />
          <div className="launcher-dialog__choices">
            <Button appearance="secondary" onClick={() => handleAction("hide")} className="launcher-tile">
              <span className="launcher-tile__layout">
                <span className="launcher-tile__icon" aria-hidden="true"><Subtract20Regular /></span>
                <span className="launcher-tile__copy">
                  <strong>隐藏到托盘</strong>
                  <span>关闭窗口后启动器留在托盘，服务继续运行。</span>
                </span>
                <ChevronRight16Regular className="launcher-tile__chevron" aria-hidden="true" />
              </span>
            </Button>
            <Button appearance="secondary" onClick={() => handleAction("exit")} className="launcher-tile" data-tone="danger">
              <span className="launcher-tile__layout">
                <span className="launcher-tile__icon" aria-hidden="true"><SignOut20Regular /></span>
                <span className="launcher-tile__copy">
                  <strong>完全退出</strong>
                  <span>关闭启动器；由启动器启动的服务会一并停止。</span>
                </span>
                <ChevronRight16Regular className="launcher-tile__chevron" aria-hidden="true" />
              </span>
            </Button>
          </div>
        </DialogContent>
        <DialogActions className="launcher-dialog__actions">
          <Button appearance="secondary" onClick={handleClose} className="launcher-button launcher-dialog__button">
            取消
          </Button>
        </DialogActions>
      </DialogBody>
    </LauncherDialog>
  );
});
