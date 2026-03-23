using System.Net.Http.Headers;
using System.Reflection;
using System.Text.Json;
using RayleaBot.Launcher.Models;

namespace RayleaBot.Launcher.Services;

internal sealed class LauncherReleaseFeedClient(HttpClient? httpClient = null) : IReleaseFeedClient
{
    private static readonly TimeSpan CacheTtl = TimeSpan.FromHours(1);
    private readonly HttpClient client = httpClient ?? CreateDefaultClient();
    private readonly SemaphoreSlim gate = new(1, 1);
    private ReleaseCheckSnapshot? cached;
    private DateTimeOffset cachedAt;

    public async Task<ReleaseCheckSnapshot> GetSnapshotAsync(CancellationToken cancellationToken)
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            if (cached is not null && DateTimeOffset.UtcNow - cachedAt < CacheTtl)
            {
                return cached;
            }

            cached = await LoadSnapshotAsync(cancellationToken).ConfigureAwait(false);
            cachedAt = DateTimeOffset.UtcNow;
            return cached;
        }
        finally
        {
            gate.Release();
        }
    }

    private async Task<ReleaseCheckSnapshot> LoadSnapshotAsync(CancellationToken cancellationToken)
    {
        var buildInfoPath = Path.Combine(AppContext.BaseDirectory, "build_info.json");
        if (!File.Exists(buildInfoPath))
        {
            return ReleaseCheckSnapshot.Unavailable("build_info.json is not present next to the launcher executable.");
        }

        using var document = JsonDocument.Parse(await File.ReadAllTextAsync(buildInfoPath, cancellationToken).ConfigureAwait(false));
        var root = document.RootElement;
        var currentVersion = root.TryGetProperty("version", out var versionValue) ? versionValue.GetString() ?? string.Empty : string.Empty;
        var releaseNotesRef = root.TryGetProperty("release_notes_ref", out var releaseValue) ? releaseValue.GetString() ?? string.Empty : string.Empty;

        if (string.IsNullOrWhiteSpace(currentVersion))
        {
            return ReleaseCheckSnapshot.Unavailable("build_info.json does not declare a package version.");
        }

        var repositoryUrl = TryResolveRepositoryUrl(releaseNotesRef);
        if (string.IsNullOrWhiteSpace(repositoryUrl))
        {
            return ReleaseCheckSnapshot.Unavailable("Package metadata does not expose a GitHub release page.");
        }

        try
        {
            var repositoryPath = repositoryUrl["https://github.com/".Length..];
            using var releaseRequest = new HttpRequestMessage(HttpMethod.Get, $"https://api.github.com/repos/{repositoryPath}/releases/latest");
            using var releaseResponse = await client.SendAsync(releaseRequest, cancellationToken).ConfigureAwait(false);
            releaseResponse.EnsureSuccessStatusCode();
            using var latestDocument = JsonDocument.Parse(await releaseResponse.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false));
            var latestRoot = latestDocument.RootElement;
            var latestTag = latestRoot.TryGetProperty("tag_name", out var tagValue) ? tagValue.GetString() ?? string.Empty : string.Empty;
            var latestPage = latestRoot.TryGetProperty("html_url", out var pageValue) ? pageValue.GetString() ?? releaseNotesRef : releaseNotesRef;

            if (latestRoot.TryGetProperty("assets", out var assetsValue))
            {
                foreach (var asset in assetsValue.EnumerateArray())
                {
                    if (!asset.TryGetProperty("name", out var nameValue) || !string.Equals(nameValue.GetString(), "release_manifest.json", StringComparison.Ordinal))
                    {
                        continue;
                    }
                    if (!asset.TryGetProperty("browser_download_url", out var downloadValue))
                    {
                        continue;
                    }
                    var manifestUrl = downloadValue.GetString();
                    if (string.IsNullOrWhiteSpace(manifestUrl))
                    {
                        continue;
                    }

                    using var manifestResponse = await client.GetAsync(manifestUrl, cancellationToken).ConfigureAwait(false);
                    manifestResponse.EnsureSuccessStatusCode();
                    using var manifestDocument = JsonDocument.Parse(await manifestResponse.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false));
                    if (manifestDocument.RootElement.TryGetProperty("version", out var manifestVersion))
                    {
                        latestTag = manifestVersion.GetString() ?? latestTag;
                    }
                    if (manifestDocument.RootElement.TryGetProperty("release_notes_ref", out var manifestNotes))
                    {
                        latestPage = manifestNotes.GetString() ?? latestPage;
                    }
                    break;
                }
            }

            if (!TryCompareSemver(latestTag, currentVersion, out var isNewer))
            {
                return ReleaseCheckSnapshot.Unavailable("The release feed returned a version that could not be compared to the packaged build.");
            }

            return isNewer
                ? ReleaseCheckSnapshot.NewUpdateAvailable(currentVersion, latestTag, latestPage)
                : ReleaseCheckSnapshot.UpToDate(currentVersion, latestPage);
        }
        catch (Exception ex)
        {
            return ReleaseCheckSnapshot.Error(currentVersion, ex.Message, releaseNotesRef);
        }
    }

    private static HttpClient CreateDefaultClient()
    {
        var client = new HttpClient();
        client.DefaultRequestHeaders.UserAgent.Add(new ProductInfoHeaderValue("RayleaBotLauncher", Assembly.GetExecutingAssembly().GetName().Version?.ToString() ?? "0.1.0"));
        client.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/vnd.github+json"));
        return client;
    }

    private static string TryResolveRepositoryUrl(string releaseNotesRef)
    {
        if (Uri.TryCreate(releaseNotesRef, UriKind.Absolute, out var uri) &&
            string.Equals(uri.Host, "github.com", StringComparison.OrdinalIgnoreCase))
        {
            var segments = uri.AbsolutePath.Trim('/').Split('/', StringSplitOptions.RemoveEmptyEntries);
            if (segments.Length >= 2)
            {
                return $"https://github.com/{segments[0]}/{segments[1]}";
            }
        }
        return string.Empty;
    }

    private static bool TryCompareSemver(string left, string right, out bool isNewer)
    {
        isNewer = false;
        if (!TryParseSemver(left, out var latest) || !TryParseSemver(right, out var current))
        {
            return false;
        }

        isNewer = latest.CompareTo(current) > 0;
        return true;
    }

    private static bool TryParseSemver(string value, out SemanticVersion version)
    {
        version = default;
        var normalized = value.Trim();
        if (normalized.StartsWith('v') || normalized.StartsWith('V'))
        {
            normalized = normalized[1..];
        }
        var plusIndex = normalized.IndexOf('+');
        if (plusIndex >= 0)
        {
            normalized = normalized[..plusIndex];
        }
        var dashIndex = normalized.IndexOf('-');
        var prerelease = dashIndex >= 0 ? normalized[(dashIndex + 1)..] : string.Empty;
        if (dashIndex >= 0)
        {
            normalized = normalized[..dashIndex];
        }

        var parts = normalized.Split('.');
        if (parts.Length != 3)
        {
            return false;
        }
        if (!int.TryParse(parts[0], out var major) || !int.TryParse(parts[1], out var minor) || !int.TryParse(parts[2], out var patch))
        {
            return false;
        }
        version = new SemanticVersion(major, minor, patch, prerelease);
        return true;
    }

    private readonly record struct SemanticVersion(int Major, int Minor, int Patch, string Prerelease) : IComparable<SemanticVersion>
    {
        public int CompareTo(SemanticVersion other)
        {
            var major = Major.CompareTo(other.Major);
            if (major != 0)
            {
                return major;
            }
            var minor = Minor.CompareTo(other.Minor);
            if (minor != 0)
            {
                return minor;
            }
            var patch = Patch.CompareTo(other.Patch);
            if (patch != 0)
            {
                return patch;
            }
            if (string.IsNullOrEmpty(Prerelease) && !string.IsNullOrEmpty(other.Prerelease))
            {
                return 1;
            }
            if (!string.IsNullOrEmpty(Prerelease) && string.IsNullOrEmpty(other.Prerelease))
            {
                return -1;
            }
            return string.CompareOrdinal(Prerelease, other.Prerelease);
        }
    }
}
