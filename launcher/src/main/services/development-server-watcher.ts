import type { DevelopmentServerWatcher } from "./launcher-coordinator.types";

const DEVELOPMENT_SERVER_WATCHER_PID_ENV = "RAYLEA_DEV_SERVER_WATCHER_PID";

type ProcessSignalTarget = {
  kill(processId: number, signal: 0): true;
};

export function consumeDevelopmentServerWatcherProcessId(
  environment: NodeJS.ProcessEnv = process.env,
) {
  const rawProcessId = environment[DEVELOPMENT_SERVER_WATCHER_PID_ENV]?.trim() ?? "";
  delete environment[DEVELOPMENT_SERVER_WATCHER_PID_ENV];
  if (!/^\d+$/.test(rawProcessId)) {
    return null;
  }

  const processId = Number(rawProcessId);
  return Number.isSafeInteger(processId) && processId > 0 ? processId : null;
}

function isProcessRunning(processId: number, processTarget: ProcessSignalTarget) {
  try {
    processTarget.kill(processId, 0);
    return true;
  } catch (error) {
    return (error as NodeJS.ErrnoException)?.code !== "ESRCH";
  }
}

export class NodeDevelopmentServerWatcher implements DevelopmentServerWatcher {
  constructor(
    readonly processId: number | null,
    private readonly processTarget: ProcessSignalTarget = process,
  ) {}

  isActive() {
    return this.processId !== null && isProcessRunning(this.processId, this.processTarget);
  }
}
