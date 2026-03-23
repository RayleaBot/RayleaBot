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
        var externalOpener = new ShellExternalOpener();

        return new LauncherCoordinator(
            settingsStore,
            endpointResolver,
            environmentInspector,
            managementClient,
            processController,
            externalOpener);
    }
}
