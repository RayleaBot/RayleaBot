using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher;

internal sealed class LauncherCopy
{
    internal static LauncherCopy Default { get; } = new();

    internal string WindowTitle => "RayleaBot 启动器";
    internal string AppSubtitle => "本地启动与管理入口";
    internal string ShellHeaderTitle => "RayleaBot";
    internal string ShellHeaderSummary => "启动、检查并打开管理界面";

    internal string StatusTitle => "状态";
    internal string EnvironmentTitle => "环境检查";
    internal string DiagnosticsTitle => "日志与诊断";
    internal string SettingsTitle => "设置";

    internal string StatusPageHeading => "服务状态";
    internal string StatusPageSummary => "查看当前状态、主操作和需要立刻处理的问题。";
    internal string EnvironmentPageHeading => "环境检查";
    internal string EnvironmentPageSummary => "优先处理阻塞项和警告项，正常结果默认收起。";
    internal string DiagnosticsPageHeading => "日志与诊断";
    internal string DiagnosticsPageSummary => "查看最近错误输出和结构化诊断摘要。";
    internal string SettingsPageHeading => "本地设置";
    internal string SettingsPageSummary => "这里只管理启动器本地路径与偏好。";

    internal string StatusEyebrow => "当前状态";
    internal string StatusLabel => "当前状态";
    internal string EndpointLabel => "服务入口";
    internal string VersionLabel => "版本";
    internal string ProcessIdLabel => "PID";
    internal string WorkdirLabel => "工作目录";
    internal string OperationLabel => "最近操作";
    internal string PrimaryActionTitle => "主操作";
    internal string SecondaryActionsTitle => "更多操作";
    internal string ServiceInfoTitle => "服务信息";
    internal string EnvironmentSummaryTitle => "环境摘要";
    internal string RecentLogsTitle => "最近错误输出";
    internal string RecentLogsEmpty => "当前没有新的错误输出。";
    internal string DiagnosticsSummaryTitle => "诊断摘要";
    internal string DiagnosticsRawTitle => "原始诊断文本";
    internal string DiagnosticsServiceStatusTitle => "服务状态";
    internal string DiagnosticsServiceEndpointTitle => "服务入口";
    internal string DiagnosticsEnvironmentTitle => "环境检查";
    internal string DiagnosticsRecentErrorTitle => "最近错误";
    internal string DiagnosticsNoRecentError => "当前没有新的错误输出。";
    internal string MainAlertActionLabel => "查看环境检查";

    internal string BlockingGroupTitle => "阻塞项";
    internal string WarningGroupTitle => "需注意";
    internal string ReadyGroupTitle => "正常项";
    internal string EmptyGroupHint => "当前没有项目。";
    internal string PackagingInfoTitle => "安装与打包信息";
    internal string PackagingInfoSummary => "版本和打包元数据会在这里提示，不进入首页主提示。";

