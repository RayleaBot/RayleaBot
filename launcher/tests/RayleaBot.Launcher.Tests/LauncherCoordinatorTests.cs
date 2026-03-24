using System.Net;
using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherCoordinatorTests
{
    [TestMethod]
    public async Task InitializeAsync_ReportsReadyWithoutLauncherSessionBootstrap()
    {
        var fixture = new LauncherFixture();
        fixture.ReleaseFeedClient.Snapshot = ReleaseCheckSnapshot.UpToDate("0.1.0", "https://example.invalid/releases/v0.1.0");
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.ExternalService, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(0, fixture.ManagementClient.IssueLauncherTokenCalls);
        Assert.AreEqual(0, fixture.ManagementClient.AdmitLauncherTokenCalls);
        Assert.AreEqual(0, fixture.ManagementClient.SystemStatusCalls);
        Assert.AreEqual("up_to_date", coordinator.Snapshot.ReleaseCheck.Status);
    }

    [TestMethod]
    public async Task InitializeAsync_KeepsLauncherReadyWhenSetupIsStillRequired()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.SetupInitialized = false;
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.ExternalService, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(0, fixture.ManagementClient.IssueLauncherTokenCalls);
        Assert.AreEqual(0, fixture.ManagementClient.AdmitLauncherTokenCalls);
        Assert.IsFalse(coordinator.Snapshot.ServiceDetail.Contains("初始化", StringComparison.Ordinal));
    }

    [TestMethod]
    public async Task InitializeAsync_DoesNotProbeHealthWhenServerIsStoppedAndBootstrapIsAvailable()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult(
                "config.bootstrap_available",
                "Config file",
                CheckSeverity.Warning,
                "User config will be generated on first start.",
                @"Missing user config file: C:\RayleaBot\config\user.yaml",
                @"Start the service to bootstrap the first config from C:\RayleaBot\config\default.yaml."),
        ],
        false,
        true);
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.Stopped, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(0, fixture.ManagementClient.HealthCalls);
        Assert.AreEqual(string.Empty, coordinator.Snapshot.LastError);
        StringAssert.Contains(coordinator.Snapshot.ServiceDetail, "first config");
    }

    [TestMethod]
    public async Task InitializeAsync_DoesNotReportConnectionFailureWhenProcessIsStopped()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        fixture.EnvironmentInspector.Inspection = new EnvironmentInspection(
        [
            new EnvironmentCheckResult(
                "server.executable",
                "Server executable",
                CheckSeverity.Ok,
                "Executable ready.",
                @"C:\RayleaBot\raylea-server.exe",
                string.Empty),
        ],
        false,
        false);
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.Stopped, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(1, fixture.ManagementClient.HealthCalls);
        Assert.AreEqual(string.Empty, coordinator.Snapshot.LastError);
        StringAssert.Contains(coordinator.Snapshot.ServiceDetail, "服务尚未启动");
    }

    [TestMethod]
    public async Task RefreshAsync_DoesNotSurfaceLauncherSessionFailures()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.IssueLauncherTokenException = new LauncherHttpStatusException(HttpStatusCode.Unauthorized, "expired");
        var coordinator = fixture.CreateCoordinator();
        await coordinator.InitializeAsync();

        await coordinator.RefreshAsync();

        Assert.AreEqual(LauncherServiceState.ExternalService, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(0, fixture.ManagementClient.SystemStatusCalls);
        Assert.IsFalse(coordinator.Snapshot.ServiceDetail.Contains("会话", StringComparison.Ordinal));
        Assert.IsFalse(coordinator.Snapshot.LastError.Contains("expired", StringComparison.Ordinal));
    }

    [TestMethod]
    public async Task InitializeAsync_DoesNotPromoteAdapterReconnectReasonToPrimaryStatus()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.Readiness = new ReadinessSnapshot("degraded", "OneBot reverse WebSocket is reconnecting");
        fixture.ManagementClient.DefaultSystemStatus = new SystemStatusSnapshot("running", "reconnecting", 1, 60);
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.ExternalService, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(string.Empty, coordinator.Snapshot.LastError);
        Assert.IsFalse(coordinator.Snapshot.ServiceDetail.Contains("OneBot", StringComparison.Ordinal));
        Assert.IsFalse(coordinator.Snapshot.ServiceDetail.Contains("reconnecting", StringComparison.Ordinal));
    }

    [TestMethod]
    public async Task StopAsync_StopsDetectedExternalServiceWhenGracefulShutdownDoesNotDrain()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = true;
        fixture.ManagementClient.SetupInitialized = false;
        fixture.EndpointProcessController.StopResult = true;
        fixture.EndpointProcessController.OnStop = () => fixture.ManagementClient.HealthDefault = false;
        var coordinator = fixture.CreateCoordinator(new LauncherCoordinatorOptions(TimeSpan.FromMilliseconds(50), TimeSpan.FromMilliseconds(5), TimeSpan.FromMilliseconds(20)));
        await coordinator.InitializeAsync();

        await coordinator.StopAsync();

        Assert.AreEqual(1, fixture.EndpointProcessController.StopCalls);
        Assert.AreEqual(LauncherServiceState.Stopped, coordinator.Snapshot.ServiceState);
    }

    [TestMethod]
    public async Task StartAsync_ForceKillsWhenHealthNeverTurnsHealthy()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.HealthDefault = false;
        var coordinator = fixture.CreateCoordinator(new LauncherCoordinatorOptions(TimeSpan.FromMilliseconds(30), TimeSpan.FromMilliseconds(5), TimeSpan.FromMilliseconds(20)));

        await coordinator.InitializeAsync();
        await coordinator.StartAsync();

        Assert.AreEqual(1, fixture.ProcessController.StartCalls);
        Assert.AreEqual(1, fixture.ProcessController.ForceKillCalls);
        Assert.AreEqual(LauncherServiceState.Failed, coordinator.Snapshot.ServiceState);
    }

    [TestMethod]
    public async Task StopAsync_FallsBackToForceKillWhenShutdownCannotDrain()
    {
        var fixture = new LauncherFixture();
        fixture.ProcessController.IsRunningValue = true;
        fixture.ManagementClient.ShutdownException = new TimeoutException("shutdown timed out");
        var coordinator = fixture.CreateCoordinator(new LauncherCoordinatorOptions(TimeSpan.FromMilliseconds(50), TimeSpan.FromMilliseconds(5), TimeSpan.FromMilliseconds(20)));
        await coordinator.InitializeAsync();

        await coordinator.StopAsync();

        Assert.AreEqual(1, fixture.ProcessController.ForceKillCalls);
    }

    [TestMethod]
    public async Task OpenWebUiAsync_AlwaysUsesRootAndAddsTokenOnlyWhenAvailable()
    {
        var initializedFixture = new LauncherFixture();
        var initializedCoordinator = initializedFixture.CreateCoordinator();
        await initializedCoordinator.InitializeAsync();
        await initializedCoordinator.OpenWebUiAsync();

        StringAssert.Contains(initializedFixture.ExternalOpener.OpenedUris.Single().ToString(), "?token=");
        Assert.AreEqual("/", initializedFixture.ExternalOpener.OpenedUris.Single().AbsolutePath);

        var setupFixture = new LauncherFixture();
        setupFixture.ManagementClient.SetupInitialized = false;
        var setupCoordinator = setupFixture.CreateCoordinator();
        await setupCoordinator.InitializeAsync();
        await setupCoordinator.OpenWebUiAsync();

        Assert.HasCount(1, setupFixture.ExternalOpener.OpenedUris);
        Assert.IsFalse(setupFixture.ExternalOpener.OpenedUris.Single().Query.Contains("token=", StringComparison.Ordinal));
        Assert.AreEqual("/", setupFixture.ExternalOpener.OpenedUris.Single().AbsolutePath);
    }

    [TestMethod]
    public async Task OpenReleasePageAsync_UsesReleaseFeedUrlWhenAvailable()
    {
        var fixture = new LauncherFixture();
        fixture.ReleaseFeedClient.Snapshot = ReleaseCheckSnapshot.NewUpdateAvailable(
            "0.1.0",
            "0.1.1",
            "https://example.invalid/releases/v0.1.1");
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();
        await coordinator.OpenReleasePageAsync();

        Assert.AreEqual("https://example.invalid/releases/v0.1.1", fixture.ExternalOpener.OpenedUris.Last().ToString());
    }
}

