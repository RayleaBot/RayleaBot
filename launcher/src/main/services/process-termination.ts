import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

type ExecFileLike = (
  file: string,
  args: string[],
) => Promise<{ stdout: string; stderr: string }>;

type ProcessKillLike = (pid: number, signal?: NodeJS.Signals | number) => void;

interface TerminateProcessIdOptions {
  platform?: NodeJS.Platform;
  execFileAsync?: ExecFileLike;
  kill?: ProcessKillLike;
}

function isMissingProcessError(error: unknown) {
  const code = (error as NodeJS.ErrnoException | undefined)?.code;
  if (code === "ESRCH") {
    return true;
  }

  const message = error instanceof Error ? error.message : String(error ?? "");
  return /not found|no running instance|does not exist/i.test(message);
}

export async function terminateProcessId(
  pid: number,
  options: TerminateProcessIdOptions = {},
) {
  const platform = options.platform ?? process.platform;

  if (platform === "win32") {
    try {
      await (options.execFileAsync ?? execFileAsync)("taskkill", ["/PID", String(pid), "/T", "/F"]);
      return true;
    } catch (error) {
      return isMissingProcessError(error);
    }
  }

  try {
    (options.kill ?? process.kill)(pid, "SIGTERM");
    return true;
  } catch (error) {
    return isMissingProcessError(error);
  }
}
