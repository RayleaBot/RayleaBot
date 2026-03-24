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
    IReleaseFeedClient? releaseFeedClient = null,
    LauncherCoordinatorOptions? options = null)
{
    private static readonly LauncherCopy Copy = LauncherCopy.Default;
    private readonly SemaphoreSlim gate = new(1, 1);
    private readonly LauncherCoordinatorOptions runtimeOptions = options ?? LauncherCoordinatorOptions.Default;
    private readonly IReleaseFeedClient? launcherReleaseFeed = releaseFeedClient;
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
            var endpoint = endpointResolver.Resolve(currentSettings!.ConfigPath);
            var inspection = await environmentInspector.InspectAsync(currentSettings!, cancellationToken).ConfigureAwait(false);
            if (inspection.HasBlockingIssues && !inspection.CanBootstrapUserConfig)
            {
                await PublishSnapshotAsync(BuildLocalStateSnapshot(
                    endpoint,
                    inspection,
                    LauncherServiceState.Stopped,
                    processController.IsRunning,
                    Copy.NoLauncherSession,
                    inspection.PrimaryIssue?.Summary ?? "启动器预检发现阻塞项。"), cancellationToken).ConfigureAwait(false);
                return;
            }

            var bootstrappedConfig = false;
            try
            {
                bootstrappedConfig = LauncherConfigBootstrap.EnsureUserConfigExists(currentSettings!.ConfigPath);
                await processController.StartAsync(currentSettings!, cancellationToken).ConfigureAwait(false);
            }
            catch (Exception ex)
            {
                await PublishSnapshotAsync(
                    BuildSnapshot(
                        endpoint,
                        inspection.Checks,
                        LauncherServiceState.Failed,
                        processController.IsRunning,
                        false,
                        false,
                        Copy.NoLauncherSession,
                        "服务端进程启动失败。",
                        ex.Message), cancellationToken).ConfigureAwait(false);
                return;
            }

            await PublishSnapshotAsync(
                snapshot with
                {
                    ServiceState = LauncherServiceState.Starting,
                    ProcessRunning = true,
                    ShutdownRequested = false,
                    ServiceDetail = bootstrappedConfig
                        ? "已基于 default.yaml 生成首份用户配置，正在等待 /healthz 返回正常。"
                        : "正在等待 /healthz 返回正常。",
                    LastError = string.Empty,
                }, cancellationToken).ConfigureAwait(false);

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
            await PublishSnapshotAsync(
                BuildSnapshot(
                    endpoint,
                    snapshot.EnvironmentChecks,
                    LauncherServiceState.Failed,
                    processController.IsRunning,
                    false,
                    false,
                    Copy.NoLauncherSession,
                    "启动超时内未通过健康检查。",
                    "服务启动已超时。"), cancellationToken).ConfigureAwait(false);
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
            await PublishSnapshotAsync(
                snapshot with
                {
                    ServiceState = LauncherServiceState.ShuttingDown,
                    ShutdownRequested = true,
                    ServiceDetail = "正在请求优雅停机。",
                    LastError = string.Empty,
                }, cancellationToken).ConfigureAwait(false);

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
            var uriBuilder = new UriBuilder(endpoint.BaseUri);
            try
            {
                if (await managementClient.GetSetupInitializedAsync(endpoint, cancellationToken).ConfigureAwait(false))
                {
                    var launcherToken = await managementClient.IssueLauncherTokenAsync(endpoint, cancellationToken).ConfigureAwait(false);
                    uriBuilder.Query = $"token={Uri.EscapeDataString(launcherToken)}";
                }
                else
                {
                    uriBuilder.Query = string.Empty;
                }
            }
            catch
            {
                uriBuilder.Query = string.Empty;
            }

            await externalOpener.OpenUriAsync(uriBuilder.Uri, cancellationToken).ConfigureAwait(false);
            await PublishSnapshotAsync(
                snapshot with
                {
                    ServiceDetail = Copy.ActionWebOpened,
                    LastError = string.Empty,
                },
                cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    }

    internal async Task OpenReleasePageAsync(CancellationToken cancellationToken = default)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            if (string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.ReleasePageUrl))
            {
                await PublishSnapshotAsync(snapshot with { ServiceDetail = Copy.VersionPageUnavailable, LastError = string.Empty }, cancellationToken).ConfigureAwait(false);
                return;
            }

            await externalOpener.OpenUriAsync(new Uri(snapshot.ReleaseCheck.ReleasePageUrl, UriKind.Absolute), cancellationToken).ConfigureAwait(false);
            await PublishSnapshotAsync(snapshot with { ServiceDetail = $"已打开 {snapshot.ReleaseCheck.ReleasePageUrl}", LastError = string.Empty }, cancellationToken).ConfigureAwait(false);
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
        builder.AppendLine($"服务状态：{Copy.FormatStatusSummary(snapshot.ServiceState)}");
        builder.AppendLine($"服务入口：{snapshot.Endpoint.BaseUri}");
        if (!string.IsNullOrWhiteSpace(snapshot.LastError))
        {
            builder.AppendLine($"最近错误：{snapshot.LastError}");
        }

        builder.AppendLine("环境检查：");
        foreach (var item in snapshot.EnvironmentChecks)
        {
            builder.AppendLine($"- {item.Title}：{Copy.FormatSeverityLabel(item.Severity)}（{item.Detail}）");
        }

        builder.AppendLine("最近错误输出：");
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
        var inspection = await environmentInspector.InspectAsync(settings, cancellationToken).ConfigureAwait(false);
        var checks = inspection.Checks;

        if (inspection.HasBlockingIssues || inspection.CanBootstrapUserConfig)
        {
            await PublishSnapshotAsync(BuildLocalStateSnapshot(
                endpoint,
                inspection,
                LauncherServiceState.Stopped,
                processController.IsRunning,
                Copy.NoLauncherSession,
                inspection.CanBootstrapUserConfig
                    ? "服务尚未启动。启动服务后会基于 default.yaml 生成首份用户配置。"
                    : inspection.PrimaryIssue?.Summary ?? "服务尚未启动。"), cancellationToken).ConfigureAwait(false);
            return;
        }

        try
        {
            if (!await managementClient.IsHealthyAsync(endpoint, cancellationToken).ConfigureAwait(false))
            {
                await PublishSnapshotAsync(BuildSnapshot(
                    endpoint,
                    checks,
                    processController.IsRunning ? LauncherServiceState.Failed : LauncherServiceState.Stopped,
                    processController.IsRunning,
                    false,
                    snapshot.ShutdownRequested && processController.IsRunning,
                    Copy.NoLauncherSession,
                    processController.IsRunning ? "子进程仍在运行，但健康检查失败。" : "服务尚未启动。",
                    processController.IsRunning ? "健康检查失败。" : string.Empty), cancellationToken).ConfigureAwait(false);
                return;
            }
        }
        catch (Exception ex)
        {
            await PublishSnapshotAsync(BuildSnapshot(
                endpoint,
                checks,
                processController.IsRunning ? LauncherServiceState.Failed : LauncherServiceState.Stopped,
                processController.IsRunning,
                false,
                snapshot.ShutdownRequested,
                Copy.NoLauncherSession,
                processController.IsRunning ? "健康检查失败。" : "服务尚未启动。",
                processController.IsRunning ? ex.Message : string.Empty), cancellationToken).ConfigureAwait(false);
            return;
        }

        sessionToken = null;
        var state = snapshot.ShutdownRequested && processController.IsRunning
            ? LauncherServiceState.ShuttingDown
            : LauncherServiceState.Ready;
        var detail = state == LauncherServiceState.ShuttingDown
            ? "服务正在停止，请稍候。"
            : "服务正在运行。";
        await PublishSnapshotAsync(BuildSnapshot(
            endpoint,
            checks,
            state,
            processController.IsRunning,
            false,
            snapshot.ShutdownRequested,
            string.Empty,
            detail,
            string.Empty), cancellationToken).ConfigureAwait(false);
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
            lastError,
            snapshot.ReleaseCheck);
    }

    private async Task PublishSnapshotAsync(LauncherSnapshot next, CancellationToken cancellationToken)
    {
        var releaseCheck = snapshot.ReleaseCheck;
        if (launcherReleaseFeed is not null)
        {
            try
            {
                releaseCheck = await launcherReleaseFeed.GetSnapshotAsync(cancellationToken).ConfigureAwait(false);
            }
            catch (Exception ex)
            {
                releaseCheck = ReleaseCheckSnapshot.Error(releaseCheck.CurrentVersion, ex.Message, releaseCheck.ReleasePageUrl);
            }
        }

        UpdateSnapshot(next with { ReleaseCheck = releaseCheck });
    }

    private void UpdateSnapshot(LauncherSnapshot next)
    {
        snapshot = next;
        SnapshotChanged?.Invoke(this, snapshot);
    }

    private LauncherSnapshot BuildLocalStateSnapshot(
        ServerEndpoint endpoint,
        EnvironmentInspection inspection,
        LauncherServiceState serviceState,
        bool processRunning,
        string sessionSummary,
        string serviceDetail)
    {
        var primaryIssue = inspection.PrimaryIssue;
        return BuildSnapshot(
            endpoint,
            inspection.Checks,
            serviceState,
            processRunning,
            false,
            false,
            sessionSummary,
            BuildLocalServiceDetail(serviceDetail, primaryIssue),
            string.Empty);
    }

    private static string BuildLocalServiceDetail(string fallbackDetail, EnvironmentCheckResult? primaryIssue)
    {
        if (primaryIssue is null)
        {
            return fallbackDetail;
        }

        var detail = string.IsNullOrWhiteSpace(primaryIssue.Detail)
            ? primaryIssue.Summary
            : $"{primaryIssue.Summary} {primaryIssue.Detail}";

        if (string.IsNullOrWhiteSpace(primaryIssue.Remediation))
        {
            return detail;
        }

        return $"{detail} {primaryIssue.Remediation}";
    }

    private void EnsureSettingsLoaded()
    {
        if (currentSettings is null)
        {
            throw new InvalidOperationException("尚未加载启动器设置。");
        }
    }
}
