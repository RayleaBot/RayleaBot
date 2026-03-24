using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher;

internal sealed class LauncherCopy
{
    internal static LauncherCopy Default { get; } = new();

    internal string WindowTitle => "RayleaBot 启动器";
    internal string AppSubtitle => "本地启动与管理入口";
    internal string TitleBarHint => "启动、检查与打开管理界面";
    internal string SidebarTitle => "功能导航";
    internal string SidebarSummary => "概览、控制、检查、设置与诊断";

    internal string OverviewTitle => "总览";
    internal string OverviewSummary => "状态与下一步";
    internal string ServiceControlsTitle => "服务控制";
    internal string ServiceControlsSummary => "启动、停止与管理界面";
    internal string EnvironmentTitle => "环境检查";
    internal string EnvironmentSummary => "阻塞项与提示";
    internal string SettingsTitle => "设置";
    internal string SettingsSummary => "路径与偏好";
    internal string DiagnosticsTitle => "诊断";
    internal string DiagnosticsSummary => "错误与摘要";

    internal string HeroEyebrow => "当前状态";
    internal string StatusLabel => "当前状态";
    internal string EndpointLabel => "服务入口";
    internal string OperationLabel => "最近操作";
    internal string VersionLabel => "版本检查";
    internal string PrimaryIssueLabel => "主要问题";
    internal string SecondaryIssueLabel => "最近操作";
    internal string MainActionsTitle => "主操作";
    internal string MainActionsSummary => "根据当前状态执行操作。";
    internal string NoPrimaryIssueTitle => "当前没有阻塞问题";
    internal string NoPrimaryIssueSummary => "当前没有阻塞项。";
    internal string NoPrimaryIssueDetail => "可以继续操作，或查看环境检查了解更多信息。";

    internal string OverviewIntro => "查看当前状态、主要问题和可执行操作。";
    internal string ControlsIntro => "启动、停止服务或打开管理界面。";
    internal string SettingsIntro => "这里只保存启动器本地路径与偏好。";
    internal string EnvironmentIntro => "按优先级查看阻塞项、提示项和正常结果。";
    internal string DiagnosticsIntro => "查看最近错误输出与诊断摘要。";

    internal string ServerExecutableLabel => "服务端可执行文件";
    internal string ConfigPathLabel => "用户配置路径";
    internal string WorkdirLabel => "工作目录";
    internal string SaveSettingsLabel => "保存设置";
    internal string OpenDirectoryLabel => "打开目录";
    internal string OpenParentDirectoryLabel => "打开位置";
    internal string CopyPathLabel => "复制路径";
    internal string CopyDiagnosticsLabel => "复制诊断";
    internal string OpenLogsDirectoryLabel => "打开日志目录";
    internal string OpenWebUiLabel => "打开管理界面";
    internal string OpenInitializationLabel => "打开管理界面";
    internal string RetryHealthAuthLabel => "重试健康检查/鉴权";
    internal string OpenReleasePageLabel => "打开发布页";
    internal string StartServiceLabel => "启动服务";
    internal string StopServiceLabel => "停止服务";
    internal string CloseToTrayEnabledLabel => "关闭窗口时隐藏到托盘";
    internal string NoLauncherSession => string.Empty;
    internal string TrayQuickPanelTitle => "托盘快捷操作";
    internal string TrayQuickPanelSummary => "恢复窗口或执行常用操作。";
    internal string RestoreLauncherLabel => "恢复窗口";
    internal string TrayPanelActionLabel => "快捷操作";
    internal string TrayPanelCloseLabel => "关闭浮层";
    internal string ExitAppLabel => "完全退出";

    internal string BlockingGroupTitle => "阻塞项";
    internal string WarningGroupTitle => "需注意";
    internal string ReadyGroupTitle => "正常";
    internal string EmptyGroupHint => "当前没有项目。";
    internal string RecentStderrTitle => "最近错误输出";
    internal string DiagnosticsSummaryTitle => "诊断摘要";
    internal string RawDiagnosticsEmpty => "暂无错误输出。";

    internal string TrayOpenLauncherLabel => "打开启动器";
    internal string TrayOpenWebLabel => "打开管理界面";
    internal string TrayExitLabel => "完全退出";
    internal string TrayTooltip => "RayleaBot 启动器";

