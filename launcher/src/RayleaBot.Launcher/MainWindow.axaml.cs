using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Markup.Xaml;
using Avalonia.Media;
using Avalonia.Threading;
using FluentAvalonia.UI.Controls;
using FluentAvalonia.UI.Windowing;
using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Views;

namespace RayleaBot.Launcher;

internal sealed partial class MainWindow : AppWindow
{
    private readonly DispatcherTimer refreshTimer;
    private readonly LauncherCopy copy = LauncherCopy.Default;
    private TrayIcon? trayIcon;
    private LauncherTrayMenu? trayMenu;
    private bool explicitExitRequested;

    internal MainWindow(MainWindowViewModel viewModel)
    {
        InitializeComponent();
        DataContext = viewModel;
        refreshTimer = new DispatcherTimer
        {
            Interval = TimeSpan.FromSeconds(2),
        };
        refreshTimer.Tick += RefreshTimerTick;
        Opened += OnOpened;
        Closing += OnClosing;
        Closed += OnClosed;
        PropertyChanged += OnWindowPropertyChanged;
        ViewModel.PropertyChanged += OnViewModelPropertyChanged;
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        AvaloniaXamlLoader.Load(this);
    }

    private async void OnOpened(object? sender, EventArgs e)
    {
        // Enable Mica backdrop on Windows 11
        EnableMicaBackdrop();
        ViewModel.SetWindowState(WindowState == WindowState.Maximized);
        EnsureTrayIcon();
        await ViewModel.InitializeAsync();
        SyncTrayPresentation();
        refreshTimer.Start();
    }

    private void EnableMicaBackdrop()
    {
        // Request Mica backdrop from the OS, with Blur as fallback on Windows 10
        TransparencyLevelHint = new[] { WindowTransparencyLevel.Mica, WindowTransparencyLevel.Blur };
        // True transparent fallback for when transparency is not supported at all
        TransparencyBackgroundFallback = Brushes.Transparent;
    }

    private async void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        var closeAction = LauncherWindowPolicies.ResolveCloseAction(explicitExitRequested, ViewModel.CloseBehavior);
        switch (closeAction)
        {
            case LauncherWindowCloseAction.ExitApplication:
                return;
            case LauncherWindowCloseAction.HideToTray:
                e.Cancel = true;
                HideToTray();
                return;
        }

        e.Cancel = true;
        var result = await ShowCloseDialogAsync();
        if (result.Action == LauncherCloseDialogAction.Cancel)
        {
            return;
        }

        if (result.RememberChoice)
        {
            var rememberedBehavior = result.Action == LauncherCloseDialogAction.HideToTray
                ? LauncherCloseBehavior.HideToTray
                : LauncherCloseBehavior.ExitApplication;
            await ViewModel.PersistCloseBehaviorAsync(rememberedBehavior);
        }

        if (result.Action == LauncherCloseDialogAction.ExitApplication)
        {
            explicitExitRequested = true;
            Close();
            return;
        }

