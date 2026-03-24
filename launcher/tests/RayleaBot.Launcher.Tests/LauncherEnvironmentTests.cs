using RayleaBot.Launcher.Models;
using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher.Tests;

[TestClass]
public sealed class LauncherEnvironmentTests
{
    [TestMethod]
    public async Task InspectAsync_TreatsMissingUserConfigAsBootstrapWarningWhenDefaultExists()
    {
        using var temp = new LauncherTempDirectory();
        var root = temp.CreateDirectory("workspace");
        var serverExe = temp.CreateFile(Path.Combine(root, "raylea-server.exe"), "stub");
        var configDir = temp.CreateDirectory(Path.Combine(root, "config"));
        temp.CreateFile(Path.Combine(configDir, "default.yaml"), "schema_version: \"2\"\nserver:\n  host: 127.0.0.1\n  port: 8080\n");
        temp.CreateDirectory(Path.Combine(root, ".deps"));
        temp.CreateFile(Path.Combine(root, ".deps", "manifest.json"), """
        {
          "resources": [
            { "platform": "windows-x64", "kind": "chromium" }
          ]
        }
        """);

        var inspector = new LauncherEnvironmentInspector();
        var inspection = await inspector.InspectAsync(
            new LauncherSettings(
                serverExe,
                Path.Combine(configDir, "user.yaml"),
                root),
            CancellationToken.None);

        Assert.IsFalse(inspection.HasBlockingIssues);
        Assert.IsTrue(inspection.CanBootstrapUserConfig);
        Assert.IsTrue(inspection.Checks.Any(item => item.Code == "config.bootstrap_available" && item.Severity == CheckSeverity.Warning));
    }

    [TestMethod]
    public async Task InspectAsync_ReportsBlockingConfigErrorWhenDefaultTemplateIsMissing()
    {
        using var temp = new LauncherTempDirectory();
        var root = temp.CreateDirectory("workspace");
        var serverExe = temp.CreateFile(Path.Combine(root, "raylea-server.exe"), "stub");
        var configDir = temp.CreateDirectory(Path.Combine(root, "config"));
        temp.CreateDirectory(Path.Combine(root, ".deps"));
        temp.CreateFile(Path.Combine(root, ".deps", "manifest.json"), """
        {
          "resources": [
            { "platform": "windows-x64", "kind": "chromium" }
          ]
        }
        """);

        var inspector = new LauncherEnvironmentInspector();
        var inspection = await inspector.InspectAsync(
            new LauncherSettings(
                serverExe,
                Path.Combine(configDir, "user.yaml"),
                root),
            CancellationToken.None);

        Assert.IsTrue(inspection.HasBlockingIssues);
        Assert.IsFalse(inspection.CanBootstrapUserConfig);
        Assert.IsTrue(inspection.Checks.Any(item => item.Code == "config.missing" && item.Severity == CheckSeverity.Error));
    }
}

internal sealed class LauncherTempDirectory : IDisposable
{
    private readonly string rootPath = Path.Combine(Path.GetTempPath(), "rayleabot-launcher-tests", Guid.NewGuid().ToString("N"));

    internal string CreateDirectory(string relativeOrAbsolute)
    {
        var path = Normalize(relativeOrAbsolute);
        Directory.CreateDirectory(path);
        return path;
    }

    internal string CreateFile(string relativeOrAbsolute, string contents)
    {
        var path = Normalize(relativeOrAbsolute);
        Directory.CreateDirectory(Path.GetDirectoryName(path)!);
        File.WriteAllText(path, contents);
        return path;
    }

    public void Dispose()
    {
        if (Directory.Exists(rootPath))
        {
            Directory.Delete(rootPath, recursive: true);
        }
    }

    private string Normalize(string relativeOrAbsolute)
    {
        if (Path.IsPathRooted(relativeOrAbsolute))
        {
            return relativeOrAbsolute;
        }

        return Path.Combine(rootPath, relativeOrAbsolute);
    }
}
