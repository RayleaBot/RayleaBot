using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherWindowPoliciesTests
{
    [TestMethod]
    public void ResolveCloseAction_ReturnsPromptForAskEveryTime()
    {
        Assert.AreEqual(
            LauncherWindowCloseAction.Prompt,
            LauncherWindowPolicies.ResolveCloseAction(explicitExitRequested: false, LauncherCloseBehavior.AskEveryTime));
    }

    [TestMethod]
    public void ResolveCloseAction_ReturnsHideToTrayWhenConfigured()
    {
        Assert.AreEqual(
            LauncherWindowCloseAction.HideToTray,
            LauncherWindowPolicies.ResolveCloseAction(explicitExitRequested: false, LauncherCloseBehavior.HideToTray));
    }

    [TestMethod]
    public void ResolveCloseAction_ReturnsExitWhenConfigured()
    {
        Assert.AreEqual(
            LauncherWindowCloseAction.ExitApplication,
            LauncherWindowPolicies.ResolveCloseAction(explicitExitRequested: false, LauncherCloseBehavior.ExitApplication));
    }

    [TestMethod]
    public void ResolveCloseAction_ReturnsExitForExplicitExit()
    {
        Assert.AreEqual(
            LauncherWindowCloseAction.ExitApplication,
            LauncherWindowPolicies.ResolveCloseAction(explicitExitRequested: true, LauncherCloseBehavior.AskEveryTime));
    }
}
