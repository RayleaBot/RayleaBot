using System.Diagnostics;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;

namespace RayleaBot.Launcher.Views;

internal sealed partial class SettingsPage : UserControl
{
    public SettingsPage()
    {
        InitializeComponent();
    }

    private MainWindowViewModel ViewModel => (MainWindowViewModel)DataContext!;

    private void InitializeComponent()
    {
        Avalonia.Markup.Xaml.AvaloniaXamlLoader.Load(this);
    }

    private async void SaveSettingsClicked(object? sender, RoutedEventArgs e)
    {
        await ViewModel.SaveSettingsAsync();
    }

    private void EditSettingsClicked(object? sender, RoutedEventArgs e)
    {
        ViewModel.BeginSettingsEditing();
    }

    private void CancelEditingClicked(object? sender, RoutedEventArgs e)
    {
        ViewModel.CancelSettingsEditing();
    }

    private async void BrowseExecutableClicked(object? sender, RoutedEventArgs e)
    {
        var path = await PickFileAsync("选择服务端可执行文件").ConfigureAwait(true);
        if (!string.IsNullOrWhiteSpace(path))
        {
            ViewModel.ServerExecutablePath = path;
        }
    }

    private async void BrowseConfigClicked(object? sender, RoutedEventArgs e)
    {
        var path = await PickFileAsync("选择用户配置文件").ConfigureAwait(true);
        if (!string.IsNullOrWhiteSpace(path))
        {
            ViewModel.ConfigPath = path;
        }
    }

    private async void BrowseWorkdirClicked(object? sender, RoutedEventArgs e)
    {
        var path = await PickFolderAsync("选择工作目录").ConfigureAwait(true);
        if (!string.IsNullOrWhiteSpace(path))
        {
            ViewModel.Workdir = path;
        }
    }

    private async void CopyServerExecutableClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(ViewModel.Copy.ServerExecutableLabel, ViewModel.ServerExecutablePath);
    }

    private async void CopyConfigPathClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(ViewModel.Copy.ConfigPathLabel, ViewModel.ConfigPath);
    }

    private async void CopyWorkdirClicked(object? sender, RoutedEventArgs e)
    {
        await CopyValueAsync(ViewModel.Copy.WorkdirLabel, ViewModel.Workdir);
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

    private async Task<string?> PickFileAsync(string title)
    {
        if (TopLevel.GetTopLevel(this)?.StorageProvider is not { CanOpen: true } storageProvider)
        {
            return null;
        }

        var result = await storageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
        {
            Title = title,
            AllowMultiple = false,
        });

        return result.Count > 0 ? result[0].Path?.LocalPath : null;
    }

    private async Task<string?> PickFolderAsync(string title)
    {
        if (TopLevel.GetTopLevel(this)?.StorageProvider is not { CanPickFolder: true } storageProvider)
        {
            return null;
        }

        var result = await storageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
        {
            Title = title,
            AllowMultiple = false,
        });

        return result.Count > 0 ? result[0].Path?.LocalPath : null;
    }

    private async Task CopyValueAsync(string label, string value)
    {
        if (string.IsNullOrWhiteSpace(value) || TopLevel.GetTopLevel(this)?.Clipboard is not { } clipboard)
        {
            return;
        }

        await clipboard.SetTextAsync(value);
        ViewModel.SetOperationSummary(ViewModel.Copy.FormatPathCopied(label));
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
        ViewModel.SetOperationSummary($"{ViewModel.Copy.OpenDirectoryLabel}：{directory}");
    }
}
