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
    private readonly bool marshalToUiThread;
    private readonly ObservableCollection<LauncherNavigationItemViewModel> navigationItems;
    private readonly LauncherCopy copy = LauncherCopy.Default;
    private string serverExecutablePath = string.Empty;
    private string configPath = string.Empty;
    private string workdir = string.Empty;
    private string statusSummary = "未启动";
    private string heroTitle = "服务未启动";
    private string sessionSummary = string.Empty;
    private string serviceDetail = string.Empty;
    private string lastError = string.Empty;
    private string diagnosticsSummary = string.Empty;
    private string operationSummary = string.Empty;
    private string webEndpoint = "http://127.0.0.1:8080/";
    private string versionSummary = LauncherCopy.Default.VersionUnavailableSummary;
    private string versionDetail = LauncherCopy.Default.VersionUnavailableDetail;
    private string primaryIssueTitle = string.Empty;
    private string primaryIssueSummary = string.Empty;
    private string primaryIssueDetail = string.Empty;
    private string primaryIssueRemediation = string.Empty;
    private string windowStateGlyph = "□";
    private LauncherSection activeSection = LauncherSection.Overview;
    private LauncherNavigationItemViewModel? selectedNavigationItem;
    private bool hasPrimaryIssue;
    private bool hasLastError;
    private bool canStart = true;
    private bool canStop;
    private bool canOpenWebUi;
    private bool canRetry = true;
    private bool canOpenReleasePage;
    private bool closeToTrayEnabled = true;
    private bool closeTipAcknowledged;
    private IBrush heroAccentBrush = Brush.Parse("#39BDF8");
    private LauncherServiceState currentServiceState;

    internal MainWindowViewModel(LauncherCoordinator coordinator, bool marshalToUiThread = true)
    {
        this.coordinator = coordinator;
        this.marshalToUiThread = marshalToUiThread;
        EnvironmentChecks = new ObservableCollection<EnvironmentCheckViewModel>();
        RecentStderr = new ObservableCollection<string>();
        navigationItems =
        [
            new LauncherNavigationItemViewModel(LauncherSection.Overview, copy.OverviewTitle, copy.OverviewSummary),
            new LauncherNavigationItemViewModel(LauncherSection.ServiceControls, copy.ServiceControlsTitle, copy.ServiceControlsSummary),
            new LauncherNavigationItemViewModel(LauncherSection.Environment, copy.EnvironmentTitle, copy.EnvironmentSummary),
            new LauncherNavigationItemViewModel(LauncherSection.Settings, copy.SettingsTitle, copy.SettingsSummary),
            new LauncherNavigationItemViewModel(LauncherSection.Diagnostics, copy.DiagnosticsTitle, copy.DiagnosticsSummary),
        ];
        NavigationItems = new ReadOnlyObservableCollection<LauncherNavigationItemViewModel>(navigationItems);
        ActivateSection(LauncherSection.Overview);
        coordinator.SnapshotChanged += CoordinatorSnapshotChanged;
    }

    internal LauncherCopy Copy => copy;

    internal ReadOnlyObservableCollection<LauncherNavigationItemViewModel> NavigationItems { get; }

    internal ObservableCollection<EnvironmentCheckViewModel> EnvironmentChecks { get; }

    internal ObservableCollection<string> RecentStderr { get; }

    internal LauncherNavigationItemViewModel? SelectedNavigationItem
    {
        get => selectedNavigationItem;
        set
        {
            if (SetProperty(ref selectedNavigationItem, value) && value is not null)
            {
                ActivateSection(value.Section, updateSelection: false);
            }
        }
    }

    internal LauncherSection ActiveSection
    {
        get => activeSection;
        private set => SetProperty(ref activeSection, value);
    }

    internal bool IsOverviewSectionActive => ActiveSection == LauncherSection.Overview;

    internal bool IsServiceControlsSectionActive => ActiveSection == LauncherSection.ServiceControls;

    internal bool IsEnvironmentSectionActive => ActiveSection == LauncherSection.Environment;

    internal bool IsSettingsSectionActive => ActiveSection == LauncherSection.Settings;

    internal bool IsDiagnosticsSectionActive => ActiveSection == LauncherSection.Diagnostics;

    internal bool IsSetupRequired => false;

    internal bool IsNotSetupRequired => true;

    internal string OpenWebUiActionLabel => copy.OpenWebUiLabel;

    internal bool IsExternalServiceDetected => currentServiceState == LauncherServiceState.ExternalService;

    internal bool RequiresExternalStopConfirmation => IsExternalServiceDetected;

    internal IEnumerable<EnvironmentCheckViewModel> BlockingEnvironmentChecks => EnvironmentChecks.Where(item => item.Severity == CheckSeverity.Error);

    internal IEnumerable<EnvironmentCheckViewModel> WarningEnvironmentChecks => EnvironmentChecks.Where(item => item.Severity == CheckSeverity.Warning);

    internal IEnumerable<EnvironmentCheckViewModel> ReadyEnvironmentChecks => EnvironmentChecks.Where(item => item.Severity == CheckSeverity.Ok);

    internal bool HasBlockingEnvironmentChecks => BlockingEnvironmentChecks.Any();

    internal bool HasWarningEnvironmentChecks => WarningEnvironmentChecks.Any();

    internal bool HasReadyEnvironmentChecks => ReadyEnvironmentChecks.Any();

    internal bool HasNoBlockingEnvironmentChecks => !HasBlockingEnvironmentChecks;

    internal bool HasNoWarningEnvironmentChecks => !HasWarningEnvironmentChecks;

    internal bool HasNoReadyEnvironmentChecks => !HasReadyEnvironmentChecks;

    internal int BlockingEnvironmentCheckCount => BlockingEnvironmentChecks.Count();

    internal int WarningEnvironmentCheckCount => WarningEnvironmentChecks.Count();

    internal int ReadyEnvironmentCheckCount => ReadyEnvironmentChecks.Count();

    internal bool HasRecentStderr => RecentStderr.Count > 0;

    internal bool HasNoRecentStderr => !HasRecentStderr;

    internal string WindowStateGlyph
    {
        get => windowStateGlyph;
        private set => SetProperty(ref windowStateGlyph, value);
    }

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

    internal string PrimaryIssueDetail
    {
        get => primaryIssueDetail;
        private set => SetProperty(ref primaryIssueDetail, value);
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

    internal bool HasNoPrimaryIssue => !HasPrimaryIssue;

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
        await ExecuteAsync(copy.ActionLauncherInitialized, () => coordinator.InitializeAsync());
    }

    internal async Task RefreshAsync()
    {
        await ExecuteAsync(null, () => coordinator.RefreshAsync());
    }

    internal async Task RetryAsync()
    {
        await ExecuteAsync(copy.ActionHealthRetryFinished, () => coordinator.RetryAsync());
    }

    internal async Task SaveSettingsAsync()
    {
        var settings = BuildSettings();
        await ExecuteAsync(copy.ActionSettingsSaved, () => coordinator.SaveSettingsAsync(settings));
    }

    internal async Task StartAsync()
    {
        await ExecuteAsync(copy.ActionStartFinished, () => coordinator.StartAsync());
    }

    internal async Task StopAsync()
    {
        await ExecuteAsync(copy.ActionStopFinished, () => coordinator.StopAsync());
    }

    internal async Task OpenWebUiAsync()
    {
        await ExecuteAsync(
            copy.ActionWebOpened,
            () => coordinator.OpenWebUiAsync());
    }

    internal async Task OpenLogsDirectoryAsync()
    {
        await ExecuteAsync(copy.ActionLogsOpened, () => coordinator.OpenLogsDirectoryAsync());
    }

    internal async Task OpenReleasePageAsync()
    {
        await ExecuteAsync(copy.ActionReleasePageOpened, () => coordinator.OpenReleasePageAsync());
    }

    internal void SetActiveSection(LauncherSection section)
    {
        ActivateSection(section);
    }

    internal string ExternalStopConfirmTitle => copy.ExternalStopConfirmTitle;

    internal string ExternalStopConfirmBody => copy.ExternalStopConfirmBody;

    internal string ExternalStopConfirmFootnote => copy.ExternalStopConfirmFootnote;

    internal string ExternalStopConfirmAction => copy.ExternalStopConfirmAction;

    internal string ExternalStopCancelAction => copy.ExternalStopCancelAction;

    internal void SetOperationSummary(string message)
    {
        Dispatcher.UIThread.Post(() => OperationSummary = message);
    }

    internal void SetWindowStateGlyph(bool maximized)
    {
        WindowStateGlyph = maximized ? "❐" : "□";
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
        if (!marshalToUiThread)
        {
            ApplySnapshot(snapshot);
            return;
        }

        Dispatcher.UIThread.Post(() => ApplySnapshot(snapshot));
    }

    private void ApplySnapshot(LauncherSnapshot snapshot)
    {
        ServerExecutablePath = snapshot.Settings.ServerExecutablePath;
        ConfigPath = snapshot.Settings.ConfigPath;
        Workdir = snapshot.Settings.Workdir;
        CloseToTrayEnabled = snapshot.Settings.CloseToTrayEnabled;
        CloseTipAcknowledged = snapshot.Settings.CloseTipAcknowledged;
        StatusSummary = copy.FormatStatusSummary(snapshot.ServiceState);
        currentServiceState = snapshot.ServiceState;
        HeroTitle = copy.FormatHeroTitle(snapshot.ServiceState, snapshot.EnvironmentChecks);
        SessionSummary = snapshot.SessionSummary;
        ServiceDetail = snapshot.ServiceDetail;
        LastError = snapshot.LastError;
        HasLastError = !string.IsNullOrWhiteSpace(snapshot.LastError);
        DiagnosticsSummary = coordinator.BuildDiagnosticsSummary();
        WebEndpoint = snapshot.Endpoint.BaseUri.ToString();
        VersionSummary = string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.Summary)
            ? copy.VersionUnavailableSummary
            : snapshot.ReleaseCheck.Summary;
        VersionDetail = snapshot.ReleaseCheck.Detail;
        HeroAccentBrush = snapshot.ServiceState switch
        {
            LauncherServiceState.Ready => Brush.Parse("#3BE38D"),
            LauncherServiceState.ExternalService => Brush.Parse("#68C3FF"),
            LauncherServiceState.Degraded or LauncherServiceState.HealthOnly => Brush.Parse("#FFB84D"),
            LauncherServiceState.Failed => Brush.Parse("#FF6B7D"),
            LauncherServiceState.Starting or LauncherServiceState.ShuttingDown => Brush.Parse("#66D0FF"),
            _ => Brush.Parse("#8DA6C8"),
        };

        var primaryIssue = snapshot.EnvironmentChecks
            .FirstOrDefault(item => item.Severity == CheckSeverity.Error) ??
            snapshot.EnvironmentChecks.FirstOrDefault(item => item.Severity == CheckSeverity.Warning);
        HasPrimaryIssue = primaryIssue is not null;
        PrimaryIssueTitle = primaryIssue?.Title ?? string.Empty;
        PrimaryIssueSummary = primaryIssue?.Summary ?? string.Empty;
        PrimaryIssueDetail = primaryIssue?.Detail ?? string.Empty;
        PrimaryIssueRemediation = primaryIssue?.Remediation ?? string.Empty;
        if (primaryIssue is null)
        {
            PrimaryIssueTitle = copy.NoPrimaryIssueTitle;
            PrimaryIssueSummary = copy.NoPrimaryIssueSummary;
            PrimaryIssueDetail = copy.NoPrimaryIssueDetail;
            PrimaryIssueRemediation = string.Empty;
        }

        var hasBlockingIssue = snapshot.EnvironmentChecks.Any(item => item.Severity == CheckSeverity.Error);
        CanStart = !snapshot.ProcessRunning &&
                   snapshot.ServiceState is not LauncherServiceState.ExternalService &&
                   !hasBlockingIssue;
        CanStop = snapshot.ProcessRunning ||
                  snapshot.ServiceState is LauncherServiceState.Starting or LauncherServiceState.ShuttingDown or LauncherServiceState.ExternalService;
        CanOpenWebUi = snapshot.ServiceState is LauncherServiceState.HealthOnly or LauncherServiceState.Ready or LauncherServiceState.Degraded or LauncherServiceState.ShuttingDown or LauncherServiceState.ExternalService;
        CanRetry = true;
        CanOpenReleasePage = !string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.ReleasePageUrl);

        EnvironmentChecks.Clear();
        foreach (var item in snapshot.EnvironmentChecks)
        {
            EnvironmentChecks.Add(new EnvironmentCheckViewModel(item, copy));
        }

        RecentStderr.Clear();
        foreach (var line in snapshot.RecentStderr)
        {
            RecentStderr.Add(line);
        }

        OnPropertyChanged(nameof(BlockingEnvironmentChecks));
        OnPropertyChanged(nameof(WarningEnvironmentChecks));
        OnPropertyChanged(nameof(ReadyEnvironmentChecks));
        OnPropertyChanged(nameof(HasBlockingEnvironmentChecks));
        OnPropertyChanged(nameof(HasWarningEnvironmentChecks));
        OnPropertyChanged(nameof(HasReadyEnvironmentChecks));
        OnPropertyChanged(nameof(HasNoBlockingEnvironmentChecks));
        OnPropertyChanged(nameof(HasNoWarningEnvironmentChecks));
        OnPropertyChanged(nameof(HasNoReadyEnvironmentChecks));
        OnPropertyChanged(nameof(BlockingEnvironmentCheckCount));
        OnPropertyChanged(nameof(WarningEnvironmentCheckCount));
        OnPropertyChanged(nameof(ReadyEnvironmentCheckCount));
        OnPropertyChanged(nameof(HasRecentStderr));
        OnPropertyChanged(nameof(HasNoRecentStderr));
        OnPropertyChanged(nameof(HasNoPrimaryIssue));
        OnPropertyChanged(nameof(IsSetupRequired));
        OnPropertyChanged(nameof(IsNotSetupRequired));
        OnPropertyChanged(nameof(IsExternalServiceDetected));
        OnPropertyChanged(nameof(RequiresExternalStopConfirmation));
        OnPropertyChanged(nameof(OpenWebUiActionLabel));
    }

    private void ActivateSection(LauncherSection section, bool updateSelection = true)
    {
        if (ActiveSection == section && (!updateSelection || SelectedNavigationItem?.Section == section))
        {
            return;
        }

        ActiveSection = section;
        foreach (var item in navigationItems)
        {
            item.SetActive(item.Section == section);
            if (updateSelection && item.Section == section)
            {
                selectedNavigationItem = item;
            }
        }

        if (updateSelection)
        {
            OnPropertyChanged(nameof(SelectedNavigationItem));
        }

        OnPropertyChanged(nameof(IsOverviewSectionActive));
        OnPropertyChanged(nameof(IsServiceControlsSectionActive));
        OnPropertyChanged(nameof(IsEnvironmentSectionActive));
        OnPropertyChanged(nameof(IsSettingsSectionActive));
        OnPropertyChanged(nameof(IsDiagnosticsSectionActive));
    }

    private LauncherSettings BuildSettings()
    {
        return new LauncherSettings(
            ServerExecutablePath.Trim(),
            ConfigPath.Trim(),
            Workdir.Trim(),
            CloseToTrayEnabled,
            false);
    }
}

