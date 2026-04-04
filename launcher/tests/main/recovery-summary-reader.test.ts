import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, test } from "vitest";
import { NodeRecoverySummaryReader } from "@main/services/recovery-summary-reader";

const tempRoots: string[] = [];

async function createTempDir(label: string) {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), `raylea-${label}-`));
  tempRoots.push(tempRoot);
  return tempRoot;
}

afterEach(async () => {
  await Promise.all(
    tempRoots.splice(0).map(async (target) => {
      await fs.rm(target, { recursive: true, force: true });
    }),
  );
});

describe("NodeRecoverySummaryReader", () => {
  test("returns null when the recovery summary file is missing", async () => {
    const reader = new NodeRecoverySummaryReader();
    const logDirectory = await createTempDir("recovery-summary-missing");

    await expect(reader.read(logDirectory)).resolves.toBeNull();
  });

  test("returns null when the recovery summary payload is not valid json", async () => {
    const reader = new NodeRecoverySummaryReader();
    const logDirectory = await createTempDir("recovery-summary-invalid");

    await fs.writeFile(path.join(logDirectory, "recovery-summary.json"), "{not valid json", "utf8");

    await expect(reader.read(logDirectory)).resolves.toBeNull();
  });

  test("reads a valid recovery summary payload", async () => {
    const reader = new NodeRecoverySummaryReader();
    const logDirectory = await createTempDir("recovery-summary-valid");
    const summary = {
      status: "degraded",
      phase: "post_startup",
      operation: "upgrade",
      created_at: "2026-04-03T00:00:00Z",
      updated_at: "2026-04-03T00:01:00Z",
      manual_actions: ["检查插件兼容性。"],
      skipped_plugins: [
        {
          plugin_id: "weather-pro",
          reason_code: "plugin.min_core_version",
          summary: "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
          review_id: "review_weather_pro",
          review_status: "pending",
        },
      ],
      audit: [
        {
          task_id: "task_recovery_confirm_0001",
          created_at: "2026-04-03T00:02:00Z",
          operator_id: "alice",
          note: "已确认当前跳过状态。",
          items: [
            {
              review_id: "review_weather_pro",
              plugin_id: "weather-pro",
              reason_code: "plugin.min_core_version",
              summary: "插件最低 core 版本要求不满足，已保留安装目录并跳过自动启用。",
            },
          ],
        },
      ],
    } as const;

    await fs.writeFile(path.join(logDirectory, "recovery-summary.json"), JSON.stringify(summary), "utf8");

    await expect(reader.read(logDirectory)).resolves.toEqual(summary);
  });
});
