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
async function command(name, args) {
  return (await execute(name, args, { timeout: 3000, env: { ...process.env, LC_ALL: "C" } })).stdout.trim();
}

test("Linux tray menu lifecycle preserves window visibility and left activation toggles it", {
  skip: process.platform !== "linux", timeout: 30000,
}, async () => {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-tray-events-"));
  const executable = path.join(directory, "RayleaLauncher");
  const source = process.env.RAYLEA_NATIVE_LAUNCHER || path.resolve("dist/package/linux-unpacked/RayleaLauncher");
  let child;
  let exited;
  let output = "";
  try {
    await fs.copyFile(source, executable);
    await fs.chmod(executable, 0o755);
    child = spawn(executable, [], { cwd: directory, stdio: ["ignore", "pipe", "pipe"] });
    exited = once(child, "exit");
    child.stdout.on("data", (data) => { output = (output + data).slice(-65536); });
    child.stderr.on("data", (data) => { output = (output + data).slice(-65536); });
    const service = `org.kde.StatusNotifierItem-${child.pid}-1`;
    const call = (object, iface, method, signature, ...args) => command("busctl", [
      "--user", "call", "--", service, object, iface, method, signature, ...args.map(String),
    ]);
    async function waitFor(probe, label) {
      const deadline = Date.now() + 15000;
      while (Date.now() < deadline) {
        assert.equal(child.exitCode, null, `Launcher exited while waiting for ${label}: ${output}`);
        if (await probe()) return;
        await delay(100);
      }
      assert.fail(`Timed out waiting for ${label}: ${output}`);
    }
    let window;
    await waitFor(async () => {
      const ids = await command("xdotool", ["search", "--onlyvisible", "--pid", String(child.pid)]).catch(() => "");
      for (const id of ids.split("\n").filter(Boolean)) {
        const info = await command("xwininfo", ["-id", id]);
        if (Number(info.match(/Width: (\d+)/)?.[1]) >= 400 && Number(info.match(/Height: (\d+)/)?.[1]) >= 300) {
          window = id;
          return true;
        }
      }
      return false;
    }, "launcher window");
    const visible = async () => /Map State: IsViewable/.test(await command("xwininfo", ["-id", window]));
    const activate = () => call("/StatusNotifierItem", "org.kde.StatusNotifierItem", "Activate", "ii", 0, 0);
    const menuEvent = (name) => call("/StatusNotifierMenu", "com.canonical.dbusmenu", "Event", "isvu", 0, name, "i", 0, 0);
    const menuPath = await command("busctl", ["--user", "get-property", service, "/StatusNotifierItem", "org.kde.StatusNotifierItem", "Menu"]);
    assert.equal(menuPath, 'o "/StatusNotifierMenu"');
    await call("/StatusNotifierMenu", "com.canonical.dbusmenu", "GetLayout", "iias", 0, -1, 0);
    for (const expected of [true, false]) {
      assert.equal(await visible(), expected);
      for (const event of ["opened", "closed", "opened", "closed"]) {
        await menuEvent(event);
        await delay(300);
        assert.equal(await visible(), expected, `${event} must not change launcher visibility (${expected})`);
      }
      await activate();
      await waitFor(async () => await visible() !== expected, "left activation to toggle window");
    }
  } catch (error) {
    console.error(output);
    throw error;
  } finally {
    if (child && child.exitCode === null) {
      child.kill("SIGTERM");
      await Promise.race([exited, delay(2000)]);
      if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
      await exited;
    }
    if (path.dirname(directory) !== path.resolve(os.tmpdir()) || !path.basename(directory).startsWith("raylea-tray-events-")) {
      throw new Error("Unexpected native tray fixture directory");
    }
    await fs.rm(directory, { recursive: true, force: true });
  }
});
