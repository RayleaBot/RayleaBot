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

export function createApplicationExitManager(
  deps: ApplicationExitManagerDependencies,
): ApplicationExitManager {
  let quitAllowed = false;
  let exitPromise: Promise<void> | null = null;

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
              await deps.stopManagedProcess();
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
