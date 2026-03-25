using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class JsonLauncherSettingsStoreTests
{
    [TestMethod]
    public async Task LoadAsync_MapsLegacyCloseToTrayTrueToHideToTray()
    {
        using var temp = new LauncherTempDirectory();
        var defaults = new LauncherSettings(@"C:\RayleaBot\raylea-server.exe", @"C:\RayleaBot\config\user.yaml", @"C:\RayleaBot");
        var settingsPath = temp.CreateFile("launcher.json", """
        {
          "ServerExecutablePath": "C:\\RayleaBot\\raylea-server.exe",
          "ConfigPath": "C:\\RayleaBot\\config\\user.yaml",
          "Workdir": "C:\\RayleaBot",
          "CloseToTrayEnabled": true
        }
        """);
        var store = new JsonLauncherSettingsStore(settingsPath, defaults);

        var settings = await store.LoadAsync(CancellationToken.None);

        Assert.AreEqual(LauncherCloseBehavior.HideToTray, settings.CloseBehavior);
    }

    [TestMethod]
    public async Task LoadAsync_MapsLegacyCloseToTrayFalseToAskEveryTime()
    {
        using var temp = new LauncherTempDirectory();
        var defaults = new LauncherSettings(@"C:\RayleaBot\raylea-server.exe", @"C:\RayleaBot\config\user.yaml", @"C:\RayleaBot");
        var settingsPath = temp.CreateFile("launcher.json", """
        {
          "ServerExecutablePath": "C:\\RayleaBot\\raylea-server.exe",
          "ConfigPath": "C:\\RayleaBot\\config\\user.yaml",
          "Workdir": "C:\\RayleaBot",
          "CloseToTrayEnabled": false
        }
        """);
        var store = new JsonLauncherSettingsStore(settingsPath, defaults);

        var settings = await store.LoadAsync(CancellationToken.None);

        Assert.AreEqual(LauncherCloseBehavior.AskEveryTime, settings.CloseBehavior);
    }

    [TestMethod]
    public async Task SaveAsync_RoundTripsCloseBehavior()
    {
        using var temp = new LauncherTempDirectory();
        var defaults = new LauncherSettings(@"C:\RayleaBot\raylea-server.exe", @"C:\RayleaBot\config\user.yaml", @"C:\RayleaBot");
        var settingsPath = Path.Combine(temp.CreateDirectory("settings"), "launcher.json");
        var store = new JsonLauncherSettingsStore(settingsPath, defaults);

        await store.SaveAsync(
            new LauncherSettings(
                @"D:\Apps\raylea-server.exe",
                @"D:\Apps\config\user.yaml",
                @"D:\Apps",
                LauncherCloseBehavior.ExitApplication),
            CancellationToken.None);

        var loaded = await store.LoadAsync(CancellationToken.None);

        Assert.AreEqual(@"D:\Apps\raylea-server.exe", loaded.ServerExecutablePath);
        Assert.AreEqual(LauncherCloseBehavior.ExitApplication, loaded.CloseBehavior);
    }
}
