import assert from "node:assert/strict";
import { execFile, spawn } from "node:child_process";
import { once } from "node:events";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { promisify } from "node:util";

const execute = promisify(execFile);
test("Wayland app_id resolves to the launcher desktop entry and embedded icon", {
  skip: process.platform !== "linux", timeout: 30000,
}, async () => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-wayland-"));
  const children = [];
  let output = "";
  const environment = {
    ...process.env, XDG_RUNTIME_DIR: directory, XDG_DATA_HOME: path.join(directory, "data"),
    GDK_BACKEND: "wayland", WAYLAND_DISPLAY: "raylea-test", WAYLAND_DEBUG: "client",
  };
  function start(file, args, env) {
    const child = spawn(file, args, { cwd: directory, env, stdio: ["ignore", "pipe", "pipe"] });
    children.push({ child, exited: once(child, "exit") });
    child.stdout.on("data", data => { output += data; });
    child.stderr.on("data", data => { output += data; });
    return child;
  }
  async function waitFor(probe, label) {
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      for (const { child } of children) assert.equal(child.exitCode, null, `${label}: ${output}`);
      if (await probe()) return;
      await delay(100);
    }
    assert.fail(`Timed out waiting for ${label}: ${output}`);
  }
  try {
    start("weston", ["--backend=headless-backend.so", "--use-pixman", "--socket=raylea-test", "--idle-time=0", "--no-config"], environment);
    await waitFor(() => fs.stat(path.join(directory, "raylea-test")).then(() => true, () => false), "Wayland compositor");
    const executable = path.join(directory, "RayleaLauncher");
    await fs.copyFile(process.env.RAYLEA_NATIVE_LAUNCHER || path.resolve("dist/package/linux-unpacked/RayleaLauncher"), executable);
    await fs.chmod(executable, 0o755);
    start(executable, [], environment);
    await waitFor(() => /xdg_toplevel[^\n]*\.set_app_id\("local\.rayleabot\.launcher"\)/.test(output), "Raylea Wayland app_id");
    const desktopFile = path.join(environment.XDG_DATA_HOME, "applications/local.rayleabot.launcher.desktop");
    await execute("desktop-file-validate", [desktopFile]);
    assert.deepEqual(await fs.readFile(path.join(environment.XDG_DATA_HOME, "rayleabot/launcher-icon.png")), await fs.readFile("assets/appicon.png"));
    const entry = await fs.readFile(desktopFile, "utf8");
    assert.ok(entry.includes(`Icon=${environment.XDG_DATA_HOME}/rayleabot/launcher-icon.png\n`));
    console.log(output.split("\n").filter(line => line.includes(".set_app_id(")).join("\n"));
  } catch (error) {
    console.error(output.slice(-65536));
    throw error;
  } finally {
    for (const { child, exited } of children.reverse()) {
      if (child.exitCode === null && child.signalCode === null) {
        child.kill("SIGTERM");
        await Promise.race([exited, delay(2000)]);
        if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
      }
      await exited;
    }
    if (path.dirname(directory) !== path.resolve(os.tmpdir()) || !path.basename(directory).startsWith("raylea-wayland-")) {
      throw new Error("Unexpected native Wayland fixture directory");
    }
    await fs.rm(directory, { recursive: true, force: true });
  }
});
