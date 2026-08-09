import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { createFileContentTracker } from "../file-content-tracker.mjs";

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
