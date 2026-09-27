import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { promisify } from "node:util";
import { loadPluginWorkspace, preparePluginGoWorkspace, renderDevelopmentGoWork, watchPluginWorkspace } from "../plugin-dev-workspace.mjs";
import { toolVersions } from "../tool-versions.mjs";

async function workspaceFixture(t, manifests) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), "raylea-plugin-isolation-"));
  t.after(() => fs.rm(root, { recursive: true, force: true }));
  const plugins = [];
  for (const [index, manifest] of manifests.entries()) {
    const directory = path.join(root, String(index));
    await fs.mkdir(directory);
    await fs.writeFile(path.join(directory, "info.json"), typeof manifest === "string" ? manifest : JSON.stringify(manifest));
    plugins.push({ path: directory });
  }
  const file = path.join(root, "workspace.json");
  await fs.writeFile(file, JSON.stringify({ workspace_version: "2", plugins }));
  return { root, file, plugins };
}

test("a broken manifest is reported independently and its repair remains observable", { timeout: 5000 }, async (t) => {
  const fixture = await workspaceFixture(t, [{ id: "healthy" }, "{"]);
  await assert.rejects(loadPluginWorkspace(fixture.file), /info.json/);
  const errors = [];
  const workspace = await loadPluginWorkspace(fixture.file, { onPluginError: (failure) => errors.push(failure) });
  assert.deepEqual(workspace.plugins.map((plugin) => plugin.id), ["healthy"]);
  assert.equal(errors.length, 1);
  assert.equal(workspace.invalidPlugins[0].path, fixture.plugins[1].path);
  const changes = [];
  const stop = await watchPluginWorkspace(workspace.invalidPlugins.map((entry) => ({ ...entry, id: entry.path })), (_plugin, file) => changes.push(file));
  t.after(stop);
  const repaired = path.join(fixture.plugins[1].path, "info.json");
  await fs.writeFile(repaired, JSON.stringify({ id: "repaired" }));
  const deadline = Date.now() + 2000;
  while (!changes.includes(repaired) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 20));
  assert.ok(changes.includes(repaired));
  const next = await loadPluginWorkspace(fixture.file, { onPluginError: () => assert.fail("repair stayed invalid") });
  assert.deepEqual(next.plugins.map((plugin) => plugin.id), ["healthy", "repaired"]);
  assert.deepEqual(next.invalidPlugins, []);
});

test("conflicting identities quarantine both sources while unrelated plugins remain usable", async (t) => {
  const fixture = await workspaceFixture(t, [{ id: "same" }, { id: "same" }, { id: "healthy" }]);
  await assert.rejects(loadPluginWorkspace(fixture.file), /duplicate plugin id/);
  const workspace = await loadPluginWorkspace(fixture.file, { onPluginError() {} });
  assert.deepEqual(workspace.plugins.map((plugin) => plugin.id), ["healthy"]);
  assert.deepEqual(workspace.invalidPlugins.map((plugin) => plugin.path), fixture.plugins.slice(0, 2).map((plugin) => plugin.path));
  await fs.writeFile(fixture.file, JSON.stringify({ workspace_version: "2", plugins: [], unsupported: true }));
  await assert.rejects(loadPluginWorkspace(fixture.file, { onPluginError() {} }), /unsupported top-level/);
});

test("one failed watch root does not remove healthy source listeners", { timeout: 5000 }, async (t) => {
  const fixture = await workspaceFixture(t, [{ id: "healthy" }]);
  const source = path.join(fixture.plugins[0].path, "main.go");
  await fs.writeFile(source, "package fixture\n");
  const badRoot = path.join(fixture.root, "not-a-directory");
  await fs.writeFile(badRoot, "fixture");
  const changes = [], errors = [];
  const stop = await watchPluginWorkspace([{ id: "healthy", path: fixture.plugins[0].path }, { id: "bad", path: badRoot }],
    (_plugin, file) => changes.push(file), (error) => errors.push(error), () => false, { isolateErrors: true });
  t.after(stop);
  await fs.writeFile(source, "package fixture\nvar Changed = true\n");
  const deadline = Date.now() + 2000;
  while (!changes.includes(source) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 20));
  assert.ok(changes.includes(source));
  assert.equal(errors.length, 1);
});

test("a malformed peer go.mod cannot poison an independent plugin's Go workspace", async (t) => {
  const fixture = await workspaceFixture(t, [{ id: "healthy" }, { id: "broken" }]);
  const sdk = path.join(fixture.root, "sdk");
  await fs.mkdir(sdk);
  await fs.writeFile(path.join(sdk, "go.mod"), `module fixture.local/sdk\n\ngo ${toolVersions.golang}\n`);
  await fs.writeFile(path.join(fixture.plugins[0].path, "go.mod"), `module fixture.local/healthy\n\ngo ${toolVersions.golang}\n`);
  await fs.writeFile(path.join(fixture.plugins[0].path, "main.go"), "package main\nfunc main() {}\n");
  await fs.writeFile(path.join(fixture.plugins[1].path, "go.mod"), "invalid module file\n");
  const workspace = await loadPluginWorkspace(fixture.file);
  const isolated = await preparePluginGoWorkspace({ directory: path.join(fixture.root, "workspaces"), sdkGoPath: sdk, plugin: workspace.plugins[0] });
  const execute = promisify(execFile);
  const list = (file) => execute("go", ["list", "."], { cwd: workspace.plugins[0].path, env: { ...process.env, GOWORK: file, CGO_ENABLED: "0" } });
  assert.equal((await list(isolated)).stdout.trim(), "fixture.local/healthy");
  const shared = path.join(fixture.root, "go.work");
  await fs.writeFile(shared, renderDevelopmentGoWork({ sdkGoPath: sdk, plugins: workspace.plugins }));
  await assert.rejects(list(shared), /go.mod/);
});
