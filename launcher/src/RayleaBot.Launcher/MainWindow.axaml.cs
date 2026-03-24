using System.Diagnostics;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Input;
using Avalonia.Interactivity;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;

namespace RayleaBot.Launcher;

internal sealed partial class MainWindow : Window
{
    private readonly DispatcherTimer refreshTimer;
    private readonly LauncherCopy copy = LauncherCopy.Default;
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
        PropertyChanged += OnWindowPropertyChanged;
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        AvaloniaXamlLoader.Load(this);
    }

    private async void OnOpened(object? sender, EventArgs e)
    {
        ViewModel.SetWindowStateGlyph(WindowState == WindowState.Maximized);
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

    private void OnWindowPropertyChanged(object? sender, AvaloniaPropertyChangedEventArgs e)
    {
        if (e.Property == WindowStateProperty)
        {
            ViewModel.SetWindowStateGlyph(WindowState == WindowState.Maximized);
        }
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
            ViewModel.SetOperationSummary(copy.ActionDiagnosticsCopied);
        }
    }

    private async void CopyServerExecutableClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(copy.ServerExecutableLabel, ViewModel.ServerExecutablePath);
    }

    private async void CopyConfigPathClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(copy.ConfigPathLabel, ViewModel.ConfigPath);
    }

    private async void CopyWorkdirClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(copy.WorkdirLabel, ViewModel.Workdir);
    }

    private void OpenServerExecutableDirectoryClicked(object? sender, RoutedEventArgs e)
    {
        OpenDirectoryForPath(ViewModel.ServerExecutablePath, parentDirectory: true);
    }

    private void OpenConfigDirectoryClicked(object? sender, RoutedEventArgs e)
    {
        OpenDirectoryForPath(ViewModel.ConfigPath, parentDirectory: true);
    }

    private void OpenWorkdirClicked(object? sender, RoutedEventArgs e)
    {
        OpenDirectoryForPath(ViewModel.Workdir, parentDirectory: false);
    }

    private void TitleBarPointerPressed(object? sender, PointerPressedEventArgs e)
    {
        if (e.GetCurrentPoint(this).Properties.IsLeftButtonPressed)
        {
            BeginMoveDrag(e);
        }
    }

    private void TitleBarDoubleTapped(object? sender, TappedEventArgs e)
    {
        ToggleWindowState();
    }

    private void MinimizeClicked(object? sender, RoutedEventArgs e)
    {
        WindowState = WindowState.Minimized;
    }

    private void ToggleMaximizeClicked(object? sender, RoutedEventArgs e)
    {
        ToggleWindowState();
    }

    private void CloseClicked(object? sender, RoutedEventArgs e)
    {
        Close();
    }

    private void EnsureTrayIcon()
    {
        if (trayIcon is not null)
        {
            return;
        }

        var openItem = new NativeMenuItem(copy.TrayOpenLauncherLabel);
        openItem.Click += (_, _) => RestoreFromTray();

        var openWebItem = new NativeMenuItem(copy.TrayOpenWebLabel);
        openWebItem.Click += async (_, _) => await ViewModel.OpenWebUiAsync();

        var exitItem = new NativeMenuItem(copy.TrayExitLabel);
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
            ToolTipText = copy.TrayTooltip,
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
        ViewModel.SetOperationSummary(copy.ActionRestoredFromTray);
    }

    private void HideToTray()
    {
        Hide();
        ViewModel.SetOperationSummary(copy.ActionHiddenToTray);
    }

    private void ToggleWindowState()
    {
        WindowState = WindowState == WindowState.Maximized
            ? WindowState.Normal
            : WindowState.Maximized;
    }

    private async Task CopyValueAsync(string label, string value)
    {
        if (string.IsNullOrWhiteSpace(value) || TopLevel.GetTopLevel(this)?.Clipboard is not { } clipboard)
        {
            return;
        }

        await clipboard.SetTextAsync(value);
        ViewModel.SetOperationSummary(copy.FormatPathCopied(label));
    }

    private void OpenDirectoryForPath(string value, bool parentDirectory)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return;
        }

        var directory = parentDirectory ? Path.GetDirectoryName(value) : value;
        if (string.IsNullOrWhiteSpace(directory))
        {
            return;
        }

        Directory.CreateDirectory(directory);
        Process.Start(new ProcessStartInfo
        {
            FileName = directory,
            UseShellExecute = true,
        });
        ViewModel.SetOperationSummary($"{copy.OpenDirectoryLabel}：{directory}");
    }
}

internal sealed class CloseToTrayDialog : Window
{
    private readonly TaskCompletionSource<bool> resultSource = new();

    private CloseToTrayDialog()
    {
        var copy = LauncherCopy.Default;
        Width = 500;
        Height = 250;
        CanResize = false;
        WindowStartupLocation = WindowStartupLocation.CenterOwner;
        Title = copy.CloseDialogTitle;
        Background = Avalonia.Media.Brush.Parse("#0D1730");
        Foreground = Avalonia.Media.Brush.Parse("#F4F8FF");
        SystemDecorations = SystemDecorations.BorderOnly;
        Content = BuildContent(copy);
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

    private Control BuildContent(LauncherCopy copy)
    {
        var hideButton = new Button
        {
            Content = copy.HideToTrayLabel,
            MinWidth = 132,
        };
        hideButton.Click += (_, _) =>
        {
            resultSource.TrySetResult(true);
            Close();
        };

        var exitButton = new Button
        {
            Content = copy.ExitCompletelyLabel,
            MinWidth = 132,
        };
        exitButton.Click += (_, _) =>
        {
            resultSource.TrySetResult(false);
            Close();
        };

        return new Border
        {
            CornerRadius = new CornerRadius(24),
            BorderBrush = Avalonia.Media.Brush.Parse("#3A628B"),
            BorderThickness = new Thickness(1),
            Background = Avalonia.Media.Brush.Parse("#C4152842"),
            Margin = new Thickness(18),
            Padding = new Thickness(24),
            Child = new StackPanel
            {
                Spacing = 16,
                Children =
                {
                    new TextBlock
                    {
                        Text = copy.CloseDialogTitle,
                        FontSize = 20,
                        FontWeight = Avalonia.Media.FontWeight.SemiBold,
                    },
                    new TextBlock
                    {
                        Text = copy.CloseDialogBody,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                    },
                    new TextBlock
                    {
                        Text = copy.CloseDialogFootnote,
                        TextWrapping = Avalonia.Media.TextWrapping.Wrap,
                        Foreground = Avalonia.Media.Brush.Parse("#B8CAE4"),
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
            },
        };
    }
}
