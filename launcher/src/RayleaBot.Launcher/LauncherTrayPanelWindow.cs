using Avalonia;
using Avalonia.Controls;
using Avalonia.Input;
using Avalonia.Interactivity;
using Avalonia.Layout;
using Avalonia.Media;

namespace RayleaBot.Launcher;

internal sealed class LauncherTrayPanelWindow : Window
{
    private readonly MainWindowViewModel viewModel;
    private readonly LauncherCopy copy = LauncherCopy.Default;

    internal LauncherTrayPanelWindow(MainWindowViewModel viewModel)
    {
        this.viewModel = viewModel;
        ShowInTaskbar = false;
        CanResize = false;
        Topmost = true;
        SystemDecorations = SystemDecorations.None;
        Background = Brushes.Transparent;
        WindowStartupLocation = WindowStartupLocation.Manual;
        SizeToContent = SizeToContent.WidthAndHeight;
        Content = BuildContent();
        Deactivated += (_, _) => Close();
        KeyDown += OnKeyDown;
    }

    internal event EventHandler<LauncherTrayAction>? ActionRequested;

    internal void ShowNear(Window owner)
    {
        var screens = owner.Screens;
        var screen = screens.ScreenFromWindow(owner) ?? screens.Primary;
        var workingArea = screen?.WorkingArea ?? new PixelRect(0, 0, 1600, 900);
        const int width = 304;
        const int height = 324;
        Position = new PixelPoint(
            workingArea.X + Math.Max(16, workingArea.Width - width - 22),
            workingArea.Y + Math.Max(16, workingArea.Height - height - 22));

        Width = width;
        Height = height;
        Show();
        Activate();
    }

    private void OnKeyDown(object? sender, KeyEventArgs e)
    {
        if (e.Key == Key.Escape)
        {
            Close();
        }
    }

    private Control BuildContent()
    {
        return new Border
        {
            CornerRadius = new CornerRadius(28),
            BorderThickness = new Thickness(1),
            BorderBrush = Brush.Parse("#7EDBFF"),
            Background = new LinearGradientBrush
            {
                StartPoint = new RelativePoint(0, 0, RelativeUnit.Relative),
                EndPoint = new RelativePoint(1, 1, RelativeUnit.Relative),
                GradientStops =
                [
                    new GradientStop(Color.Parse("#E31D3557"), 0),
                    new GradientStop(Color.Parse("#D1122038"), 0.55),
                    new GradientStop(Color.Parse("#CC0C172B"), 1),
                ],
            },
            BoxShadow = BoxShadows.Parse("0 26 48 0 #66000000"),
            Padding = new Thickness(18),
            Child = new Grid
            {
                RowDefinitions = new RowDefinitions("Auto,Auto,Auto,*,Auto"),
                RowSpacing = 14,
                Children =
                {
                    BuildHeader(),
                    BuildSummary().WithRow(1),
                    BuildActions().WithRow(2),
                    BuildHint().WithRow(3),
                    BuildExitButton().WithRow(4),
                },
            },
        };
    }

    private Control BuildHeader()
    {
        return new Grid
        {
            ColumnDefinitions = new ColumnDefinitions("*,Auto"),
            ColumnSpacing = 12,
            Children =
            {
                new StackPanel
                {
                    Spacing = 4,
                    Children =
                    {
                        new TextBlock
                        {
                            Text = copy.TrayQuickPanelTitle,
                            FontSize = 20,
                            FontWeight = FontWeight.SemiBold,
                            Foreground = Brush.Parse("#F7FBFF"),
                        },
                        new TextBlock
                        {
                            Text = copy.TrayQuickPanelSummary,
                            TextWrapping = TextWrapping.Wrap,
                            Foreground = Brush.Parse("#C9DCF4"),
                            FontSize = 12,
                        },
                    },
                },
                new Border
                {
                    CornerRadius = new CornerRadius(999),
                    Padding = new Thickness(10, 6),
                    BorderThickness = new Thickness(1),
                    BorderBrush = Brush.Parse("#4C6F9A"),
                    Background = Brush.Parse("#99162946"),
                    Child = new TextBlock
                    {
                        Text = viewModel.StatusSummary,
                        Foreground = viewModel.HeroAccentBrush,
                        FontWeight = FontWeight.SemiBold,
                    },
                }.WithPosition(0, 1),
            },
        };
    }

