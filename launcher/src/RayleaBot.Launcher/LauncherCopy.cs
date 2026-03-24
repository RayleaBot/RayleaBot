using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher;

internal sealed class LauncherCopy
{
    internal static LauncherCopy Default { get; } = new();

    internal string WindowTitle => "RayleaBot 启动器";
    internal string AppSubtitle => "本地服务控制台";
    internal string TitleBarHint => "本地启动、诊断与恢复";

    internal string OverviewTitle => "总览";
    internal string OverviewSummary => "状态与关键问题";
    internal string ServiceControlsTitle => "服务控制";
    internal string ServiceControlsSummary => "启动、停止与重试";
    internal string EnvironmentTitle => "环境检查";
    internal string EnvironmentSummary => "阻塞项与引导信息";
    internal string SettingsTitle => "设置";
    internal string SettingsSummary => "本地启动路径与偏好";
    internal string DiagnosticsTitle => "诊断";
    internal string DiagnosticsSummary => "日志、错误输出与诊断摘要";

    internal string HeroEyebrow => "启动器总览";
    internal string StatusLabel => "当前状态";
    internal string EndpointLabel => "服务入口";
    internal string SessionLabel => "会话状态";
    internal string OperationLabel => "最近操作";
    internal string VersionLabel => "版本检查";
    internal string PrimaryIssueLabel => "当前重点";
    internal string SecondaryIssueLabel => "次级诊断";

    internal string OverviewIntro => "优先查看当前状态、主要问题和建议动作。长路径与原始异常已下沉到环境检查和诊断页。";
    internal string ControlsIntro => "操作会根据本地预检结果和服务实时状态自动启用或禁用。";
    internal string SettingsIntro => "启动器仅保存本地启动路径与偏好设置。平台配置仍以 default.yaml 与 user.yaml 为准。";
    internal string EnvironmentIntro => "环境检查按严重级别分组展示，先处理阻塞项，再处理需注意项。";
    internal string DiagnosticsIntro => "原始错误输出只作为诊断辅助手段。先看状态、环境检查和操作结果，再回看这里。";

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

    internal string CloseDialogTitle => "关闭窗口时如何处理？";
    internal string CloseDialogBody => "关闭窗口后，启动器会继续驻留在系统托盘中，服务控制与恢复能力不会中断。";
    internal string CloseDialogFootnote => "如需完全退出启动器进程，请选择“完全退出”。之后仍可从托盘菜单执行完整退出。";
    internal string HideToTrayLabel => "隐藏到托盘";
    internal string ExitCompletelyLabel => "完全退出";

    internal string VersionUnavailableSummary => "暂时无法检查版本";
    internal string VersionUnavailableDetail => "当前构建缺少可用的发布元数据。";
    internal string VersionPageUnavailable => "当前构建没有可用的发布页链接。";

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
            LauncherServiceState.Stopped => "已停止",
            LauncherServiceState.Starting => "启动中",
            LauncherServiceState.HealthOnly => "仅健康存活",
            LauncherServiceState.Ready => "已就绪",
            LauncherServiceState.Degraded => "已降级",
            LauncherServiceState.SetupRequired => "需初始化",
            LauncherServiceState.ShuttingDown => "停止中",
            LauncherServiceState.Failed => "失败",
            _ => "未知状态",
        };

    internal string FormatHeroTitle(LauncherServiceState state, IReadOnlyList<EnvironmentCheckResult> checks)
    {
        if (state == LauncherServiceState.Stopped &&
            checks.Any(item => string.Equals(item.Code, "config.bootstrap_available", StringComparison.Ordinal)))
        {
            return "首启配置已准备好，可直接引导";
        }

        return state switch
        {
            LauncherServiceState.Stopped => "服务尚未启动",
            LauncherServiceState.Starting => "正在启动 RayleaBot",
            LauncherServiceState.HealthOnly => "服务已启动，但管理能力受限",
            LauncherServiceState.Ready => "服务已就绪",
            LauncherServiceState.Degraded => "服务运行中，但依赖降级",
            LauncherServiceState.SetupRequired => "仍需完成初始化",
            LauncherServiceState.ShuttingDown => "正在优雅停止服务",
            LauncherServiceState.Failed => "服务启动或运行失败",
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
