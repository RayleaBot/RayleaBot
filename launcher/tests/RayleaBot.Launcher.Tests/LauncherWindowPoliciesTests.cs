namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherWindowPoliciesTests
{
    [TestMethod]
    public void ShouldPromptBeforeClose_ReturnsTrueForNormalClose()
    {
        Assert.IsTrue(LauncherWindowPolicies.ShouldPromptBeforeClose(explicitExitRequested: false));
    }

    [TestMethod]
    public void ShouldPromptBeforeClose_ReturnsFalseForExplicitExit()
    {
        Assert.IsFalse(LauncherWindowPolicies.ShouldPromptBeforeClose(explicitExitRequested: true));
    }
}
