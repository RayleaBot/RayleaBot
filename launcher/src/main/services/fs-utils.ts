import fs from "node:fs/promises";

export async function pathExists(targetPath: string) {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
}

export async function fileExists(targetPath: string) {
  try {
    const stat = await fs.stat(targetPath);
    return stat.isFile();
  } catch {
    return false;
  }
}

export async function directoryHasFiles(targetPath: string) {
  try {
    const entries = await fs.readdir(targetPath);
    return entries.length > 0;
  } catch {
    return false;
  }
}
