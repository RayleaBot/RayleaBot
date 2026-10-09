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

export async function createWailsLinuxOverlay({ moduleDirectory, version, cacheDirectory }) {
  const original = path.join(moduleDirectory, "pkg", "application", "systemtray_linux.go");
  const patched = patchLinuxTraySource(await fs.readFile(original, "utf8"), version);
  const key = createHash("sha256").update(original).update(patched).digest("hex");
  const directory = path.join(cacheDirectory, key);
  await fs.mkdir(directory, { recursive: true });
  const replacement = path.join(directory, "systemtray_linux.go");
  const overlay = path.join(directory, "overlay.json");
  // Publish complete files atomically when concurrent build and vet processes share this cache.
  const staging = await fs.mkdtemp(path.join(directory, "staging-"));
  try {
    await fs.writeFile(path.join(staging, "source.go"), patched);
    await fs.writeFile(path.join(staging, "overlay.json"), JSON.stringify({ Replace: { [original]: replacement } }));
    await fs.rename(path.join(staging, "source.go"), replacement);
    await fs.rename(path.join(staging, "overlay.json"), overlay);
  } finally {
    await fs.rm(path.join(staging, "source.go"), { force: true });
    await fs.rm(path.join(staging, "overlay.json"), { force: true });
    await fs.rmdir(staging);
  }
  return overlay;
}
