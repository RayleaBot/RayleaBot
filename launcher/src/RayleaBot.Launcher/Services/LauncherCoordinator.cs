using System.Net;
using System.Text;
using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Services;

internal sealed class LauncherCoordinator(
    ILauncherSettingsStore settingsStore,
    IServerEndpointResolver endpointResolver,
    IEnvironmentInspector environmentInspector,
    ILauncherManagementClient managementClient,
    IServerProcessController processController,
    IExternalOpener externalOpener,
    LauncherCoordinatorOptions? options = null)
{
    private readonly SemaphoreSlim gate = new(1, 1);
    private readonly LauncherCoordinatorOptions runtimeOptions = options ?? LauncherCoordinatorOptions.Default;
    private string? sessionToken;
    private LauncherSettings? currentSettings;
    private LauncherSnapshot snapshot = LauncherSnapshot.CreateDefault(new LauncherSettings(string.Empty, string.Empty, string.Empty), new ServerEndpoint("127.0.0.1", 8080));

    internal event EventHandler<LauncherSnapshot>? SnapshotChanged;

    internal LauncherSnapshot Snapshot => snapshot;

    internal async Task InitializeAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            currentSettings = await settingsStore.LoadAsync(cancellationToken).ConfigureAwait(false);
            snapshot = LauncherSnapshot.CreateDefault(currentSettings, endpointResolver.Resolve(currentSettings.ConfigPath));
            await RefreshCoreAsync(forceReauthentication: false, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task SaveSettingsAsync(LauncherSettings settings, CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            currentSettings = settings;
            await settingsStore.SaveAsync(settings, cancellationToken).ConfigureAwait(false);
            sessionToken = null;
            await RefreshCoreAsync(forceReauthentication: true, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task RefreshAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            await RefreshCoreAsync(forceReauthentication: false, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task RetryAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            await RefreshCoreAsync(forceReauthentication: true, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task StartAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            EnsureSettingsLoaded();
            try
            {
                await processController.StartAsync(currentSettings!, cancellationToken).ConfigureAwait(false);
            }
            catch (Exception ex)
            {
                UpdateSnapshot(
                    BuildSnapshot(
                        endpointResolver.Resolve(currentSettings!.ConfigPath),
                        await environmentInspector.InspectAsync(currentSettings!, cancellationToken).ConfigureAwait(false),
                        LauncherServiceState.Failed,
                        processController.IsRunning,
                        false,
                        false,
                        "No launcher session.",
                        "Server process failed to start.",
                        ex.Message));
                return;
            }

            UpdateSnapshot(
                snapshot with
                {
                    ServiceState = LauncherServiceState.Starting,
                    ProcessRunning = true,
                    ShutdownRequested = false,
                    ServiceDetail = "Waiting for /healthz to report ok.",
                    LastError = string.Empty,
                });

            var endpoint = endpointResolver.Resolve(currentSettings!.ConfigPath);
            using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            timeoutCts.CancelAfter(runtimeOptions.StartupTimeout);

            try
            {
                while (!timeoutCts.IsCancellationRequested)
                {
                    if (await managementClient.IsHealthyAsync(endpoint, timeoutCts.Token).ConfigureAwait(false))
                    {
                        await RefreshCoreAsync(forceReauthentication: true, timeoutCts.Token).ConfigureAwait(false);
                        return;
                    }

                    await Task.Delay(runtimeOptions.PollInterval, timeoutCts.Token).ConfigureAwait(false);
                }
            }
            catch (OperationCanceledException) when (timeoutCts.IsCancellationRequested)
            {
            }

            await processController.ForceKillAsync(cancellationToken).ConfigureAwait(false);
            UpdateSnapshot(
                BuildSnapshot(
                    endpoint,
                    snapshot.EnvironmentChecks,
                    LauncherServiceState.Failed,
                    processController.IsRunning,
                    false,
                    false,
                    "No launcher session.",
                    "Health probe did not succeed within the startup timeout.",
                    "Server start timed out."));
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task StopAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            EnsureSettingsLoaded();
            var endpoint = endpointResolver.Resolve(currentSettings!.ConfigPath);
            UpdateSnapshot(
                snapshot with
                {
                    ServiceState = LauncherServiceState.ShuttingDown,
                    ShutdownRequested = true,
                    ServiceDetail = "Requesting graceful shutdown.",
                    LastError = string.Empty,
                });

            var gracefulShutdownCompleted = false;
            try
            {
                if (await managementClient.IsHealthyAsync(endpoint, cancellationToken).ConfigureAwait(false))
                {
                    var initialized = await managementClient.GetSetupInitializedAsync(endpoint, cancellationToken).ConfigureAwait(false);
                    if (initialized)
                    {
                        var token = await EnsureSessionAsync(endpoint, cancellationToken).ConfigureAwait(false);
                        await managementClient.ShutdownAsync(endpoint, token, cancellationToken).ConfigureAwait(false);
                    }

                    using var timeoutCts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
                    timeoutCts.CancelAfter(runtimeOptions.ShutdownTimeout);
                    while (!timeoutCts.IsCancellationRequested)
                    {
                        if (!await managementClient.IsHealthyAsync(endpoint, timeoutCts.Token).ConfigureAwait(false))
                        {
                            gracefulShutdownCompleted = true;
                            break;
                        }

                        await Task.Delay(runtimeOptions.PollInterval, timeoutCts.Token).ConfigureAwait(false);
                    }
                }
            }
            catch (LauncherHttpStatusException ex) when (ex.StatusCode == HttpStatusCode.Unauthorized)
            {
                sessionToken = null;
            }
            catch
            {
            }

            if (!gracefulShutdownCompleted && processController.IsRunning)
            {
                await processController.ForceKillAsync(cancellationToken).ConfigureAwait(false);
            }

            sessionToken = null;
            await RefreshCoreAsync(forceReauthentication: true, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task OpenWebUiAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            EnsureSettingsLoaded();
            var endpoint = endpointResolver.Resolve(currentSettings!.ConfigPath);
            var initialized = false;
            try
            {
                initialized = await managementClient.GetSetupInitializedAsync(endpoint, cancellationToken).ConfigureAwait(false);
            }
            catch
            {
                initialized = false;
            }

            var uriBuilder = new UriBuilder(endpoint.BaseUri);
            if (initialized)
            {
                var launcherToken = await managementClient.IssueLauncherTokenAsync(endpoint, cancellationToken).ConfigureAwait(false);
                uriBuilder.Query = $"token={Uri.EscapeDataString(launcherToken)}";
            }

            await externalOpener.OpenUriAsync(uriBuilder.Uri, cancellationToken).ConfigureAwait(false);
            UpdateSnapshot(snapshot with { ServiceDetail = $"Opened {uriBuilder.Uri}", LastError = string.Empty });
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task OpenLogsDirectoryAsync(CancellationToken cancellationToken = default)
    {
        await externalOpener.OpenDirectoryAsync(processController.LogDirectory, cancellationToken).ConfigureAwait(false);
    }

    internal string BuildDiagnosticsSummary()
    {
        var builder = new StringBuilder();
        builder.AppendLine($"service_state: {snapshot.ServiceState}");
        builder.AppendLine($"endpoint: {snapshot.Endpoint.BaseUri}");
        builder.AppendLine($"session: {snapshot.SessionSummary}");
        if (!string.IsNullOrWhiteSpace(snapshot.LastError))
        {
            builder.AppendLine($"last_error: {snapshot.LastError}");
        }

        builder.AppendLine("environment_checks:");
        foreach (var item in snapshot.EnvironmentChecks)
        {
            builder.AppendLine($"- {item.Title}: {item.Severity} ({item.Detail})");
        }

        builder.AppendLine("recent_stderr:");
        foreach (var line in snapshot.RecentStderr)
        {
            builder.AppendLine($"- {line}");
        }

        return builder.ToString().Trim();
    }

    private async Task RefreshCoreAsync(bool forceReauthentication, CancellationToken cancellationToken)
    {
        EnsureSettingsLoaded();
        if (forceReauthentication)
        {
            sessionToken = null;
        }

        var settings = currentSettings!;
        var endpoint = endpointResolver.Resolve(settings.ConfigPath);
        var checks = await environmentInspector.InspectAsync(settings, cancellationToken).ConfigureAwait(false);

        try
        {
            if (!await managementClient.IsHealthyAsync(endpoint, cancellationToken).ConfigureAwait(false))
            {
                UpdateSnapshot(BuildSnapshot(
                    endpoint,
                    checks,
                    processController.IsRunning ? LauncherServiceState.Failed : LauncherServiceState.Stopped,
                    processController.IsRunning,
                    false,
                    snapshot.ShutdownRequested && processController.IsRunning,
                    "No launcher session.",
                    processController.IsRunning ? "Health probe failed while the child process is running." : "Server is not responding.",
                    processController.IsRunning ? "Health probe failed." : string.Empty));
                return;
            }
        }
        catch (Exception ex)
        {
            UpdateSnapshot(BuildSnapshot(
                endpoint,
                checks,
                processController.IsRunning ? LauncherServiceState.Failed : LauncherServiceState.Stopped,
                processController.IsRunning,
                false,
                snapshot.ShutdownRequested,
                "No launcher session.",
                "Health probe failed.",
                ex.Message));
            return;
        }

        ReadinessSnapshot readiness;
        try
        {
            readiness = await managementClient.GetReadinessAsync(endpoint, cancellationToken).ConfigureAwait(false);
        }
        catch (Exception ex)
        {
            UpdateSnapshot(BuildSnapshot(
                endpoint,
                checks,
                LauncherServiceState.HealthOnly,
                processController.IsRunning,
                false,
                snapshot.ShutdownRequested,
                "No launcher session.",
                "Health probe succeeded but readiness is unavailable.",
                ex.Message));
            return;
        }

        var initialized = await managementClient.GetSetupInitializedAsync(endpoint, cancellationToken).ConfigureAwait(false);
        if (!initialized)
        {
            sessionToken = null;
            UpdateSnapshot(BuildSnapshot(
                endpoint,
                checks,
                LauncherServiceState.SetupRequired,
                processController.IsRunning,
                false,
                snapshot.ShutdownRequested,
                "No launcher session.",
                string.IsNullOrWhiteSpace(readiness.Reason) ? "Bootstrap is still required." : readiness.Reason,
                string.Empty));
            return;
        }

        try
        {
            var token = await EnsureSessionAsync(endpoint, cancellationToken).ConfigureAwait(false);
            var systemStatus = await managementClient.GetSystemStatusAsync(endpoint, token, cancellationToken).ConfigureAwait(false);
            UpdateSnapshot(BuildSnapshot(
                endpoint,
                checks,
                MapServiceState(readiness.Status, systemStatus.Status),
                processController.IsRunning,
                true,
                systemStatus.Status == "shutting_down",
                $"Authenticated launcher session. Adapter={systemStatus.AdapterState}, plugins={systemStatus.ActivePlugins}, uptime={systemStatus.UptimeSeconds}s.",
                string.IsNullOrWhiteSpace(readiness.Reason) ? $"System status: {systemStatus.Status}" : readiness.Reason,
                string.Empty));
        }
        catch (LauncherHttpStatusException ex) when (ex.StatusCode == HttpStatusCode.Unauthorized)
        {
            sessionToken = null;
            try
            {
                var token = await EnsureSessionAsync(endpoint, cancellationToken).ConfigureAwait(false);
                var systemStatus = await managementClient.GetSystemStatusAsync(endpoint, token, cancellationToken).ConfigureAwait(false);
                UpdateSnapshot(BuildSnapshot(
                    endpoint,
                    checks,
                    MapServiceState(readiness.Status, systemStatus.Status),
                    processController.IsRunning,
                    true,
                    systemStatus.Status == "shutting_down",
                    $"Authenticated launcher session. Adapter={systemStatus.AdapterState}, plugins={systemStatus.ActivePlugins}, uptime={systemStatus.UptimeSeconds}s.",
                    string.IsNullOrWhiteSpace(readiness.Reason) ? $"System status: {systemStatus.Status}" : readiness.Reason,
                    string.Empty));
            }
            catch (Exception inner)
            {
                UpdateSnapshot(BuildSnapshot(
                    endpoint,
                    checks,
                    LauncherServiceState.HealthOnly,
                    processController.IsRunning,
                    true,
                    snapshot.ShutdownRequested,
                    "Launcher session bootstrap failed.",
                    "Management session is unavailable. Health endpoints remain reachable.",
                    inner.Message));
            }
        }
        catch (Exception ex)
        {
            UpdateSnapshot(BuildSnapshot(
                endpoint,
                checks,
                LauncherServiceState.HealthOnly,
                processController.IsRunning,
                true,
                snapshot.ShutdownRequested,
                "Launcher session bootstrap failed.",
                "Management session is unavailable. Health endpoints remain reachable.",
                ex.Message));
        }
    }

    private async Task<string> EnsureSessionAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        if (!string.IsNullOrWhiteSpace(sessionToken))
        {
            return sessionToken!;
        }

        var launcherToken = await managementClient.IssueLauncherTokenAsync(endpoint, cancellationToken).ConfigureAwait(false);
        sessionToken = await managementClient.AdmitLauncherTokenAsync(endpoint, launcherToken, cancellationToken).ConfigureAwait(false);
        return sessionToken;
    }

    private LauncherSnapshot BuildSnapshot(
        ServerEndpoint endpoint,
        IReadOnlyList<EnvironmentCheckResult> checks,
        LauncherServiceState serviceState,
        bool processRunning,
        bool setupInitialized,
        bool shutdownRequested,
        string sessionSummary,
        string serviceDetail,
        string lastError)
    {
        return new LauncherSnapshot(
            currentSettings!,
            endpoint,
            checks,
            processController.GetRecentStderr(),
            serviceState,
            setupInitialized,
            processRunning,
            shutdownRequested,
            sessionSummary,
            serviceDetail,
            lastError);
    }

    private void UpdateSnapshot(LauncherSnapshot next)
    {
        snapshot = next;
        SnapshotChanged?.Invoke(this, snapshot);
    }

    private static LauncherServiceState MapServiceState(string readinessStatus, string systemStatus)
    {
        if (string.Equals(systemStatus, "shutting_down", StringComparison.Ordinal))
        {
            return LauncherServiceState.ShuttingDown;
        }

        return readinessStatus switch
        {
            "ready" => LauncherServiceState.Ready,
            "degraded" => LauncherServiceState.Degraded,
            "setup_required" => LauncherServiceState.SetupRequired,
            "failed" => LauncherServiceState.Failed,
            _ => LauncherServiceState.HealthOnly,
        };
    }

    private void EnsureSettingsLoaded()
    {
        if (currentSettings is null)
        {
            throw new InvalidOperationException("Launcher settings have not been loaded yet.");
        }
    }
}
