using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;
using FluentAvalonia.UI.Controls;

namespace RayleaBot.Launcher;

internal sealed partial class MainWindow : Window
{
    private readonly DispatcherTimer refreshTimer;
    private readonly LauncherCopy copy = LauncherCopy.Default;
    private TrayIcon? trayIcon;
    private LauncherTrayPanelWindow? trayPanel;
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
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        AvaloniaXamlLoader.Load(this);
    }

    private async void OnOpened(object? sender, EventArgs e)
    {
        ViewModel.SetWindowState(WindowState == WindowState.Maximized);
        EnsureTrayIcon();
        await ViewModel.InitializeAsync();
        refreshTimer.Start();
    }

    private async void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (!LauncherWindowPolicies.ShouldPromptBeforeClose(explicitExitRequested))
        {
            return;
        }

        e.Cancel = true;
        var hideToTray = await ShowCloseDialogAsync();
        if (!hideToTray)
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
        trayPanel?.Close();
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
        trayIcon.Clicked += TrayIconClicked;
    }

    private void TrayIconClicked(object? sender, EventArgs e)
    {
        if (trayPanel is not null)
        {
            trayPanel.Close();
            trayPanel = null;
            return;
        }

        trayPanel = new LauncherTrayPanelWindow(ViewModel);
        trayPanel.ActionRequested += TrayPanelActionRequested;
        trayPanel.Closed += (_, _) => trayPanel = null;
        trayPanel.ShowNear(this);
    }

    private async void TrayPanelActionRequested(object? sender, LauncherTrayAction action)
    {
        switch (action)
        {
            case LauncherTrayAction.Restore:
                RestoreFromTray();
                break;
            case LauncherTrayAction.OpenWeb:
                await ViewModel.OpenWebUiAsync();
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
        trayPanel?.Close();
        Show();
        WindowState = WindowState.Normal;
        Activate();
        ViewModel.SetOperationSummary(copy.ActionRestoredFromTray);
    }

    private void HideToTray()
    {
        trayPanel?.Close();
        Hide();
        ViewModel.SetOperationSummary(copy.ActionHiddenToTray);
    }

    private async Task<bool> ShowCloseDialogAsync()
    {
        var dialog = new ContentDialog
        {
            Title = copy.CloseDialogTitle,
            Content = new StackPanel
            {
                Spacing = 12,
                Children =
                {
                    new TextBlock
                    {
                        Text = copy.CloseDialogBody,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                    },
                    new TextBlock
                    {
                        Text = copy.CloseDialogFootnote,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                        Foreground = Avalonia.Media.Brush.Parse("#94A3B8"),
                    },
                },
            },
            PrimaryButtonText = copy.HideToTrayLabel,
            CloseButtonText = copy.ExitCompletelyLabel,
            DefaultButton = ContentDialogButton.Primary,
        };

        var result = await dialog.ShowAsync(this);
        return result == ContentDialogResult.Primary;
    }

    private async Task<bool> ShowExternalStopDialogAsync()
    {
        var dialog = new ContentDialog
        {
            Title = ViewModel.ExternalStopConfirmTitle,
            Content = new StackPanel
            {
                Spacing = 12,
                Children =
                {
                    new TextBlock
                    {
                        Text = ViewModel.ExternalStopConfirmBody,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                    },
                    new TextBlock
                    {
                        Text = ViewModel.ExternalStopConfirmFootnote,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                        Foreground = Avalonia.Media.Brush.Parse("#94A3B8"),
                    },
                },
            },
            PrimaryButtonText = ViewModel.ExternalStopConfirmAction,
            CloseButtonText = ViewModel.ExternalStopCancelAction,
            DefaultButton = ContentDialogButton.Close,
        };

        var result = await dialog.ShowAsync(this);
        return result == ContentDialogResult.Primary;
    }
}
