using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Services;

internal interface ILauncherSettingsStore
{
    Task<LauncherSettings> LoadAsync(CancellationToken cancellationToken);

    Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken);
}

internal interface IServerEndpointResolver
{
    ServerEndpoint Resolve(string configPath);
}

internal interface IEnvironmentInspector
{
    Task<EnvironmentInspection> InspectAsync(LauncherSettings settings, CancellationToken cancellationToken);
}

internal interface ILauncherManagementClient
{
    Task<bool> IsHealthyAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);

    Task<ReadinessSnapshot> GetReadinessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);

    Task<bool> GetSetupInitializedAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);

    Task<string> IssueLauncherTokenAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);

    Task<string> AdmitLauncherTokenAsync(ServerEndpoint endpoint, string launcherToken, CancellationToken cancellationToken);

    Task<SystemStatusSnapshot> GetSystemStatusAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken);

    Task ShutdownAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken);
}

internal interface IServerProcessController
{
    bool IsRunning { get; }

    string LogDirectory { get; }

    IReadOnlyList<string> GetRecentStderr();

    Task StartAsync(LauncherSettings settings, CancellationToken cancellationToken);

    Task ForceKillAsync(CancellationToken cancellationToken);
}

internal interface IEndpointProcessController
{
    Task<bool> IsEndpointListeningAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);

    Task<bool> TryStopEndpointProcessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken);
}

internal interface IExternalOpener
{
    Task OpenUriAsync(Uri uri, CancellationToken cancellationToken);

    Task OpenDirectoryAsync(string directoryPath, CancellationToken cancellationToken);
}

internal interface IReleaseFeedClient
{
    Task<ReleaseCheckSnapshot> GetSnapshotAsync(CancellationToken cancellationToken);
}

internal sealed record LauncherCoordinatorOptions(TimeSpan StartupTimeout, TimeSpan PollInterval, TimeSpan ShutdownTimeout)
{
    internal static LauncherCoordinatorOptions Default { get; } = new(TimeSpan.FromSeconds(30), TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(10));
}
