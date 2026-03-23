using System.Collections.ObjectModel;
using Avalonia.Media;
using Avalonia.Threading;
using RayleaBot.Launcher.Infrastructure;
using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher;

internal sealed class MainWindowViewModel : ObservableObject
{
    private readonly LauncherCoordinator coordinator;
    private string serverExecutablePath = string.Empty;
    private string configPath = string.Empty;
    private string workdir = string.Empty;
    private string statusSummary = "Initializing launcher...";
    private string heroTitle = "Inspecting local environment";
    private string sessionSummary = "No launcher session.";
    private string serviceDetail = string.Empty;
    private string lastError = string.Empty;
    private string diagnosticsSummary = string.Empty;
    private string operationSummary = string.Empty;
    private string webEndpoint = "http://127.0.0.1:8080/";
    private string versionSummary = "Version check is unavailable.";
    private string versionDetail = string.Empty;
    private string primaryIssueTitle = string.Empty;
    private string primaryIssueSummary = string.Empty;
    private string primaryIssueRemediation = string.Empty;
    private bool hasPrimaryIssue;
    private bool hasLastError;
    private bool canStart = true;
    private bool canStop;
    private bool canOpenWebUi;
    private bool canRetry = true;
    private bool canOpenReleasePage;
    private bool closeToTrayEnabled = true;
    private bool closeTipAcknowledged;
    private IBrush heroAccentBrush = Brush.Parse("#0E7490");

    internal MainWindowViewModel(LauncherCoordinator coordinator)
    {
        this.coordinator = coordinator;
        EnvironmentChecks = new ObservableCollection<EnvironmentCheckViewModel>();
        RecentStderr = new ObservableCollection<string>();
        coordinator.SnapshotChanged += CoordinatorSnapshotChanged;
    }

    internal ObservableCollection<EnvironmentCheckViewModel> EnvironmentChecks { get; }

    internal ObservableCollection<string> RecentStderr { get; }

    internal string ServerExecutablePath
    {
        get => serverExecutablePath;
        set => SetProperty(ref serverExecutablePath, value);
    }

    internal string ConfigPath
    {
        get => configPath;
        set => SetProperty(ref configPath, value);
    }

    internal string Workdir
    {
        get => workdir;
        set => SetProperty(ref workdir, value);
    }

    internal string StatusSummary
    {
        get => statusSummary;
        private set => SetProperty(ref statusSummary, value);
    }

    internal string HeroTitle
    {
        get => heroTitle;
        private set => SetProperty(ref heroTitle, value);
    }

    internal string SessionSummary
    {
        get => sessionSummary;
        private set => SetProperty(ref sessionSummary, value);
    }

    internal string ServiceDetail
    {
        get => serviceDetail;
        private set => SetProperty(ref serviceDetail, value);
    }

    internal string LastError
    {
        get => lastError;
        private set => SetProperty(ref lastError, value);
    }

    internal string DiagnosticsSummary
    {
        get => diagnosticsSummary;
        private set => SetProperty(ref diagnosticsSummary, value);
    }

    internal string OperationSummary
    {
        get => operationSummary;
        private set => SetProperty(ref operationSummary, value);
    }

    internal string WebEndpoint
    {
        get => webEndpoint;
        private set => SetProperty(ref webEndpoint, value);
    }

    internal string VersionSummary
    {
        get => versionSummary;
        private set => SetProperty(ref versionSummary, value);
    }

    internal string VersionDetail
    {
        get => versionDetail;
        private set => SetProperty(ref versionDetail, value);
    }

    internal string PrimaryIssueTitle
    {
        get => primaryIssueTitle;
        private set => SetProperty(ref primaryIssueTitle, value);
    }

    internal string PrimaryIssueSummary
    {
        get => primaryIssueSummary;
        private set => SetProperty(ref primaryIssueSummary, value);
    }

    internal string PrimaryIssueRemediation
    {
        get => primaryIssueRemediation;
        private set => SetProperty(ref primaryIssueRemediation, value);
    }

    internal bool HasPrimaryIssue
    {
        get => hasPrimaryIssue;
        private set => SetProperty(ref hasPrimaryIssue, value);
    }

    internal bool HasLastError
    {
        get => hasLastError;
        private set => SetProperty(ref hasLastError, value);
    }

    internal bool CanStart
    {
        get => canStart;
        private set => SetProperty(ref canStart, value);
    }

    internal bool CanStop
    {
        get => canStop;
        private set => SetProperty(ref canStop, value);
    }

