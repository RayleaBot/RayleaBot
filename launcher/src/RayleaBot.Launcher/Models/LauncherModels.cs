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

internal sealed record LauncherSettings(string ServerExecutablePath, string ConfigPath, string Workdir);

internal sealed record ServerEndpoint(string Host, int Port)
{
    internal Uri BaseUri => new($"http://{Host}:{Port}/", UriKind.Absolute);
}

internal sealed record EnvironmentCheckResult(string Title, CheckSeverity Severity, string Detail);

internal sealed record ReadinessSnapshot(string Status, string Reason);

internal sealed record SystemStatusSnapshot(string Status, string AdapterState, int ActivePlugins, long UptimeSeconds);

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
    string LastError)
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
            string.Empty);
    }
}
