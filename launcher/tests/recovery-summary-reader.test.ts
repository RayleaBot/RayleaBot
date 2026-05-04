import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { NodeRecoverySummaryReader } from "../src/main/services/recovery-summary-reader";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

async function writeSummary(dir: string, payload: unknown) {
  await fs.mkdir(dir, { recursive: true });
  await fs.writeFile(path.join(dir, "recovery-summary.json"), JSON.stringify(payload), "utf8");
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("NodeRecoverySummaryReader", () => {
  it("reads a valid local recovery summary", async () => {
    const dir = await createTempDir("recovery");
    await writeSummary(dir, {
      status: "degraded",
      phase: "post_startup",
      operation: "restore",
      created_at: "2026-04-01T00:00:00Z",
      updated_at: "2026-04-01T00:05:00Z",
      requires_post_start_checks: true,
      manual_actions: ["检查插件 weather"],
      skipped_plugins: [
        {
          plugin_id: "weather",
          version: "1.0.0",
          reason_code: "plugin.version_incompatible",
          summary: "插件版本不兼容",
          review_id: "review_weather",
          review_status: "pending",
        },
      ],
    });

    await expect(new NodeRecoverySummaryReader().read(dir)).resolves.toEqual({
      status: "degraded",
      phase: "post_startup",
      operation: "restore",
      created_at: "2026-04-01T00:00:00Z",
      updated_at: "2026-04-01T00:05:00Z",
      requires_post_start_checks: true,
      manual_actions: ["检查插件 weather"],
      skipped_plugins: [
        {
          plugin_id: "weather",
          version: "1.0.0",
          reason_code: "plugin.version_incompatible",
          summary: "插件版本不兼容",
          review_id: "review_weather",
          review_status: "pending",
        },
      ],
    });
  });

  it("ignores missing files, invalid JSON, missing required fields, and unknown enums", async () => {
    const missingDir = await createTempDir("recovery");
    await expect(new NodeRecoverySummaryReader().read(missingDir)).resolves.toBeNull();

    const invalidJsonDir = await createTempDir("recovery");
    await fs.writeFile(path.join(invalidJsonDir, "recovery-summary.json"), "{", "utf8");
    await expect(new NodeRecoverySummaryReader().read(invalidJsonDir)).resolves.toBeNull();

    const missingFieldDir = await createTempDir("recovery");
    await writeSummary(missingFieldDir, {
      status: "pending",
      phase: "pre_restore",
      operation: "restore",
      created_at: "2026-04-01T00:00:00Z",
    });
    await expect(new NodeRecoverySummaryReader().read(missingFieldDir)).resolves.toBeNull();

    const unknownEnumDir = await createTempDir("recovery");
    await writeSummary(unknownEnumDir, {
      status: "paused",
      phase: "pre_restore",
      operation: "restore",
      created_at: "2026-04-01T00:00:00Z",
      updated_at: "2026-04-01T00:05:00Z",
    });
    await expect(new NodeRecoverySummaryReader().read(unknownEnumDir)).resolves.toBeNull();
  });
});
