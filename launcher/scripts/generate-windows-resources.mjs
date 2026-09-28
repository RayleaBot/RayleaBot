import { execFile } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";
import { createBuildCache } from "../../scripts/dev-build-cache.mjs";
import { createProcessInvocation } from "../../scripts/process-invocation.mjs";
import { runWails, wailsVersion } from "./run-go.mjs";

const root = path.resolve(import.meta.dirname, "..");
const execute = promisify(execFile);

async function currentArchitecture(launcherRoot) {
  const invocation = createProcessInvocation("go", ["env", "GOARCH"]);
  const result = await execute(invocation.command, invocation.args, {
    cwd: launcherRoot,
    env: { ...process.env, GOWORK: "off" },
    encoding: "utf8",
    windowsHide: true,
  });
  return result.stdout.trim();
}

export async function ensureWindowsResources({
  platform = process.platform,
  launcherRoot = root,
  arch,
  resolveArchitecture = currentArchitecture,
  generate = runWails,
  cacheDirectory = path.resolve(launcherRoot, "../.tmp/dev-cache"),
} = {}) {
  if (platform !== "win32") return false;
  arch ??= await resolveArchitecture(launcherRoot);
  if (!["amd64", "arm64", "386"].includes(arch)) {
    throw new Error(`Unsupported Windows architecture: ${arch}`);
  }
  const filename = `rsrc_windows_${arch}.syso`;
  return createBuildCache(cacheDirectory).run(`launcher-windows-resources-${arch}`, {
    inputs: async () => [
      path.join(launcherRoot, "assets/icon.ico"),
      path.join(launcherRoot, "assets/windows.manifest"),
      path.join(launcherRoot, "go.mod"),
      path.join(launcherRoot, "go.sum"),
      path.join(launcherRoot, "scripts/run-go.mjs"),
      import.meta.filename,
    ],
    identity: { arch, wailsVersion },
    outputs: [path.join(launcherRoot, filename)],
    build: async () => {
      const code = await generate([
        "generate", "syso", "-manifest", "assets/windows.manifest",
        "-icon", "assets/icon.ico", "-arch", arch, "-out", filename,
      ]);
      if (code !== 0) throw new Error(`Wails resource generation exited with code ${code}`);
    },
  });
}

if (path.resolve(process.argv[1] ?? "") === import.meta.filename) {
  const generated = await ensureWindowsResources();
  if (process.platform === "win32") {
    console.log(generated ? "Windows launcher icon resources generated." : "Windows launcher icon resources are current.");
  }
}
