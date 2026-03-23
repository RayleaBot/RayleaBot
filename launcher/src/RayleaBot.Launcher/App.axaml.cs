using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;

namespace RayleaBot.Launcher;

internal sealed partial class App : Application
{
    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            var coordinator = LauncherCompositionRoot.CreateCoordinator();
            desktop.MainWindow = new MainWindow(new MainWindowViewModel(coordinator));
        }

        base.OnFrameworkInitializationCompleted();
    }
}