    private Control BuildSummary()
    {
        var text = string.IsNullOrWhiteSpace(viewModel.OperationSummary)
            ? viewModel.HeroTitle
            : viewModel.OperationSummary;

        return new Border
        {
            Classes = { },
            CornerRadius = new CornerRadius(20),
            BorderThickness = new Thickness(1),
            BorderBrush = Brush.Parse("#365B86"),
            Background = Brush.Parse("#8A13243E"),
            Padding = new Thickness(14),
            Child = new StackPanel
            {
                Spacing = 6,
                Children =
                {
                    new TextBlock
                    {
                        Text = text,
                        TextWrapping = TextWrapping.Wrap,
                        Foreground = Brush.Parse("#F4F8FF"),
                        FontWeight = FontWeight.SemiBold,
                    },
                    new TextBlock
                    {
                        Text = viewModel.ServiceDetail,
                        TextWrapping = TextWrapping.Wrap,
                        Foreground = Brush.Parse("#C9DCF4"),
                        FontSize = 12,
                    },
                },
            },
        };
    }

    private Control BuildActions()
    {
        var grid = new Grid
        {
            ColumnDefinitions = new ColumnDefinitions("*,*"),
            ColumnSpacing = 10,
            RowDefinitions = new RowDefinitions("Auto,Auto,Auto"),
            RowSpacing = 10,
        };

        grid.Children.Add(BuildActionButton(copy.RestoreLauncherLabel, LauncherTrayAction.Restore, primary: true).WithPosition(0, 0));
        grid.Children.Add(BuildActionButton(copy.OpenWebUiLabel, LauncherTrayAction.OpenWeb, enabled: viewModel.CanOpenWebUi).WithPosition(0, 1));
        grid.Children.Add(BuildActionButton(copy.StartServiceLabel, LauncherTrayAction.Start, enabled: viewModel.CanStart).WithPosition(1, 0));
        grid.Children.Add(BuildActionButton(copy.StopServiceLabel, LauncherTrayAction.Stop, enabled: viewModel.CanStop).WithPosition(1, 1));
        grid.Children.Add(BuildActionButton(copy.TrayPanelCloseLabel, LauncherTrayAction.Restore, closeOnly: true).WithPosition(2, 0));

        return grid;
    }

    private Control BuildHint()
    {
        return new TextBlock
        {
            Text = "托盘面板只保留常用操作。更完整的信息在启动器主窗口中查看。",
            TextWrapping = TextWrapping.Wrap,
            Foreground = Brush.Parse("#AFC5E2"),
            FontSize = 12,
            VerticalAlignment = VerticalAlignment.Bottom,
        };
    }

    private Control BuildExitButton()
    {
        return new Button
        {
            Content = copy.ExitAppLabel,
            HorizontalAlignment = HorizontalAlignment.Stretch,
            MinHeight = 44,
            Background = Brush.Parse("#A42A3044"),
            BorderBrush = Brush.Parse("#D76B76"),
            BorderThickness = new Thickness(1),
            CornerRadius = new CornerRadius(16),
            Foreground = Brush.Parse("#FFF3F4"),
        }.OnClick((_, _) => RaiseAction(LauncherTrayAction.Exit));
    }

    private Button BuildActionButton(string label, LauncherTrayAction action, bool enabled = true, bool primary = false, bool closeOnly = false)
    {
        var button = new Button
        {
            Content = label,
            MinHeight = 46,
            IsEnabled = enabled || closeOnly,
            HorizontalAlignment = HorizontalAlignment.Stretch,
            CornerRadius = new CornerRadius(16),
            BorderThickness = new Thickness(1),
            Foreground = Brush.Parse(primary ? "#082238" : "#F4F8FF"),
            BorderBrush = Brush.Parse(primary ? "#95F2FF" : "#4A74A7"),
            Background = primary
                ? new LinearGradientBrush
                {
                    StartPoint = new RelativePoint(0, 0, RelativeUnit.Relative),
                    EndPoint = new RelativePoint(1, 1, RelativeUnit.Relative),
                    GradientStops =
                    [
                        new GradientStop(Color.Parse("#78E8FF"), 0),
                        new GradientStop(Color.Parse("#36BDE9"), 1),
                    ],
                }
                : Brush.Parse("#90142540"),
        };

        button.Click += (_, _) =>
        {
            if (closeOnly)
            {
                Close();
                return;
            }

            RaiseAction(action);
        };
        return button;
    }

    private void RaiseAction(LauncherTrayAction action)
    {
        ActionRequested?.Invoke(this, action);
        Close();
    }
}

internal static class LauncherTrayPanelControlExtensions
{
    internal static T WithRow<T>(this T control, int row)
        where T : Control
    {
        Grid.SetRow(control, row);
        return control;
    }

    internal static T WithPosition<T>(this T control, int row, int column)
        where T : Control
    {
        Grid.SetRow(control, row);
        Grid.SetColumn(control, column);
        return control;
    }

    internal static T OnClick<T>(this T button, EventHandler<RoutedEventArgs> handler)
        where T : Button
    {
        button.Click += handler;
        return button;
    }
}
