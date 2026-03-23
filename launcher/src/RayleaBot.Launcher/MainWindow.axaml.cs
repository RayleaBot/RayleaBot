using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;

namespace RayleaBot.Launcher;

internal sealed partial class MainWindow : Window
{
    private readonly DispatcherTimer refreshTimer;
    private TrayIcon? trayIcon;
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
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        AvaloniaXamlLoader.Load(this);
    }

    private async void OnOpened(object? sender, EventArgs e)
    {
        EnsureTrayIcon();
        await ViewModel.InitializeAsync();
        refreshTimer.Start();
    }

    private async void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (explicitExitRequested || !ViewModel.CloseToTrayEnabled)
        {
            return;
        }

        e.Cancel = true;
        if (!ViewModel.CloseTipAcknowledged)
        {
            var hideToTray = await CloseToTrayDialog.ShowAsync(this);
            if (!hideToTray)
            {
                explicitExitRequested = true;
                Close();
                return;
            }

            await ViewModel.AcknowledgeCloseTipAsync();
        }

        HideToTray();
    }

    private void OnClosed(object? sender, EventArgs e)
    {
        refreshTimer.Stop();
        trayIcon?.Dispose();
    }

    private async void RefreshTimerTick(object? sender, EventArgs e)
    {
        await ViewModel.RefreshAsync();
    }

    private async void SaveSettingsClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.SaveSettingsAsync();
    }

    private async void StartClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.StartAsync();
    }

    private async void StopClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.StopAsync();
    }

    private async void OpenWebClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.OpenWebUiAsync();
    }

    private async void RetryClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.RetryAsync();
    }

    private async void OpenLogsClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.OpenLogsDirectoryAsync();
    }

    private async void OpenReleasePageClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.OpenReleasePageAsync();
    }

    private async void CopyDiagnosticsClicked(object? sender, RoutedEventArgs e)
    {
        if (TopLevel.GetTopLevel(this)?.Clipboard is { } clipboard)
        {
            await clipboard.SetTextAsync(ViewModel.DiagnosticsSummary);
            ViewModel.SetOperationSummary("Diagnostics copied to clipboard.");
        }
    }

    private void EnsureTrayIcon()
    {
        if (trayIcon is not null)
        {
            return;
        }

        var openItem = new NativeMenuItem("Open launcher");
        openItem.Click += (_, _) => RestoreFromTray();

        var openWebItem = new NativeMenuItem("Open Web UI");
        openWebItem.Click += async (_, _) => await ViewModel.OpenWebUiAsync();

        var exitItem = new NativeMenuItem("Exit");
        exitItem.Click += (_, _) =>
        {
            explicitExitRequested = true;
            Show();
            Activate();
            Close();
        };

        var menu = new NativeMenu
        {
            Items =
            {
                openItem,
                openWebItem,
                new NativeMenuItemSeparator(),
                exitItem,
            },
        };

        trayIcon = new TrayIcon
        {
            ToolTipText = "RayleaBot Launcher",
            Icon = LauncherIcons.CreateTrayIcon(),
            Menu = menu,
            IsVisible = true,
        };
        trayIcon.Clicked += (_, _) => RestoreFromTray();
    }

    private void RestoreFromTray()
    {
        Show();
        WindowState = WindowState.Normal;
        Activate();
        ViewModel.SetOperationSummary("Launcher restored from the system tray.");
    }

    private void HideToTray()
    {
        Hide();
        ViewModel.SetOperationSummary("Launcher is still running in the system tray.");
    }
}

internal sealed class CloseToTrayDialog : Window
{
    private readonly TaskCompletionSource<bool> resultSource = new();

    private CloseToTrayDialog()
    {
        Width = 460;
        Height = 220;
        CanResize = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;
        Title = "Keep RayleaBot Launcher running?";
        Background = Avalonia.Media.Brush.Parse("#0F172A");
        Foreground = Avalonia.Media.Brush.Parse("#E5EEF8");
        Content = BuildContent();
        Closed += (_, _) =>
        {
            if (!resultSource.Task.IsCompleted)
            {
                resultSource.TrySetResult(true);
            }
        };
    }

    internal static async Task<bool> ShowAsync(Window owner)
    {
        var dialog = new CloseToTrayDialog();
        await dialog.ShowDialog(owner);
        return await dialog.resultSource.Task.ConfigureAwait(true);
    }

    private Control BuildContent()
    {
        var hideButton = new Button
        {
            Content = "Hide to tray",
            MinWidth = 120,
        };
        hideButton.Click += (_, _) =>
        {
            resultSource.TrySetResult(true);
            Close();
        };

        var exitButton = new Button
        {
            Content = "Exit completely",
            MinWidth = 120,
        };
        exitButton.Click += (_, _) =>
        {
            resultSource.TrySetResult(false);
            Close();
        };

        return new StackPanel
        {
            Margin = new Thickness(24),
            Spacing = 16,
            Children =
            {
                new TextBlock
                {
                    Text = "Closing the window keeps RayleaBot Launcher available in the system tray so you can reopen it without restarting the service shell.",
                    TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                    FontSize = 16,
                },
                new TextBlock
                {
                    Text = "Use Exit completely to close the launcher process. The tray menu always includes a full exit action later.",
                    TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                    Foreground = Avalonia.Media.Brush.Parse("#9FB4CF"),
                },
                new StackPanel
                {
                    Orientation = Avalonia.Layout.Orientation.Horizontal,
                    Spacing = 12,
                    HorizontalAlignment = Avalonia.Layout.HorizontalAlignment.Right,
                    Children =
                    {
                        hideButton,
                        exitButton,
                    },
                },
            },
        };
    }
}
