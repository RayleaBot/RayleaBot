import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { createLauncherGoArgs } from "../../scripts/start-dev-support.mjs";
import { normalizeChildExitCode, terminateDevProcessTree } from "./dev-support.mjs";
import { createProcessInvocation } from "./process-invocation.mjs";
import { runWails, WAILS_GENERATE_BINDINGS_ARGS } from "./run-go.mjs";

const root = path.resolve(import.meta.dirname, "..");
const viteCli = path.join(root, "node_modules", "vite", "bin", "vite.js");
const children = new Set();
let shutdownPromise;

function run(command, args, options = {}) {
  const invocation = createProcessInvocation(command, args);
  const child = spawn(invocation.command, invocation.args, {
    cwd: root,
    env: { ...process.env, ...options.env },
    stdio: "inherit",
    shell: false,
    detached: process.platform !== "win32",
  });
  children.add(child);
  child.once("exit", () => children.delete(child));
  child.once("error", (error) => {
    children.delete(child);
    console.error(`Failed to start ${command}: ${error.message}`);
    void shutdown(1);
  });
  return child;
}

async function waitForVite(url, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url, {
        signal: AbortSignal.timeout(Math.min(1_000, Math.max(1, deadline - Date.now()))),
      });
      if (response.ok) return;
    } catch {
      // Vite is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 150));
  }
  throw new Error(`Vite did not become ready at ${url}`);
}

function shutdown(code = 0) {
  if (shutdownPromise) {
    return shutdownPromise;
  }
  shutdownPromise = (async () => {
    const results = await Promise.allSettled(
      [...children].map((child) => terminateDevProcessTree(child)),
    );
    for (const result of results) {
      if (result.status === "rejected") {
        console.error(`Failed to stop a development process tree: ${result.reason?.message ?? result.reason}`);
        code = 1;
      }
    }
    process.exit(code);
  })();
  return shutdownPromise;
}

process.on("SIGINT", () => void shutdown(0));
process.on("SIGTERM", () => void shutdown(0));

const generateExitCode = await runWails(WAILS_GENERATE_BINDINGS_ARGS);
if (generateExitCode !== 0) {
  throw new Error(`Wails binding generation exited with code ${generateExitCode}`);
}
const vite = run(process.execPath, [viteCli, "--host", "127.0.0.1", "--port", "5174", "--strictPort"]);
vite.once("exit", (code, signal) => void shutdown(normalizeChildExitCode(code, signal) || 1));
try {
  await waitForVite("http://127.0.0.1:5174/");
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  await shutdown(1);
}

const launcher = run("go", createLauncherGoArgs("run", ["."]), {
  env: { FRONTEND_DEVSERVER_URL: "http://127.0.0.1:5174", GOWORK: "off" },
});
launcher.once("exit", (code, signal) => void shutdown(normalizeChildExitCode(code, signal)));
