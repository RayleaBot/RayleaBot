import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";
import { runWails, WAILS_GENERATE_BINDINGS_ARGS } from "./run-go.mjs";

const root = path.resolve(import.meta.dirname, "..");

async function run(args) {
  const child = spawn(process.execPath, args, {
    cwd: root,
    env: process.env,
    stdio: "inherit",
    shell: false,
  });
  const code = await new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (exitCode) => resolve(exitCode ?? 1));
  });
  if (code !== 0) {
    throw new Error(`node ${args.join(" ")} exited with code ${code}`);
  }
}

const generateExitCode = await runWails(WAILS_GENERATE_BINDINGS_ARGS);
if (generateExitCode !== 0) {
  throw new Error(`Wails binding generation exited with code ${generateExitCode}`);
}
await run([path.join("node_modules", "vite", "bin", "vite.js"), "build"]);
