using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher;

internal sealed class LauncherCopy
{
    internal static LauncherCopy Default { get; } = new();

    internal string WindowTitle => "RayleaBot 启动器";
    internal string AppSubtitle => "本地启动与恢复";
    internal string TitleBarHint => "启动、检查与恢复";
    internal string SidebarTitle => "功能导航";
    internal string SidebarSummary => "切换页面查看状态、操作和诊断。";

    internal string OverviewTitle => "总览";
    internal string OverviewSummary => "状态与下一步";
    internal string ServiceControlsTitle => "服务控制";
    internal string ServiceControlsSummary => "启动、停止与常用操作";
    internal string EnvironmentTitle => "环境检查";
    internal string EnvironmentSummary => "阻塞项与提示项";
    internal string SettingsTitle => "设置";
    internal string SettingsSummary => "本地路径与偏好";
    internal string DiagnosticsTitle => "诊断";
    internal string DiagnosticsSummary => "日志与诊断摘要";

    internal string HeroEyebrow => "当前状态";
    internal string StatusLabel => "当前状态";
    internal string EndpointLabel => "服务入口";
    internal string SessionLabel => "会话状态";
    internal string OperationLabel => "最近操作";
    internal string VersionLabel => "版本检查";
    internal string PrimaryIssueLabel => "主要问题";
    internal string SecondaryIssueLabel => "补充说明";
    internal string MainActionsTitle => "常用操作";
    internal string MainActionsSummary => "先处理当前问题，再执行操作。";
    internal string NoPrimaryIssueTitle => "当前没有需要立即处理的问题";
    internal string NoPrimaryIssueSummary => "可以直接启动服务、打开管理界面或继续检查环境。";
    internal string NoPrimaryIssueDetail => "更详细的信息保留在环境检查和诊断页。";

    internal string OverviewIntro => "先看状态，再决定下一步。";
    internal string ControlsIntro => "按钮会随当前状态自动启用或禁用。";
    internal string SettingsIntro => "这里只保存启动器本地路径。平台配置仍以 default.yaml 与 user.yaml 为准。";
    internal string EnvironmentIntro => "先处理阻塞项，再处理提示项。";
    internal string DiagnosticsIntro => "需要排查时再看这里。";

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
    internal string RetryHealthAuthLabel => "重试健康检查/鉴权";
    internal string OpenReleasePageLabel => "打开发布页";
    internal string StartServiceLabel => "启动服务";
    internal string StopServiceLabel => "停止服务";
    internal string CloseToTrayEnabledLabel => "关闭窗口时隐藏到托盘";
    internal string NoLauncherSession => "无启动器会话。";
    internal string TrayQuickPanelTitle => "托盘快捷面板";
    internal string TrayQuickPanelSummary => "可在这里恢复窗口、执行常用操作或直接退出。";
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
    internal string VersionUnavailableDetail => "当前开发运行未附带版本包信息。";
    internal string VersionPageUnavailable => "当前运行没有可打开的发布页。";

    internal string ActionLauncherInitialized => "启动器已初始化。";
    internal string ActionHealthRetryFinished => "健康检查与鉴权已刷新。";
    internal string ActionSettingsSaved => "启动器设置已保存。";
    internal string ActionStartFinished => "启动请求已完成。";
    internal string ActionStopFinished => "停止请求已完成。";
    internal string ActionWebOpened => "已在默认浏览器中打开管理界面。";
    internal string ActionLogsOpened => "已打开日志目录。";
    internal string ActionReleasePageOpened => "已在默认浏览器中打开发布页。";
    internal string ActionDiagnosticsCopied => "诊断摘要已复制到剪贴板。";
    internal string ActionRestoredFromTray => "启动器已从系统托盘恢复。";
    internal string ActionHiddenToTray => "启动器仍在系统托盘中运行。";

    internal string FormatStatusSummary(LauncherServiceState state) =>
        state switch
        {
            LauncherServiceState.Stopped => "未启动",
            LauncherServiceState.Starting => "启动中",
            LauncherServiceState.HealthOnly => "可用",
            LauncherServiceState.Ready => "可用",
            LauncherServiceState.Degraded => "可用",
            LauncherServiceState.SetupRequired => "需初始化",
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
            LauncherServiceState.HealthOnly => "服务已经可用",
            LauncherServiceState.Ready => "服务已经可用",
            LauncherServiceState.Degraded => "服务已经可用",
            LauncherServiceState.SetupRequired => "先完成初始化",
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

    internal string FormatReleaseFeedError() => "版本源暂时不可达。";

    internal string FormatDirectoryOpenFailed(string path) => $"无法打开目录：{path}";

    internal string FormatPathCopied(string label) => $"已复制{label}。";
}
