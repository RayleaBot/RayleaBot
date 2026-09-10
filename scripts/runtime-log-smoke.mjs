import { LAUNCHER_CONTROL_TOKEN_HEADER } from "./start-dev-support.mjs";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import fs from "node:fs";
import fsp from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRedactedOutput } from "./log-redaction.mjs";
import { resolveBackendBaseUrl } from "./start-dev-support.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const duration = Number(process.argv[2] ?? 1800);
if (!Number.isSafeInteger(duration) || duration < 1) throw new Error("duration must be positive seconds");
const base = await resolveBackendBaseUrl({ rootDir: root, env: process.env });
try {
  await fetch(`${base}/healthz`, { signal: AbortSignal.timeout(1000) });
  throw new Error("refusing to replace an already running server");
} catch (error) {
  if (error.message === "refusing to replace an already running server") throw error;
}
const stamp = new Date().toISOString().replace(/[:.]/g, "-");
const directory = path.join(root, "logs", "dev", "validation");
await fsp.mkdir(directory, { recursive: true });
const logPath = path.join(directory, `${stamp}.log`);
const reportPath = path.join(directory, `${stamp}.json`);
const log = fs.createWriteStream(logPath, { flags: "wx" });
const controlToken = randomBytes(32).toString("base64url");
const child = spawn(path.join(root, "server", "dist", process.platform === "win32" ? "raylea-server.exe" : "raylea-server"), ["-config", path.join(root, "config", "user.yaml")], {
  cwd: root, windowsHide: true, stdio: ["ignore", "pipe", "pipe"],
  env: { ...process.env, RAYLEA_LAUNCHER_CONTROL_TOKEN: controlToken },
});
const report = { startedAt: new Date().toISOString(), pid: child.pid, durationSeconds: duration, logPath, health: [], levels: {}, warningCodes: {}, messages: { inbound: 0, outbound: 0 }, completed: false };
const exited = new Promise((resolve) => child.once("exit", (code, signal) => resolve({ code, signal })));
const output = createRedactedOutput((line) => {
  log.write(line);
  try {
    const entry = JSON.parse(line);
    const level = String(entry.level ?? "unknown").toLowerCase();
    report.levels[level] = (report.levels[level] ?? 0) + 1;
    if (entry.direction in report.messages) report.messages[entry.direction]++;
    if (level === "warn" || level === "error") {
      const key = `${entry.component ?? "server"}:${entry.error_code ?? "unspecified"}`;
      report.warningCodes[key] = (report.warningCodes[key] ?? 0) + 1;
    }
  } catch { /* Plain stderr remains in the redacted diagnostic file. */ }
});
const stderr = createRedactedOutput((line) => log.write(line));
child.stdout.on("data", (chunk) => output.write(chunk));
child.stderr.on("data", (chunk) => stderr.write(chunk));
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
let stopped = false;
async function stop() {
  if (stopped || child.exitCode !== null || child.signalCode !== null) return;
  stopped = true;
  try {
    const response = await fetch(`${base}/api/launcher/shutdown`, { method: "POST", headers: { [LAUNCHER_CONTROL_TOKEN_HEADER]: controlToken }, signal: AbortSignal.timeout(5000) });
    report.shutdownStatus = response.status;
  } catch { report.shutdownStatus = "unavailable"; }
  const result = await Promise.race([exited, sleep(10000).then(() => null)]);
  if (!result) { report.forcedShutdown = true; child.kill(); await exited; }
}
process.once("SIGINT", () => { void stop(); });
process.once("SIGTERM", () => { void stop(); });
console.log(JSON.stringify({ event: "validation_started", pid: child.pid, logPath, reportPath }));
try {
  const started = Date.now();
  let nextHealth = started + 5000;
  while (Date.now() - started < duration * 1000) {
    if (child.exitCode !== null || child.signalCode !== null || stopped) throw new Error("validation runtime exited early");
    if (Date.now() >= nextHealth) {
      let status;
      try {
        const response = await fetch(`${base}/readyz`, { signal: AbortSignal.timeout(3000) });
        const body = await response.json();
        status = { at: new Date().toISOString(), http: response.status, status: body.status };
      } catch { status = { at: new Date().toISOString(), status: "unreachable" }; }
      report.health.push(status);
      console.log(JSON.stringify({ event: "validation_health", elapsedSeconds: Math.floor((Date.now() - started) / 1000), ...status }));
      nextHealth = Date.now() + 60000;
    }
    await sleep(Math.min(5000, Math.max(1, duration * 1000 - (Date.now() - started))));
  }
  report.completed = true;
} finally {
  await stop();
  output.end();
  stderr.end();
  await new Promise((resolve) => log.end(resolve));
  report.finishedAt = new Date().toISOString();
  report.exit = await exited;
  await fsp.writeFile(reportPath, JSON.stringify(report, null, 2) + "\n");
  console.log(JSON.stringify({ event: "validation_finished", reportPath, ...report }));
}
