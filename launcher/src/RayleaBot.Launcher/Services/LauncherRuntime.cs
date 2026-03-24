using System.Diagnostics;
using System.Net;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Services;

internal sealed class LauncherHttpStatusException(HttpStatusCode statusCode, string message) : Exception(message)
{
    internal HttpStatusCode StatusCode { get; } = statusCode;
}

internal sealed class LauncherManagementClient : ILauncherManagementClient
{
    private readonly HttpClient httpClient;

    internal LauncherManagementClient(HttpClient? httpClient = null)
    {
        this.httpClient = httpClient ?? new HttpClient
        {
            Timeout = TimeSpan.FromSeconds(5),
        };
    }

    public async Task<bool> IsHealthyAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        using var response = await httpClient.GetAsync(new Uri(endpoint.BaseUri, "healthz"), cancellationToken).ConfigureAwait(false);
        return response.IsSuccessStatusCode;
    }

    public async Task<ReadinessSnapshot> GetReadinessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        using var response = await httpClient.GetAsync(new Uri(endpoint.BaseUri, "readyz"), cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
        using var content = await ParseAsync(response, cancellationToken).ConfigureAwait(false);
        return new ReadinessSnapshot(GetString(content, "status", "failed"), GetString(content, "reason", string.Empty));
    }

    public async Task<bool> GetSetupInitializedAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        using var response = await httpClient.GetAsync(new Uri(endpoint.BaseUri, "api/setup/status"), cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
        using var content = await ParseAsync(response, cancellationToken).ConfigureAwait(false);
        return content.RootElement.TryGetProperty("initialized", out var initialized) && initialized.GetBoolean();
    }

    public async Task<string> IssueLauncherTokenAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, new Uri(endpoint.BaseUri, "api/session/launcher-token"));
        using var response = await httpClient.SendAsync(request, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
        using var content = await ParseAsync(response, cancellationToken).ConfigureAwait(false);
        return GetString(content, "launcher_token", string.Empty);
    }

    public async Task<string> AdmitLauncherTokenAsync(ServerEndpoint endpoint, string launcherToken, CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(HttpMethod.Post, new Uri(endpoint.BaseUri, "api/session/launcher-admission"))
        {
            Content = JsonContent(new { launcher_token = launcherToken }),
        };
        using var response = await httpClient.SendAsync(request, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
        using var content = await ParseAsync(response, cancellationToken).ConfigureAwait(false);
        return GetString(content, "session_token", string.Empty);
    }

    public async Task<SystemStatusSnapshot> GetSystemStatusAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken)
    {
        using var request = CreateAuthedRequest(HttpMethod.Get, new Uri(endpoint.BaseUri, "api/system/status"), sessionToken);
        using var response = await httpClient.SendAsync(request, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
        using var content = await ParseAsync(response, cancellationToken).ConfigureAwait(false);
        return new SystemStatusSnapshot(
            GetString(content, "status", "running"),
            GetString(content, "adapter_state", "unknown"),
            GetInt(content, "active_plugins"),
            GetLong(content, "uptime_seconds"));
    }

    public async Task ShutdownAsync(ServerEndpoint endpoint, string sessionToken, CancellationToken cancellationToken)
    {
        using var request = CreateAuthedRequest(HttpMethod.Post, new Uri(endpoint.BaseUri, "api/system/shutdown"), sessionToken);
        using var response = await httpClient.SendAsync(request, cancellationToken).ConfigureAwait(false);
        await EnsureSuccessAsync(response, cancellationToken).ConfigureAwait(false);
    }

    private static HttpRequestMessage CreateAuthedRequest(HttpMethod method, Uri uri, string sessionToken)
    {
        var request = new HttpRequestMessage(method, uri);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", sessionToken);
        return request;
    }

    private static StringContent JsonContent<T>(T value)
    {
        return new StringContent(JsonSerializer.Serialize(value), Encoding.UTF8, "application/json");
    }

    private static async Task<JsonDocument> ParseAsync(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken).ConfigureAwait(false);
        return await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken).ConfigureAwait(false);
    }

    private static async Task EnsureSuccessAsync(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var payload = await response.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
        throw new LauncherHttpStatusException(response.StatusCode, payload);
    }

    private static string GetString(JsonDocument document, string propertyName, string fallback)
    {
        return document.RootElement.TryGetProperty(propertyName, out var property) && property.ValueKind == JsonValueKind.String
            ? property.GetString() ?? fallback
            : fallback;
    }

    private static int GetInt(JsonDocument document, string propertyName)
    {
        return document.RootElement.TryGetProperty(propertyName, out var property) && property.TryGetInt32(out var value)
            ? value
            : 0;
    }

    private static long GetLong(JsonDocument document, string propertyName)
    {
        return document.RootElement.TryGetProperty(propertyName, out var property) && property.TryGetInt64(out var value)
            ? value
            : 0;
    }
}

internal sealed class ServerProcessController : IServerProcessController
{
    private const int MaxStderrLines = 40;
    private readonly object gate = new();
    private readonly Queue<string> stderrLines = new();
    private Process? process;

    public bool IsRunning => process is { HasExited: false };

    public string LogDirectory { get; private set; } = Path.Combine(AppContext.BaseDirectory, "logs");

