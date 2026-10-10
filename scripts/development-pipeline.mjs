// A plugin failure is a result of that item, not a reason to abandon its peers.
export async function runDevelopmentPluginBatch(plugins, operation, { onFailure = () => {}, onProgress = () => {}, concurrency = 1 } = {}) {
  if (!Number.isSafeInteger(concurrency) || concurrency < 1) throw new Error("invalid plugin batch concurrency");
  const completed = [], failed = [];
  const active = new Map();
  let next = 0, finished = 0, fatal;
  const worker = async () => {
    while (!fatal && next < plugins.length) {
      const index = next++, plugin = plugins[index];
      active.set(index, plugin);
      onProgress(finished, plugins.length, plugin);
      try {
        const value = await operation(plugin);
        completed[index] = { plugin, value };
      } catch (error) {
        if (error?.code === "DEV_SHUTDOWN") { fatal = error; return; }
        failed[index] = { plugin, error };
        onFailure(plugin, error);
      }
      active.delete(index);
      onProgress(++finished, plugins.length, active.values().next().value);
    }
  };
  // Drain admitted work before propagating shutdown or a callback error.
  await Promise.all(Array.from({ length: Math.min(concurrency, plugins.length) }, () => worker().catch((error) => { fatal ??= error; })));
  if (fatal) throw fatal;
  return { completed: completed.filter(Boolean), failed: failed.filter(Boolean) };
}

export function currentDevelopmentPlugins(plugins, queue) {
  return queue.hasWorkspaceChanges() ? [] : plugins.filter((plugin) => !queue.hasPluginChanges(plugin.id));
}

export async function prepareStableDevelopmentPlugins(prepare, attempts = 3) {
  let result = await prepare();
  for (let attempt = 1; attempt < attempts; attempt++) {
    const retryIDs = result.failed.filter(({ error }) => error.code === "DEV_INPUT_CHANGED").map(({ plugin }) => plugin.id);
    if (!retryIDs.length) break;
    const retried = await prepare(retryIDs);
    const retrySet = new Set(retryIDs);
    result = { ...retried,
      plugins: [...result.plugins.filter((plugin) => !retrySet.has(plugin.id)), ...retried.plugins],
      failed: [...result.failed.filter(({ plugin }) => !retrySet.has(plugin.id)), ...retried.failed],
    };
  }
  return result;
}

// Preparation never takes ownership of the running environment. A failed
// preflight must not stop a healthy Server just to discover the same failure.
export async function startPreparedDevelopmentRuntime({ prepare, existingIsHealthy, acquire, install, start }) {
  const prepared = await prepare();
  if ((prepared.failed.length || prepared.workspace.invalidPlugins?.length) && await existingIsHealthy()) {
    throw Object.assign(new Error("开发插件预检未全部通过，原开发环境保持运行；修复后重新启动。"), { code: "DEV_PREFLIGHT_FAILED" });
  }
  await acquire();
  const installed = await install(prepared);
  await start();
  return { prepared, installed };
}
