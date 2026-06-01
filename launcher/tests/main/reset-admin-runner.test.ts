import { afterEach, describe, expect, test, vi } from "vitest";
import type { LauncherResolvedSettings } from "@shared/launcher-models";

const childProcessMocks = vi.hoisted(() => ({
  execFile: vi.fn(),
}));

vi.mock("node:child_process", () => childProcessMocks);

import { NodeResetAdminRunner } from "@main/services/reset-admin-runner";

const settings: LauncherResolvedSettings = {
  installationRoot: "C:\\RayleaBot",
  serverExecutablePath: "C:\\RayleaBot\\server\\raylea-server.exe",
  configPath: "C:\\RayleaBot\\config\\user.yaml",
  workdir: "C:\\RayleaBot",
};

describe("NodeResetAdminRunner", () => {
  afterEach(() => {
    childProcessMocks.execFile.mockReset();
  });

  test("runs reset-admin with the config path only", async () => {
    childProcessMocks.execFile.mockImplementation((_file, _args, _options, callback) => {
      callback(null, "", "");
    });

    await new NodeResetAdminRunner().run(settings);

    expect(childProcessMocks.execFile).toHaveBeenCalledWith(
      settings.serverExecutablePath,
      ["-config", settings.configPath, "reset-admin"],
      { timeout: 15000 },
      expect.any(Function),
    );
  });
});
