using System.Text.Json;
using Microsoft.Win32;
using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Services;

internal sealed class JsonLauncherSettingsStore : ILauncherSettingsStore
{
    private readonly string settingsPath;
    private readonly LauncherSettings defaults;

    internal JsonLauncherSettingsStore()
    {
        defaults = LauncherDefaults.CreateDefaultSettings(AppContext.BaseDirectory);
        var settingsDir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "RayleaBot");
        settingsPath = Path.Combine(settingsDir, "launcher.json");
    }

    public async Task<LauncherSettings> LoadAsync(CancellationToken cancellationToken)
    {
        if (!File.Exists(settingsPath))
        {
            return defaults;
        }

        await using var stream = File.OpenRead(settingsPath);
        var settings = await JsonSerializer.DeserializeAsync<LauncherSettings>(stream, cancellationToken: cancellationToken).ConfigureAwait(false);
        return settings ?? defaults;
    }

    public async Task SaveAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        Directory.CreateDirectory(Path.GetDirectoryName(settingsPath)!);
        await using var stream = File.Create(settingsPath);
        await JsonSerializer.SerializeAsync(stream, settings, cancellationToken: cancellationToken).ConfigureAwait(false);
    }
}

internal static class LauncherDefaults
{
    internal static LauncherSettings CreateDefaultSettings(string baseDirectory)
    {
        var root = FindWorkspaceRoot(baseDirectory) ?? baseDirectory;
        var serverExecutablePath = ResolveFirstExisting(
            Path.Combine(root, "raylea-server.exe"),
            Path.Combine(root, "server", "raylea-server.exe"),
            Path.Combine(root, "server", "bin", "raylea-server.exe"))
            ?? Path.Combine(root, "raylea-server.exe");

        return new LauncherSettings(
            serverExecutablePath,
            Path.Combine(root, "config", "user.yaml"),
            root);
    }

    private static string? FindWorkspaceRoot(string startPath)
    {
        var current = new DirectoryInfo(Path.GetFullPath(startPath));
        while (current is not null)
        {
            if (File.Exists(Path.Combine(current.FullName, "launcher", "global.json")) &&
                File.Exists(Path.Combine(current.FullName, "contracts", "config.user.schema.json")) &&
                File.Exists(Path.Combine(current.FullName, "server", "go.mod")))
            {
                return current.FullName;
            }

            current = current.Parent;
        }

        return null;
    }

    private static string? ResolveFirstExisting(params string[] candidates)
    {
        return candidates.FirstOrDefault(File.Exists);
    }
}

internal sealed class ConfigServerEndpointResolver : IServerEndpointResolver
{
    public ServerEndpoint Resolve(string configPath)
    {
        if (!File.Exists(configPath))
        {
            return new ServerEndpoint("127.0.0.1", 8080);
        }

        var host = "127.0.0.1";
        var port = 8080;
        var insideServer = false;

        foreach (var rawLine in File.ReadLines(configPath))
        {
            var line = rawLine.Split('#', 2)[0].TrimEnd();
            if (string.IsNullOrWhiteSpace(line))
            {
                continue;
            }

            if (!char.IsWhiteSpace(rawLine[0]))
            {
                insideServer = string.Equals(line.Trim(), "server:", StringComparison.Ordinal);
                continue;
            }

            if (!insideServer)
            {
                continue;
            }

            var trimmed = line.Trim();
            if (trimmed.StartsWith("host:", StringComparison.Ordinal))
            {
                host = trimmed["host:".Length..].Trim().Trim('"', '\'');
                continue;
            }

            if (trimmed.StartsWith("port:", StringComparison.Ordinal) &&
                int.TryParse(trimmed["port:".Length..].Trim().Trim('"', '\''), out var parsedPort))
            {
                port = parsedPort;
            }
        }

        return new ServerEndpoint(NormalizeClientHost(host), port);
    }

    private static string NormalizeClientHost(string host)
    {
        if (string.IsNullOrWhiteSpace(host))
        {
            return "127.0.0.1";
        }

        var trimmed = host.Trim().Trim('[', ']');
        if (trimmed is "0.0.0.0" or "::" or "*")
        {
            return "127.0.0.1";
        }

        return trimmed;
    }
}

internal sealed class LauncherEnvironmentInspector : IEnvironmentInspector
{
    public Task<EnvironmentInspection> InspectAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        var results = new List<EnvironmentCheckResult>
        {
            CheckExecutable(settings.ServerExecutablePath),
            CheckConfig(settings.ConfigPath),
            CheckWorkdir(settings.Workdir),
            CheckLongPaths(),
            CheckDepsManifest(settings.Workdir),
            CheckChromiumResources(settings.Workdir),
            CheckTemplateResources(settings.Workdir),
        };

