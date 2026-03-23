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
    private string statusSummary = "Initializing Launcher...";
    private string sessionSummary = "No launcher session.";
    private string serviceDetail = string.Empty;
    private string lastError = string.Empty;
    private string diagnosticsSummary = string.Empty;
    private string operationSummary = string.Empty;
    private string webEndpoint = "http://127.0.0.1:8080/";

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
        var settings = new LauncherSettings(ServerExecutablePath.Trim(), ConfigPath.Trim(), Workdir.Trim());
        await ExecuteAsync("Launcher settings saved.", () => coordinator.SaveSettingsAsync(settings));
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
        StatusSummary = $"Service state: {snapshot.ServiceState}";
        SessionSummary = snapshot.SessionSummary;
        ServiceDetail = snapshot.ServiceDetail;
        LastError = snapshot.LastError;
        DiagnosticsSummary = coordinator.BuildDiagnosticsSummary();
        WebEndpoint = snapshot.Endpoint.BaseUri.ToString();

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
}

internal sealed class EnvironmentCheckViewModel
{
    internal EnvironmentCheckViewModel(EnvironmentCheckResult check)
    {
        Title = $"{check.Title} [{check.Severity}]";
        Summary = check.Severity switch
        {
            CheckSeverity.Ok => "Ready",
            CheckSeverity.Warning => "Attention needed",
            _ => "Blocking issue",
        };
        Detail = check.Detail;
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

    internal IBrush AccentBrush { get; }
}
