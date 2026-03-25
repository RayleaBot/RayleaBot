using System.Diagnostics;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;

namespace RayleaBot.Launcher.Views;

internal sealed partial class EnvironmentPage : UserControl
{
    public EnvironmentPage()
    {
        InitializeComponent();
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        Avalonia.Markup.Xaml.AvaloniaXamlLoader.Load(this);
    }

    private async void CopyEvidenceClicked(object? sender, RoutedEventArgs e)
    {
        if (sender is Button { Tag: string evidence } &&
            !string.IsNullOrWhiteSpace(evidence) &&
            TopLevel.GetTopLevel(this)?.Clipboard is { } clipboard)
        {
            await clipboard.SetTextAsync(evidence);
            ViewModel.SetOperationSummary(ViewModel.Copy.FormatPathCopied(ViewModel.Copy.CopyEvidenceLabel));
        }
    }

    private void OpenLocationClicked(object? sender, RoutedEventArgs e)
    {
        if (sender is not Button { Tag: string path } || string.IsNullOrWhiteSpace(path))
        {
            return;
        }

        Directory.CreateDirectory(path);
        Process.Start(new ProcessStartInfo
        {
            FileName = path,
            UseShellExecute = true,
        });
        ViewModel.SetOperationSummary($"{ViewModel.Copy.OpenLocationLabel}：{path}");
    }

    private async void RetryClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.RetryAsync();
    }
}