internal sealed class LauncherNavigationItemViewModel : ObservableObject
{
    private bool isActive;
    private IBrush backgroundBrush = Brush.Parse("#0F1C30");
    private IBrush borderBrush = Brush.Parse("#203549");
    private IBrush titleBrush = Brush.Parse("#EAF3FF");
    private IBrush summaryBrush = Brush.Parse("#9EB3D1");

    internal LauncherNavigationItemViewModel(LauncherSection section, string title, string summary)
    {
        Section = section;
        Title = title;
        Summary = summary;
    }

    internal LauncherSection Section { get; }

    internal string Title { get; }

    internal string Summary { get; }

    internal bool IsActive
    {
        get => isActive;
        private set => SetProperty(ref isActive, value);
    }

    internal IBrush BackgroundBrush
    {
        get => backgroundBrush;
        private set => SetProperty(ref backgroundBrush, value);
    }

    internal IBrush BorderBrush
    {
        get => borderBrush;
        private set => SetProperty(ref borderBrush, value);
    }

    internal IBrush TitleBrush
    {
        get => titleBrush;
        private set => SetProperty(ref titleBrush, value);
    }

    internal IBrush SummaryBrush
    {
        get => summaryBrush;
        private set => SetProperty(ref summaryBrush, value);
    }

    internal void SetActive(bool active)
    {
        IsActive = active;
        BackgroundBrush = active ? Brush.Parse("#18304A") : Brush.Parse("#0F1C30");
        BorderBrush = active ? Brush.Parse("#4DBFFF") : Brush.Parse("#203549");
        TitleBrush = Brush.Parse("#F7FBFF");
        SummaryBrush = active ? Brush.Parse("#E6F4FF") : Brush.Parse("#AFC1D6");
    }
}

internal sealed class EnvironmentCheckViewModel
{
    internal EnvironmentCheckViewModel(EnvironmentCheckResult check, LauncherCopy copy)
    {
        Severity = check.Severity;
        Title = check.Title;
        Summary = check.Summary;
        Detail = check.Detail;
        Remediation = check.Remediation;
        SeverityLabel = copy.FormatSeverityLabel(check.Severity);
        AccentBrush = check.Severity switch
        {
            CheckSeverity.Ok => Brush.Parse("#3BE38D"),
            CheckSeverity.Warning => Brush.Parse("#FFB84D"),
            _ => Brush.Parse("#FF6B7D"),
        };
    }

    internal CheckSeverity Severity { get; }

    internal string Title { get; }

    internal string Summary { get; }

    internal string Detail { get; }

    internal string Remediation { get; }

    internal string SeverityLabel { get; }

    internal IBrush AccentBrush { get; }
}
