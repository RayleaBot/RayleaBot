import { afterEach, describe, expect, test, vi } from "vitest";
import type { LauncherResolvedSettings } from "@shared/launcher-models";

const childProcessMocks = vi.hoisted(() => ({
  execFile: vi.fn(),
}));

vi.mock("node:child_process", () => childProcessMocks);

import { NodeConfigInitializer } from "@main/services/config-initializer";

const settings: LauncherResolvedSettings = {
  installationRoot: "C:\\RayleaBot",
  serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
  configPath: "C:\\RayleaBot\\config\\user.yaml",
  workdir: "C:\\RayleaBot",
};

describe("NodeConfigInitializer", () => {
  afterEach(() => {
    childProcessMocks.execFile.mockReset();
  });

  test("runs config init before first service start", async () => {
    childProcessMocks.execFile.mockImplementation((_file, _args, _options, callback) => {
      callback(null, "", "");
    });

    await new NodeConfigInitializer().run(settings);

    expect(childProcessMocks.execFile).toHaveBeenCalledWith(
      settings.serverExecutablePath,
      ["-config", settings.configPath, "config", "init"],
      { timeout: 15000 },
      expect.any(Function),
    );
  });
});
