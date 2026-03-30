export interface ApplicationExitManager {
  requestExit(): Promise<void>;
  shouldAllowQuit(): boolean;
}

interface ApplicationExitManagerDependencies {
  isManagedProcessRunning(): boolean;
  stopManagedProcess(): Promise<void>;
  forceKillManagedProcess(): Promise<void>;
  quitApplication(): void;
}

interface ApplicationExitManagerOptions {
  stopTimeoutMs?: number;
}

const DEFAULT_STOP_TIMEOUT_MS = 5000;

function withTimeout<T>(operation: Promise<T>, timeoutMs: number) {
  return new Promise<T>((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error("coordinated shutdown timed out"));
    }, timeoutMs);

    operation.then(
      (value) => {
        clearTimeout(timer);
        resolve(value);
      },
      (error) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}

export function createApplicationExitManager(
  deps: ApplicationExitManagerDependencies,
  options: ApplicationExitManagerOptions = {},
): ApplicationExitManager {
  let quitAllowed = false;
  let exitPromise: Promise<void> | null = null;
  const stopTimeoutMs = options.stopTimeoutMs ?? DEFAULT_STOP_TIMEOUT_MS;

  return {
    shouldAllowQuit() {
      return quitAllowed;
    },
    async requestExit() {
      if (exitPromise) {
        return exitPromise;
      }

      exitPromise = (async () => {
        try {
          if (deps.isManagedProcessRunning()) {
            try {
              await withTimeout(deps.stopManagedProcess(), stopTimeoutMs);
            } catch {
              // Fall back to direct process termination when coordinated shutdown fails.
            }

            if (deps.isManagedProcessRunning()) {
              await deps.forceKillManagedProcess();
            }
          }
        } finally {
          quitAllowed = true;
          deps.quitApplication();
        }
      })();

      return exitPromise;
    },
  };
}
