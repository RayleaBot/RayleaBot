using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;

namespace RayleaBot.Launcher.Views;

internal sealed partial class DiagnosticsPage : UserControl
{
    public DiagnosticsPage()
    {
        InitializeComponent();
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        Avalonia.Markup.Xaml.AvaloniaXamlLoader.Load(this);
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
            ViewModel.SetOperationSummary(ViewModel.Copy.ActionDiagnosticsCopied);
        }
    }
}
