import fs from "node:fs/promises";
import path from "node:path";

const root = path.resolve(import.meta.dirname, "..");
const distPath = path.join(root, "dist");

await fs.rm(distPath, { recursive: true, force: true });
