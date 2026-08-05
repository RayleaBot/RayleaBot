import { describe, expect, test } from "vitest";
import {
  LauncherServerCredentials,
  consumeLauncherControlTokenEnvironment,
} from "../../src/main/services/server-credentials";

describe("LauncherServerCredentials", () => {
  test("consumes the development control token without retaining it in the environment", () => {
    const environment = {
      RAYLEA_LAUNCHER_CONTROL_TOKEN: "  shared-development-token  ",
    };

    expect(consumeLauncherControlTokenEnvironment(environment)).toBe("shared-development-token");
    expect(environment).not.toHaveProperty("RAYLEA_LAUNCHER_CONTROL_TOKEN");
  });

  test("uses the initial control token until a launcher-managed server rotates credentials", () => {
    const credentials = new LauncherServerCredentials("shared-development-token");

    expect(credentials.setupToken).toBe("");
    expect(credentials.controlToken).toBe("shared-development-token");

    credentials.rotate();

    expect(credentials.setupToken).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(credentials.controlToken).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(credentials.controlToken).not.toBe("shared-development-token");
  });
});