    internal string ServerExecutableLabel => "服务端可执行文件";
    internal string ConfigPathLabel => "用户配置文件";
    internal string SettingsPathGroupTitle => "路径";
    internal string SettingsPathGroupDescription => "默认只读，进入编辑态后才能修改。";
    internal string SettingsBehaviorGroupTitle => "关闭行为";
    internal string SettingsBehaviorGroupDescription => "定义点击窗口关闭按钮时的默认处理方式。";
    internal string EditSettingsLabel => "编辑路径";
    internal string CancelEditingLabel => "取消编辑";
    internal string SaveSettingsLabel => "保存设置";
    internal string SettingsReadOnlyStateTitle => "当前为只读模式";
    internal string SettingsReadOnlyStateSummary => "点击“编辑路径”后再修改本地路径设置。";
    internal string SettingsEditingStateTitle => "正在编辑本地路径";
    internal string SettingsEditingStateSummary => "修改完成后保存，才会写回启动器设置。";
    internal string SettingsDirtyStateTitle => "有未保存修改";
    internal string SettingsDirtyStateSummary => "保存设置后会写回启动器本地配置。";
    internal string BrowseExecutableLabel => "浏览文件";
    internal string BrowseConfigLabel => "浏览文件";
    internal string BrowseWorkdirLabel => "浏览目录";
    internal string OpenDirectoryLabel => "打开目录";
    internal string OpenParentDirectoryLabel => "打开位置";
    internal string CopyPathLabel => "复制路径";
    internal string CopyEvidenceLabel => "复制证据";
    internal string OpenLocationLabel => "打开位置";
    internal string AskEveryTimeBehaviorTitle => "每次询问";
    internal string AskEveryTimeBehaviorDescription => "关闭窗口时显示确认对话框，再决定隐藏到托盘或完全退出。";
    internal string HideToTrayBehaviorTitle => "隐藏到托盘";
    internal string HideToTrayBehaviorDescription => "关闭窗口时保留后台运行，后续可从系统托盘恢复。";
    internal string ExitApplicationBehaviorTitle => "完全退出";
    internal string ExitApplicationBehaviorDescription => "关闭窗口时直接退出启动器和当前托盘入口。";
    internal string SettingsEditingHint => "保存后会写回启动器本地设置。";

    internal string CopyDiagnosticsLabel => "复制诊断";
    internal string OpenLogsDirectoryLabel => "打开日志目录";
    internal string OpenWebUiLabel => "打开管理界面";
    internal string RetryHealthAuthLabel => "重新检查";
    internal string OpenReleasePageLabel => "打开发布页";
    internal string StartServiceLabel => "启动服务";
    internal string StopServiceLabel => "停止服务";
    internal string StopServicePendingLabel => "停止中...";
    internal string RetryPendingLabel => "检查中...";
    internal string OpenLogsPendingLabel => "打开中...";
    internal string OpenReleasePagePendingLabel => "打开中...";
    internal string SaveSettingsPendingLabel => "保存中...";
    internal string ExitAppLabel => "完全退出";
    internal string NoLauncherSession => string.Empty;

    internal string RestoreLauncherLabel => "恢复窗口";
    internal string TrayTitleLabel => "RayleaBot 启动器";
    internal string TrayPrimaryGroupLabel => "主操作";
    internal string TraySecondaryGroupLabel => "次操作";
    internal string TrayDangerGroupLabel => "危险操作";
    internal string TrayTooltip => "RayleaBot 启动器";

    internal string CloseDialogTitle => "关闭窗口";
    internal string CloseDialogBody => "请选择当前关闭窗口时的处理方式。";
    internal string CloseDialogFootnote => "隐藏到托盘后，启动器仍可从系统托盘恢复并继续执行常用操作。";
    internal string CloseDialogRememberChoiceLabel => "将本次选择设为默认行为";
    internal string CloseDialogCurrentDefaultPrefix => "当前默认";
    internal string HideToTrayLabel => "隐藏到托盘";
    internal string ExitCompletelyLabel => "完全退出";
    internal string CancelDialogLabel => "取消";

    internal string VersionUnavailableSummary => "版本信息不可用";
    internal string VersionUnavailableDetail => "当前运行没有可读取的版本包元数据。";
    internal string VersionPageUnavailable => "当前运行没有可打开的发布页。";

