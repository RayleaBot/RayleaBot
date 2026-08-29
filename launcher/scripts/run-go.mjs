import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { createLauncherGoArgs } from "../../scripts/start-dev-support.mjs";
import { createProcessInvocation } from "./process-invocation.mjs";

const root = path.resolve(import.meta.dirname, "..");
const goModuleText = fs.readFileSync(path.join(root, "go.mod"), "utf8");
const wailsVersionMatch = goModuleText.match(/^\s*github\.com\/wailsapp\/wails\/v3\s+(v\S+)\s*$/m);
if (!wailsVersionMatch) {
  throw new Error("launcher/go.mod does not declare github.com/wailsapp/wails/v3");
}
export const wailsVersion = wailsVersionMatch[1];

export function wailsGenerateBindingsArgs(platform = process.platform) {
  const args = [
    "run",
    `github.com/wailsapp/wails/v3/cmd/wails3@${wailsVersion}`,
    "generate",
    "bindings",
    "-clean=true",
  ];
  if (platform === "linux") {
    args.push("-f", "-tags gtk3");
  }
  args.push("-d", "src/renderer/bindings", "-names", "-ts", "-i");
  return args;
}

export const WAILS_GENERATE_BINDINGS_ARGS = wailsGenerateBindingsArgs();

export async function runGo(args) {
  const invocation = createProcessInvocation("go", args);
  const child = spawn(invocation.command, invocation.args, {
    cwd: root,
    env: { ...process.env, GOWORK: "off" },
    stdio: "inherit",
    shell: false,
  });
  return new Promise((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code) => resolve(code ?? 1));
  });
}

if (path.resolve(process.argv[1] ?? "") === import.meta.filename) {
  const command = process.argv[2] ?? "";
  const args = command === "generate:wails"
    ? WAILS_GENERATE_BINDINGS_ARGS
    : command.endsWith(":platform")
      ? createLauncherGoArgs(command.slice(0, -":platform".length), process.argv.slice(3))
      : process.argv.slice(2);
  process.exitCode = await runGo(args);
}
