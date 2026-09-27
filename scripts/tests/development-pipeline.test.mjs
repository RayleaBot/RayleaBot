import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { createDevelopmentReloadQueue } from "../plugin-dev-workspace.mjs";
import { currentDevelopmentPlugins, prepareStableDevelopmentPlugins, runDevelopmentPluginBatch, startPreparedDevelopmentRuntime } from "../development-pipeline.mjs";

const plugins = ["a", "b"].map((id) => ({ id, path: path.resolve("fixture", id) }));
const source = (plugin) => path.join(plugin.path, "main.go");

for (const phase of ["build", "sync"]) {
  test(`${phase} failure leaves its peer processed and retains only the failed item for a later edit`, async () => {
    const queue = createDevelopmentReloadQueue(), calls = [];
    for (const plugin of plugins) queue.addPlugin(plugin, source(plugin));
    const batch = queue.take();
    const result = await runDevelopmentPluginBatch(batch.pluginChanges.map(({ plugin }) => plugin), async (plugin) => {
      calls.push(plugin.id);
      if (plugin.id === "a") throw new Error("fixture failure");
      return "applied";
    }, { onFailure: (plugin) => queue.deferPlugin(plugin, source(plugin)) });
    assert.deepEqual(calls, ["a", "b"]);
    assert.deepEqual(result.completed.map(({ plugin, value }) => [plugin.id, value]), [["b", "applied"]]);
    assert.equal(queue.hasChanges(), false, "a failed source must not spin in an automatic rebuild loop");
    assert.deepEqual(queue.deferred().pluginChanges.map(({ plugin }) => plugin.id), ["a"]);
    queue.addPlugin(plugins[0], source(plugins[0]));
    assert.deepEqual(queue.take().pluginChanges.map(({ plugin }) => plugin.id), ["a"]);
    assert.deepEqual(queue.deferred().pluginChanges, []);
  });
}

test("a newer edit to one plugin does not hold back an unchanged peer", async () => {
  const queue = createDevelopmentReloadQueue(), installed = [];
  const built = await runDevelopmentPluginBatch(plugins, async (plugin) => {
    if (plugin.id === "a") queue.addPlugin(plugin, source(plugin));
  });
  const ready = currentDevelopmentPlugins(built.completed.map(({ plugin }) => plugin), queue);
  await runDevelopmentPluginBatch(ready, async (plugin) => installed.push(plugin.id));
  assert.deepEqual(installed, ["b"]);
  assert.deepEqual(queue.take().pluginChanges.map(({ plugin }) => plugin.id), ["a"]);
});

test("deferred batch work survives workspace repair without replacing newer events", () => {
  const queue = createDevelopmentReloadQueue();
  queue.addServer("server/old.go");
  for (const plugin of plugins) queue.addPlugin(plugin, source(plugin));
  const batch = queue.take();
  const newer = path.join(plugins[1].path, "new.go");
  queue.addPlugin(plugins[1], newer);
  queue.deferBatch(batch);
  queue.addWorkspace("workspace.json");
  const retry = queue.take();
  assert.equal(retry.serverSourcePath, "server/old.go");
  assert.deepEqual(retry.pluginChanges.map(({ plugin, sourcePath }) => [plugin.id, sourcePath]).sort(), [["a", source(plugins[0])], ["b", newer]]);
});

test("shutdown interrupts a batch without admitting another plugin", async () => {
  const calls = [];
  await assert.rejects(runDevelopmentPluginBatch(plugins, async (plugin) => {
    calls.push(plugin.id);
    throw Object.assign(new Error("stopping"), { code: "DEV_SHUTDOWN" });
  }), { code: "DEV_SHUTDOWN" });
  assert.deepEqual(calls, ["a"]);
});

test("startup retries only superseded builds and keeps completed peers", async () => {
  const calls = [];
  const prepared = await prepareStableDevelopmentPlugins(async (ids) => {
    calls.push(ids);
    return ids === undefined ? { workspace: {}, plugins: [plugins[1]], failed: [{ plugin: plugins[0], error: { code: "DEV_INPUT_CHANGED" } }] }
      : { workspace: {}, plugins: [plugins[0]], failed: [] };
  });
  assert.deepEqual(calls, [undefined, ["a"]]);
  assert.deepEqual(prepared.plugins.map((plugin) => plugin.id).sort(), ["a", "b"]);
  assert.deepEqual(prepared.failed, []);
});

test("failed preflight never takes over a healthy environment", async () => {
  for (const failure of ["compile", "manifest", "core"]) {
    const events = [];
    await assert.rejects(startPreparedDevelopmentRuntime({
      prepare: async () => {
        events.push("prepare");
        if (failure === "core") throw new Error("core build failed");
        return { plugins: [plugins[1]], failed: failure === "compile" ? [{ plugin: plugins[0], error: new Error("compile failed") }] : [], workspace: { invalidPlugins: failure === "manifest" ? [{ path: "broken" }] : [] } };
      },
      existingIsHealthy: async () => true,
      acquire: async () => events.push("stop old environment"),
      install: async () => events.push("install"),
      start: async () => events.push("start"),
    }), failure === "core" ? /core build failed/ : { code: "DEV_PREFLIGHT_FAILED" });
    assert.deepEqual(events, ["prepare"]);
  }
});

test("cold startup keeps existing packages and starts healthy peers despite one build failure", async () => {
  const installed = new Map([["a", "last good"]]), events = [];
  await startPreparedDevelopmentRuntime({
    prepare: async () => { events.push("prepare"); return { plugins: [plugins[1]], failed: [{ plugin: plugins[0] }], workspace: {} }; },
    existingIsHealthy: async () => false,
    acquire: async () => events.push("acquire"),
    install: async (prepared) => runDevelopmentPluginBatch(prepared.plugins, async (plugin) => { installed.set(plugin.id, "new"); events.push("install " + plugin.id); }),
    start: async () => events.push("start"),
  });
  assert.deepEqual([...installed], [["a", "last good"], ["b", "new"]]);
  assert.deepEqual(events, ["prepare", "acquire", "install b", "start"]);
});

test("an isolated installation failure cannot prevent Server startup", async () => {
  let started = false;
  const result = await startPreparedDevelopmentRuntime({
    prepare: async () => ({ plugins, failed: [], workspace: {} }), existingIsHealthy: async () => false, acquire: async () => {},
    install: async (prepared) => runDevelopmentPluginBatch(prepared.plugins, async (plugin) => { if (plugin.id === "a") throw new Error("install failed"); }),
    start: async () => { started = true; },
  });
  assert.equal(started, true);
  assert.deepEqual(result.installed.completed.map(({ plugin }) => plugin.id), ["b"]);
});
