using RayleaBot.Launcher.Services;

namespace RayleaBot.Launcher;

internal static class LauncherCompositionRoot
{
    internal static LauncherCoordinator CreateCoordinator()
    {
        var settingsStore = new JsonLauncherSettingsStore();
        var endpointResolver = new ConfigServerEndpointResolver();
        var environmentInspector = new LauncherEnvironmentInspector();
        var managementClient = new LauncherManagementClient();
        var processController = new ServerProcessController();
        var endpointProcessController = new EndpointProcessController();
        var externalOpener = new ShellExternalOpener();
        var releaseFeedClient = new LauncherReleaseFeedClient();

        return new LauncherCoordinator(
            settingsStore,
            endpointResolver,
            environmentInspector,
            managementClient,
            processController,
            endpointProcessController,
            externalOpener,
            releaseFeedClient);
    }
}
