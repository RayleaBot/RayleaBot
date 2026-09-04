import fs from "node:fs/promises";
import path from "node:path";
import { randomUUID, createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const setupCredential = /(\/setup#setup_token=)[A-Za-z0-9_%.-]{20,}/g;
export function redactHistoricalText(text) {
  let replacements = 0;
  const output = text.replace(setupCredential, (_, prefix) => { replacements++; return prefix + "[REDACTED]"; });
  return { output, replacements };
}

// Only confirmed setup URL credentials are replaced. All other bytes are retained.
export async function redactHistoricalLogs(root, apply = false) {
  root = await fs.realpath(root);
  const results = [];
  async function visit(directory) {
    for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
      if (entry.isSymbolicLink()) continue;
      const target = path.join(directory, entry.name);
      if (entry.isDirectory()) { await visit(target); continue; }
      if (!entry.isFile() || !/\.(log|jsonl|json)$/i.test(entry.name)) continue;
      const original = await fs.readFile(target);
      const { output, replacements } = redactHistoricalText(original.toString("latin1"));
      if (!replacements) continue;
      const sanitized = Buffer.from(output, "latin1");
      const digest = (buffer) => createHash("sha256").update(buffer).digest("hex");
      if (apply) {
        const stat = await fs.stat(target);
        const temporary = target + ".redacted-" + randomUUID();
        try {
          await fs.writeFile(temporary, sanitized, { flag: "wx", mode: stat.mode });
          if (digest(await fs.readFile(target)) !== digest(original)) throw new Error(`日志仍在写入，未替换：${path.relative(root, target)}`);
          await fs.rename(temporary, target);
          await fs.utimes(target, stat.atime, stat.mtime);
          if (digest(await fs.readFile(target)) !== digest(sanitized)) throw new Error("脱敏结果校验失败");
        } finally {
          await fs.rm(temporary, { force: true });
        }
      }
      results.push({ path: path.relative(root, target), replacements, lines: original.filter((byte) => byte === 10).length, applied: apply });
    }
  }
  await visit(root);
  return results;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const rootIndex = process.argv.indexOf("--root");
  const root = rootIndex >= 0 ? process.argv[rootIndex + 1] : path.resolve("logs");
  console.log(JSON.stringify(await redactHistoricalLogs(root, process.argv.includes("--apply")), null, 2));
}