    internal bool CanOpenWebUi
    {
        get => canOpenWebUi;
        private set => SetProperty(ref canOpenWebUi, value);
    }

    internal bool CanRetry
    {
        get => canRetry;
        private set => SetProperty(ref canRetry, value);
    }

    internal bool CanOpenReleasePage
    {
        get => canOpenReleasePage;
        private set => SetProperty(ref canOpenReleasePage, value);
    }

    internal bool CloseToTrayEnabled
    {
        get => closeToTrayEnabled;
        private set => SetProperty(ref closeToTrayEnabled, value);
    }

    internal bool CloseTipAcknowledged
    {
        get => closeTipAcknowledged;
        private set => SetProperty(ref closeTipAcknowledged, value);
    }

    internal IBrush HeroAccentBrush
    {
        get => heroAccentBrush;
        private set => SetProperty(ref heroAccentBrush, value);
    }

    internal async Task InitializeAsync()
    {
        await ExecuteAsync("Launcher initialized.", () => coordinator.InitializeAsync());
    }

    internal async Task RefreshAsync()
    {
        await ExecuteAsync(null, () => coordinator.RefreshAsync());
    }

    internal async Task RetryAsync()
    {
        await ExecuteAsync("Health/auth retry completed.", () => coordinator.RetryAsync());
    }

    internal async Task SaveSettingsAsync()
    {
        var settings = BuildSettings();
        await ExecuteAsync("Launcher settings saved.", () => coordinator.SaveSettingsAsync(settings));
    }

    internal async Task AcknowledgeCloseTipAsync()
    {
        if (CloseTipAcknowledged)
        {
            return;
        }

        CloseTipAcknowledged = true;
        await ExecuteAsync(null, () => coordinator.SaveSettingsAsync(BuildSettings()));
    }

    internal async Task StartAsync()
    {
        await ExecuteAsync("Start request completed.", () => coordinator.StartAsync());
    }

    internal async Task StopAsync()
    {
        await ExecuteAsync("Stop request completed.", () => coordinator.StopAsync());
    }

    internal async Task OpenWebUiAsync()
    {
        await ExecuteAsync("Web UI opened in the default browser.", () => coordinator.OpenWebUiAsync());
    }

    internal async Task OpenLogsDirectoryAsync()
    {
        await ExecuteAsync("Launcher log directory opened.", () => coordinator.OpenLogsDirectoryAsync());
    }

    internal async Task OpenReleasePageAsync()
    {
        await ExecuteAsync("Release page opened in the default browser.", () => coordinator.OpenReleasePageAsync());
    }

    internal void SetOperationSummary(string message)
    {
        Dispatcher.UIThread.Post(() => OperationSummary = message);
    }

    private async Task ExecuteAsync(string? successMessage, Func<Task> action)
    {
        try
        {
            await action().ConfigureAwait(false);
            if (!string.IsNullOrWhiteSpace(successMessage))
            {
                SetOperationSummary(successMessage);
            }
        }
        catch (Exception ex)
        {
            Dispatcher.UIThread.Post(() =>
            {
                LastError = ex.Message;
                OperationSummary = ex.Message;
            });
        }
    }

    private void CoordinatorSnapshotChanged(object? sender, LauncherSnapshot snapshot)
    {
        Dispatcher.UIThread.Post(() => ApplySnapshot(snapshot));
    }

