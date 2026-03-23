namespace RayleaBot.Launcher.Models;

internal enum LauncherServiceState
{
    Stopped,
    Starting,
    HealthOnly,
    Ready,
    Degraded,
    SetupRequired,
    ShuttingDown,
    Failed,
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
    bool CloseToTrayEnabled = true,
    bool CloseTipAcknowledged = false);

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
        new("unavailable", string.Empty, string.Empty, "Version check is unavailable.", detail, string.Empty, false);

    internal static ReleaseCheckSnapshot UpToDate(string currentVersion, string releasePageUrl) =>
        new("up_to_date", currentVersion, currentVersion, $"Current version {currentVersion} is up to date.", string.Empty, releasePageUrl, false);

    internal static ReleaseCheckSnapshot NewUpdateAvailable(string currentVersion, string latestVersion, string releasePageUrl) =>
        new("update_available", currentVersion, latestVersion, $"Update available: {currentVersion} -> {latestVersion}.", "Open the release page to review the published package metadata and notes.", releasePageUrl, true);

    internal static ReleaseCheckSnapshot Error(string currentVersion, string detail, string releasePageUrl) =>
        new("error", currentVersion, string.Empty, "Version check could not reach the release feed.", detail, releasePageUrl, false);
}

internal sealed record LauncherSnapshot(
    LauncherSettings Settings,
    ServerEndpoint Endpoint,
    IReadOnlyList<EnvironmentCheckResult> EnvironmentChecks,
    IReadOnlyList<string> RecentStderr,
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
            LauncherServiceState.Stopped,
            false,
            false,
            false,
            "No launcher session.",
            "Service is not running.",
            string.Empty,
            ReleaseCheckSnapshot.Unavailable("Package build metadata is not available yet."));
    }
}
