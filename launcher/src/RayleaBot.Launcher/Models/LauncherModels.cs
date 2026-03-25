using RayleaBot.Launcher;

namespace RayleaBot.Launcher.Models;

internal enum LauncherServiceState
{
    Stopped,
    Starting,
    ExternalService,
    HealthOnly,
    Ready,
    Degraded,
    SetupRequired,
    ShuttingDown,
    Failed,
}

internal enum LauncherSection
{
    Status,
    Environment,
    Diagnostics,
    Settings,
}

internal enum LauncherPrimaryAction
{
    None,
    OpenWebUi,
    StartService,
}

internal enum LauncherCloseBehavior
{
    AskEveryTime,
    HideToTray,
    ExitApplication,
}

internal enum CheckSeverity
{
    Ok,
    Warning,
    Error,
}

internal sealed record LauncherSettings(
    string ServerExecutablePath,
    string ConfigPath,
    string Workdir,
    LauncherCloseBehavior CloseBehavior = LauncherCloseBehavior.HideToTray);

internal sealed record ServerEndpoint(string Host, int Port)
{
    internal Uri BaseUri => new($"http://{Host}:{Port}/", UriKind.Absolute);
}

internal sealed record EnvironmentCheckResult(
    string Code,
    string Title,
    CheckSeverity Severity,
    string Summary,
    string Detail,
    string Remediation);

internal sealed record EnvironmentInspection(
    IReadOnlyList<EnvironmentCheckResult> Checks,
    bool HasBlockingIssues,
    bool CanBootstrapUserConfig)
{
    internal EnvironmentCheckResult? PrimaryIssue =>
        Checks.FirstOrDefault(item => item.Severity == CheckSeverity.Error) ??
        Checks.FirstOrDefault(item => item.Severity == CheckSeverity.Warning);
}

internal sealed record ReadinessSnapshot(string Status, string Reason);

internal sealed record SystemStatusSnapshot(string Status, string AdapterState, int ActivePlugins, long UptimeSeconds);

internal sealed record ReleaseCheckSnapshot(
    string Status,
    string CurrentVersion,
    string LatestVersion,
    string Summary,
    string Detail,
    string ReleasePageUrl,
    bool UpdateAvailable)
{
    internal static ReleaseCheckSnapshot Unavailable(string detail) =>
        new("unavailable", string.Empty, string.Empty, LauncherCopy.Default.VersionUnavailableSummary, detail, string.Empty, false);

    internal static ReleaseCheckSnapshot UpToDate(string currentVersion, string releasePageUrl) =>
        new("up_to_date", currentVersion, currentVersion, LauncherCopy.Default.FormatReleaseUpToDate(currentVersion), string.Empty, releasePageUrl, false);

    internal static ReleaseCheckSnapshot NewUpdateAvailable(string currentVersion, string latestVersion, string releasePageUrl) =>
        new("update_available", currentVersion, latestVersion, LauncherCopy.Default.FormatReleaseUpdateAvailable(currentVersion, latestVersion), "打开发布页即可查看已发布包的元数据和版本说明。", releasePageUrl, true);

    internal static ReleaseCheckSnapshot Error(string currentVersion, string detail, string releasePageUrl) =>
        new("error", currentVersion, string.Empty, LauncherCopy.Default.FormatReleaseFeedError(), detail, releasePageUrl, false);
}

internal sealed record LauncherSnapshot(
    LauncherSettings Settings,
    ServerEndpoint Endpoint,
    IReadOnlyList<EnvironmentCheckResult> EnvironmentChecks,
    IReadOnlyList<string> RecentStderr,
    int? ProcessId,
    LauncherServiceState ServiceState,
    bool SetupInitialized,
    bool ProcessRunning,
    bool ShutdownRequested,
    string SessionSummary,
    string ServiceDetail,
    string LastError,
    ReleaseCheckSnapshot ReleaseCheck)
{
    internal static LauncherSnapshot CreateDefault(LauncherSettings settings, ServerEndpoint endpoint)
    {
        return new LauncherSnapshot(
            settings,
            endpoint,
            [],
            [],
            null,
            LauncherServiceState.Stopped,
            false,
            false,
            false,
            LauncherCopy.Default.NoLauncherSession,
            "服务尚未启动。",
            string.Empty,
            ReleaseCheckSnapshot.Unavailable(LauncherCopy.Default.VersionUnavailableDetail));
    }
}