        var canBootstrapUserConfig = results.Any(item => item.Code == "config.bootstrap_available");
        var hasBlockingIssues = results.Any(item => item.Severity == CheckSeverity.Error);
        return Task.FromResult(new EnvironmentInspection(results, hasBlockingIssues, canBootstrapUserConfig));
    }

    private static EnvironmentCheckResult CheckExecutable(string path)
    {
        return File.Exists(path)
            ? new EnvironmentCheckResult("server.executable", "Server executable", CheckSeverity.Ok, "Executable ready.", path, string.Empty)
            : new EnvironmentCheckResult("server.executable_missing", "Server executable", CheckSeverity.Error, "Server executable is missing.", $"Missing server executable: {path}", "Update the launcher settings to point at a valid raylea-server executable.");
    }

    private static EnvironmentCheckResult CheckConfig(string path)
    {
        var defaultPath = LauncherConfigBootstrap.GetDefaultTemplatePath(path);
        if (!File.Exists(path))
        {
            if (File.Exists(defaultPath))
            {
                return new EnvironmentCheckResult(
                    "config.bootstrap_available",
                    "Config file",
                    CheckSeverity.Warning,
                    "User config will be generated on first start.",
                    $"Missing user config file: {path}",
                    $"Start the service to bootstrap the first config from {defaultPath}.");
            }

            return new EnvironmentCheckResult(
                "config.missing",
                "Config file",
                CheckSeverity.Error,
                "Config baseline is incomplete.",
                $"Missing config file: {path}",
                $"Provide {defaultPath} so the launcher and server can bootstrap the first user config.");
        }

        try
        {
            using var stream = File.Open(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            return new EnvironmentCheckResult("config.file", "Config file", CheckSeverity.Ok, "Config ready.", path, string.Empty);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("config.unreadable", "Config file", CheckSeverity.Error, "Config file is not readable.", $"Config is not readable: {ex.Message}", "Fix file permissions or point the launcher at a readable config file.");
        }
    }

    private static EnvironmentCheckResult CheckWorkdir(string path)
    {
        try
        {
            Directory.CreateDirectory(path);
            var probe = Path.Combine(path, ".launcher-write-test");
            File.WriteAllText(probe, "ok");
            File.Delete(probe);
            return new EnvironmentCheckResult("workdir.ready", "Workdir", CheckSeverity.Ok, "Workdir is writable.", path, string.Empty);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("workdir.unwritable", "Workdir", CheckSeverity.Error, "Workdir is not writable.", $"Workdir is not writable: {ex.Message}", "Choose a writable work directory before starting the service.");
        }
    }

    private static EnvironmentCheckResult CheckLongPaths()
    {
        if (!OperatingSystem.IsWindows())
        {
            return new EnvironmentCheckResult("os.long_paths_unavailable", "LongPathsEnabled", CheckSeverity.Warning, "Long path registry check is unavailable.", "Long path registry check is only available on Windows.", "No action required on this platform.");
        }

        try
        {
            using var key = Registry.LocalMachine.OpenSubKey(@"SYSTEM\CurrentControlSet\Control\FileSystem");
            var enabled = key?.GetValue("LongPathsEnabled") switch
            {
                1 or 1L => true,
                _ => false,
            };

            return enabled
                ? new EnvironmentCheckResult("os.long_paths_enabled", "LongPathsEnabled", CheckSeverity.Ok, "Long path support is enabled.", "Registry flag is enabled.", string.Empty)
                : new EnvironmentCheckResult("os.long_paths_disabled", "LongPathsEnabled", CheckSeverity.Warning, "Long path support is disabled.", "Registry flag is disabled.", "Enable LongPathsEnabled to reduce path-length failures during runtime bootstrap.");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("os.long_paths_unknown", "LongPathsEnabled", CheckSeverity.Warning, "Long path status could not be determined.", $"Registry check failed: {ex.Message}", "Verify LongPathsEnabled manually if bootstrap or render assets hit path limits.");
        }
    }

    private static EnvironmentCheckResult CheckDepsManifest(string workdir)
    {
        var manifestPath = Path.Combine(workdir, ".deps", "manifest.json");
        if (!File.Exists(manifestPath))
        {
            return new EnvironmentCheckResult("deps.manifest_missing", ".deps manifest", CheckSeverity.Warning, "Dependency manifest is missing.", $"Missing manifest: {manifestPath}", "Restore the packaged .deps resources before using render or managed runtimes.");
        }

        try
        {
            using var document = JsonDocument.Parse(File.ReadAllText(manifestPath));
            var hasWindowsResource = document.RootElement.GetProperty("resources")
                .EnumerateArray()
                .Any(item => item.TryGetProperty("platform", out var platform) && string.Equals(platform.GetString(), "windows-x64", StringComparison.Ordinal));

            return hasWindowsResource
                ? new EnvironmentCheckResult("deps.manifest", ".deps manifest", CheckSeverity.Ok, "Dependency manifest is available.", manifestPath, string.Empty)
                : new EnvironmentCheckResult("deps.manifest_platform_missing", ".deps manifest", CheckSeverity.Warning, "Dependency manifest is missing windows-x64 resources.", "Manifest does not contain windows-x64 resources.", "Rebuild or restore the packaged .deps manifest for the current platform.");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("deps.manifest_invalid", ".deps manifest", CheckSeverity.Warning, "Dependency manifest could not be parsed.", $"Manifest parse failed: {ex.Message}", "Fix the manifest file before relying on packaged runtime assets.");
        }
    }

    private static EnvironmentCheckResult CheckChromiumResources(string workdir)
    {
        var manifestPath = Path.Combine(workdir, ".deps", "manifest.json");
        if (!File.Exists(manifestPath))
        {
            return new EnvironmentCheckResult("deps.chromium_unknown", "Chromium resources", CheckSeverity.Warning, "Chromium availability is unknown.", "Chromium cannot be checked because the .deps manifest is missing.", "Restore packaged Chromium resources before enabling render.image.");
        }

        try
        {
            using var document = JsonDocument.Parse(File.ReadAllText(manifestPath));
            var hasChromium = document.RootElement.GetProperty("resources")
                .EnumerateArray()
                .Any(item =>
                    item.TryGetProperty("platform", out var platform) &&
                    string.Equals(platform.GetString(), "windows-x64", StringComparison.Ordinal) &&
                    item.TryGetProperty("kind", out var kind) &&
                    string.Equals(kind.GetString(), "chromium", StringComparison.Ordinal));

            return hasChromium
                ? new EnvironmentCheckResult("deps.chromium", "Chromium resources", CheckSeverity.Ok, "Chromium resource entry is present.", "windows-x64 Chromium resource is declared in the dependency manifest.", string.Empty)
                : new EnvironmentCheckResult("deps.chromium_missing", "Chromium resources", CheckSeverity.Warning, "Chromium resource entry is missing.", "The dependency manifest does not declare a windows-x64 Chromium resource.", "Restore Chromium resources before enabling render.image in packaged builds.");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("deps.chromium_invalid", "Chromium resources", CheckSeverity.Warning, "Chromium resource status could not be determined.", $"Manifest parse failed: {ex.Message}", "Fix the dependency manifest before validating Chromium resources.");
        }
    }

    private static EnvironmentCheckResult CheckTemplateResources(string workdir)
    {
        var templatesPath = Path.Combine(workdir, "templates");
        if (!Directory.Exists(templatesPath))
        {
            return new EnvironmentCheckResult("render.templates_missing", "Template resources", CheckSeverity.Warning, "Template resources are missing.", $"Missing templates directory: {templatesPath}", "Add packaged templates before enabling render.image preview flows.");
        }

        var fileCount = Directory.EnumerateFiles(templatesPath, "*", SearchOption.AllDirectories).Take(1).Count();
        return fileCount > 0
            ? new EnvironmentCheckResult("render.templates", "Template resources", CheckSeverity.Ok, "Template resources are available.", templatesPath, string.Empty)
            : new EnvironmentCheckResult("render.templates_empty", "Template resources", CheckSeverity.Warning, "Template resources are empty.", $"Templates directory has no files: {templatesPath}", "Populate packaged templates before enabling render.image preview flows.");
    }
}

internal static class LauncherConfigBootstrap
{
    internal static string GetDefaultTemplatePath(string userConfigPath)
    {
        var configDirectory = Path.GetDirectoryName(userConfigPath);
        if (string.IsNullOrWhiteSpace(configDirectory))
        {
            configDirectory = AppContext.BaseDirectory;
        }

        return Path.Combine(configDirectory, "default.yaml");
    }

    internal static bool EnsureUserConfigExists(string userConfigPath)
    {
        if (File.Exists(userConfigPath))
        {
            return false;
        }

        var defaultTemplatePath = GetDefaultTemplatePath(userConfigPath);
        if (!File.Exists(defaultTemplatePath))
        {
            return false;
        }

        Directory.CreateDirectory(Path.GetDirectoryName(userConfigPath)!);
        File.Copy(defaultTemplatePath, userConfigPath, overwrite: false);
        return true;
    }
}