internal sealed class LauncherFixture
{
    internal FakeSettingsStore SettingsStore { get; } = new();
    internal FakeEndpointResolver EndpointResolver { get; } = new();
    internal FakeEnvironmentInspector EnvironmentInspector { get; } = new();
    internal FakeManagementClient ManagementClient { get; } = new();
    internal FakeProcessController ProcessController { get; } = new();
    internal FakeEndpointProcessController EndpointProcessController { get; } = new();
    internal FakeExternalOpener ExternalOpener { get; } = new();
    internal FakeReleaseFeedClient ReleaseFeedClient { get; } = new();

    internal LauncherCoordinator CreateCoordinator(LauncherCoordinatorOptions? options = null)
    {
        return new LauncherCoordinator(
            SettingsStore,
            EndpointResolver,
            EnvironmentInspector,
            ManagementClient,
            ProcessController,
            EndpointProcessController,
            ExternalOpener,
            ReleaseFeedClient,
            options);
    }
}

internal sealed class FakeSettingsStore : ILauncherSettingsStore
{
    internal LauncherSettings Settings { get; set; } = new("C:\\RayleaBot\\raylea-server.exe", "C:\\RayleaBot\\config\\user.yaml", "C:\\RayleaBot");

    public Task<LauncherSettings> LoadAsync(CancellationToken cancellationToken)
    {
        return Task.FromResult(Settings);
    }

    public Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        Settings = settings;
        return Task.CompletedTask;
    }
}

internal sealed class FakeEndpointResolver : IServerEndpointResolver
{
    public ServerEndpoint Resolve(string configPath)
    {
        return new ServerEndpoint("127.0.0.1", 8080);
    }
}

