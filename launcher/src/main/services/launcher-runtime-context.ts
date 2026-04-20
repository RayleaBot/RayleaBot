import type { LauncherResolvedSettings } from "../../shared/launcher-models";
import type {
  LauncherManagementClient,
  LauncherOperationContext,
  LauncherRuntimeContext,
  LauncherSettings,
  LauncherSettingsStore,
  ServerEndpointResolver,
} from "./launcher-coordinator.types";
import { resolveLauncherSettings } from "./settings-store";

interface LauncherRuntimeContextDependencies {
  settingsStore: LauncherSettingsStore;
  endpointResolver: ServerEndpointResolver;
  managementClient: LauncherManagementClient;
}

function defaultResolvedSettings(): LauncherResolvedSettings {
  return {
    installationRoot: "",
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
  };
}

export function createLauncherRuntimeContext(deps: LauncherRuntimeContextDependencies): LauncherRuntimeContext {
  let currentSettings: LauncherSettings | null = null;
  let currentResolvedSettings: LauncherResolvedSettings = defaultResolvedSettings();
  let sessionToken = "";

  function ensureSettings() {
    if (!currentSettings) {
      throw new Error("尚未加载启动器设置。");
    }
    return currentSettings;
  }

  async function buildOperationContext(settings: LauncherSettings): Promise<LauncherOperationContext> {
    currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
    const endpoint = await deps.endpointResolver.resolve(currentResolvedSettings.configPath);

    return {
      settings,
      resolvedSettings: currentResolvedSettings,
      endpoint,
    };
  }

  return {
    getCurrentSettings() {
      return ensureSettings();
    },
    async initialize() {
      currentSettings = await deps.settingsStore.load();
      return await buildOperationContext(currentSettings);
    },
    async createOperationContext() {
      return await buildOperationContext(ensureSettings());
    },
    async saveSettings(settings) {
      currentSettings = settings;
      sessionToken = "";
      await deps.settingsStore.save(settings);
      return await buildOperationContext(settings);
    },
    async ensureSessionToken(endpoint) {
      if (sessionToken) {
        return sessionToken;
      }

      const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
      sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
      return sessionToken;
    },
    clearSessionToken() {
      sessionToken = "";
    },
  };
}
