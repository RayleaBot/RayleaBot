using System.Collections.ObjectModel;
using Avalonia.Media;
using Avalonia.Threading;
using FluentAvalonia.UI.Controls;
using RayleaBot.Launcher.Infrastructure;
using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher;

internal sealed class MainWindowViewModel : ObservableObject
{
    private readonly LauncherCoordinator coordinator;
    private readonly bool marshalToUiThread;
    private readonly List<LauncherNavigationItemViewModel> navigationItems;
    private readonly LauncherCopy copy = LauncherCopy.Default;
    private LauncherSettings? appliedSettings;
    private string serverExecutablePath = string.Empty;
    private string configPath = string.Empty;
    private string workdir = string.Empty;
    private string statusSummary = "未启动";
    private string heroTitle = "服务未启动";
    private string serviceDetail = string.Empty;
    private string lastError = string.Empty;
    private string diagnosticsSummary = string.Empty;
    private string operationSummary = string.Empty;
    private string webEndpoint = "http://127.0.0.1:8080/";
    private string versionSummary = LauncherCopy.Default.VersionUnavailableSummary;
    private string versionDetail = LauncherCopy.Default.VersionUnavailableDetail;
    private string processIdSummary = string.Empty;
    private string homeAlertTitle = string.Empty;
    private string homeAlertMessage = string.Empty;
    private string pendingActionMessage = string.Empty;
    private string environmentPackagingSummary = string.Empty;
    private string environmentPackagingDetail = string.Empty;
    private bool isWindowMaximized;
    private LauncherSection activeSection = LauncherSection.Status;
    private LauncherNavigationItemViewModel? selectedNavigationItem;
    private bool hasLastError;
    private bool hasHomeAlert;
    private bool hasProcessId;
    private bool hasEnvironmentPackagingNotice;
    private bool canStart = true;
    private bool canStop;
    private bool canOpenWebUi;
    private bool canRetry = true;
    private bool canOpenReleasePage;
    private bool closeToTrayEnabled = true;
    private bool closeTipAcknowledged;
    private bool isSettingsEditing;
    private bool isActionInProgress;
    private IBrush heroAccentBrush = Brush.Parse("#38BDF8");
    private LauncherServiceState currentServiceState;
    private LauncherPrimaryAction primaryAction;
    private LauncherUiAction pendingAction = LauncherUiAction.None;
    private InfoBarSeverity homeAlertSeverity = InfoBarSeverity.Informational;

    internal MainWindowViewModel(LauncherCoordinator coordinator, bool marshalToUiThread = true)
    {
        this.coordinator = coordinator;
        this.marshalToUiThread = marshalToUiThread;

        EnvironmentChecks = new ObservableCollection<EnvironmentCheckViewModel>();
        RecentStderr = new ObservableCollection<string>();
        navigationItems =
        [
            new LauncherNavigationItemViewModel(LauncherSection.Status, copy.StatusTitle, string.Empty, isFooterItem: false),
            new LauncherNavigationItemViewModel(LauncherSection.Environment, copy.EnvironmentTitle, string.Empty, isFooterItem: false),
            new LauncherNavigationItemViewModel(LauncherSection.Diagnostics, copy.DiagnosticsTitle, string.Empty, isFooterItem: false),
            new LauncherNavigationItemViewModel(LauncherSection.Settings, copy.SettingsTitle, string.Empty, isFooterItem: true),
        ];

        NavigationItems = navigationItems.AsReadOnly();
        MainNavigationItems = navigationItems.Where(item => !item.IsFooterItem).ToArray();
        FooterNavigationItems = navigationItems.Where(item => item.IsFooterItem).ToArray();
        ActivateSection(LauncherSection.Status);
        coordinator.SnapshotChanged += CoordinatorSnapshotChanged;
    }

    internal LauncherCopy Copy => copy;

    internal IReadOnlyList<LauncherNavigationItemViewModel> NavigationItems { get; }

