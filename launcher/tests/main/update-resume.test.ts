import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import {
  completeUpdateHeartbeat,
  consumeLauncherEntryProcessId,
  consumeUpdateHeartbeatEnvironment,
  launchInterruptedUpdateRecovery,
} from "@main/services/update-resume";
import { createLauncherSnapshot } from "../helpers/snapshot";

const roots: string[] = [];

async function tempRoot(label: string) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-resume-${label}-`));
  roots.push(root);
  return root;
}

afterEach(async () => {
  await Promise.all(roots.splice(0).map((root) => fs.rm(root, { recursive: true, force: true })));
});

describe("update resume", () => {
  test("accepts and consumes the supervised launcher entry process id", () => {
    const environment = { RAYLEA_LAUNCHER_ENTRY_PID: "314" };

    expect(consumeLauncherEntryProcessId(environment, 271, 314)).toBe(314);
    expect(environment).not.toHaveProperty("RAYLEA_LAUNCHER_ENTRY_PID");
  });

  test("ignores launcher entry process ids that do not match the parent process", () => {
    const environment = { RAYLEA_LAUNCHER_ENTRY_PID: "999" };

    expect(consumeLauncherEntryProcessId(environment, 271, 314)).toBe(271);
    expect(environment).not.toHaveProperty("RAYLEA_LAUNCHER_ENTRY_PID");
  });

  test("consumes heartbeat secrets from the process environment", () => {
    const environment = {
      RAYLEA_UPDATE_HEARTBEAT: "C:/update/heartbeat.json",
      RAYLEA_UPDATE_TOKEN: "a".repeat(64),
      RAYLEA_UPDATE_RESTART_SERVICE: "true",
    };
    const request = consumeUpdateHeartbeatEnvironment(environment);

    expect(request).toEqual({
      heartbeatPath: "C:/update/heartbeat.json",
      token: "a".repeat(64),
      restartService: true,
    });
    expect(environment).not.toHaveProperty("RAYLEA_UPDATE_TOKEN");
  });

  test("writes a ready heartbeat only after service health and readiness pass", async () => {
    const parent = await tempRoot("heartbeat");
    const installRoot = path.join(parent, "RayleaBot");
    const transactionRoot = path.join(parent, ".rayleabot-update-test");
    await fs.mkdir(installRoot);
    await fs.mkdir(transactionRoot);
    await fs.writeFile(path.join(installRoot, "build_info.json"), JSON.stringify({
      version: "1.1.0",
      artifact_id: "windows-x64-full",
      update_protocol_version: 2,
    }));
    const heartbeatPath = path.join(transactionRoot, "postflight-launcher-heartbeat.json");
    const startService = vi.fn(async () => undefined);
    await completeUpdateHeartbeat(installRoot, {
      heartbeatPath,
      token: "b".repeat(64),
      restartService: true,
    }, {
      startService,
      getSnapshot: () => createLauncherSnapshot({
        launcher: { processLifecycle: "running" },
        server: { health: { status: "ok" }, readiness: { status: "ready", checks: {} } },
      }),
      launcherPid: 42,
      servicePid: () => 84,
    });

    const heartbeat = JSON.parse(await fs.readFile(heartbeatPath, "utf8")) as Record<string, unknown>;
    expect(startService).toHaveBeenCalledTimes(1);
    expect(heartbeat).toMatchObject({
      status: "ready",
      version: "1.1.0",
      launcher_pid: 42,
      service_pid: 84,
      service_running: true,
    });
  });

  test("reports failed postflight when readyz does not pass", async () => {
    const parent = await tempRoot("failed");
    const installRoot = path.join(parent, "RayleaBot");
    const transactionRoot = path.join(parent, ".rayleabot-update-test");
    await fs.mkdir(installRoot);
    await fs.mkdir(transactionRoot);
    await fs.writeFile(path.join(installRoot, "build_info.json"), JSON.stringify({
      version: "1.1.0",
      artifact_id: "windows-x64-full",
      update_protocol_version: 2,
    }));
    const heartbeatPath = path.join(transactionRoot, "postflight-launcher-heartbeat.json");
    await completeUpdateHeartbeat(installRoot, { heartbeatPath, token: "c".repeat(64), restartService: true }, {
      startService: async () => undefined,
      getSnapshot: () => createLauncherSnapshot({
        launcher: { processLifecycle: "running" },
        server: { health: { status: "ok" }, readiness: { status: "failed", checks: {} } },
      }),
      launcherPid: 42,
      servicePid: () => 84,
    });

    const heartbeat = JSON.parse(await fs.readFile(heartbeatPath, "utf8")) as Record<string, unknown>;
    expect(heartbeat.status).toBe("failed");
    expect(heartbeat.service_running).toBe(false);
  });

  test("launches only a validated external helper for interrupted recovery", async () => {
    const parent = await tempRoot("recover");
    const installRoot = path.join(parent, "RayleaBot");
    const transactionRoot = path.join(parent, ".rayleabot-update-valid");
    await fs.mkdir(installRoot);
    await fs.mkdir(transactionRoot);
    await fs.writeFile(path.join(transactionRoot, "raylea-updater.exe"), "helper");
    await fs.writeFile(path.join(transactionRoot, "journal.json"), JSON.stringify({
      version: 1,
      state: "installing",
      phase: "swap",
      install_root: installRoot,
      transaction_root: transactionRoot,
    }));
    const recoveryLauncher = { spawnDetached: vi.fn(async () => undefined) };

    await expect(launchInterruptedUpdateRecovery(installRoot, 123, recoveryLauncher)).resolves.toBe(true);
    expect(recoveryLauncher.spawnDetached).toHaveBeenCalledWith(
      path.join(transactionRoot, "raylea-updater.exe"),
      ["recover", "--transaction-root", transactionRoot, "--launcher-pid", "123"],
    );
  });
});
