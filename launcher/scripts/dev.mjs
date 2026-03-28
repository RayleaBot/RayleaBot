import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import waitOn from "wait-on";

const root = path.resolve(import.meta.dirname, "..");
const children = [];

function run(command, args, extraEnv = {}) {
  const child = spawn(command, args, {
    cwd: root,
    env: { ...process.env, ...extraEnv },
    stdio: "inherit",
    shell: process.platform === "win32",
  });
  children.push(child);
  return child;
}

function shutdown(code = 0) {
  for (const child of children) {
    if (!child.killed) {
      child.kill();
    }
  }
  process.exit(code);
}

process.on("SIGINT", () => shutdown(0));
process.on("SIGTERM", () => shutdown(0));

run("pnpm", ["exec", "tsc", "-p", "tsconfig.main.json", "--watch", "--preserveWatchOutput"]);
run("pnpm", ["exec", "tsc", "-p", "tsconfig.preload.json", "--watch", "--preserveWatchOutput"]);
run("pnpm", ["exec", "vite", "--host", "127.0.0.1", "--port", "5174"]);

await waitOn({
  resources: [
    path.join(root, "dist", "main", "main", "index.js"),
    path.join(root, "dist", "preload", "preload", "index.js"),
    "tcp:127.0.0.1:5174",
  ],
  timeout: 120000,
});

const electron = run("pnpm", ["exec", "electron", "."], {
  RAYLEA_DEV_SERVER_URL: "http://127.0.0.1:5174",
});

electron.on("exit", (code) => shutdown(code ?? 0));