    internal IReadOnlyList<LauncherNavigationItemViewModel> MainNavigationItems { get; }

    internal IReadOnlyList<LauncherNavigationItemViewModel> FooterNavigationItems { get; }

    internal ObservableCollection<EnvironmentCheckViewModel> EnvironmentChecks { get; }

    internal ObservableCollection<string> RecentStderr { get; }

    internal IEnumerable<string> HomeRecentStderr => RecentStderr.Take(10);

    internal bool HasHomeRecentStderr => HomeRecentStderr.Any();

    internal bool HasNoHomeRecentStderr => !HasHomeRecentStderr;

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

    internal bool IsStatusSectionActive => ActiveSection == LauncherSection.Status;

    internal bool IsEnvironmentSectionActive => ActiveSection == LauncherSection.Environment;

    internal bool IsDiagnosticsSectionActive => ActiveSection == LauncherSection.Diagnostics;

    internal bool IsSettingsSectionActive => ActiveSection == LauncherSection.Settings;

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

    internal bool IsWindowMaximized
    {
        get => isWindowMaximized;
        private set => SetProperty(ref isWindowMaximized, value);
    }

    internal bool IsWindowNormal => !IsWindowMaximized;

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

    internal string ProcessIdSummary
    {
        get => processIdSummary;
        private set => SetProperty(ref processIdSummary, value);
    }

    internal string PendingActionMessage
    {
        get => pendingActionMessage;
        private set => SetProperty(ref pendingActionMessage, value);
    }

    internal bool IsActionInProgress
    {
        get => isActionInProgress;
        private set => SetProperty(ref isActionInProgress, value);
    }

    internal bool HasProcessId
    {
        get => hasProcessId;
        private set => SetProperty(ref hasProcessId, value);
    }

    internal bool HasNoProcessId => !HasProcessId;

    internal bool HasLastError
    {
        get => hasLastError;
        private set => SetProperty(ref hasLastError, value);
    }

    internal bool HasHomeAlert
    {
        get => hasHomeAlert;
        private set => SetProperty(ref hasHomeAlert, value);
    }

    internal string HomeAlertTitle
    {
        get => homeAlertTitle;
        private set => SetProperty(ref homeAlertTitle, value);
    }

    internal string HomeAlertMessage
    {
        get => homeAlertMessage;
        private set => SetProperty(ref homeAlertMessage, value);
    }

    internal InfoBarSeverity HomeAlertSeverity
    {
        get => homeAlertSeverity;
        private set => SetProperty(ref homeAlertSeverity, value);
    }

    internal bool HasEnvironmentPackagingNotice
    {
        get => hasEnvironmentPackagingNotice;
        private set => SetProperty(ref hasEnvironmentPackagingNotice, value);
    }

    internal string EnvironmentPackagingSummary
    {
        get => environmentPackagingSummary;
        private set => SetProperty(ref environmentPackagingSummary, value);
    }

    internal string EnvironmentPackagingDetail
    {
        get => environmentPackagingDetail;
        private set => SetProperty(ref environmentPackagingDetail, value);
    }

    internal bool CanStart
    {
        get => canStart && !IsActionInProgress;
        private set => SetProperty(ref canStart, value);
    }

    internal bool CanStop
    {
        get => canStop && !IsActionInProgress;
        private set => SetProperty(ref canStop, value);
    }

    internal bool CanOpenWebUi
    {
        get => canOpenWebUi && !IsActionInProgress;
        private set => SetProperty(ref canOpenWebUi, value);
    }

    internal bool CanRetry
    {
        get => canRetry && !IsActionInProgress;
        private set => SetProperty(ref canRetry, value);
    }

    internal bool CanOpenReleasePage
    {
        get => canOpenReleasePage && !IsActionInProgress;
        private set => SetProperty(ref canOpenReleasePage, value);
    }

    internal bool CloseToTrayEnabled
    {
        get => closeToTrayEnabled;
        set => SetProperty(ref closeToTrayEnabled, value);
    }

