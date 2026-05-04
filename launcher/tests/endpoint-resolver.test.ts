import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";

import { resolveServerEndpoint } from "../src/main/services/endpoint-resolver";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeConfig(dir: string, text: string) {
  const configPath = path.join(dir, "user.yaml");
  await fs.writeFile(configPath, text, "utf8");
  return configPath;
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("resolveServerEndpoint", () => {
  it("reads server host and port from user config", async () => {
    const configPath = await writeConfig(
      await createTempDir("endpoint"),
      "server:\n  host: 192.168.1.10\n  port: 19080\n",
    );

    await expect(resolveServerEndpoint(configPath)).resolves.toEqual({
      host: "192.168.1.10",
      port: 19080,
      baseUrl: "http://192.168.1.10:19080/",
    });
  });

  it("normalizes wildcard listener hosts for local client access", async () => {
    const configPath = await writeConfig(
      await createTempDir("endpoint"),
      "server:\n  host: 0.0.0.0\n  port: \"18080\"\n",
    );

    await expect(resolveServerEndpoint(configPath)).resolves.toEqual({
      host: "127.0.0.1",
      port: 18080,
      baseUrl: "http://127.0.0.1:18080/",
    });
  });

  it("keeps the default port when server.port is absent", async () => {
    const configPath = await writeConfig(
      await createTempDir("endpoint"),
      "server:\n  host: 127.0.0.1\n",
    );

    await expect(resolveServerEndpoint(configPath)).resolves.toEqual({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });
  });

  it("formats IPv6 hosts in the base URL", async () => {
    const configPath = await writeConfig(
      await createTempDir("endpoint"),
      "server:\n  host: \"[::1]\"\n  port: 18080\n",
    );

    await expect(resolveServerEndpoint(configPath)).resolves.toEqual({
      host: "::1",
      port: 18080,
      baseUrl: "http://[::1]:18080/",
    });
  });

  it("uses localhost defaults and emits a warning when the config cannot be read", async () => {
    const onWarning = vi.fn();
    await expect(resolveServerEndpoint("missing-user.yaml", { onWarning })).resolves.toEqual({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });
    expect(onWarning).toHaveBeenCalledTimes(1);
  });

  it("uses localhost defaults and emits a warning when the config is invalid", async () => {
    const configPath = await writeConfig(await createTempDir("endpoint"), "server: [\n");
    const onWarning = vi.fn();

    await expect(resolveServerEndpoint(configPath, { onWarning })).resolves.toEqual({
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    });
    expect(onWarning).toHaveBeenCalledTimes(1);
  });
});
