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
        const snapshot = await readSnapshot(sourceKey, { readFile, stat });
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
        const currentMetadata = await readMetadata(sourceKey, stat);
        if (currentMetadata === null) {
          fingerprints.delete(sourceKey);
          return wasTracked;
        }
        if (wasTracked && sameContentMetadata(previousSnapshot, currentMetadata)) {
          return false;
        }
        const currentSnapshot = await readSnapshot(sourceKey, { readFile, stat });
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
    const sourceStat = await stat(sourcePath);
    if (!sourceStat.isFile()) {
      return null;
    }
    return {
      size: sourceStat.size,
      mtimeMs: sourceStat.mtimeMs,
      ctimeMs: sourceStat.ctimeMs,
    };
  } catch (error) {
    if (error?.code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

async function readSnapshot(sourcePath, { readFile, stat }) {
  const metadata = await readMetadata(sourcePath, stat);
  if (metadata === null) {
    return null;
  }
  try {
    const content = await readFile(sourcePath);
    return {
      ...metadata,
      digest: createHash("sha256").update(content).digest("hex"),
    };
  } catch (error) {
    if (error?.code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

function sameContentMetadata(left, right) {
  return left.size === right.size
    && left.mtimeMs === right.mtimeMs
    && left.ctimeMs === right.ctimeMs;
}
