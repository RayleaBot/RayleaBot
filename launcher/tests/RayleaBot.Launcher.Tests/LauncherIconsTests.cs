namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherIconsTests
{
    [TestMethod]
    public void CreateTrayIcon_ReturnsWindowIcon()
    {
        Program.BuildAvaloniaApp().SetupWithoutStarting();

        var icon = LauncherIcons.CreateTrayIcon();

        Assert.IsNotNull(icon);
    }
}
