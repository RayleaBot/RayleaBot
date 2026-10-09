import fs from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";

const openedHandler = `\tcase "opened":
\t\tif s.parent.clickHandler != nil {
\t\t\ts.parent.clickHandler()
\t\t}
\t\tif s.parent.onMenuOpen != nil {`;

export function patchLinuxTraySource(source, version) {
  if (version !== "v3.0.0-beta.9") {
    throw new Error(`Review the Linux tray event patch for Wails ${version}`);
  }
  const normalized = source.replaceAll("\r\n", "\n");
  if (normalized.split(openedHandler).length !== 2) {
    throw new Error("Wails Linux tray event source no longer matches the compatibility patch");
  }
  // DBusMenu's opened event is a menu lifecycle notification, not tray activation.
  return normalized.replace(openedHandler, `\tcase "opened":
\t\tif s.parent.onMenuOpen != nil {`);
}

export async function createWailsLinuxCompatibility({ moduleDirectory, version, launcherDirectory, cacheDirectory }) {
  const original = path.join(moduleDirectory, "pkg", "application", "systemtray_linux.go");
  const patched = patchLinuxTraySource(await fs.readFile(original, "utf8"), version);
  const goMod = await fs.readFile(path.join(launcherDirectory, "go.mod"), "utf8");
  const goSum = await fs.readFile(path.join(launcherDirectory, "go.sum"), "utf8");
  const key = createHash("sha256").update(original).update(patched).update(goMod).update(goSum).digest("hex");
  const directory = path.join(cacheDirectory, key);
  const modfile = path.join(directory, "launcher.mod");
  if (await fs.stat(modfile).then(() => true, () => false)) return modfile;
  await fs.mkdir(cacheDirectory, { recursive: true });
  // Go forbids overlays in GOMODCACHE. Keep the frozen module intact and build a private copy.
  const staging = await fs.mkdtemp(path.join(cacheDirectory, "staging-"));
  try {
    await fs.cp(moduleDirectory, path.join(staging, "wails"), { recursive: true });
    const replacement = path.join(staging, "wails", "pkg", "application", "systemtray_linux.go");
    await fs.chmod(replacement, 0o644);
    await fs.writeFile(replacement, patched);
    await fs.writeFile(path.join(staging, "launcher.mod"), `${goMod.trimEnd()}\n\nreplace github.com/wailsapp/wails/v3 => ${JSON.stringify(path.join(directory, "wails"))}\n`);
    await fs.writeFile(path.join(staging, "launcher.sum"), goSum);
    // Publish the module and its modfile together, including when build and vet run concurrently.
    await fs.rename(staging, directory).catch((error) => {
      if (!["EEXIST", "ENOTEMPTY"].includes(error.code)) throw error;
    });
  } finally {
    await removeStaging(staging, cacheDirectory);
  }
  return modfile;
}

async function removeStaging(directory, cacheDirectory) {
  if (path.dirname(directory) !== path.resolve(cacheDirectory) || !path.basename(directory).startsWith("staging-")) {
    throw new Error("Unexpected Wails compatibility staging directory");
  }
  async function makeDirectoriesWritable(current) {
    await fs.chmod(current, 0o755);
    for (const entry of await fs.readdir(current, { withFileTypes: true })) {
      if (entry.isDirectory()) await makeDirectoriesWritable(path.join(current, entry.name));
    }
  }
  try {
    await makeDirectoriesWritable(directory);
    await fs.rm(directory, { recursive: true, force: true });
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
