import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { createFileContentTracker, readFileContentSnapshot } from "../file-content-tracker.mjs";
import { createHash } from "node:crypto";

test("distinguishes file content changes from metadata-only events", async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-file-content-tracker-"));
  const sourcePath = path.join(root, "source.go");
  await fs.writeFile(sourcePath, "package source\n", "utf8");
  t.after(() => fs.rm(root, { recursive: true, force: true }));

  let contentReads = 0;
  const tracker = createFileContentTracker({
    readFile: async (...args) => {
      contentReads += 1;
      return fs.readFile(...args);
    },
  });
  await tracker.prime(sourcePath);

  assert.equal(await tracker.hasChanged(sourcePath), false);
  assert.equal(contentReads, 1);

  await fs.writeFile(sourcePath, "package source\n", "utf8");
  await fs.utimes(sourcePath, 1, 1);
  assert.equal(await tracker.hasChanged(sourcePath), false);
  assert.equal(contentReads, 2);

  await fs.writeFile(sourcePath, "package target\n", "utf8");
  assert.equal(await tracker.hasChanged(sourcePath), true);
  assert.equal(await tracker.hasChanged(sourcePath), false);

  await fs.rm(sourcePath);
  assert.equal(await tracker.hasChanged(sourcePath), true);
  assert.equal(await tracker.hasChanged(sourcePath), false);

  await fs.writeFile(sourcePath, "package target\n", "utf8");
  assert.equal(await tracker.hasChanged(sourcePath), true);
});

test("a file replaced during hashing cannot cache a digest for the newer metadata", async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-snapshot-race-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const source = path.join(root, "source");
  await fs.writeFile(source, "first");
  let reads = 0;
  const snapshot = await readFileContentSnapshot(source, undefined, { readFile: async (file) => {
    const bytes = await fs.readFile(file);
    if (++reads === 1) await fs.writeFile(file, "other");
    return bytes;
  } });
  assert.equal(reads, 2);
  assert.equal(snapshot.digest, createHash("sha256").update("other").digest("hex"));
});
