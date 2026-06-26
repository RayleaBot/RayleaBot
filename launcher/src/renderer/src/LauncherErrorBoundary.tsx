import { Component, type ErrorInfo, type ReactNode } from "react";

type LauncherErrorBoundaryProps = {
  children: ReactNode;
};

type LauncherErrorBoundaryState = {
  hasError: boolean;
};

export class LauncherErrorBoundary extends Component<LauncherErrorBoundaryProps, LauncherErrorBoundaryState> {
  state: LauncherErrorBoundaryState = {
    hasError: false,
  };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(_error: Error, _errorInfo: ErrorInfo) {}

  render() {
    if (this.state.hasError) {
      return (
        <div className="launcher-loading-shell launcher-loading-shell--error">
          <div className="launcher-loading-shell__eyebrow">Raylea 启动器</div>
          <h1 className="launcher-loading-shell__title">启动器界面暂时不可用</h1>
          <p className="launcher-loading-shell__detail">请关闭后重新打开 Raylea 启动器。</p>
        </div>
      );
    }

    return this.props.children;
  }
}