    internal string CloseDialogTitle => "关闭窗口";
    internal string CloseDialogBody => "可以隐藏到托盘继续运行，也可以直接完全退出。";
    internal string CloseDialogFootnote => "隐藏后仍可从托盘恢复窗口或执行常用操作。";
    internal string HideToTrayLabel => "隐藏到托盘";
    internal string ExitCompletelyLabel => "完全退出";

    internal string VersionUnavailableSummary => "暂时无法检查版本";
    internal string VersionUnavailableDetail => "当前开发运行没有版本包信息。";
    internal string VersionPageUnavailable => "当前运行没有可打开的发布页。";

    internal string ActionLauncherInitialized => "已完成本地检查。";
    internal string ActionHealthRetryFinished => "已重新检查服务状态。";
    internal string ActionSettingsSaved => "启动器设置已保存。";
    internal string ActionStartFinished => "启动请求已完成。";
    internal string ActionStopFinished => "停止请求已完成。";
    internal string ActionWebOpened => "已在默认浏览器中打开管理界面。";
    internal string ActionInitializationOpened => "已在默认浏览器中打开管理界面。";
    internal string ActionLogsOpened => "已打开日志目录。";
    internal string ActionReleasePageOpened => "已在默认浏览器中打开发布页。";
    internal string ActionDiagnosticsCopied => "诊断摘要已复制到剪贴板。";
    internal string ActionRestoredFromTray => "启动器已从系统托盘恢复。";
    internal string ActionHiddenToTray => "启动器仍在系统托盘中运行。";
    internal string DetectedServiceSummary => "检测到现有服务";
    internal string DetectedServiceDetail => "端口上已有服务正在运行。可以直接打开管理界面，或先停止它再由启动器重新启动。";
    internal string ExternalStopConfirmTitle => "停止现有服务";
    internal string ExternalStopConfirmBody => "当前端口上的服务不是由启动器拉起。继续后会尝试停止这个现有服务。";
    internal string ExternalStopConfirmFootnote => "如果这是另一个独立运行中的实例，请先确认停止它不会影响当前工作。";
    internal string ExternalStopConfirmAction => "继续停止";
    internal string ExternalStopCancelAction => "取消";

    internal string FormatStatusSummary(LauncherServiceState state) =>
        state switch
        {
            LauncherServiceState.Stopped => "未启动",
            LauncherServiceState.Starting => "启动中",
            LauncherServiceState.ExternalService => "检测到现有服务",
            LauncherServiceState.HealthOnly => "运行中",
            LauncherServiceState.Ready => "运行中",
            LauncherServiceState.Degraded => "运行中",
            LauncherServiceState.SetupRequired => "运行中",
            LauncherServiceState.ShuttingDown => "停止中",
            LauncherServiceState.Failed => "启动失败",
            _ => "未知状态",
        };

    internal string FormatHeroTitle(LauncherServiceState state, IReadOnlyList<EnvironmentCheckResult> checks)
    {
        if (state == LauncherServiceState.Stopped &&
            checks.Any(item => string.Equals(item.Code, "config.bootstrap_available", StringComparison.Ordinal)))
        {
            return "首次启动前还差一步";
        }

        return state switch
        {
            LauncherServiceState.Stopped => "服务未启动",
            LauncherServiceState.Starting => "正在启动服务",
            LauncherServiceState.ExternalService => "检测到现有服务",
            LauncherServiceState.HealthOnly => "服务正在运行",
            LauncherServiceState.Ready => "服务正在运行",
            LauncherServiceState.Degraded => "服务正在运行",
            LauncherServiceState.SetupRequired => "服务正在运行",
            LauncherServiceState.ShuttingDown => "正在停止服务",
            LauncherServiceState.Failed => "服务启动失败",
            _ => "启动器状态",
        };
    }

    internal string FormatSeverityLabel(CheckSeverity severity) =>
        severity switch
        {
            CheckSeverity.Ok => "正常",
            CheckSeverity.Warning => "需注意",
            _ => "阻塞",
        };

    internal string FormatReleaseUpToDate(string currentVersion) =>
        $"当前版本 {currentVersion} 已是最新。";

    internal string FormatReleaseUpdateAvailable(string currentVersion, string latestVersion) =>
        $"发现新版本：{currentVersion} -> {latestVersion}。";

    internal string FormatReleaseFeedError() => "暂时无法连接版本源。";

    internal string FormatDirectoryOpenFailed(string path) => $"无法打开目录：{path}";

    internal string FormatPathCopied(string label) => $"已复制{label}。";
}
