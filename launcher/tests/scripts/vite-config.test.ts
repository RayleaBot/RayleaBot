import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { afterEach, describe, expect, test } from "vitest";

import { createPreserveWailsEmbedPlaceholderPlugin } from "../../scripts/vite-placeholder";

const temporaryDirectories: string[] = [];

afterEach(async () => {
  await Promise.all(temporaryDirectories.splice(0).map((directory) => fs.rm(directory, { recursive: true, force: true })));
});

describe("Wails frontend embed placeholder", () => {
  test("is restored when the renderer build fails", async () => {
    const outputDirectory = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-launcher-vite-"));
    temporaryDirectories.push(outputDirectory);
    const plugin = createPreserveWailsEmbedPlaceholderPlugin(outputDirectory);

    await plugin.buildEnd(new Error("renderer build failed"));

    await expect(fs.readFile(path.join(outputDirectory, ".gitkeep"), "utf8")).resolves.toBe("\n");
  });
});
