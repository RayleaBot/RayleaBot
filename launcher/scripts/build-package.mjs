import { spawn, execFileSync } from "node:child_process";
import fs from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import { createProcessInvocation } from "./process-invocation.mjs";
import { resolveMacBundleVersion } from "./package-metadata.mjs";
import { runWails } from "./run-go.mjs";

const root = path.resolve(import.meta.dirname, "..");
const packageRoot = path.join(root, "dist", "package");

async function run(command, args) {
  const invocation = createProcessInvocation(command, args);
  const child = spawn(invocation.command, invocation.args, {
    cwd: root,
    env: { ...process.env, ...(command === "go" ? { GOWORK: "off" } : {}) },
    stdio: "inherit",
    shell: false,
  });
  const code = await new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (exitCode) => resolve(exitCode ?? 1));
  });
  if (code !== 0) {
    throw new Error(`${command} ${args.join(" ")} exited with code ${code}`);
  }
}

await run(process.execPath, ["../scripts/generate-launcher-icons.mjs", "--check"]);
await run(process.execPath, ["scripts/build-app.mjs"]);
if (process.platform === "win32") {
  const arch = execFileSync("go", ["env", "GOARCH"], { cwd: root, env: { ...process.env, GOWORK: "off" }, encoding: "utf8" }).trim();
  if (!["amd64", "arm64", "386"].includes(arch)) throw new Error(`Unsupported Windows architecture: ${arch}`);
  const code = await runWails(["generate", "syso", "-manifest", "assets/windows.manifest", "-icon", "assets/icon.ico", "-arch", arch, "-out", `rsrc_windows_${arch}.syso`]);
  if (code !== 0) throw new Error(`Wails resource generation exited with code ${code}`);
}

let output;
if (process.platform === "win32") {
  output = path.join(packageRoot, "win-unpacked", "RayleaLauncher.exe");
} else if (process.platform === "darwin") {
  const packageMetadata = JSON.parse(await fs.readFile(path.join(root, "package.json"), "utf8"));
  const bundleVersion = resolveMacBundleVersion({
    releaseTag: process.env.GITHUB_REF_NAME,
    packageVersion: packageMetadata.version,
  });
  const bundleRoot = path.join(packageRoot, process.arch === "arm64" ? "mac-arm64" : "mac", "RayleaLauncher.app", "Contents");
  output = path.join(bundleRoot, "MacOS", "RayleaLauncher");
  await fs.mkdir(path.join(bundleRoot, "MacOS"), { recursive: true });
  await fs.writeFile(
    path.join(bundleRoot, "Info.plist"),
    `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>RayleaLauncher</string>
<key>CFBundleIdentifier</key><string>local.rayleabot.launcher</string>
<key>CFBundleName</key><string>RayleaLauncher</string>
<key>CFBundleDisplayName</key><string>RayleaBot 启动器</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>${bundleVersion}</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`,
    "utf8",
  );
} else {
  output = path.join(packageRoot, "linux-unpacked", "RayleaLauncher");
}

await fs.mkdir(path.dirname(output), { recursive: true });
const ldflags = process.platform === "win32" ? "-w -s -H windowsgui" : "-w -s";
const buildTags = process.platform === "linux" ? "production,gtk3" : "production";
await run("go", ["build", "-tags", buildTags, "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", output, "."]);

const stat = await fs.stat(output);
if (!stat.isFile() || stat.size === 0) {
  throw new Error(`Wails launcher output was not created: ${output}`);
}
if (process.platform !== "win32") {
  await fs.chmod(output, 0o755);
}
if (process.platform === "win32") {
  await fs.copyFile(
    path.resolve(root, "..", "docs", "release", "windows-desktop-runtime.md"),
    path.join(path.dirname(output), "WINDOWS-RUNTIME.md"),
  );
} else if (process.platform === "linux") {
  await fs.copyFile(
    path.resolve(root, "..", "docs", "release", "linux-desktop-runtime.md"),
    path.join(path.dirname(output), "LINUX-RUNTIME.md"),
  );
}