    public IReadOnlyList<string> GetRecentStderr()
    {
        lock (gate)
        {
            return stderrLines.ToArray();
        }
    }

    public Task StartAsync(LauncherSettings settings, CancellationToken cancellationToken)
    {
        if (IsRunning)
        {
            return Task.CompletedTask;
        }

        Directory.CreateDirectory(settings.Workdir);
        LogDirectory = Path.Combine(settings.Workdir, "logs");
        Directory.CreateDirectory(LogDirectory);

        var logPath = Path.Combine(LogDirectory, "launcher.log");
        var startInfo = new ProcessStartInfo
        {
            FileName = settings.ServerExecutablePath,
            Arguments = $"-config \"{settings.ConfigPath}\"",
            WorkingDirectory = settings.Workdir,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };

        process = new Process
        {
            StartInfo = startInfo,
            EnableRaisingEvents = true,
        };

        process.OutputDataReceived += (_, args) => AppendLog(logPath, "stdout", args.Data);
        process.ErrorDataReceived += (_, args) =>
        {
            AppendLog(logPath, "stderr", args.Data);
            if (!string.IsNullOrWhiteSpace(args.Data))
            {
                lock (gate)
                {
                    stderrLines.Enqueue(args.Data);
                    while (stderrLines.Count > MaxStderrLines)
                    {
                        stderrLines.Dequeue();
                    }
                }
            }
        };

        if (!process.Start())
        {
            throw new InvalidOperationException("启动 raylea-server 失败。");
        }

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
        return Task.CompletedTask;
    }

    public async Task ForceKillAsync(CancellationToken cancellationToken)
    {
        if (process is null || process.HasExited)
        {
            return;
        }

        process.Kill(entireProcessTree: true);
        await process.WaitForExitAsync(cancellationToken).ConfigureAwait(false);
    }

    private static void AppendLog(string logPath, string stream, string? message)
    {
        if (string.IsNullOrWhiteSpace(message))
        {
            return;
        }

        var line = $"[{DateTimeOffset.Now:O}] {stream}: {message}{Environment.NewLine}";
        File.AppendAllText(logPath, line, Encoding.UTF8);
    }
}

internal sealed class EndpointProcessController : IEndpointProcessController
{
    public async Task<bool> IsEndpointListeningAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        return await TryResolveOwningProcessIdAsync(endpoint, cancellationToken).ConfigureAwait(false) is not null;
    }

    public async Task<bool> TryStopEndpointProcessAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        var processId = await TryResolveOwningProcessIdAsync(endpoint, cancellationToken).ConfigureAwait(false);
        if (processId is null)
        {
            return false;
        }

        try
        {
            using var process = Process.GetProcessById(processId.Value);
            if (process.HasExited)
            {
                return true;
            }

            process.Kill(entireProcessTree: true);
            await process.WaitForExitAsync(cancellationToken).ConfigureAwait(false);
            return true;
        }
        catch
        {
            return false;
        }
    }

    private static async Task<int?> TryResolveOwningProcessIdAsync(ServerEndpoint endpoint, CancellationToken cancellationToken)
    {
        using var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = "netstat",
                Arguments = "-ano -p tcp",
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                UseShellExecute = false,
                CreateNoWindow = true,
            },
        };

        if (!process.Start())
        {
            return null;
        }

        var output = await process.StandardOutput.ReadToEndAsync(cancellationToken).ConfigureAwait(false);
        await process.WaitForExitAsync(cancellationToken).ConfigureAwait(false);
        if (process.ExitCode != 0)
        {
            return null;
        }

        foreach (var rawLine in output.Split(['\r', '\n'], StringSplitOptions.RemoveEmptyEntries))
        {
            var line = rawLine.Trim();
            if (!line.StartsWith("TCP", StringComparison.OrdinalIgnoreCase) ||
                !line.Contains("LISTENING", StringComparison.OrdinalIgnoreCase))
            {
                continue;
            }

            var columns = line.Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries);
            if (columns.Length < 5)
            {
                continue;
            }

            if (!TryParsePort(columns[1], out var localPort) || localPort != endpoint.Port)
            {
                continue;
            }

            if (int.TryParse(columns[^1], out var processId))
            {
                return processId;
            }
        }

        return null;
    }

    private static bool TryParsePort(string value, out int port)
    {
        port = 0;
        var lastColon = value.LastIndexOf(':');
        if (lastColon < 0 || lastColon == value.Length - 1)
        {
            return false;
        }

        return int.TryParse(value[(lastColon + 1)..], out port);
    }
}

internal sealed class ShellExternalOpener : IExternalOpener
{
    public Task OpenUriAsync(Uri uri, CancellationToken cancellationToken)
    {
        Process.Start(new ProcessStartInfo
        {
            FileName = uri.ToString(),
            UseShellExecute = true,
        });
        return Task.CompletedTask;
    }

    public Task OpenDirectoryAsync(string directoryPath, CancellationToken cancellationToken)
    {
        Directory.CreateDirectory(directoryPath);
        Process.Start(new ProcessStartInfo
        {
            FileName = directoryPath,
            UseShellExecute = true,
        });
        return Task.CompletedTask;
    }
}
