import fs from "node:fs/promises";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const distPath = path.join(root, "dist");
const frontendDistPath = path.join(root, "internal", "frontend", "dist");

await fs.rm(distPath, { recursive: true, force: true });
await fs.mkdir(frontendDistPath, { recursive: true });
for (const entry of await fs.readdir(frontendDistPath, { withFileTypes: true })) {
  if (entry.name !== ".gitkeep") {
    await fs.rm(path.join(frontendDistPath, entry.name), { recursive: true, force: true });
  }
}
