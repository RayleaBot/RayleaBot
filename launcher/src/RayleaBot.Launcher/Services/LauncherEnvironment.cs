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
            ? new EnvironmentCheckResult("server.executable", "服务端可执行文件", CheckSeverity.Ok, "已找到可执行文件。", path, string.Empty)
            : new EnvironmentCheckResult("server.executable_missing", "服务端可执行文件", CheckSeverity.Error, "未找到服务端可执行文件。", $"缺少服务端可执行文件：{path}", "请将启动器设置更新为有效的 raylea-server 可执行文件路径。");
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
                    "用户配置",
                    CheckSeverity.Warning,
                    "首次启动时会自动生成用户配置。",
                    $"缺少用户配置文件：{path}",
                    $"启动服务后会基于 {defaultPath} 生成首份用户配置。");
            }

            return new EnvironmentCheckResult(
                "config.missing",
                "配置基线",
                CheckSeverity.Error,
                "配置基线不完整。",
                $"缺少配置文件：{path}",
                $"请提供 {defaultPath}，以便启动器和服务端自动生成首份用户配置。");
        }

        try
        {
            using var stream = File.Open(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite);
            return new EnvironmentCheckResult("config.file", "用户配置", CheckSeverity.Ok, "配置文件可读。", path, string.Empty);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("config.unreadable", "用户配置", CheckSeverity.Error, "配置文件不可读。", $"配置读取失败：{ex.Message}", "请修复文件权限，或将启动器指向可读取的配置文件。");
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
            return new EnvironmentCheckResult("workdir.ready", "工作目录", CheckSeverity.Ok, "工作目录可写。", path, string.Empty);
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("workdir.unwritable", "工作目录", CheckSeverity.Error, "工作目录不可写。", $"工作目录写入失败：{ex.Message}", "请先选择可写的工作目录，再启动服务。");
        }
    }

    private static EnvironmentCheckResult CheckLongPaths()
    {
        if (!OperatingSystem.IsWindows())
        {
            return new EnvironmentCheckResult("os.long_paths_unavailable", "长路径支持", CheckSeverity.Warning, "当前平台无法检查长路径注册表。", "长路径注册表检查仅在 Windows 上可用。", "当前平台无需额外处理。");
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
                ? new EnvironmentCheckResult("os.long_paths_enabled", "长路径支持", CheckSeverity.Ok, "已启用长路径支持。", "注册表开关已开启。", string.Empty)
                : new EnvironmentCheckResult("os.long_paths_disabled", "长路径支持", CheckSeverity.Warning, "长路径支持未启用。", "注册表开关当前关闭。", "建议启用 LongPathsEnabled，以减少运行时引导和资源展开时的路径长度失败。");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("os.long_paths_unknown", "长路径支持", CheckSeverity.Warning, "无法确定长路径支持状态。", $"注册表检查失败：{ex.Message}", "若引导或渲染资源遇到路径限制，请手动确认 LongPathsEnabled。");
        }
    }

    private static EnvironmentCheckResult CheckDepsManifest(string workdir)
    {
        var manifestPath = Path.Combine(workdir, ".deps", "manifest.json");
        if (!File.Exists(manifestPath))
        {
            return new EnvironmentCheckResult("deps.manifest_missing", ".deps 清单", CheckSeverity.Warning, "依赖清单缺失。", $"缺少清单文件：{manifestPath}", "请先恢复打包后的 .deps 资源，再使用渲染或托管运行时能力。");
        }

        try
        {
            using var document = JsonDocument.Parse(File.ReadAllText(manifestPath));
            var hasWindowsResource = document.RootElement.GetProperty("resources")
                .EnumerateArray()
                .Any(item => item.TryGetProperty("platform", out var platform) && string.Equals(platform.GetString(), "windows-x64", StringComparison.Ordinal));

            return hasWindowsResource
                ? new EnvironmentCheckResult("deps.manifest", ".deps 清单", CheckSeverity.Ok, "依赖清单可用。", manifestPath, string.Empty)
                : new EnvironmentCheckResult("deps.manifest_platform_missing", ".deps 清单", CheckSeverity.Warning, "依赖清单缺少 windows-x64 资源。", "清单中没有当前平台所需的 windows-x64 资源。", "请为当前平台重新生成或恢复打包后的 .deps 清单。");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("deps.manifest_invalid", ".deps 清单", CheckSeverity.Warning, "依赖清单无法解析。", $"清单解析失败：{ex.Message}", "请先修复清单文件，再依赖打包运行时资源。");
        }
    }

    private static EnvironmentCheckResult CheckChromiumResources(string workdir)
    {
        var manifestPath = Path.Combine(workdir, ".deps", "manifest.json");
        if (!File.Exists(manifestPath))
        {
            return new EnvironmentCheckResult("deps.chromium_unknown", "Chromium 资源", CheckSeverity.Warning, "无法确认 Chromium 资源状态。", "由于 .deps 清单缺失，当前无法检查 Chromium 资源。", "启用 render.image 之前，请先恢复打包的 Chromium 资源。");
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
                ? new EnvironmentCheckResult("deps.chromium", "Chromium 资源", CheckSeverity.Ok, "已声明 Chromium 资源。", "依赖清单中已包含 windows-x64 Chromium 资源。", string.Empty)
                : new EnvironmentCheckResult("deps.chromium_missing", "Chromium 资源", CheckSeverity.Warning, "缺少 Chromium 资源声明。", "依赖清单中没有 windows-x64 Chromium 资源。", "启用打包版 render.image 之前，请先恢复 Chromium 资源。");
        }
        catch (Exception ex)
        {
            return new EnvironmentCheckResult("deps.chromium_invalid", "Chromium 资源", CheckSeverity.Warning, "无法判断 Chromium 资源状态。", $"清单解析失败：{ex.Message}", "请先修复依赖清单，再验证 Chromium 资源。");
        }
    }

    private static EnvironmentCheckResult CheckTemplateResources(string workdir)
    {
        var templatesPath = Path.Combine(workdir, "templates");
        if (!Directory.Exists(templatesPath))
        {
            return new EnvironmentCheckResult("render.templates_missing", "模板资源", CheckSeverity.Warning, "模板资源缺失。", $"缺少模板目录：{templatesPath}", "启用 render.image 预览链路之前，请先补齐打包模板资源。");
        }

        var fileCount = Directory.EnumerateFiles(templatesPath, "*", SearchOption.AllDirectories).Take(1).Count();
        return fileCount > 0
            ? new EnvironmentCheckResult("render.templates", "模板资源", CheckSeverity.Ok, "模板资源可用。", templatesPath, string.Empty)
            : new EnvironmentCheckResult("render.templates_empty", "模板资源", CheckSeverity.Warning, "模板资源为空。", $"模板目录中没有文件：{templatesPath}", "启用 render.image 预览链路之前，请先补齐打包模板资源。");
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
