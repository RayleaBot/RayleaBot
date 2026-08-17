# Windows Desktop Runtime

`windows-x64-full` 中的 `RayleaLauncher.exe` 使用系统安装的 Microsoft Edge WebView2 Runtime。发布包不内嵌该运行库，Microsoft Edge 浏览器本身也不能替代生产环境所需的 WebView2 Runtime。

Windows 11 包含 Evergreen WebView2 Runtime，绝大多数 Windows 10 设备也已安装。离线、精简或受企业更新策略管理的系统仍可能缺少该运行库；此时 Launcher 可能在显示窗口前退出。

联网设备从 Microsoft 的 [WebView2 Runtime 下载页](https://developer.microsoft.com/microsoft-edge/webview2/) 获取并运行 Evergreen Bootstrapper。离线设备在可联网设备上下载 x64 Evergreen Standalone Installer，传输到目标设备后安装。安装完成后重新启动 `RayleaLauncher.exe`。

运行库的检测、联网部署和离线部署要求见 Microsoft 的 [WebView2 Runtime 分发说明](https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution)。
