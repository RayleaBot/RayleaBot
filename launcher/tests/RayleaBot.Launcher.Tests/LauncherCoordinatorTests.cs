using System.Net;
using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherCoordinatorTests
{
    [TestMethod]
    public async Task InitializeAsync_BootstrapsLauncherSessionAndReportsReady()
    {
        var fixture = new LauncherFixture();
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.Ready, coordinator.Snapshot.ServiceState);
        Assert.IsTrue(coordinator.Snapshot.SetupInitialized);
        Assert.AreEqual(1, fixture.ManagementClient.IssueLauncherTokenCalls);
        Assert.AreEqual(1, fixture.ManagementClient.AdmitLauncherTokenCalls);
        Assert.AreEqual(1, fixture.ManagementClient.SystemStatusCalls);
    }

    [TestMethod]
    public async Task InitializeAsync_LeavesSetupRequiredWithoutSessionBootstrap()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.SetupInitialized = false;
        var coordinator = fixture.CreateCoordinator();

        await coordinator.InitializeAsync();

        Assert.AreEqual(LauncherServiceState.SetupRequired, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(0, fixture.ManagementClient.IssueLauncherTokenCalls);
        Assert.AreEqual(0, fixture.ManagementClient.AdmitLauncherTokenCalls);
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
        StringAssert.Contains(coordinator.Snapshot.ServiceDetail, "not running");
    }

    [TestMethod]
    public async Task RefreshAsync_ReauthenticatesAfterUnauthorizedSystemStatus()
    {
        var fixture = new LauncherFixture();
        fixture.ManagementClient.SystemStatusResponses.Enqueue(new LauncherHttpStatusException(HttpStatusCode.Unauthorized, "expired"));
        fixture.ManagementClient.SystemStatusResponses.Enqueue(new SystemStatusSnapshot("running", "connected", 2, 42));
        var coordinator = fixture.CreateCoordinator();
        await coordinator.InitializeAsync();

        await coordinator.RefreshAsync();

        Assert.AreEqual(LauncherServiceState.Ready, coordinator.Snapshot.ServiceState);
        Assert.AreEqual(2, fixture.ManagementClient.IssueLauncherTokenCalls);
        Assert.AreEqual(2, fixture.ManagementClient.AdmitLauncherTokenCalls);
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
    public async Task OpenWebUiAsync_UsesTokenOnlyForInitializedServers()
    {
        var initializedFixture = new LauncherFixture();
        var initializedCoordinator = initializedFixture.CreateCoordinator();
        await initializedCoordinator.InitializeAsync();
        await initializedCoordinator.OpenWebUiAsync();

        StringAssert.Contains(initializedFixture.ExternalOpener.OpenedUris.Single().ToString(), "?token=");

        var setupFixture = new LauncherFixture();
        setupFixture.ManagementClient.SetupInitialized = false;
        var setupCoordinator = setupFixture.CreateCoordinator();
        await setupCoordinator.InitializeAsync();
        await setupCoordinator.OpenWebUiAsync();

        Assert.HasCount(1, setupFixture.ExternalOpener.OpenedUris);
        Assert.IsFalse(setupFixture.ExternalOpener.OpenedUris.Single().Query.Contains("token=", StringComparison.Ordinal));
    }
}

internal sealed class LauncherFixture
{
    internal FakeSettingsStore SettingsStore { get; } = new();
    internal FakeEndpointResolver EndpointResolver { get; } = new();
    internal FakeEnvironmentInspector EnvironmentInspector { get; } = new();
    internal FakeManagementClient ManagementClient { get; } = new();
    internal FakeProcessController ProcessController { get; } = new();
    internal FakeExternalOpener ExternalOpener { get; } = new();

    internal LauncherCoordinator CreateCoordinator(LauncherCoordinatorOptions? options = null)
    {
        return new LauncherCoordinator(
            SettingsStore,
            EndpointResolver,
            EnvironmentInspector,
            ManagementClient,
            ProcessController,
            ExternalOpener,
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
