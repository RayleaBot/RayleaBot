using Avalonia.Controls;

namespace RayleaBot.Launcher;

internal sealed record LauncherTrayMenuEntry(
    string Header,
    LauncherTrayAction? Action,
    bool IsEnabled,
    bool IsSeparator = false);

internal sealed class LauncherTrayMenu : IDisposable
{
    private readonly MainWindowViewModel viewModel;
    private readonly Action<LauncherTrayAction> actionRequested;
    private readonly LauncherCopy copy;

    internal LauncherTrayMenu(MainWindowViewModel viewModel, Action<LauncherTrayAction> actionRequested, LauncherCopy? copy = null)
    {
        this.viewModel = viewModel;
        this.actionRequested = actionRequested;
        this.copy = copy ?? LauncherCopy.Default;
        Menu = new NativeMenu();
        Menu.Opening += OnNativeMenuOpening;
        Menu.Closed += OnNativeMenuClosed;
        RefreshMenu();
    }

    internal NativeMenu Menu { get; }

    internal bool IsMenuOpen { get; private set; }

    internal IReadOnlyList<LauncherTrayMenuEntry> BuildEntries()
    {
        var serviceAction = viewModel.TrayServiceAction;
        var serviceActionLabel = viewModel.TrayServiceActionLabel;
        var canRunServiceAction = viewModel.CanRunTrayServiceAction;

        return
        [
            new LauncherTrayMenuEntry(copy.TrayTitleLabel, null, false),
            new LauncherTrayMenuEntry(copy.FormatTrayStatusLabel(viewModel.TrayStatusSummary), null, false),
            new LauncherTrayMenuEntry(string.Empty, null, false, IsSeparator: true),
            new LauncherTrayMenuEntry(copy.RestoreLauncherLabel, LauncherTrayAction.Restore, true),
            new LauncherTrayMenuEntry(copy.OpenWebUiLabel, LauncherTrayAction.OpenWeb, viewModel.CanOpenWebUi),
            new LauncherTrayMenuEntry(serviceActionLabel, serviceAction, canRunServiceAction),
            new LauncherTrayMenuEntry(string.Empty, null, false, IsSeparator: true),
            new LauncherTrayMenuEntry(copy.OpenLogsDirectoryLabel, LauncherTrayAction.OpenLogs, true),
            new LauncherTrayMenuEntry(string.Empty, null, false, IsSeparator: true),
            new LauncherTrayMenuEntry(copy.ExitAppLabel, LauncherTrayAction.Exit, true),
        ];
    }

    internal void RefreshMenu()
    {
        Menu.Items.Clear();
        foreach (var entry in BuildEntries())
        {
            Menu.Items.Add(CreateMenuItem(entry));
        }
    }

    internal bool ShouldHandleTrayClick() => !IsMenuOpen;

    internal void OnMenuOpening()
    {
        IsMenuOpen = true;
        RefreshMenu();
    }

    internal void OnMenuClosed()
    {
        IsMenuOpen = false;
    }

    public void Dispose()
    {
        Menu.Opening -= OnNativeMenuOpening;
        Menu.Closed -= OnNativeMenuClosed;
    }

    private NativeMenuItemBase CreateMenuItem(LauncherTrayMenuEntry entry)
    {
        if (entry.IsSeparator)
        {
            return new NativeMenuItemSeparator();
        }

        var item = new NativeMenuItem(entry.Header)
        {
            IsEnabled = entry.IsEnabled,
        };

        if (entry.Action is { } action)
        {
            item.Click += (_, _) => actionRequested(action);
        }

        return item;
    }

    private void OnNativeMenuOpening(object? sender, EventArgs e) => OnMenuOpening();

    private void OnNativeMenuClosed(object? sender, EventArgs e) => OnMenuClosed();
}
