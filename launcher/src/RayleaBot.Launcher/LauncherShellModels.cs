namespace RayleaBot.Launcher;

internal enum LauncherTrayAction
{
    Restore,
    OpenWeb,
    Start,
    Stop,
    Exit,
}

internal static class LauncherWindowPolicies
{
    internal static bool ShouldPromptBeforeClose(bool explicitExitRequested) => !explicitExitRequested;
}
