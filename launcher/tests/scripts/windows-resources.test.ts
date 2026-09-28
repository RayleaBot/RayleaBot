import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test, vi } from "vitest";
import { ensureWindowsResources } from "../../scripts/generate-windows-resources.mjs";

const temporaryRoots: string[] = [];

afterEach(async () => {
  for (const directory of temporaryRoots.splice(0)) {
    if (path.dirname(directory) !== path.resolve(os.tmpdir()) || !path.basename(directory).startsWith("raylea-windows-resources-")) {
      throw new Error("Unexpected resource fixture directory");
    }
    await fs.rm(directory, { recursive: true, force: true });
  }
});

async function fixture() {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-windows-resources-"));
  temporaryRoots.push(directory);
  const launcherRoot = path.join(directory, "launcher");
  const assets = path.join(launcherRoot, "assets");
  await fs.mkdir(assets, { recursive: true });
  await fs.writeFile(path.join(assets, "icon.ico"), "icon-v1");
  await fs.writeFile(path.join(assets, "windows.manifest"), "manifest-v1");
  const generate = vi.fn(async (args: string[]) => {
    const output = path.join(launcherRoot, args[args.indexOf("-out") + 1]);
    const icon = await fs.readFile(path.join(launcherRoot, args[args.indexOf("-icon") + 1]), "utf8");
    const manifest = await fs.readFile(path.join(launcherRoot, args[args.indexOf("-manifest") + 1]), "utf8");
    await fs.writeFile(output, `${args[args.indexOf("-arch") + 1]}:${icon}:${manifest}`);
    return 0;
  });
  return { directory, assets, launcherRoot, generate, platform: "win32", arch: "amd64", cacheDirectory: path.join(directory, "cache") };
}

describe("Windows launcher resource generation", () => {
  test.each(["darwin", "linux"])("does not request Windows tooling on %s", async (platform) => {
    const generate = vi.fn();
    const resolveArchitecture = vi.fn();
    expect(await ensureWindowsResources({ platform, generate, resolveArchitecture })).toBe(false);
    expect(resolveArchitecture).not.toHaveBeenCalled();
    expect(generate).not.toHaveBeenCalled();
  });

  test("rebuilds stale resources when ICO or manifest contents change", async () => {
    const options = await fixture();
    const output = path.join(options.launcherRoot, "rsrc_windows_amd64.syso");
    await fs.writeFile(output, "old-icon-resources");
    expect(await ensureWindowsResources(options)).toBe(true);
    expect(await fs.readFile(output, "utf8")).toBe("amd64:icon-v1:manifest-v1");
    expect(await ensureWindowsResources(options)).toBe(false);

    const icon = path.join(options.assets, "icon.ico");
    const previous = await fs.stat(icon);
    await fs.writeFile(icon, "icon-v2");
    await fs.utimes(icon, previous.atime, previous.mtime);
    expect(await ensureWindowsResources(options)).toBe(true);
    expect(await fs.readFile(output, "utf8")).toBe("amd64:icon-v2:manifest-v1");

    await fs.writeFile(path.join(options.assets, "windows.manifest"), "manifest-v2");
    expect(await ensureWindowsResources(options)).toBe(true);
    expect(await fs.readFile(output, "utf8")).toBe("amd64:icon-v2:manifest-v2");
  });

  test("restores a missing generated resource even when inputs are unchanged", async () => {
    const options = await fixture();
    const output = path.join(options.launcherRoot, "rsrc_windows_amd64.syso");
    await ensureWindowsResources(options);
    await fs.unlink(output);
    expect(await ensureWindowsResources(options)).toBe(true);
    expect(await fs.readFile(output, "utf8")).toBe("amd64:icon-v1:manifest-v1");
  });

  test("targets the architecture resolved from Go", async () => {
    const options = await fixture();
    expect(await ensureWindowsResources({ ...options, arch: undefined, resolveArchitecture: async () => "arm64" })).toBe(true);
    expect(await fs.readFile(path.join(options.launcherRoot, "rsrc_windows_arm64.syso"), "utf8")).toBe("arm64:icon-v1:manifest-v1");
  });

  test("does not cache a failed resource build", async () => {
    const options = await fixture();
    await expect(ensureWindowsResources({ ...options, generate: async () => 7 })).rejects.toThrow("exited with code 7");
    expect(await ensureWindowsResources(options)).toBe(true);
  });

  test("rejects success without an output and unsupported architectures", async () => {
    const options = await fixture();
    await expect(ensureWindowsResources({ ...options, generate: async () => 0 })).rejects.toThrow("expected output");
    await expect(ensureWindowsResources({ ...options, arch: "riscv64" })).rejects.toThrow("Unsupported Windows architecture");
  });
});
