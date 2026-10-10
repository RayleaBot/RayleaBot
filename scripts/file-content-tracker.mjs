import { createHash } from "node:crypto";
import fsp from "node:fs/promises";
import path from "node:path";

export function createFileContentTracker({
  readFile = fsp.readFile,
  stat = fsp.stat,
} = {}) {
  const fingerprints = new Map();
  const pendingOperations = new Map();

  const runSerialized = (sourcePath, operation) => {
    const sourceKey = path.resolve(sourcePath);
    const previous = pendingOperations.get(sourceKey) ?? Promise.resolve();
    const current = previous.catch(() => undefined).then(() => operation(sourceKey));
    pendingOperations.set(sourceKey, current);
    return current.finally(() => {
      if (pendingOperations.get(sourceKey) === current) {
        pendingOperations.delete(sourceKey);
      }
    });
  };

  return {
    prime(sourcePath) {
      return runSerialized(sourcePath, async (sourceKey) => {
        const snapshot = await readFileContentSnapshot(sourceKey, undefined, { readFile, stat });
        if (snapshot === null) {
          fingerprints.delete(sourceKey);
        } else {
          fingerprints.set(sourceKey, snapshot);
        }
      });
    },

    hasChanged(sourcePath) {
      return runSerialized(sourcePath, async (sourceKey) => {
        const wasTracked = fingerprints.has(sourceKey);
        const previousSnapshot = fingerprints.get(sourceKey);
        const currentSnapshot = await readFileContentSnapshot(sourceKey, previousSnapshot, { readFile, stat });
        if (currentSnapshot === null) {
          fingerprints.delete(sourceKey);
          return wasTracked;
        }
        fingerprints.set(sourceKey, currentSnapshot);
        return !wasTracked || previousSnapshot.digest !== currentSnapshot.digest;
      });
    },
  };
}

async function readMetadata(sourcePath, stat) {
  try {
    const sourceStat = await stat(sourcePath, { bigint: true });
    if (!sourceStat.isFile()) {
      return null;
    }
    return {
      // ctime detects same-size edits with restored mtime; inode/device detect
      // replacement. Nanoseconds avoid rounding rapid writes to milliseconds.
      metadata: [sourceStat.dev, sourceStat.ino, sourceStat.size, sourceStat.mtimeNs, sourceStat.ctimeNs, sourceStat.mode].join(":"),
      mode: Number(sourceStat.mode & 0o111n),
    };
  } catch (error) {
    if (["ENOENT", "ENOTDIR"].includes(error?.code)) {
      return null;
    }
    throw error;
  }
}

export async function readFileContentSnapshot(sourcePath, previous, { readFile = fsp.readFile, stat = fsp.stat } = {}) {
  for (let attempt = 0; attempt < 3; attempt++) {
    const before = await readMetadata(sourcePath, stat);
    if (before === null) return null;
    if (previous?.metadata === before.metadata && /^[a-f0-9]{64}$/.test(previous.digest) && previous.mode === before.mode) return previous;
    try {
      const content = await readFile(sourcePath);
      const after = await readMetadata(sourcePath, stat);
      if (after?.metadata !== before.metadata) continue;
      return { ...before, digest: createHash("sha256").update(content).digest("hex") };
    } catch (error) {
      if (["ENOENT", "ENOTDIR"].includes(error?.code)) return null;
      throw error;
    }
  }
  throw Object.assign(new Error(`${sourcePath}: changed while reading build input`), { code: "DEV_INPUT_CHANGED" });
}