    internal bool CloseTipAcknowledged
    {
        get => closeTipAcknowledged;
        private set => SetProperty(ref closeTipAcknowledged, value);
    }

    internal bool IsSettingsEditing
    {
        get => isSettingsEditing;
        private set => SetProperty(ref isSettingsEditing, value);
    }

    internal bool AreSettingsReadOnly => !IsSettingsEditing;

    internal bool CanSaveSettings => IsSettingsEditing && !IsActionInProgress;

    internal IBrush HeroAccentBrush
    {
        get => heroAccentBrush;
        private set => SetProperty(ref heroAccentBrush, value);
    }

    internal LauncherPrimaryAction PrimaryAction
    {
        get => primaryAction;
        private set => SetProperty(ref primaryAction, value);
    }

    internal string PrimaryActionLabel =>
        PrimaryAction switch
        {
            LauncherPrimaryAction.OpenWebUi => copy.OpenWebUiLabel,
            LauncherPrimaryAction.StartService => copy.StartServiceLabel,
            _ => string.Empty,
        };

    internal string PrimaryActionDisplayLabel =>
        pendingAction is LauncherUiAction.OpenWebUi or LauncherUiAction.StartService
            ? copy.FormatPendingPrimaryActionLabel(PrimaryAction)
            : PrimaryActionLabel;

    internal string StopActionDisplayLabel =>
        pendingAction == LauncherUiAction.StopService ? copy.StopServicePendingLabel : copy.StopServiceLabel;

    internal string RetryActionDisplayLabel =>
        pendingAction == LauncherUiAction.Retry ? copy.RetryPendingLabel : copy.RetryHealthAuthLabel;

    internal string OpenLogsActionDisplayLabel =>
        pendingAction == LauncherUiAction.OpenLogs ? copy.OpenLogsPendingLabel : copy.OpenLogsDirectoryLabel;

    internal string OpenReleasePageActionDisplayLabel =>
        pendingAction == LauncherUiAction.OpenReleasePage ? copy.OpenReleasePagePendingLabel : copy.OpenReleasePageLabel;

    internal string SaveSettingsActionDisplayLabel =>
        pendingAction == LauncherUiAction.SaveSettings ? copy.SaveSettingsPendingLabel : copy.SaveSettingsLabel;

    internal bool CanRunPrimaryAction =>
        PrimaryAction switch
        {
            LauncherPrimaryAction.OpenWebUi => CanOpenWebUi,
            LauncherPrimaryAction.StartService => CanStart,
            _ => false,
        };

    internal async Task InitializeAsync()
    {
        await ExecuteAsync(LauncherUiAction.Initialize, copy.ActionLauncherInitializing, copy.ActionLauncherInitialized, () => coordinator.InitializeAsync()).ConfigureAwait(false);
    }

    internal async Task RefreshAsync()
    {
        await ExecuteAsync(LauncherUiAction.Refresh, null, null, () => coordinator.RefreshAsync()).ConfigureAwait(false);
    }

    internal async Task RetryAsync()
    {
        await ExecuteAsync(LauncherUiAction.Retry, copy.ActionHealthRetryPending, copy.ActionHealthRetryFinished, () => coordinator.RetryAsync()).ConfigureAwait(false);
    }

    internal async Task SaveSettingsAsync()
    {
        var settings = BuildSettings();
        var saved = await ExecuteAsync(LauncherUiAction.SaveSettings, copy.ActionSettingsSaving, copy.ActionSettingsSaved, () => coordinator.SaveSettingsAsync(settings)).ConfigureAwait(false);
        if (saved)
        {
            appliedSettings = settings;
            SetSettingsEditing(false);
        }
    }

    internal async Task StartAsync()
    {
        await ExecuteAsync(LauncherUiAction.StartService, copy.ActionStartPending, copy.ActionStartFinished, () => coordinator.StartAsync()).ConfigureAwait(false);
    }

