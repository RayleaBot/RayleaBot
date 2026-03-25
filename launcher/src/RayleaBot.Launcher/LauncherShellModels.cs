namespace RayleaBot.Launcher;

using RayleaBot.Launcher.Models;

internal enum LauncherTrayAction
{
    Restore,
    OpenWeb,
    OpenLogs,
    Start,
    Stop,
    Exit,
}

internal enum LauncherCloseDialogAction
{
    Cancel,
    HideToTray,
    ExitApplication,
}

internal readonly record struct LauncherCloseDialogResult(
    LauncherCloseDialogAction Action,
    bool RememberChoice);

internal enum LauncherWindowCloseAction
{
    Prompt,
    HideToTray,
    ExitApplication,
}

internal static class LauncherWindowPolicies
{
    internal static LauncherWindowCloseAction ResolveCloseAction(bool explicitExitRequested, LauncherCloseBehavior closeBehavior)
    {
        if (explicitExitRequested)
        {
            return LauncherWindowCloseAction.ExitApplication;
        }

        return closeBehavior switch
        {
            LauncherCloseBehavior.HideToTray => LauncherWindowCloseAction.HideToTray,
            LauncherCloseBehavior.ExitApplication => LauncherWindowCloseAction.ExitApplication,
            _ => LauncherWindowCloseAction.Prompt,
        };
    }
}
