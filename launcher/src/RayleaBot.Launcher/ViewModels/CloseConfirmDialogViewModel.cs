using RayleaBot.Launcher.Infrastructure;
using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher;

internal sealed class CloseConfirmDialogViewModel : ObservableObject
{
    private bool rememberChoice;

    internal CloseConfirmDialogViewModel(LauncherCloseBehavior currentDefaultBehavior, LauncherCopy? copy = null)
    {
        CurrentDefaultBehavior = currentDefaultBehavior;
        Copy = copy ?? LauncherCopy.Default;
    }

    internal LauncherCopy Copy { get; }

    internal LauncherCloseBehavior CurrentDefaultBehavior { get; }

    internal bool RememberChoice
    {
        get => rememberChoice;
        set => SetProperty(ref rememberChoice, value);
    }

    internal string CurrentDefaultCaption => $"{Copy.CloseDialogCurrentDefaultPrefix}：";

    internal string CurrentDefaultSummary => Copy.FormatCloseBehaviorSummary(CurrentDefaultBehavior);
}