    internal async Task StopAsync()
    {
        await ExecuteAsync(LauncherUiAction.StopService, copy.ActionStopPending, copy.ActionStopFinished, () => coordinator.StopAsync()).ConfigureAwait(false);
    }

    internal async Task OpenWebUiAsync()
    {
        await ExecuteAsync(LauncherUiAction.OpenWebUi, copy.ActionWebOpening, copy.ActionWebOpened, () => coordinator.OpenWebUiAsync()).ConfigureAwait(false);
    }

    internal async Task OpenLogsDirectoryAsync()
    {
        await ExecuteAsync(LauncherUiAction.OpenLogs, copy.ActionLogsOpening, copy.ActionLogsOpened, () => coordinator.OpenLogsDirectoryAsync()).ConfigureAwait(false);
    }

    internal async Task OpenReleasePageAsync()
    {
        await ExecuteAsync(LauncherUiAction.OpenReleasePage, copy.ActionReleasePageOpening, copy.ActionReleasePageOpened, () => coordinator.OpenReleasePageAsync()).ConfigureAwait(false);
    }

    internal async Task RunPrimaryActionAsync()
    {
        switch (PrimaryAction)
        {
            case LauncherPrimaryAction.OpenWebUi:
                await OpenWebUiAsync().ConfigureAwait(false);
                break;
            case LauncherPrimaryAction.StartService:
                await StartAsync().ConfigureAwait(false);
                break;
        }
    }

    internal void SetActiveSection(LauncherSection section)
    {
        ActivateSection(section);
    }

    internal void NavigateToEnvironment()
    {
        ActivateSection(LauncherSection.Environment);
    }

    internal string ExternalStopConfirmTitle => copy.ExternalStopConfirmTitle;

    internal string ExternalStopConfirmBody => copy.ExternalStopConfirmBody;

    internal string ExternalStopConfirmFootnote => copy.ExternalStopConfirmFootnote;

    internal string ExternalStopConfirmAction => copy.ExternalStopConfirmAction;

    internal string ExternalStopCancelAction => copy.ExternalStopCancelAction;

    internal void SetOperationSummary(string message)
    {
        if (!marshalToUiThread || Dispatcher.UIThread.CheckAccess())
        {
            OperationSummary = message;
            return;
        }

        Dispatcher.UIThread.Post(() => OperationSummary = message);
    }

    internal void SetWindowState(bool maximized)
    {
        IsWindowMaximized = maximized;
        OnPropertyChanged(nameof(IsWindowNormal));
    }

    internal void BeginSettingsEditing()
    {
        if (IsSettingsEditing)
        {
            return;
        }

        SetSettingsEditing(true);
        SetOperationSummary(copy.ActionSettingsEditStarted);
    }

    internal void CancelSettingsEditing()
    {
        if (!IsSettingsEditing)
        {
            return;
        }

        if (appliedSettings is not null)
        {
            ServerExecutablePath = appliedSettings.ServerExecutablePath;
            ConfigPath = appliedSettings.ConfigPath;
            Workdir = appliedSettings.Workdir;
            CloseToTrayEnabled = appliedSettings.CloseToTrayEnabled;
            CloseTipAcknowledged = appliedSettings.CloseTipAcknowledged;
        }

        SetSettingsEditing(false);
        SetOperationSummary(copy.ActionSettingsEditCanceled);
    }

    private void SetSettingsEditing(bool value)
    {
        IsSettingsEditing = value;
        OnPropertyChanged(nameof(AreSettingsReadOnly));
        OnPropertyChanged(nameof(CanSaveSettings));
    }