    private void ApplySnapshot(LauncherSnapshot snapshot)
    {
        ServerExecutablePath = snapshot.Settings.ServerExecutablePath;
        ConfigPath = snapshot.Settings.ConfigPath;
        Workdir = snapshot.Settings.Workdir;
        CloseToTrayEnabled = snapshot.Settings.CloseToTrayEnabled;
        CloseTipAcknowledged = snapshot.Settings.CloseTipAcknowledged;
        StatusSummary = snapshot.ServiceState switch
        {
            LauncherServiceState.Stopped => "Stopped",
            LauncherServiceState.Starting => "Starting",
            LauncherServiceState.HealthOnly => "Health only",
            LauncherServiceState.Ready => "Ready",
            LauncherServiceState.Degraded => "Degraded",
            LauncherServiceState.SetupRequired => "Setup required",
            LauncherServiceState.ShuttingDown => "Shutting down",
            LauncherServiceState.Failed => "Failed",
            _ => snapshot.ServiceState.ToString(),
        };
        HeroTitle = snapshot.ServiceState switch
        {
            LauncherServiceState.Stopped when snapshot.EnvironmentChecks.Any(item => item.Code == "config.bootstrap_available") => "First-start config is ready to bootstrap",
            LauncherServiceState.Stopped => "Service is not running",
            LauncherServiceState.Starting => "Starting RayleaBot",
            LauncherServiceState.HealthOnly => "Service is alive, management is limited",
            LauncherServiceState.Ready => "Service is ready",
            LauncherServiceState.Degraded => "Service is running with degraded dependencies",
            LauncherServiceState.SetupRequired => "Initial setup is still required",
            LauncherServiceState.ShuttingDown => "Graceful shutdown in progress",
            LauncherServiceState.Failed => "Service startup or runtime failed",
            _ => "Launcher status",
        };
        SessionSummary = snapshot.SessionSummary;
        ServiceDetail = snapshot.ServiceDetail;
        LastError = snapshot.LastError;
        HasLastError = !string.IsNullOrWhiteSpace(snapshot.LastError);
        DiagnosticsSummary = coordinator.BuildDiagnosticsSummary();
        WebEndpoint = snapshot.Endpoint.BaseUri.ToString();
        VersionSummary = snapshot.ReleaseCheck.Summary;
        VersionDetail = snapshot.ReleaseCheck.Detail;
        HeroAccentBrush = snapshot.ServiceState switch
        {
            LauncherServiceState.Ready => Brush.Parse("#16A34A"),
            LauncherServiceState.Degraded or LauncherServiceState.SetupRequired or LauncherServiceState.HealthOnly => Brush.Parse("#D97706"),
            LauncherServiceState.Failed => Brush.Parse("#DC2626"),
            LauncherServiceState.Starting or LauncherServiceState.ShuttingDown => Brush.Parse("#0EA5E9"),
            _ => Brush.Parse("#475569"),
        };

        var primaryIssue = snapshot.EnvironmentChecks
            .FirstOrDefault(item => item.Severity == CheckSeverity.Error) ??
            snapshot.EnvironmentChecks.FirstOrDefault(item => item.Severity == CheckSeverity.Warning);
        HasPrimaryIssue = primaryIssue is not null;
        PrimaryIssueTitle = primaryIssue?.Title ?? string.Empty;
        PrimaryIssueSummary = primaryIssue?.Detail ?? string.Empty;
        PrimaryIssueRemediation = primaryIssue?.Remediation ?? string.Empty;

        var hasBlockingIssue = snapshot.EnvironmentChecks.Any(item => item.Severity == CheckSeverity.Error);
        CanStart = !snapshot.ProcessRunning && !hasBlockingIssue;
        CanStop = snapshot.ProcessRunning || snapshot.ServiceState is LauncherServiceState.Starting or LauncherServiceState.ShuttingDown;
        CanOpenWebUi = snapshot.ServiceState is LauncherServiceState.HealthOnly or LauncherServiceState.Ready or LauncherServiceState.Degraded or LauncherServiceState.SetupRequired or LauncherServiceState.ShuttingDown;
        CanRetry = true;
        CanOpenReleasePage = !string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.ReleasePageUrl);

        EnvironmentChecks.Clear();
        foreach (var item in snapshot.EnvironmentChecks)
        {
            EnvironmentChecks.Add(new EnvironmentCheckViewModel(item));
        }

        RecentStderr.Clear();
        foreach (var line in snapshot.RecentStderr)
        {
            RecentStderr.Add(line);
        }
    }

    private LauncherSettings BuildSettings()
    {
        return new LauncherSettings(
            ServerExecutablePath.Trim(),
            ConfigPath.Trim(),
            Workdir.Trim(),
            CloseToTrayEnabled,
            CloseTipAcknowledged);
    }
}

internal sealed class EnvironmentCheckViewModel
{
    internal EnvironmentCheckViewModel(EnvironmentCheckResult check)
    {
        Title = check.Title;
        Summary = check.Summary;
        Detail = check.Detail;
        Remediation = check.Remediation;
        SeverityLabel = check.Severity switch
        {
            CheckSeverity.Ok => "Ready",
            CheckSeverity.Warning => "Needs attention",
            _ => "Blocking",
        };
        AccentBrush = check.Severity switch
        {
            CheckSeverity.Ok => Brush.Parse("#12B76A"),
            CheckSeverity.Warning => Brush.Parse("#FDB022"),
            _ => Brush.Parse("#F04438"),
        };
    }

    internal string Title { get; }

    internal string Summary { get; }

    internal string Detail { get; }

    internal string Remediation { get; }

    internal string SeverityLabel { get; }

    internal IBrush AccentBrush { get; }
}
