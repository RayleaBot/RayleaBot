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
    public Task<IReadOnlyList<EnvironmentCheckResult>> InspectAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        var results = new List<EnvironmentCheckResult>
        {
            CheckExecutable(settings.ServerExecutablePath),
            CheckConfig(settings.ConfigPath),
            CheckWorkdir(settings.Workdir),
            CheckLongPaths(),
            CheckDepsManifest(settings.Workdir),
        };

        return Task.FromResult<IReadOnlyList<EnvironmentCheckResult>>(results);
    }

    private static EnvironmentCheckResult CheckExecutable(string path)
    {
        return File.Exists(path)
            ? new EnvironmentCheckResult("Server executable", CheckSeverity.Ok, path)
            : new EnvironmentCheckResult("Server executable", CheckSeverity.Error, $"Missing server executable: {path}");
    }

    private static EnvironmentCheckResult CheckConfig(string path)
    {
        if (!File.Exists(path))
        {
            return new EnvironmentCheckResult("Config file", CheckSeverity.Error, $"Missing config file: {path}");
        }

        try
        {
            using var stream = File.Open(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            return new EnvironmentCheckResult("Config file", CheckSeverity.Ok, path);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("Config file", CheckSeverity.Error, $"Config is not readable: {ex.Message}");
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
            return new EnvironmentCheckResult("Workdir", CheckSeverity.Ok, path);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("Workdir", CheckSeverity.Error, $"Workdir is not writable: {ex.Message}");
        }
    }

    private static EnvironmentCheckResult CheckLongPaths()
    {
        if (!OperatingSystem.IsWindows())
        {
            return new EnvironmentCheckResult("LongPathsEnabled", CheckSeverity.Warning, "Long path registry check is only available on Windows.");
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
                ? new EnvironmentCheckResult("LongPathsEnabled", CheckSeverity.Ok, "Registry flag is enabled.")
                : new EnvironmentCheckResult("LongPathsEnabled", CheckSeverity.Warning, "Registry flag is disabled.");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("LongPathsEnabled", CheckSeverity.Warning, $"Registry check failed: {ex.Message}");
        }
    }

    private static EnvironmentCheckResult CheckDepsManifest(string workdir)
    {
        var manifestPath = Path.Combine(workdir, ".deps", "manifest.json");
        if (!File.Exists(manifestPath))
        {
            return new EnvironmentCheckResult(".deps manifest", CheckSeverity.Warning, $"Missing manifest: {manifestPath}");
        }

        try
        {
            using var document = JsonDocument.Parse(File.ReadAllText(manifestPath));
            var hasWindowsResource = document.RootElement.GetProperty("resources")
                .EnumerateArray()
                .Any(item => item.TryGetProperty("platform", out var platform) && string.Equals(platform.GetString(), "windows-x64", StringComparison.Ordinal));

            return hasWindowsResource
                ? new EnvironmentCheckResult(".deps manifest", CheckSeverity.Ok, manifestPath)
                : new EnvironmentCheckResult(".deps manifest", CheckSeverity.Warning, "Manifest does not contain windows-x64 resources.");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult(".deps manifest", CheckSeverity.Warning, $"Manifest parse failed: {ex.Message}");
        }
    }
}
