using Avalonia.Controls;

namespace RayleaBot.Launcher;

internal static class LauncherIcons
{
    private const string TrayPngBase64 =
        "iVBORw0KGgoAAAANSUhEUgAAACAAAAAgCAYAAABzenr0AAAAl0lEQVRYR+2WQQrAIAwER0v//2eXlyBFhB8W3YwzB4U0jTmsuMThA1h4m+zvAkA0VQGQFIVAUhQBSgEM1k0V7tXvA/1wXr0p3sU9IhS9y3n9cG3wD4Jm1J0tX7f7d4C+YxQq8H8M8D0D2Qqj3hC/ErP8R9J8P2ImB9y9Yw0hQKp1Y3cG4L5Zk2wY7s0prf4UQ0v4Q2wVdN4r6zM1m3T2WgO4s5jvX7mE7d4w0QwQ0QwQ0QwQ0QwQ0QwQ0QwQ0QwQ8Q0a1A7mQ0GXnLwAAAABJRU5ErkJggg==";

    internal static WindowIcon CreateTrayIcon()
    {
        var bytes = Convert.FromBase64String(TrayPngBase64);
        return new WindowIcon(new MemoryStream(bytes));
    }
}
