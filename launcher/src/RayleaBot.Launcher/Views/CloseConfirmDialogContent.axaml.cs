using Avalonia.Controls;

namespace RayleaBot.Launcher.Views;

internal sealed partial class CloseConfirmDialogContent : UserControl
{
    public CloseConfirmDialogContent()
    {
        InitializeComponent();
    }

    private void InitializeComponent()
    {
        Avalonia.Markup.Xaml.AvaloniaXamlLoader.Load(this);
    }
}