        HideToTray();
    }

    private void OnClosed(object? sender, EventArgs e)
    {
        refreshTimer.Stop();
        ViewModel.PropertyChanged -= OnViewModelPropertyChanged;
        trayMenu?.Dispose();
        trayIcon?.Dispose();
    }

    private void OnWindowPropertyChanged(object? sender, AvaloniaPropertyChangedEventArgs e)
    {
        if (e.Property == WindowStateProperty)
        {
            ViewModel.SetWindowState(WindowState == WindowState.Maximized);
        }
    }

    private async void RefreshTimerTick(object? sender, EventArgs e)
    {
        await ViewModel.RefreshAsync();
    }

    private void EnsureTrayIcon()
    {
        if (trayIcon is not null)
        {
            return;
        }

        trayIcon = new TrayIcon
        {
            ToolTipText = copy.TrayTooltip,
            Icon = LauncherIcons.CreateTrayIcon(),
            IsVisible = true,
        };
        trayMenu = new LauncherTrayMenu(ViewModel, OnTrayMenuActionRequested);
        trayIcon.Menu = trayMenu.Menu;
        trayIcon.Clicked += TrayIconClicked;
    }

    private void TrayIconClicked(object? sender, EventArgs e)
    {
        if (trayMenu is not null && !trayMenu.ShouldHandleTrayClick())
        {
            return;
        }

        RestoreFromTray();
    }

    private async void OnTrayMenuActionRequested(LauncherTrayAction action)
    {
        switch (action)
        {
            case LauncherTrayAction.Restore:
                RestoreFromTray();
                break;
            case LauncherTrayAction.OpenWeb:
                await ViewModel.OpenWebUiAsync();
                break;
            case LauncherTrayAction.OpenLogs:
                await ViewModel.OpenLogsDirectoryAsync();
                break;
            case LauncherTrayAction.Start:
                await ViewModel.StartAsync();
                break;
            case LauncherTrayAction.Stop:
                if (ViewModel.RequiresExternalStopConfirmation)
                {
                    var shouldContinue = await ShowExternalStopDialogAsync();
                    if (!shouldContinue)
                    {
                        return;
                    }
                }

                await ViewModel.StopAsync();
                break;
            case LauncherTrayAction.Exit:
                explicitExitRequested = true;
                Show();
                Activate();
                Close();
                break;
        }
    }

    private void RestoreFromTray()
    {
        if (!IsVisible)
        {
            Show();
        }

        if (WindowState == WindowState.Minimized)
        {
            WindowState = WindowState.Normal;
        }

        Activate();
        ViewModel.SetOperationSummary(copy.ActionRestoredFromTray);
    }

    private void HideToTray()
    {
        Hide();
        ViewModel.SetOperationSummary(copy.ActionHiddenToTray);
    }

    private async Task<LauncherCloseDialogResult> ShowCloseDialogAsync()
    {
        var dialogViewModel = new CloseConfirmDialogViewModel(ViewModel.CloseBehavior, copy);
        var dialog = new ContentDialog
        {
            Title = copy.CloseDialogTitle,
            Content = new CloseConfirmDialogContent
            {
                DataContext = dialogViewModel,
            },
            PrimaryButtonText = copy.HideToTrayLabel,
            SecondaryButtonText = copy.ExitCompletelyLabel,
            CloseButtonText = copy.CancelDialogLabel,
            DefaultButton = ContentDialogButton.Primary,
        };

        var result = await dialog.ShowAsync(this);
        return result switch
        {
            ContentDialogResult.Primary => new LauncherCloseDialogResult(LauncherCloseDialogAction.HideToTray, dialogViewModel.RememberChoice),
            ContentDialogResult.Secondary => new LauncherCloseDialogResult(LauncherCloseDialogAction.ExitApplication, dialogViewModel.RememberChoice),
            _ => new LauncherCloseDialogResult(LauncherCloseDialogAction.Cancel, false),
        };
    }

    private async Task<bool> ShowExternalStopDialogAsync()
    {
        var dialogSurface = new Border();
        dialogSurface.Classes.Add("dialog-surface");
        dialogSurface.Child = new StackPanel
        {
            Spacing = 10,
            Children =
            {
                new TextBlock
                {
                    Text = ViewModel.ExternalStopConfirmBody,
                    TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                },
                new Border
                {
                    Classes = { "summary-strip" },
                    Child = new TextBlock
                    {
                        Text = ViewModel.ExternalStopConfirmFootnote,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                        Foreground = Avalonia.Media.Brush.Parse("#A9B4C4"),
                    },
                },
            },
        };

        var dialog = new ContentDialog
        {
            Title = ViewModel.ExternalStopConfirmTitle,
            Content = dialogSurface,
            PrimaryButtonText = ViewModel.ExternalStopConfirmAction,
            CloseButtonText = ViewModel.ExternalStopCancelAction,
            DefaultButton = ContentDialogButton.Close,
        };

        var result = await dialog.ShowAsync(this);
        return result == ContentDialogResult.Primary;
    }

    private void OnViewModelPropertyChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e)
    {
        switch (e.PropertyName)
        {
            case nameof(MainWindowViewModel.TrayTooltipText):
            case nameof(MainWindowViewModel.TrayStatusSummary):
            case nameof(MainWindowViewModel.TrayServiceAction):
            case nameof(MainWindowViewModel.TrayServiceActionLabel):
            case nameof(MainWindowViewModel.CanRunTrayServiceAction):
            case nameof(MainWindowViewModel.CanOpenWebUi):
                SyncTrayPresentation();
                break;
        }
    }

    private void SyncTrayPresentation()
    {
        if (trayIcon is null)
        {
            return;
        }

        trayIcon.ToolTipText = ViewModel.TrayTooltipText;
        trayMenu?.RefreshMenu();
    }
}