    internal string ActionLauncherInitialized => "已完成本地检查。";
    internal string ActionLauncherInitializing => "正在检查本地运行环境...";
    internal string ActionHealthRetryPending => "正在重新检查服务状态...";
    internal string ActionHealthRetryFinished => "已重新检查服务状态。";
    internal string ActionSettingsSaving => "正在保存启动器设置...";
    internal string ActionSettingsSaved => "启动器设置已保存。";
    internal string ActionStartPending => "正在启动服务...";
    internal string ActionStartFinished => "启动请求已完成。";
    internal string ActionStopPending => "正在停止服务...";
    internal string ActionStopFinished => "停止请求已完成。";
    internal string ActionWebOpening => "正在打开管理界面...";
    internal string ActionWebOpened => "已在默认浏览器中打开管理界面。";
    internal string ActionLogsOpening => "正在打开日志目录...";
    internal string ActionLogsOpened => "已打开日志目录。";
    internal string ActionReleasePageOpening => "正在打开发布页...";
    internal string ActionReleasePageOpened => "已在默认浏览器中打开发布页。";
    internal string ActionDiagnosticsCopied => "诊断摘要已复制到剪贴板。";
    internal string ActionRestoredFromTray => "启动器已从系统托盘恢复。";
    internal string ActionHiddenToTray => "启动器仍在系统托盘中运行。";
    internal string ActionSettingsEditStarted => "已进入路径编辑状态。";
    internal string ActionSettingsEditCanceled => "已放弃未保存的路径修改。";
    internal string ActionCloseBehaviorSaved => "关闭行为默认策略已保存。";

    internal string DetectedServiceSummary => "检测到现有服务";
    internal string DetectedServiceDetail => "端口上已有服务正在运行。可以直接打开管理界面，或先停止它再由启动器重新启动。";
    internal string ExternalStopConfirmTitle => "停止现有服务";
    internal string ExternalStopConfirmBody => "当前端口上的服务不是由启动器拉起。继续后会尝试停止这个现有服务。";
    internal string ExternalStopConfirmFootnote => "如果这是另一个独立运行中的实例，请先确认停止它不会影响当前工作。";
    internal string ExternalStopConfirmAction => "继续停止";
    internal string ExternalStopCancelAction => "取消";

    internal string NoHomeAlertTitle => string.Empty;

    internal string FormatStatusSummary(LauncherServiceState state) =>
        state switch
        {
            LauncherServiceState.Stopped => "未启动",
            LauncherServiceState.Starting => "启动中",
            LauncherServiceState.ExternalService => "运行中",
            LauncherServiceState.HealthOnly => "运行中",
            LauncherServiceState.Ready => "运行中",
            LauncherServiceState.Degraded => "受限运行",
            LauncherServiceState.SetupRequired => "需要设置",
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
            LauncherServiceState.Degraded => "服务在受限状态下运行",
            LauncherServiceState.SetupRequired => "服务仍需完成设置",
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

    internal string FormatProcessId(int processId) => processId.ToString();

    internal string FormatEnvironmentSummary(int blockingCount, int warningCount, int readyCount) =>
        $"阻塞项 {blockingCount} · 需注意 {warningCount} · 正常 {readyCount}";

    internal string FormatTrayStatusLabel(string statusSummary) => $"状态：{statusSummary}";

    internal string FormatTrayStatusSummary(string statusSummary, bool hasBlockingIssues, bool hasWarnings)
    {
        if (hasBlockingIssues)
        {
            return $"{statusSummary} · 有阻塞项";
        }

        if (hasWarnings)
        {
            return $"{statusSummary} · 有警告";
        }

        return statusSummary;
    }

    internal string FormatTrayTooltip(string trayStatusSummary) => $"{TrayTooltip} · {trayStatusSummary}";

    internal string FormatCloseBehaviorSummary(LauncherCloseBehavior closeBehavior) =>
        closeBehavior switch
        {
            LauncherCloseBehavior.AskEveryTime => AskEveryTimeBehaviorTitle,
            LauncherCloseBehavior.ExitApplication => ExitApplicationBehaviorTitle,
            _ => HideToTrayBehaviorTitle,
        };

    internal string FormatPendingPrimaryActionLabel(LauncherPrimaryAction action) =>
        action switch
        {
            LauncherPrimaryAction.OpenWebUi => "打开中...",
            LauncherPrimaryAction.StartService => "启动中...",
            _ => string.Empty,
        };
}
