import { execFile, spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";
import { createBuildCache } from "../../scripts/dev-build-cache.mjs";
import { createLauncherGoArgs } from "../../scripts/start-dev-support.mjs";
import { createProcessInvocation } from "../../scripts/process-invocation.mjs";

const root = path.resolve(import.meta.dirname, "..");
const goModuleText = fs.readFileSync(path.join(root, "go.mod"), "utf8");
const wailsVersionMatch = goModuleText.match(/^\s*github\.com\/wailsapp\/wails\/v3\s+(v\S+)\s*$/m);
if (!wailsVersionMatch) {
  throw new Error("launcher/go.mod does not declare github.com/wailsapp/wails/v3");
}
export const wailsVersion = wailsVersionMatch[1];
export const wailsModuleQuery = `github.com/wailsapp/wails/v3@${wailsVersion}`;
const execute = promisify(execFile);

export function wailsGenerateBindingsArgs(platform = process.platform) {
  const args = [
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

export function wailsCLIBuildArgs(toolchain, executable) {
  const args = ["build", "-mod=readonly", "-buildvcs=false"];
  if (toolchain.GOHOSTOS === "linux") {
    args.push("-tags", "gtk3");
  }
  args.push("-o", executable, "./cmd/wails3");
  return args;
}

export async function runGo(args) {
  const invocation = createProcessInvocation("go", args);
  return runProcess(invocation.command, invocation.args);
}

export async function runWails(args) {
  const { command: go } = createProcessInvocation("go", []);
  const options = { cwd: root, env: { ...process.env, GOWORK: "off" } };
  const [moduleResult, toolchainResult] = await Promise.all([
    execute(go, ["mod", "download", "-json", wailsModuleQuery], options),
    execute(go, ["env", "-json", "GOROOT", "GOVERSION", "GOHOSTOS", "GOHOSTARCH", "CGO_ENABLED", "GOFLAGS"], options),
  ]);
  const module = JSON.parse(moduleResult.stdout);
  const toolchain = JSON.parse(toolchainResult.stdout);
  const executableSuffix = toolchain.GOHOSTOS === "windows" ? ".exe" : "";
  const compiler = path.join(toolchain.GOROOT, "bin", "go" + executableSuffix);
  const env = { ...options.env, GOOS: toolchain.GOHOSTOS, GOARCH: toolchain.GOHOSTARCH };
  const cacheDirectory = path.join(root, "..", ".tmp", "dev-cache", "wails-cli");
  const executable = path.join(cacheDirectory, "wails3" + executableSuffix);
  const buildArgs = wailsCLIBuildArgs(toolchain, executable);
  await fs.promises.mkdir(cacheDirectory, { recursive: true });
  await createBuildCache(cacheDirectory).run("wails-cli", {
    inputs: async () => [import.meta.filename, module.GoMod, path.join(module.Dir, "go.sum")],
    identity: { ...toolchain, module: wailsModuleQuery, sum: module.Sum, buildArgs },
    outputs: [executable],
    build: async () => {
      const code = await runProcess(compiler, buildArgs, {
        cwd: module.Dir, env,
      });
      if (code !== 0) throw new Error(`Wails CLI build exited with code ${code}`);
    },
  });
  return runProcess(executable, args, { env });
}

function runProcess(command, args, { cwd = root, env = { ...process.env, GOWORK: "off" } } = {}) {
  const child = spawn(command, args, {
    cwd,
    env,
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
  if (command === "generate:wails") {
    process.exitCode = await runWails(WAILS_GENERATE_BINDINGS_ARGS);
  } else {
    const args = command.endsWith(":platform")
      ? createLauncherGoArgs(command.slice(0, -":platform".length), process.argv.slice(3))
      : process.argv.slice(2);
    process.exitCode = await runGo(args);
  }
}