internal sealed class FakeEnvironmentInspector : IEnvironmentInspector
{
    internal EnvironmentInspection Inspection { get; set; } = new(
    [
        new EnvironmentCheckResult("server.executable", "Server executable", CheckSeverity.Ok, "Executable ready.", "ok", string.Empty),
        new EnvironmentCheckResult("config.file", "Config file", CheckSeverity.Ok, "Config ready.", "ok", string.Empty),
    ],
    false,
    false);

    public Task<EnvironmentInspection> InspectAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        return Task.FromResult(Inspection);
    }
}

internal sealed class FakeManagementClient : ILauncherManagementClient
{
    internal Queue<bool> HealthResponses { get; } = new();
    internal bool HealthDefault { get; set; } = true;
    internal Queue<object> SystemStatusResponses { get; } = new();
    internal bool SetupInitialized { get; set; } = true;
    internal ReadinessSnapshot Readiness { get; set; } = new("ready", string.Empty);
    internal string LauncherToken { get; set; } = "launcher_fixture_token";
    internal string SessionToken { get; set; } = "session_fixture_token";
    internal SystemStatusSnapshot DefaultSystemStatus { get; set; } = new("running", "connected", 1, 60);
    internal Exception? ShutdownException { get; set; }
    internal Exception? IssueLauncherTokenException { get; set; }
    internal int IssueLauncherTokenCalls { get; private set; }
    internal int AdmitLauncherTokenCalls { get; private set; }
    internal int SystemStatusCalls { get; private set; }
    internal int HealthCalls { get; private set; }

    public Task<bool> IsHealthyAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        HealthCalls++;
        if (HealthResponses.Count > 0)
        {
            return Task.FromResult(HealthResponses.Dequeue());
        }

        return Task.FromResult(HealthDefault);
    }

    public Task<ReadinessSnapshot> GetReadinessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        return Task.FromResult(Readiness);
    }

    public Task<bool> GetSetupInitializedAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        return Task.FromResult(SetupInitialized);
    }

    public Task<string> IssueLauncherTokenAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        IssueLauncherTokenCalls++;
        if (IssueLauncherTokenException is not null)
        {
            throw IssueLauncherTokenException;
        }
        return Task.FromResult(LauncherToken);
    }

    public Task<string> AdmitLauncherTokenAsync(ServerEndpoint endpoint, string launcherToken, CancellationToken cancellationToken)
    {
        AdmitLauncherTokenCalls++;
        return Task.FromResult(SessionToken);
    }

    public Task<SystemStatusSnapshot> GetSystemStatusAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken)
    {
        SystemStatusCalls++;
        if (SystemStatusResponses.Count > 0)
        {
            var next = SystemStatusResponses.Dequeue();
            if (next is Exception exception)
            {
                throw exception;
            }

            return Task.FromResult((SystemStatusSnapshot)next);
        }

        return Task.FromResult(DefaultSystemStatus);
    }

    public Task ShutdownAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken)
    {
        if (ShutdownException is not null)
        {
            throw ShutdownException;
        }

        return Task.CompletedTask;
    }
}

internal sealed class FakeProcessController : IServerProcessController
{
    internal bool IsRunningValue { get; set; }
    internal int StartCalls { get; private set; }
    internal int ForceKillCalls { get; private set; }

    public bool IsRunning => IsRunningValue;

    public string LogDirectory => "C:\\RayleaBot\\logs";

    public IReadOnlyList<string> GetRecentStderr()
    {
        return ["stderr line"];
    }

    public Task StartAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        StartCalls++;
        IsRunningValue = true;
        return Task.CompletedTask;
    }

    public Task ForceKillAsync(CancellationToken cancellationToken)
    {
        ForceKillCalls++;
        IsRunningValue = false;
        return Task.CompletedTask;
    }
}

internal sealed class FakeExternalOpener : IExternalOpener
{
    internal List<Uri> OpenedUris { get; } = [];
    internal List<string> OpenedDirectories { get; } = [];

    public Task OpenUriAsync(Uri uri, CancellationToken cancellationToken)
    {
        OpenedUris.Add(uri);
        return Task.CompletedTask;
    }

    public Task OpenDirectoryAsync(string directoryPath, CancellationToken cancellationToken)
    {
        OpenedDirectories.Add(directoryPath);
        return Task.CompletedTask;
    }
}

internal sealed class FakeEndpointProcessController : IEndpointProcessController
{
    internal bool StopResult { get; set; }
    internal int StopCalls { get; private set; }
    internal Action? OnStop { get; set; }

    public Task<bool> TryStopEndpointProcessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        StopCalls++;
        OnStop?.Invoke();
        return Task.FromResult(StopResult);
    }
}

internal sealed class FakeReleaseFeedClient : IReleaseFeedClient
{
    internal ReleaseCheckSnapshot Snapshot { get; set; } = ReleaseCheckSnapshot.Unavailable("release feed not configured");

    public Task<ReleaseCheckSnapshot> GetSnapshotAsync(CancellationToken cancellationToken)
    {
        return Task.FromResult(Snapshot);
    }
}