    private async Task<bool> ExecuteAsync(LauncherUiAction actionKind, string? pendingMessage, string? successMessage, Func<Task> action)
    {
        BeginAction(actionKind, pendingMessage);
        try
        {
            if (!string.IsNullOrWhiteSpace(pendingMessage))
            {
                await Task.Yield();
            }

            await action();
            if (!string.IsNullOrWhiteSpace(successMessage))
            {
                SetOperationSummary(successMessage);
            }

            return true;
        }
        catch (Exception ex)
        {
            if (!marshalToUiThread || Dispatcher.UIThread.CheckAccess())
            {
                LastError = ex.Message;
                HasLastError = !string.IsNullOrWhiteSpace(ex.Message);
                OperationSummary = ex.Message;
            }
            else
            {
                Dispatcher.UIThread.Post(() =>
                {
                    LastError = ex.Message;
                    HasLastError = !string.IsNullOrWhiteSpace(ex.Message);
                    OperationSummary = ex.Message;
                });
            }

            return false;
        }
        finally
        {
            ClearActionState();
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
        appliedSettings = snapshot.Settings;
        if (!IsSettingsEditing)
        {
            ServerExecutablePath = snapshot.Settings.ServerExecutablePath;
            ConfigPath = snapshot.Settings.ConfigPath;
            Workdir = snapshot.Settings.Workdir;
        }

        CloseToTrayEnabled = snapshot.Settings.CloseToTrayEnabled;
        CloseTipAcknowledged = snapshot.Settings.CloseTipAcknowledged;
        StatusSummary = copy.FormatStatusSummary(snapshot.ServiceState);
        currentServiceState = snapshot.ServiceState;
        HeroTitle = copy.FormatHeroTitle(snapshot.ServiceState, snapshot.EnvironmentChecks);
        ServiceDetail = snapshot.ServiceDetail;
        LastError = snapshot.LastError;
        HasLastError = !string.IsNullOrWhiteSpace(snapshot.LastError);
        DiagnosticsSummary = coordinator.BuildDiagnosticsSummary();
        WebEndpoint = snapshot.Endpoint.BaseUri.ToString();
        VersionSummary = string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.Summary)
            ? copy.VersionUnavailableSummary
            : snapshot.ReleaseCheck.Summary;
        VersionDetail = snapshot.ReleaseCheck.Detail;
        ProcessIdSummary = snapshot.ProcessId is int processId ? copy.FormatProcessId(processId) : string.Empty;
        HasProcessId = snapshot.ProcessId is not null;
        HeroAccentBrush = snapshot.ServiceState switch
        {
            LauncherServiceState.Ready => Brush.Parse("#22C55E"),
            LauncherServiceState.ExternalService => Brush.Parse("#38BDF8"),
            LauncherServiceState.Degraded or LauncherServiceState.HealthOnly => Brush.Parse("#F59E0B"),
            LauncherServiceState.Failed => Brush.Parse("#EF4444"),
            LauncherServiceState.Starting or LauncherServiceState.ShuttingDown => Brush.Parse("#38BDF8"),
            _ => Brush.Parse("#94A3B8"),
        };

        var topIssue = snapshot.EnvironmentChecks
            .FirstOrDefault(item => item.Severity == CheckSeverity.Error) ??
            snapshot.EnvironmentChecks.FirstOrDefault(item => item.Severity == CheckSeverity.Warning);
        HasHomeAlert = topIssue is not null;
        HomeAlertTitle = topIssue?.Title ?? copy.NoHomeAlertTitle;
        HomeAlertMessage = topIssue?.Summary ?? string.Empty;
        HomeAlertSeverity = topIssue?.Severity switch
        {
            CheckSeverity.Error => InfoBarSeverity.Error,
            CheckSeverity.Warning => InfoBarSeverity.Warning,
            _ => InfoBarSeverity.Informational,
        };

        HasEnvironmentPackagingNotice =
            snapshot.ReleaseCheck.Status is "unavailable" or "error" ||
            snapshot.ReleaseCheck.Detail.Contains("build_info", StringComparison.OrdinalIgnoreCase);
        EnvironmentPackagingSummary = VersionSummary;
        EnvironmentPackagingDetail = VersionDetail;

        var hasBlockingIssue = snapshot.EnvironmentChecks.Any(item => item.Severity == CheckSeverity.Error);
        CanStart = !snapshot.ProcessRunning &&
                   snapshot.ServiceState is not LauncherServiceState.ExternalService &&
                   !hasBlockingIssue;
        CanStop = snapshot.ProcessRunning ||
                  snapshot.ServiceState is LauncherServiceState.Starting or LauncherServiceState.ShuttingDown or LauncherServiceState.ExternalService;
        CanOpenWebUi = snapshot.ServiceState is LauncherServiceState.HealthOnly or LauncherServiceState.Ready or LauncherServiceState.Degraded or LauncherServiceState.ShuttingDown or LauncherServiceState.ExternalService;
        CanRetry = true;
        CanOpenReleasePage = !string.IsNullOrWhiteSpace(snapshot.ReleaseCheck.ReleasePageUrl);
        PrimaryAction = ResolvePrimaryAction(snapshot.ServiceState, canStart, canOpenWebUi);

        EnvironmentChecks.Clear();
        foreach (var item in snapshot.EnvironmentChecks)
        {
            EnvironmentChecks.Add(new EnvironmentCheckViewModel(item, copy, snapshot.Settings));
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
        OnPropertyChanged(nameof(HasHomeRecentStderr));
        OnPropertyChanged(nameof(HasNoHomeRecentStderr));
        OnPropertyChanged(nameof(HomeRecentStderr));
        OnPropertyChanged(nameof(HasNoProcessId));
        OnPropertyChanged(nameof(IsExternalServiceDetected));
        OnPropertyChanged(nameof(RequiresExternalStopConfirmation));
        OnPropertyChanged(nameof(OpenWebUiActionLabel));
        OnPropertyChanged(nameof(PrimaryActionLabel));
        OnPropertyChanged(nameof(PrimaryActionDisplayLabel));
        OnPropertyChanged(nameof(StopActionDisplayLabel));
        OnPropertyChanged(nameof(RetryActionDisplayLabel));
        OnPropertyChanged(nameof(OpenLogsActionDisplayLabel));
        OnPropertyChanged(nameof(OpenReleasePageActionDisplayLabel));
        OnPropertyChanged(nameof(SaveSettingsActionDisplayLabel));
        OnPropertyChanged(nameof(CanRunPrimaryAction));
    }

    private void ActivateSection(LauncherSection section, bool updateSelection = true)
    {
        if (ActiveSection == section && (!updateSelection || SelectedNavigationItem?.Section == section))
        {
            return;
        }

        ActiveSection = section;
        if (updateSelection)
        {
            selectedNavigationItem = navigationItems.First(item => item.Section == section);
            OnPropertyChanged(nameof(SelectedNavigationItem));
        }

        OnPropertyChanged(nameof(IsStatusSectionActive));
        OnPropertyChanged(nameof(IsEnvironmentSectionActive));
        OnPropertyChanged(nameof(IsDiagnosticsSectionActive));
        OnPropertyChanged(nameof(IsSettingsSectionActive));
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

    private static LauncherPrimaryAction ResolvePrimaryAction(LauncherServiceState serviceState, bool canStart, bool canOpenWebUi)
    {
        if (canOpenWebUi &&
            serviceState is LauncherServiceState.ExternalService or LauncherServiceState.HealthOnly or LauncherServiceState.Ready or LauncherServiceState.Degraded or LauncherServiceState.ShuttingDown)
        {
            return LauncherPrimaryAction.OpenWebUi;
        }

        if (canStart)
        {
            return LauncherPrimaryAction.StartService;
        }

        return LauncherPrimaryAction.None;
    }

    private void BeginAction(LauncherUiAction actionKind, string? pendingMessage)
    {
        pendingAction = actionKind;
        IsActionInProgress = actionKind != LauncherUiAction.None;
        PendingActionMessage = pendingMessage ?? string.Empty;
        if (!string.IsNullOrWhiteSpace(pendingMessage))
        {
            SetOperationSummary(pendingMessage);
        }

        NotifyActionStateChanged();
    }

    private void ClearActionState()
    {
        pendingAction = LauncherUiAction.None;
        IsActionInProgress = false;
        PendingActionMessage = string.Empty;
        NotifyActionStateChanged();
    }

    private void NotifyActionStateChanged()
    {
        OnPropertyChanged(nameof(CanStart));
        OnPropertyChanged(nameof(CanStop));
        OnPropertyChanged(nameof(CanOpenWebUi));
        OnPropertyChanged(nameof(CanRetry));
        OnPropertyChanged(nameof(CanOpenReleasePage));
        OnPropertyChanged(nameof(CanSaveSettings));
        OnPropertyChanged(nameof(CanRunPrimaryAction));
        OnPropertyChanged(nameof(PrimaryActionDisplayLabel));
        OnPropertyChanged(nameof(StopActionDisplayLabel));
        OnPropertyChanged(nameof(RetryActionDisplayLabel));
        OnPropertyChanged(nameof(OpenLogsActionDisplayLabel));
        OnPropertyChanged(nameof(OpenReleasePageActionDisplayLabel));
        OnPropertyChanged(nameof(SaveSettingsActionDisplayLabel));
    }
}

internal enum LauncherUiAction
{
    None,
    Initialize,
    Refresh,
    Retry,
    StartService,
    StopService,
    OpenWebUi,
    OpenLogs,
    SaveSettings,
    OpenReleasePage,
}

internal sealed class LauncherNavigationItemViewModel
{
    internal LauncherNavigationItemViewModel(LauncherSection section, string title, string summary, bool isFooterItem)
    {
        Section = section;
        Title = title;
        Summary = summary;
        IsFooterItem = isFooterItem;
    }

    internal LauncherSection Section { get; }

    internal string Title { get; }

    internal string Summary { get; }

    internal bool IsFooterItem { get; }
}

internal sealed class EnvironmentCheckViewModel
{
    internal EnvironmentCheckViewModel(EnvironmentCheckResult check, LauncherCopy copy, LauncherSettings settings)
    {
        Code = check.Code;
        Severity = check.Severity;
        Title = check.Title;
        Summary = check.Summary;
        Detail = check.Detail;
        Remediation = check.Remediation;
        SeverityLabel = copy.FormatSeverityLabel(check.Severity);
        AccentBrush = check.Severity switch
        {
            CheckSeverity.Ok => Brush.Parse("#22C55E"),
            CheckSeverity.Warning => Brush.Parse("#F59E0B"),
            _ => Brush.Parse("#EF4444"),
        };
        LocationPath = ResolveLocationPath(check.Code, settings);
    }

    internal string Code { get; }

    internal CheckSeverity Severity { get; }

    internal string Title { get; }

    internal string Summary { get; }

    internal string Detail { get; }

    internal string Remediation { get; }

    internal string SeverityLabel { get; }

    internal IBrush AccentBrush { get; }

    internal string? LocationPath { get; }

    internal bool HasLocationPath => !string.IsNullOrWhiteSpace(LocationPath);

    private static string? ResolveLocationPath(string code, LauncherSettings settings)
    {
        return code switch
        {
            "server.executable" or "server.executable_missing" => Path.GetDirectoryName(settings.ServerExecutablePath),
            "config.file" or "config.unreadable" or "config.bootstrap_available" or "config.missing" => Path.GetDirectoryName(settings.ConfigPath),
            "workdir.ready" or "workdir.unwritable" => settings.Workdir,
            "deps.manifest" or "deps.manifest_missing" or "deps.manifest_platform_missing" or "deps.manifest_invalid" => Path.Combine(settings.Workdir, ".deps"),
            "deps.chromium" or "deps.chromium_missing" or "deps.chromium_invalid" or "deps.chromium_unknown" => Path.Combine(settings.Workdir, ".deps"),
            "render.templates" or "render.templates_missing" or "render.templates_empty" => Path.Combine(settings.Workdir, "templates"),
            _ => null,
        };
    }
}
