using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Markup.Xaml;
using Avalonia.Threading;

namespace RayleaBot.Launcher;

internal sealed partial class MainWindow : Window
{
    private readonly DispatcherTimer refreshTimer;

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
        Closed += OnClosed;
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        AvaloniaXamlLoader.Load(this);
    }

    private async void OnOpened(object? sender, EventArgs e)
    {
        await ViewModel.InitializeAsync();
        refreshTimer.Start();
    }

    private void OnClosed(object? sender, EventArgs e)
    {
        refreshTimer.Stop();
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

    private async void CopyDiagnosticsClicked(object? sender, RoutedEventArgs e)
    {
        if (TopLevel.GetTopLevel(this)?.Clipboard is { } clipboard)
        {
            await clipboard.SetTextAsync(ViewModel.DiagnosticsSummary);
            ViewModel.SetOperationSummary("Diagnostics copied to clipboard.");
        }
    }
}
